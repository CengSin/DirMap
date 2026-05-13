package writer

import (
	"fmt"
	"os"
	"path/filepath"
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

func (w *Writer) WriteDescriptions(watchPath string, dirs []model.FileInfo) error {
	filename := sanitizePath(watchPath) + ".md"
	outPath := filepath.Join(w.outputDir, filename)

	var b strings.Builder
	fmt.Fprintf(&b, "# Directory Descriptions: %s\n", watchPath)
	fmt.Fprintf(&b, "Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
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
	if err := os.WriteFile(tmpPath, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmpPath, outPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}

func sanitizePath(path string) string {
	// Replace path separators and special chars with dashes
	s := strings.ReplaceAll(path, string(os.PathSeparator), "-")
	s = strings.ReplaceAll(s, ":", "-")
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.Trim(s, "-.")
	return s
}

