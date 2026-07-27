package writer

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/cengsin/system-agent-rag/internal/config"
	"github.com/cengsin/system-agent-rag/internal/model"
)

type Writer struct {
	outputDir string
}

func New(cfg *config.Config) *Writer {
	return &Writer{outputDir: cfg.OutputDir}
}

// WriteDescriptions writes only when the rendered table changes. It returns
// whether a write was performed.
func (w *Writer) WriteDescriptions(watchPath string, dirs []model.FileInfo) (bool, error) {
	filename := sanitizePath(watchPath) + ".md"
	outPath := filepath.Join(w.outputDir, filename)
	slices.SortFunc(dirs, func(a, b model.FileInfo) int { return strings.Compare(a.Path, b.Path) })

	var b strings.Builder
	fmt.Fprintf(&b, "# Directory Descriptions: %s\n", watchPath)
	// Use the latest source modification time rather than wall-clock time so
	// an unchanged snapshot produces identical output and needs no rewrite.
	latest := time.Time{}
	for _, d := range dirs {
		if d.ModTime.After(latest) {
			latest = d.ModTime
		}
	}
	fmt.Fprintf(&b, "Generated: %s\n\n", latest.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "| Path | Modified | Description |\n")
	fmt.Fprintf(&b, "|------|----------|-------------|\n")

	for _, d := range dirs {
		relPath := d.Path
		if strings.HasPrefix(relPath, watchPath) {
			relPath = strings.TrimPrefix(relPath, watchPath)
			relPath = strings.TrimPrefix(relPath, string(os.PathSeparator))
		}
		if relPath == "" {
			relPath = "."
		}
		relPath += "/"

		desc := d.Description
		if desc == "" {
			desc = "-"
		}

		fmt.Fprintf(&b, "| %s | %s | %s |\n",
			relPath, d.ModTime.Format("2006-01-02"), desc)
	}

	tmpPath := outPath + ".tmp"
	data := []byte(b.String())
	if existing, err := os.ReadFile(outPath); err == nil && bytes.Equal(existing, data) {
		return false, nil
	} else if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read existing: %w", err)
	}

	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return false, fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmpPath, outPath); err != nil {
		os.Remove(tmpPath)
		return false, fmt.Errorf("rename: %w", err)
	}

	return true, nil
}

func sanitizePath(path string) string {
	// Replace path separators and special chars with dashes
	s := strings.ReplaceAll(path, string(os.PathSeparator), "-")
	s = strings.ReplaceAll(s, ":", "-")
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.Trim(s, "-.")
	return s
}
