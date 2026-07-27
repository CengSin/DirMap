package watcher

import (
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/cengsin/system-agent-rag/internal/config"
	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	fs        *fsnotify.Watcher
	Events    chan string
	cfg       *config.Config
	done      chan struct{}
	closeOnce sync.Once
}

func New(cfg *config.Config) (*Watcher, error) {
	fs, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		fs:     fs,
		Events: make(chan string, 100),
		cfg:    cfg,
		done:   make(chan struct{}),
	}

	for _, p := range cfg.WatchPaths {
		if err := w.addRecursive(p); err != nil {
			fs.Close()
			return nil, err
		}
	}

	go w.loop()

	return w, nil
}

func (w *Watcher) addRecursive(path string) error {
	return filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if w.shouldIgnore(p, d.Name()) {
				return filepath.SkipDir
			}
			return w.fs.Add(p)
		}
		return nil
	})
}

func (w *Watcher) shouldIgnore(path, name string) bool {
	for _, pattern := range w.cfg.IgnorePatterns {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}
	return false
}

func (w *Watcher) loop() {
	defer close(w.done)
	defer close(w.Events)
	for {
		select {
		case event, ok := <-w.fs.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove) == 0 {
				continue
			}

			name := filepath.Base(event.Name)
			if w.shouldIgnore(event.Name, name) {
				continue
			}

			// New directory: add to watcher and notify
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if err := w.addRecursive(event.Name); err != nil {
						log.Printf("watch: failed to add %s: %v", event.Name, err)
					}
					w.sendEvent(event.Name)
				}
				continue
			}

			// Remove: send directly (os.Stat will fail on deleted paths)
			if event.Op&fsnotify.Remove != 0 {
				w.sendEvent(event.Name)
				continue
			}

			// Write on a directory: notify
			if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
				w.sendEvent(event.Name)
			}

		case err, ok := <-w.fs.Errors:
			if !ok {
				return
			}
			log.Printf("watch error: %v", err)
		}
	}
}

func (w *Watcher) sendEvent(path string) {
	select {
	case w.Events <- path:
	default:
		log.Printf("watch: event channel full, dropping %s", path)
	}
}

func (w *Watcher) Close() error {
	var err error
	w.closeOnce.Do(func() {
		err = w.fs.Close()
		<-w.done
	})
	return err
}
