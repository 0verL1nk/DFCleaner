package scanner

import "time"

type FileEntry struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	ModTime    time.Time `json:"modTime"`
	AccessTime time.Time `json:"accessTime"`
	IsDir      bool      `json:"isDir"`
	Extension  string    `json:"extension"`
	Error      string    `json:"error,omitempty"`
}

type ScanOptions struct {
	MaxDepth    int      `json:"maxDepth"`
	ExcludeDirs []string `json:"excludeDirs"`
}

type ScanProgress struct {
	FilesScanned int    `json:"filesScanned"`
	DirsScanned  int    `json:"dirsScanned"`
	CurrentPath  string `json:"currentPath"`
}

type ScanResult struct {
	Root       string       `json:"root"`
	TotalFiles int          `json:"totalFiles"`
	TotalDirs  int          `json:"totalDirs"`
	TotalSize  int64        `json:"totalSize"`
	DurationMs int64        `json:"durationMs"`
	Entries    []FileEntry  `json:"entries"`
	Errors     []FileEntry  `json:"errors"`
}

type ScanChunk struct {
	Entries []FileEntry `json:"entries"`
	Progress ScanProgress `json:"progress"`
}
