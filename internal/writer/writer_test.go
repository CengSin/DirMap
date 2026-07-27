package writer

import (
	"testing"
	"time"

	"github.com/cengsin/system-agent-rag/internal/config"
	"github.com/cengsin/system-agent-rag/internal/model"
)

func TestWriteDescriptionsSkipsEquivalentSnapshot(t *testing.T) {
	w := New(&config.Config{OutputDir: t.TempDir()})
	dirs := []model.FileInfo{{
		Path:        "/watched/dir",
		Name:        "dir",
		ModTime:     time.Unix(1, 0),
		Description: "a directory",
	}}

	changed, err := w.WriteDescriptions("/watched", dirs)
	if err != nil || !changed {
		t.Fatalf("first write = (%v, %v), want (true, nil)", changed, err)
	}
	changed, err = w.WriteDescriptions("/watched", dirs)
	if err != nil || changed {
		t.Fatalf("equivalent write = (%v, %v), want (false, nil)", changed, err)
	}
}
