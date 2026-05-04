package cleaner

import "dfcleaner/internal/store"

type CleanupItem struct {
	Path      string `json:"path"`
	Operation string `json:"operation"` // "trash", "delete", "move"
	Target    string `json:"target,omitempty"`
}

type CleanupResult struct {
	Path      string `json:"path"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	FreedBytes int64 `json:"freedBytes"`
}

type Cleaner struct {
	store *store.Store
}

func New(s *store.Store) *Cleaner {
	return &Cleaner{store: s}
}
