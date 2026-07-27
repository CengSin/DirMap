package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cengsin/system-agent-rag/internal/model"
)

func TestSaveIfChangedUsesStableOrdering(t *testing.T) {
	store := New(t.TempDir())
	files := []model.FileInfo{
		{Path: "/watched/b", Name: "b", ModTime: time.Unix(2, 0), Description: "B"},
		{Path: "/watched/a", Name: "a", ModTime: time.Unix(1, 0), Description: "A"},
	}

	changed, err := store.SaveIfChanged("/watched", files)
	if err != nil || !changed {
		t.Fatalf("first save = (%v, %v), want (true, nil)", changed, err)
	}

	changed, err = store.SaveIfChanged("/watched", []model.FileInfo{files[1], files[0]})
	if err != nil || changed {
		t.Fatalf("equivalent save = (%v, %v), want (false, nil)", changed, err)
	}

	data, err := os.ReadFile(filepath.Join(store.dir, "watched.cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" || string(data)[1:] == "" {
		t.Fatal("cache file should contain the saved entries")
	}
}
