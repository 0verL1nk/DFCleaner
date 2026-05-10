package store

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := InitDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	return New(db)
}

// --- Cleanup Logs ---

func TestCleanupLog(t *testing.T) {
	s := newTestStore(t)

	log := &CleanupLog{
		FilePath:   "/tmp/cache/test.log",
		Operation:  "trash",
		Result:     "success",
		FreedBytes: 1024,
	}
	if err := s.LogCleanup(log); err != nil {
		t.Fatalf("LogCleanup: %v", err)
	}

	recent := s.GetRecentCleanups(5)
	if len(recent) != 1 {
		t.Fatalf("expected 1 log, got %d", len(recent))
	}
	if recent[0].FilePath != "/tmp/cache/test.log" {
		t.Errorf("FilePath = %q", recent[0].FilePath)
	}
	if recent[0].FreedBytes != 1024 {
		t.Errorf("FreedBytes = %d, want 1024", recent[0].FreedBytes)
	}
}

func TestGetRecentCleanupsLimit(t *testing.T) {
	s := newTestStore(t)

	for i := 0; i < 10; i++ {
		s.LogCleanup(&CleanupLog{
			FilePath:  "/tmp/file",
			Operation: "delete",
			Result:    "success",
		})
	}

	recent := s.GetRecentCleanups(3)
	if len(recent) != 3 {
		t.Errorf("expected 3, got %d", len(recent))
	}
}
