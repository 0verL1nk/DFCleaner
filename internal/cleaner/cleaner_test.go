package cleaner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsDangerousPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/", true},
		{"/home", false},
		{"/home/user", false},
		{"/home/user/downloads", false},
		{"/tmp", false},
		{"/usr", false},
		{"/usr/local", false},
	}
	for _, tt := range tests {
		got := isDangerousPath(tt.path)
		if got != tt.want {
			t.Errorf("isDangerousPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestGetPathSize(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	os.WriteFile(file, []byte("hello world"), 0644)

	size := getPathSize(file)
	if size != 11 {
		t.Errorf("getPathSize(file) = %d, want 11", size)
	}

	dirSize := getPathSize(dir)
	if dirSize < 11 {
		t.Errorf("getPathSize(dir) = %d, want >= 11", dirSize)
	}
}

func TestPermanentDelete(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "deleteme.txt")
	os.WriteFile(file, []byte("bye"), 0644)

	if err := permanentDelete(file); err != nil {
		t.Fatalf("permanentDelete: %v", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Error("file should be deleted")
	}
}

func TestMoveToDir(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	file := filepath.Join(src, "move.txt")
	os.WriteFile(file, []byte("content"), 0644)

	if err := moveToDir(file, dst); err != nil {
		t.Fatalf("moveToDir: %v", err)
	}

	moved := filepath.Join(dst, "move.txt")
	if _, err := os.Stat(moved); os.IsNotExist(err) {
		t.Error("file should exist in destination")
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Error("source file should be gone")
	}
}

func TestCleanupUnknownOperation(t *testing.T) {
	c := New(nil)
	results, err := c.Cleanup([]CleanupItem{{Path: "/tmp/test", Operation: "unknown"}})
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Success {
		t.Error("should fail for unknown operation")
	}
}

func TestCleanupRelativePath(t *testing.T) {
	c := New(nil)
	results, err := c.Cleanup([]CleanupItem{{Path: "relative/path", Operation: OpDelete}})
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if results[0].Success {
		t.Error("should fail for relative path")
	}
}
