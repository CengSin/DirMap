package model

import "time"

type FileInfo struct {
	Path           string
	Name           string
	IsDir          bool
	Size           int64
	ModTime        time.Time
	Extension      string
	ContentPreview string
	Description    string
}
