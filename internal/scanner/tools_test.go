package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// --- Tracer bullet: scan_directory tool ---

func TestToolScanDirectory(t *testing.T) {
	// Setup: create a temp directory with known structure
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, "big.log"), []byte(makeString(1000)), 0644)
	os.WriteFile(filepath.Join(dir, "small.txt"), []byte("hi"), 0644)
	os.WriteFile(filepath.Join(dir, "subdir", "nested.json"), []byte(`{"a":1}`), 0644)

	s := New(context.Background())
	out, err := s.toolScanDirectory(context.Background(), &ScanDirInput{Path: dir})
	if err != nil {
		t.Fatalf("toolScanDirectory: %v", err)
	}

	// Verify observable behavior
	if out.Path != dir {
		t.Errorf("Path = %q, want %q", out.Path, dir)
	}
	if out.SubDirs != 1 {
		t.Errorf("SubDirs = %d, want 1", out.SubDirs)
	}
	if out.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2 (top-level files only)", out.FileCount)
	}
	if out.TotalSize != int64(1000+2) {
		t.Errorf("TotalSize = %d, want %d", out.TotalSize, 1002)
	}
}

func TestToolScanDirectoryNonexistent(t *testing.T) {
	s := New(context.Background())
	_, err := s.toolScanDirectory(context.Background(), &ScanDirInput{Path: "/nonexistent/path/xyz"})
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

// --- get_file_info tool ---

func TestToolGetFileInfo(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	os.WriteFile(file, []byte("hello"), 0644)

	s := New(context.Background())
	entry, err := s.toolGetFileInfo(context.Background(), &GetFileInfoInput{Path: file})
	if err != nil {
		t.Fatalf("toolGetFileInfo: %v", err)
	}

	if entry.Name != "test.txt" {
		t.Errorf("Name = %q, want 'test.txt'", entry.Name)
	}
	if entry.Size != 5 {
		t.Errorf("Size = %d, want 5", entry.Size)
	}
	if entry.IsDir {
		t.Error("IsDir should be false")
	}
	if entry.Extension != "txt" {
		t.Errorf("Extension = %q, want 'txt'", entry.Extension)
	}
}

// --- get_large_files tool ---

func TestToolGetLargeFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "big.bin"), []byte(makeString(5000)), 0644)
	os.WriteFile(filepath.Join(dir, "medium.dat"), []byte(makeString(2000)), 0644)
	os.WriteFile(filepath.Join(dir, "small.txt"), []byte("hi"), 0644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "sub", "huge.bin"), []byte(makeString(10000)), 0644)

	s := New(context.Background())
	out, err := s.toolGetLargeFiles(context.Background(), &GetLargeFilesInput{
		Path:      dir,
		Threshold: 1000,
	})
	if err != nil {
		t.Fatalf("toolGetLargeFiles: %v", err)
	}

	if len(out.Files) != 3 {
		t.Fatalf("expected 3 files above threshold, got %d", len(out.Files))
	}
	// Should be sorted by size descending
	if out.Files[0].Name != "huge.bin" {
		t.Errorf("largest file = %q, want 'huge.bin'", out.Files[0].Name)
	}
	if out.Files[0].Size != 10000 {
		t.Errorf("largest size = %d, want 10000", out.Files[0].Size)
	}
}

func TestToolGetLargeFilesNothingAboveThreshold(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "tiny.txt"), []byte("x"), 0644)

	s := New(context.Background())
	out, err := s.toolGetLargeFiles(context.Background(), &GetLargeFilesInput{
		Path:      dir,
		Threshold: 1000,
	})
	if err != nil {
		t.Fatalf("toolGetLargeFiles: %v", err)
	}
	if len(out.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(out.Files))
	}
}

// --- find_duplicates tool ---

func TestToolFindDuplicates(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a"), 0755)
	os.MkdirAll(filepath.Join(dir, "b"), 0755)
	// Same name, same size → duplicate candidate
	os.WriteFile(filepath.Join(dir, "a", "photo.jpg"), []byte(makeString(100)), 0644)
	os.WriteFile(filepath.Join(dir, "b", "photo.jpg"), []byte(makeString(100)), 0644)
	// Same name, different size → NOT duplicate
	os.WriteFile(filepath.Join(dir, "a", "doc.pdf"), []byte(makeString(50)), 0644)
	os.WriteFile(filepath.Join(dir, "b", "doc.pdf"), []byte(makeString(200)), 0644)
	// Unique file
	os.WriteFile(filepath.Join(dir, "unique.txt"), []byte("hello"), 0644)

	s := New(context.Background())
	out, err := s.toolFindDuplicates(context.Background(), &FindDuplicatesInput{Path: dir})
	if err != nil {
		t.Fatalf("toolFindDuplicates: %v", err)
	}

	if len(out.Groups) != 1 {
		t.Fatalf("expected 1 duplicate group, got %d", len(out.Groups))
	}
	g := out.Groups[0]
	if g.Name != "photo.jpg" {
		t.Errorf("group name = %q, want 'photo.jpg'", g.Name)
	}
	if len(g.Files) != 2 {
		t.Errorf("group files = %d, want 2", len(g.Files))
	}
}

// --- get_old_files tool ---
// Note: this test is limited because we can't reliably set access times in tests.
// We test that it runs without error and returns the right type.

func TestToolGetOldFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "old.txt"), []byte("old file"), 0644)

	s := New(context.Background())
	out, err := s.toolGetOldFiles(context.Background(), &GetOldFilesInput{
		Path:       dir,
		MaxAgeDays: 365,
	})
	if err != nil {
		t.Fatalf("toolGetOldFiles: %v", err)
	}
	// Just verify it returns without error; atime behavior is platform-dependent
	_ = out
}

// --- helper ---

func makeString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'A'
	}
	return string(b)
}
