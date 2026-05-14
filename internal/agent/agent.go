package agent

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/cengsin/system-agent-rag/internal/cache"
	"github.com/cengsin/system-agent-rag/internal/config"
	"github.com/cengsin/system-agent-rag/internal/model"
	"github.com/cengsin/system-agent-rag/internal/scanner"
	"github.com/cengsin/system-agent-rag/internal/summarizer"
	"github.com/cengsin/system-agent-rag/internal/watcher"
	"github.com/cengsin/system-agent-rag/internal/writer"
)

type Agent struct {
	cfg        *config.Config
	closer     io.Closer
	debouncer  *watcher.Debouncer
	summarizer *summarizer.Summarizer
	writer     *writer.Writer
	cacheStore *cache.Store
	cache      map[string]map[string]model.FileInfo
	mu         sync.Mutex
}

func New(cfg *config.Config) (*Agent, error) {
	var eventCh <-chan string
	var closer io.Closer

	if cfg.Polling.Enabled {
		p := watcher.NewPollingWatcher(cfg)
		go p.Run()
		eventCh = p.Events
		closer = p
		log.Println("agent: using polling mode")
	} else {
		w, err := watcher.New(cfg)
		if err != nil {
			return nil, err
		}
		eventCh = w.Events
		closer = w
		log.Println("agent: using fsnotify mode")
	}

	deb := watcher.NewDebouncer(cfg.Debounce.Interval, cfg.Debounce.MaxWait, eventCh)

	return &Agent{
		cfg:        cfg,
		closer:     closer,
		debouncer:  deb,
		summarizer: summarizer.New(cfg),
		writer:     writer.New(cfg),
		cacheStore: cache.New(cfg.OutputDir),
		cache:      make(map[string]map[string]model.FileInfo),
	}, nil
}

func (a *Agent) Run(ctx context.Context) error {
	log.Println("agent: starting...")

	if a.cfg.InitialScan {
		log.Println("agent: running initial scan...")
		if err := a.initialScan(ctx); err != nil {
			log.Printf("agent: initial scan error: %v", err)
		}
	}

	log.Println("agent: watching for changes...")

	for {
		select {
		case <-ctx.Done():
			log.Println("agent: shutting down...")
			a.debouncer.Stop()
			return a.closer.Close()

		case paths, ok := <-a.debouncer.OutCh:
			if !ok {
				return nil
			}
			a.handleChanges(ctx, paths)
		}
	}
}

func (a *Agent) initialScan(ctx context.Context) error {
	results, err := scanner.Scan(a.cfg)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	for watchPath, scanned := range results {
		wg.Add(1)
		go func(wp string, sc []model.FileInfo) {
			defer wg.Done()

			// Load persisted cache
			saved, err := a.cacheStore.Load(wp)
			if err != nil {
				log.Printf("agent: cache load error for %s: %v", wp, err)
			}

			// Diff: only call LLM for new/changed directories
			newDirs, cachedDirs := cache.Diff(sc, saved)
			log.Printf("agent: %s — %d new, %d cached", wp, len(newDirs), len(cachedDirs))

			var allDirs []model.FileInfo
			allDirs = append(allDirs, cachedDirs...)

			if len(newDirs) > 0 {
				summarized, err := a.summarizer.SummarizeBatch(ctx, newDirs)
				if err != nil {
					log.Printf("agent: summarize error for %s: %v", wp, err)
					summarized = newDirs
				}
				allDirs = append(allDirs, summarized...)
			}

			// Update in-memory cache
			dirCache := make(map[string]model.FileInfo, len(allDirs))
			for _, f := range allDirs {
				dirCache[f.Path] = f
			}
			a.mu.Lock()
			a.cache[wp] = dirCache
			a.mu.Unlock()

			// Persist cache
			if err := a.cacheStore.Save(wp, allDirs); err != nil {
				log.Printf("agent: cache save error for %s: %v", wp, err)
			}

			// Write descriptions
			if err := a.writer.WriteDescriptions(wp, allDirs); err != nil {
				log.Printf("agent: write error for %s: %v", wp, err)
			} else {
				log.Printf("agent: wrote descriptions for %s (%d items)", wp, len(allDirs))
			}
		}(watchPath, scanned)
	}

	wg.Wait()
	return nil
}

func (a *Agent) handleChanges(ctx context.Context, paths []string) {
	log.Printf("agent: processing %d changed paths", len(paths))

	grouped := make(map[string][]model.FileInfo)

	for _, path := range paths {
		watchPath := a.findWatchPath(path)
		if watchPath == "" {
			continue
		}

		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				a.mu.Lock()
				delete(a.cache[watchPath], path)
				a.mu.Unlock()
				grouped[watchPath] = nil
			}
			continue
		}

		// Only handle directories
		if !info.IsDir() {
			continue
		}

		fi := model.FileInfo{
			Path:    path,
			Name:    info.Name(),
			IsDir:   true,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}

		grouped[watchPath] = append(grouped[watchPath], fi)
	}

	var wg sync.WaitGroup

	for watchPath, changedDirs := range grouped {
		wg.Add(1)
		go func(wp string, dirs []model.FileInfo) {
			defer wg.Done()

			// Summarize new/changed directories
			if len(dirs) > 0 {
				summarized, err := a.summarizer.SummarizeBatch(ctx, dirs)
				if err != nil {
					log.Printf("agent: summarize error for %s: %v", wp, err)
					summarized = dirs
				}

				a.mu.Lock()
				if a.cache[wp] == nil {
					a.cache[wp] = make(map[string]model.FileInfo)
				}
				for _, f := range summarized {
					a.cache[wp][f.Path] = f
				}
				a.mu.Unlock()
			}

			// Build snapshot of all dirs under lock
			a.mu.Lock()
			allDirs := make([]model.FileInfo, 0, len(a.cache[wp]))
			for _, f := range a.cache[wp] {
				allDirs = append(allDirs, f)
			}
			a.mu.Unlock()

			if err := a.cacheStore.Save(wp, allDirs); err != nil {
				log.Printf("agent: cache save error for %s: %v", wp, err)
			}

			if err := a.writer.WriteDescriptions(wp, allDirs); err != nil {
				log.Printf("agent: write error for %s: %v", wp, err)
			} else {
				log.Printf("agent: updated descriptions for %s", wp)
			}
		}(watchPath, changedDirs)
	}

	wg.Wait()
}

func (a *Agent) findWatchPath(path string) string {
	for _, wp := range a.cfg.WatchPaths {
		if path == wp || len(path) > len(wp) && path[:len(wp)] == wp && path[len(wp)] == filepath.Separator {
			return wp
		}
	}
	return ""
}

