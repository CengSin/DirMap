package scanner

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cengsin/system-agent-rag/internal/config"
	"github.com/cengsin/system-agent-rag/internal/model"
)

func Scan(cfg *config.Config) (map[string][]model.FileInfo, error) {
	result := make(map[string][]model.FileInfo)

	for _, watchPath := range cfg.WatchPaths {
		var files []model.FileInfo

		err := filepath.WalkDir(watchPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			name := d.Name()
			if shouldIgnore(path, name, cfg.IgnorePatterns) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// Only collect directories
			if !d.IsDir() {
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return nil
			}

			files = append(files, model.FileInfo{
				Path:    path,
				Name:    name,
				IsDir:   true,
				Size:    info.Size(),
				ModTime: info.ModTime(),
			})

			return nil
		})

		if err != nil {
			return nil, err
		}

		result[watchPath] = files
	}

	return result, nil
}

func shouldIgnore(path, name string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
	}
	// Also ignore hidden files/dirs
	if strings.HasPrefix(name, ".") && name != "." {
		return true
	}
	return false
}

