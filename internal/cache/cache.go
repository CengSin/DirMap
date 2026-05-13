package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/cengsin/system-agent-rag/internal/model"
)

type Entry struct {
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	ModTime     time.Time `json:"mod_time"`
	Description string    `json:"description"`
}

type Store struct {
	dir string
}

func New(outputDir string) *Store {
	return &Store{dir: outputDir}
}

func (s *Store) Load(watchPath string) (map[string]Entry, error) {
	p := s.filePath(watchPath)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]Entry), nil
		}
		return nil, err
	}

	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return make(map[string]Entry), nil
	}

	m := make(map[string]Entry, len(entries))
	for _, e := range entries {
		m[e.Path] = e
	}
	return m, nil
}

func (s *Store) Save(watchPath string, files []model.FileInfo) error {
	entries := make([]Entry, 0, len(files))
	for _, f := range files {
		entries = append(entries, Entry{
			Path:        f.Path,
			Name:        f.Name,
			ModTime:     f.ModTime,
			Description: f.Description,
		})
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	p := s.filePath(watchPath)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (s *Store) filePath(watchPath string) string {
	name := sanitizePath(watchPath) + ".cache.json"
	return filepath.Join(s.dir, name)
}

func sanitizePath(path string) string {
	s := filepath.ToSlash(path)
	result := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '/' || r == ':' || r == ' ' {
			result = append(result, '-')
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

// Diff compares scanned directories with cached entries.
// Returns: newDirs (need LLM), cachedDirs (reuse description).
func Diff(scanned []model.FileInfo, cached map[string]Entry) (newDirs []model.FileInfo, cachedDirs []model.FileInfo) {
	for _, f := range scanned {
		if e, ok := cached[f.Path]; ok && e.ModTime.Equal(f.ModTime) && e.Description != "" {
			f.Description = e.Description
			cachedDirs = append(cachedDirs, f)
		} else {
			newDirs = append(newDirs, f)
		}
	}
	return
}
