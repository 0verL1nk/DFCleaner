package cleaner

import (
	"os"
	"path/filepath"
	"testing"

	"dfcleaner/internal/store"
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

func TestIsDangerousPathShortRoot(t *testing.T) {
	if !isDangerousPath("/a") {
		t.Error("expected /a (length 2) to be dangerous")
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

func TestGetPathSizeNonExistent(t *testing.T) {
	size := getPathSize("/nonexistent/path/file.txt")
	if size != 0 {
		t.Errorf("getPathSize(nonexistent) = %d, want 0", size)
	}
}

func TestGetPathSizeDirWithSubDirs(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("12345"), 0644)
	os.WriteFile(filepath.Join(sub, "b.txt"), []byte("67890"), 0644)

	size := getPathSize(dir)
	if size != 10 {
		t.Errorf("getPathSize(dir with subdir) = %d, want 10", size)
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

func TestPermanentDeleteDir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, "file.txt"), []byte("data"), 0644)

	if err := permanentDelete(sub); err != nil {
		t.Fatalf("permanentDelete dir: %v", err)
	}
	if _, err := os.Stat(sub); !os.IsNotExist(err) {
		t.Error("directory should be deleted")
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

func TestMoveToDirCreatesTargetDir(t *testing.T) {
	src := t.TempDir()
	base := t.TempDir()
	dst := filepath.Join(base, "new", "nested", "dir")

	file := filepath.Join(src, "file.txt")
	os.WriteFile(file, []byte("data"), 0644)

	if err := moveToDir(file, dst); err != nil {
		t.Fatalf("moveToDir with nested target: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dst, "file.txt")); os.IsNotExist(err) {
		t.Error("file should exist in nested destination")
	}
}

func TestMoveDirToDir(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.MkdirAll(filepath.Join(src, "mydir"), 0755)
	os.WriteFile(filepath.Join(src, "mydir", "a.txt"), []byte("aaa"), 0644)
	os.WriteFile(filepath.Join(src, "mydir", "b.txt"), []byte("bbb"), 0644)

	srcDir := filepath.Join(src, "mydir")
	if err := moveDirToDir(srcDir, dst); err != nil {
		t.Fatalf("moveDirToDir: %v", err)
	}

	if _, err := os.Stat(srcDir); !os.IsNotExist(err) {
		t.Error("source dir should be gone")
	}
	if _, err := os.Stat(filepath.Join(dst, "mydir", "a.txt")); os.IsNotExist(err) {
		t.Error("a.txt should exist in destination")
	}
	if _, err := os.Stat(filepath.Join(dst, "mydir", "b.txt")); os.IsNotExist(err) {
		t.Error("b.txt should exist in destination")
	}
}

func TestMoveDirToDirNestedSubDirs(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.MkdirAll(filepath.Join(src, "top", "sub"), 0755)
	os.WriteFile(filepath.Join(src, "top", "sub", "deep.txt"), []byte("deep"), 0644)
	os.WriteFile(filepath.Join(src, "top", "shallow.txt"), []byte("shallow"), 0644)

	if err := moveDirToDir(filepath.Join(src, "top"), dst); err != nil {
		t.Fatalf("moveDirToDir nested: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dst, "top", "sub", "deep.txt")); os.IsNotExist(err) {
		t.Error("nested file should exist in destination")
	}
	if _, err := os.Stat(filepath.Join(dst, "top", "shallow.txt")); os.IsNotExist(err) {
		t.Error("shallow file should exist in destination")
	}
}

func TestCleanupEmptyItems(t *testing.T) {
	c := New(nil)
	results, err := c.Cleanup([]CleanupItem{})
	if err != nil {
		t.Fatalf("Cleanup empty: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for empty input, got %+v", results)
	}
}

func TestCleanupNilItems(t *testing.T) {
	c := New(nil)
	results, err := c.Cleanup(nil)
	if err != nil {
		t.Fatalf("Cleanup nil: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for nil input, got %+v", results)
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
	if results[0].Error != "unknown operation: unknown" {
		t.Errorf("error = %q", results[0].Error)
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
	if results[0].Error != "path must be absolute" {
		t.Errorf("error = %q, want 'path must be absolute'", results[0].Error)
	}
}

func TestCleanupDangerousPath(t *testing.T) {
	c := New(nil)
	results, err := c.Cleanup([]CleanupItem{{Path: "/", Operation: OpDelete}})
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if results[0].Success {
		t.Error("should fail for dangerous path")
	}
	if results[0].Error != "refused: path is too broad" {
		t.Errorf("error = %q", results[0].Error)
	}
}

func TestCleanupDeleteSuccess(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "todelete.txt")
	content := []byte("delete me") // 9 bytes
	os.WriteFile(file, content, 0644)

	c := New(nil)
	results, err := c.Cleanup([]CleanupItem{{Path: file, Operation: OpDelete}})
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if !results[0].Success {
		t.Errorf("expected success, got error: %q", results[0].Error)
	}
	if results[0].FreedBytes != int64(len(content)) {
		t.Errorf("FreedBytes = %d, want %d", results[0].FreedBytes, len(content))
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Error("file should be deleted")
	}
}

func TestCleanupDeleteNonExistentSucceeds(t *testing.T) {
	// os.RemoveAll succeeds even when the path does not exist.
	c := New(nil)
	results, err := c.Cleanup([]CleanupItem{{Path: "/tmp/does_not_exist_xyz.txt", Operation: OpDelete}})
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if !results[0].Success {
		t.Errorf("os.RemoveAll succeeds on nonexistent paths, got error: %q", results[0].Error)
	}
}

func TestCleanupMoveMissingTarget(t *testing.T) {
	c := New(nil)
	results, err := c.Cleanup([]CleanupItem{{Path: "/tmp/something", Operation: OpMove}})
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if results[0].Success {
		t.Error("should fail when target is missing for move")
	}
	if results[0].Error != "move requires a target path" {
		t.Errorf("error = %q", results[0].Error)
	}
}

func TestCleanupMoveSuccess(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	file := filepath.Join(srcDir, "move.txt")
	os.WriteFile(file, []byte("move me"), 0644)

	c := New(nil)
	results, err := c.Cleanup([]CleanupItem{{Path: file, Operation: OpMove, Target: dstDir}})
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if !results[0].Success {
		t.Errorf("expected success, got error: %q", results[0].Error)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "move.txt")); os.IsNotExist(err) {
		t.Error("file should exist in target dir")
	}
}

func TestCleanupMoveDirSuccess(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	dirToMove := filepath.Join(srcDir, "mydir")
	os.MkdirAll(dirToMove, 0755)
	os.WriteFile(filepath.Join(dirToMove, "inner.txt"), []byte("inner"), 0644)

	c := New(nil)
	results, err := c.Cleanup([]CleanupItem{{Path: dirToMove, Operation: OpMove, Target: dstDir}})
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if !results[0].Success {
		t.Errorf("expected success, got error: %q", results[0].Error)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "mydir", "inner.txt")); os.IsNotExist(err) {
		t.Error("inner file should exist in target dir")
	}
}

func TestCleanupTrashSuccess(t *testing.T) {
	// Create the file on the same filesystem as the home trash dir
	// to avoid cross-device rename errors with os.Rename.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	dir := filepath.Join(home, ".dfcleaner-test-trash")
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	file := filepath.Join(dir, "trashme.txt")
	os.WriteFile(file, []byte("trash me"), 0644)

	c := New(nil)
	results, err := c.Cleanup([]CleanupItem{{Path: file, Operation: OpTrash}})
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if !results[0].Success {
		t.Errorf("expected success, got error: %q", results[0].Error)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Error("file should be gone from original location")
	}
}

func TestCleanupWithStore(t *testing.T) {
	db, err := store.InitDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	s := store.New(db)

	dir := t.TempDir()
	file := filepath.Join(dir, "store_test.txt")
	os.WriteFile(file, []byte("log this"), 0644)

	c := New(s)
	results, err := c.Cleanup([]CleanupItem{{Path: file, Operation: OpDelete}})
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if !results[0].Success {
		t.Errorf("expected success, got error: %q", results[0].Error)
	}

	logs := s.GetRecentCleanups(10)
	if len(logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(logs))
	}
	if logs[0].FilePath != file {
		t.Errorf("log FilePath = %q, want %q", logs[0].FilePath, file)
	}
	if logs[0].Result != "success" {
		t.Errorf("log Result = %q, want 'success'", logs[0].Result)
	}
	if logs[0].FreedBytes != 8 {
		t.Errorf("log FreedBytes = %d, want 8", logs[0].FreedBytes)
	}
}

func TestCleanupWithStoreLogsFailure(t *testing.T) {
	db, err := store.InitDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	s := store.New(db)

	dir := t.TempDir()
	file := filepath.Join(dir, "failmove.txt")
	os.WriteFile(file, []byte("data"), 0644)

	c := New(s)
	// Move to an impossible target path triggers a real failure
	results, err := c.Cleanup([]CleanupItem{{Path: file, Operation: OpMove, Target: "/dev/null/impossible"}})
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if results[0].Success {
		t.Error("expected failure for impossible move target")
	}

	logs := s.GetRecentCleanups(10)
	if len(logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(logs))
	}
	if logs[0].Result != "failed" {
		t.Errorf("log Result = %q, want 'failed'", logs[0].Result)
	}
}

func TestCleanupMultipleItems(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	f2 := filepath.Join(dir, "b.txt")
	os.WriteFile(f1, []byte("aa"), 0644)
	os.WriteFile(f2, []byte("bb"), 0644)

	c := New(nil)
	results, err := c.Cleanup([]CleanupItem{
		{Path: f1, Operation: OpDelete},
		{Path: f2, Operation: OpDelete},
	})
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !results[0].Success || !results[1].Success {
		t.Errorf("both should succeed: %v %v", results[0].Error, results[1].Error)
	}
}

func TestUniquePathNoCollision(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "file.txt")

	result := uniquePath(original)
	if result != original {
		t.Errorf("expected %q when no collision, got %q", original, result)
	}
}

func TestUniquePathWithCollision(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "file.txt")
	os.WriteFile(original, []byte("exists"), 0644)

	result := uniquePath(original)
	expected := filepath.Join(dir, "file (1).txt")
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestUniquePathMultipleCollisions(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "file.txt")
	os.WriteFile(original, []byte("1"), 0644)
	os.WriteFile(filepath.Join(dir, "file (1).txt"), []byte("2"), 0644)

	result := uniquePath(original)
	expected := filepath.Join(dir, "file (2).txt")
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestUniquePathWithExtension(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "data.csv")
	os.WriteFile(original, []byte("exists"), 0644)

	result := uniquePath(original)
	expected := filepath.Join(dir, "data (1).csv")
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestUniquePathNoExtension(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "README")
	os.WriteFile(original, []byte("exists"), 0644)

	result := uniquePath(original)
	expected := filepath.Join(dir, "README (1)")
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
