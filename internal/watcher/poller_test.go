package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cengsin/system-agent-rag/internal/config"
)

func TestPollDetectsNestedDirectoryWithoutRootMtimeChange(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}

	p := NewPollingWatcher(&config.Config{WatchPaths: []string{root}})
	p.snapshots[root] = p.walkWatchPath(root)

	child := filepath.Join(parent, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	p.poll()

	select {
	case got := <-p.Events:
		if got != child {
			t.Fatalf("event path = %q, want %q", got, child)
		}
	case <-time.After(time.Second):
		t.Fatal("poll did not report a nested directory")
	}
}
