package watcher

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cengsin/system-agent-rag/internal/config"
)

// PollingWatcher detects directory changes by periodically walking the filesystem.
// Use this when fsnotify doesn't work reliably (e.g., Docker on macOS).
type PollingWatcher struct {
	Events    chan string
	cfg       *config.Config
	interval  time.Duration
	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
	snapshots map[string]map[string]bool
}

func NewPollingWatcher(cfg *config.Config) *PollingWatcher {
	return &PollingWatcher{
		Events:    make(chan string, 100),
		cfg:       cfg,
		interval:  cfg.Polling.Interval,
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
		snapshots: make(map[string]map[string]bool, len(cfg.WatchPaths)),
	}
}

func (p *PollingWatcher) Run() {
	defer close(p.done)
	defer close(p.Events)
	log.Printf("poller: started with interval %v", p.interval)

	// Initial snapshot per watch path.
	for _, watchPath := range p.cfg.WatchPaths {
		p.snapshots[watchPath] = p.walkWatchPath(watchPath)
	}

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stop:
			return
		case <-ticker.C:
			p.poll()
		}
	}
}

func (p *PollingWatcher) Close() error {
	p.closeOnce.Do(func() {
		close(p.stop)
		<-p.done
	})
	return nil
}

// poll compares directory snapshots. Checking only the root directory's mtime
// misses changes under nested directories on common filesystems.
func (p *PollingWatcher) poll() {
	for _, watchPath := range p.cfg.WatchPaths {
		old := p.snapshots[watchPath]
		current := p.walkWatchPath(watchPath)
		p.snapshots[watchPath] = current

		// Diff and send events for this watch path only.
		for path := range current {
			if !old[path] {
				p.sendEvent(path)
			}
		}
		for path := range old {
			if !current[path] {
				p.sendEvent(path)
			}
		}
	}
}

func (p *PollingWatcher) walkWatchPath(watchPath string) map[string]bool {
	dirs := make(map[string]bool)
	filepath.WalkDir(watchPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if shouldIgnorePoll(path, name, p.cfg.IgnorePatterns) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			dirs[path] = true
		}
		return nil
	})
	return dirs
}

func (p *PollingWatcher) sendEvent(path string) {
	select {
	case p.Events <- path:
	default:
		log.Printf("poller: event channel full, dropping %s", path)
	}
}

func shouldIgnorePoll(path, name string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
	}
	if strings.HasPrefix(name, ".") && name != "." {
		return true
	}
	return false
}
