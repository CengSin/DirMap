package watcher

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cengsin/system-agent-rag/internal/config"
)

// PollingWatcher detects directory changes by periodically walking the filesystem.
// Use this when fsnotify doesn't work reliably (e.g., Docker on macOS).
type PollingWatcher struct {
	Events   chan string
	cfg      *config.Config
	interval time.Duration
	stop     chan struct{}
}

func NewPollingWatcher(cfg *config.Config) *PollingWatcher {
	return &PollingWatcher{
		Events:   make(chan string, 100),
		cfg:      cfg,
		interval: cfg.Polling.Interval,
		stop:     make(chan struct{}),
	}
}

func (p *PollingWatcher) Run() {
	log.Printf("poller: started with interval %v", p.interval)

	snapshot := p.takeSnapshot()

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stop:
			return
		case <-ticker.C:
			current := p.takeSnapshot()
			p.diffAndSend(snapshot, current)
			snapshot = current
		}
	}
}

func (p *PollingWatcher) Close() error {
	close(p.stop)
	close(p.Events)
	return nil
}

func (p *PollingWatcher) takeSnapshot() map[string]bool {
	dirs := make(map[string]bool)
	for _, watchPath := range p.cfg.WatchPaths {
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
	}
	return dirs
}

func (p *PollingWatcher) diffAndSend(old, current map[string]bool) {
	// New directories
	for path := range current {
		if !old[path] {
			p.sendEvent(path)
		}
	}
	// Deleted directories
	for path := range old {
		if !current[path] {
			p.sendEvent(path)
		}
	}
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
