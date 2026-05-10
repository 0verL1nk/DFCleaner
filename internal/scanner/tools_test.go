package scanner

import (
	"context"
	"encoding/json"
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
	out, err := s.toolScanDirectory(context.Background(), &ScanDirInput{Path: "/nonexistent/path/xyz"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Path != "/nonexistent/path/xyz" {
		t.Errorf("Path = %q, want /nonexistent/path/xyz", out.Path)
	}
	if out.FileCount != 0 || out.SubDirs != 0 {
		t.Errorf("expected empty result for nonexistent path, got fileCount=%d subDirs=%d", out.FileCount, out.SubDirs)
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

// --- overflowFiles tests ---

func TestOverflowFiles_SmallList(t *testing.T) {
	// A small list should not overflow -- returned as-is with no result file
	files := []FileEntry{
		{Name: "a.txt", Size: 100},
		{Name: "b.txt", Size: 200},
	}

	kept, resultFile, moreItems := overflowFiles(files)

	if len(kept) != 2 {
		t.Errorf("kept length = %d, want 2", len(kept))
	}
	if resultFile != "" {
		t.Errorf("resultFile = %q, want empty (no overflow)", resultFile)
	}
	if moreItems != 0 {
		t.Errorf("moreItems = %d, want 0", moreItems)
	}
}

func TestOverflowFiles_LargeListTriggersOverflow(t *testing.T) {
	// Create enough FileEntry objects to exceed maxResultChars (6000 bytes)
	files := make([]FileEntry, 200)
	for i := range files {
		files[i] = FileEntry{
			Path:      "/some/long/path/to/file_" + makeString(20) + ".txt",
			Name:      "file_" + makeString(20) + ".txt",
			Size:      int64(i * 1000),
			Extension: "txt",
		}
	}

	kept, resultFile, moreItems := overflowFiles(files)

	if len(kept) == 0 {
		t.Error("expected at least 1 kept item")
	}
	if len(kept) >= len(files) {
		t.Error("expected some items to be truncated")
	}
	if resultFile == "" {
		t.Error("expected a result file to be created")
	}
	defer os.Remove(resultFile)

	if moreItems <= 0 {
		t.Errorf("moreItems = %d, want > 0", moreItems)
	}
	if len(kept)+moreItems != len(files) {
		t.Errorf("kept(%d) + moreItems(%d) = %d, want total %d", len(kept), moreItems, len(kept)+moreItems, len(files))
	}

	// Verify the temp file contains valid JSON with all files
	data, err := os.ReadFile(resultFile)
	if err != nil {
		t.Fatalf("failed to read result file: %v", err)
	}
	var loadedFiles []FileEntry
	if err := json.Unmarshal(data, &loadedFiles); err != nil {
		t.Fatalf("failed to parse result file JSON: %v", err)
	}
	if len(loadedFiles) != len(files) {
		t.Errorf("result file contains %d files, want %d", len(loadedFiles), len(files))
	}
}

func TestOverflowFiles_EmptyList(t *testing.T) {
	files := []FileEntry{}

	kept, resultFile, moreItems := overflowFiles(files)

	if len(kept) != 0 {
		t.Errorf("kept length = %d, want 0", len(kept))
	}
	if resultFile != "" {
		t.Errorf("resultFile = %q, want empty", resultFile)
	}
	if moreItems != 0 {
		t.Errorf("moreItems = %d, want 0", moreItems)
	}
}

// --- overflowGroups tests ---

func TestOverflowGroups_SmallList(t *testing.T) {
	groups := []DuplicateGroup{
		{Name: "photo.jpg", Size: 1000, Files: []FileEntry{{Name: "photo.jpg"}}},
	}

	kept, resultFile, moreGroups := overflowGroups(groups)

	if len(kept) != 1 {
		t.Errorf("kept length = %d, want 1", len(kept))
	}
	if resultFile != "" {
		t.Errorf("resultFile = %q, want empty", resultFile)
	}
	if moreGroups != 0 {
		t.Errorf("moreGroups = %d, want 0", moreGroups)
	}
}

func TestOverflowGroups_LargeListTriggersOverflow(t *testing.T) {
	groups := make([]DuplicateGroup, 200)
	for i := range groups {
		files := make([]FileEntry, 3)
		for j := range files {
			files[j] = FileEntry{
				Path: "/path/to/dir" + makeString(15) + "/file.txt",
				Name: "file_" + makeString(10) + ".txt",
				Size: int64(i * 100),
			}
		}
		groups[i] = DuplicateGroup{
			Name:  "dup_" + makeString(20) + ".txt",
			Size:  int64(i * 100),
			Files: files,
		}
	}

	kept, resultFile, moreGroups := overflowGroups(groups)

	if len(kept) == 0 {
		t.Error("expected at least 1 kept group")
	}
	if resultFile == "" {
		t.Error("expected a result file to be created")
	}
	defer os.Remove(resultFile)

	if moreGroups <= 0 {
		t.Errorf("moreGroups = %d, want > 0", moreGroups)
	}

	// Verify the temp file contains valid JSON with all groups
	data, err := os.ReadFile(resultFile)
	if err != nil {
		t.Fatalf("failed to read result file: %v", err)
	}
	var loadedGroups []DuplicateGroup
	if err := json.Unmarshal(data, &loadedGroups); err != nil {
		t.Fatalf("failed to parse result file JSON: %v", err)
	}
	if len(loadedGroups) != len(groups) {
		t.Errorf("result file contains %d groups, want %d", len(loadedGroups), len(groups))
	}
}

// --- toolViewResult tests ---

func TestToolViewResult_NormalPaging(t *testing.T) {
	// Create a temp file with known JSON content
	dir := t.TempDir()
	files := make([]FileEntry, 10)
	for i := range files {
		files[i] = FileEntry{
			Path: "/test/file" + string(rune('0'+i)) + ".txt",
			Name: "file" + string(rune('0'+i)) + ".txt",
			Size: int64((i + 1) * 100),
		}
	}
	data, _ := json.Marshal(files)
	resultFile := filepath.Join(dir, "result.json")
	os.WriteFile(resultFile, data, 0644)

	s := New(context.Background())
	out, err := s.toolViewResult(context.Background(), &ViewResultInput{
		Path:   resultFile,
		Offset: 0,
		Limit:  3,
	})

	if err != nil {
		t.Fatalf("toolViewResult error: %v", err)
	}
	if len(out.Items) != 3 {
		t.Fatalf("Items length = %d, want 3", len(out.Items))
	}
	if out.TotalCount != 10 {
		t.Errorf("TotalCount = %d, want 10", out.TotalCount)
	}
	if out.Offset != 0 {
		t.Errorf("Offset = %d, want 0", out.Offset)
	}
	if !out.HasMore {
		t.Error("HasMore should be true")
	}
	if out.Items[0].Name != "file0.txt" {
		t.Errorf("first item Name = %q, want 'file0.txt'", out.Items[0].Name)
	}
}

func TestToolViewResult_NonexistentFile(t *testing.T) {
	s := New(context.Background())
	out, err := s.toolViewResult(context.Background(), &ViewResultInput{
		Path:   "/nonexistent/result.json",
		Offset: 0,
		Limit:  10,
	})

	if err != nil {
		t.Fatalf("toolViewResult should not error on missing file: %v", err)
	}
	if len(out.Items) != 0 {
		t.Errorf("Items length = %d, want 0", len(out.Items))
	}
	if out.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", out.TotalCount)
	}
	if out.HasMore {
		t.Error("HasMore should be false")
	}
}

func TestToolViewResult_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	badFile := filepath.Join(dir, "bad.json")
	os.WriteFile(badFile, []byte("this is not valid json"), 0644)

	s := New(context.Background())
	out, err := s.toolViewResult(context.Background(), &ViewResultInput{
		Path:   badFile,
		Offset: 0,
		Limit:  10,
	})

	if err != nil {
		t.Fatalf("toolViewResult should not error on invalid JSON: %v", err)
	}
	if len(out.Items) != 0 {
		t.Errorf("Items length = %d, want 0 for invalid JSON", len(out.Items))
	}
	if out.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0 for invalid JSON", out.TotalCount)
	}
}

func TestToolViewResult_OffsetPastEnd(t *testing.T) {
	dir := t.TempDir()
	files := []FileEntry{{Name: "a.txt"}, {Name: "b.txt"}}
	data, _ := json.Marshal(files)
	resultFile := filepath.Join(dir, "result.json")
	os.WriteFile(resultFile, data, 0644)

	s := New(context.Background())
	out, err := s.toolViewResult(context.Background(), &ViewResultInput{
		Path:   resultFile,
		Offset: 100,
		Limit:  10,
	})

	if err != nil {
		t.Fatalf("toolViewResult error: %v", err)
	}
	if len(out.Items) != 0 {
		t.Errorf("Items length = %d, want 0 for offset past end", len(out.Items))
	}
	if out.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2", out.TotalCount)
	}
	if out.HasMore {
		t.Error("HasMore should be false when offset is past end")
	}
}

func TestToolViewResult_NegativeOffsetAndLimit(t *testing.T) {
	dir := t.TempDir()
	files := []FileEntry{{Name: "a.txt"}, {Name: "b.txt"}, {Name: "c.txt"}}
	data, _ := json.Marshal(files)
	resultFile := filepath.Join(dir, "result.json")
	os.WriteFile(resultFile, data, 0644)

	s := New(context.Background())
	out, err := s.toolViewResult(context.Background(), &ViewResultInput{
		Path:   resultFile,
		Offset: -5,
		Limit:  -10,
	})

	if err != nil {
		t.Fatalf("toolViewResult error: %v", err)
	}
	// Negative offset should default to 0, negative limit should default to 50
	if out.Offset != 0 {
		t.Errorf("Offset = %d, want 0 (clamped from negative)", out.Offset)
	}
	if len(out.Items) != 3 {
		t.Errorf("Items length = %d, want 3 (default limit 50 covers all)", len(out.Items))
	}
}

func TestToolViewResult_LastPage(t *testing.T) {
	dir := t.TempDir()
	files := make([]FileEntry, 5)
	for i := range files {
		files[i] = FileEntry{Name: "file" + string(rune('0'+i)) + ".txt"}
	}
	data, _ := json.Marshal(files)
	resultFile := filepath.Join(dir, "result.json")
	os.WriteFile(resultFile, data, 0644)

	s := New(context.Background())
	out, err := s.toolViewResult(context.Background(), &ViewResultInput{
		Path:   resultFile,
		Offset: 3,
		Limit:  10,
	})

	if err != nil {
		t.Fatalf("toolViewResult error: %v", err)
	}
	if len(out.Items) != 2 {
		t.Errorf("Items length = %d, want 2", len(out.Items))
	}
	if out.HasMore {
		t.Error("HasMore should be false on last page")
	}
	if out.TotalCount != 5 {
		t.Errorf("TotalCount = %d, want 5", out.TotalCount)
	}
}

// --- RegisterTools tests ---

func TestRegisterTools_ReturnsSixTools(t *testing.T) {
	s := New(context.Background())
	tools, err := RegisterTools(s)

	if err != nil {
		t.Fatalf("RegisterTools error: %v", err)
	}
	if len(tools) != 6 {
		t.Errorf("expected 6 tools, got %d", len(tools))
	}
}

func TestRegisterTools_AllToolsHaveInfo(t *testing.T) {
	s := New(context.Background())
	tools, err := RegisterTools(s)
	if err != nil {
		t.Fatalf("RegisterTools error: %v", err)
	}

	ctx := context.Background()
	expectedNames := map[string]bool{
		"scan_directory":  false,
		"get_file_info":   false,
		"get_large_files": false,
		"get_old_files":   false,
		"find_duplicates": false,
		"view_result":     false,
	}

	for _, tl := range tools {
		info, err := tl.Info(ctx)
		if err != nil {
			t.Errorf("tool Info() error: %v", err)
			continue
		}
		if _, ok := expectedNames[info.Name]; !ok {
			t.Errorf("unexpected tool name: %q", info.Name)
		}
		expectedNames[info.Name] = true
		if info.Desc == "" {
			t.Errorf("tool %q has empty description", info.Name)
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("missing tool: %q", name)
		}
	}
}

// --- get_file_info edge cases ---

func TestToolGetFileInfo_NonexistentFile(t *testing.T) {
	s := New(context.Background())
	entry, err := s.toolGetFileInfo(context.Background(), &GetFileInfoInput{
		Path: "/nonexistent/file.txt",
	})

	if err != nil {
		t.Fatalf("toolGetFileInfo should not error on nonexistent file: %v", err)
	}
	if entry.Name != "file.txt" {
		t.Errorf("Name = %q, want 'file.txt'", entry.Name)
	}
	if entry.Size != 0 {
		t.Errorf("Size = %d, want 0 for nonexistent file", entry.Size)
	}
}

func TestToolGetFileInfo_Directory(t *testing.T) {
	dir := t.TempDir()
	s := New(context.Background())
	entry, err := s.toolGetFileInfo(context.Background(), &GetFileInfoInput{Path: dir})

	if err != nil {
		t.Fatalf("toolGetFileInfo on directory error: %v", err)
	}
	if !entry.IsDir {
		t.Error("IsDir should be true for directory")
	}
	if entry.Name == "" {
		t.Error("Name should not be empty for directory")
	}
}

// --- scan_directory edge cases ---

func TestToolScanDirectory_EmptyPath(t *testing.T) {
	s := New(context.Background())
	_, err := s.toolScanDirectory(context.Background(), &ScanDirInput{Path: ""})

	if err == nil {
		t.Fatal("expected error for empty path, got nil")
	}
}

func TestToolScanDirectory_WithSubDirs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "docs"), 0755)
	os.MkdirAll(filepath.Join(dir, "images"), 0755)
	os.WriteFile(filepath.Join(dir, "docs", "readme.md"), []byte(makeString(500)), 0644)
	os.WriteFile(filepath.Join(dir, "docs", "guide.md"), []byte(makeString(300)), 0644)
	os.WriteFile(filepath.Join(dir, "images", "logo.png"), []byte(makeString(1000)), 0644)
	os.WriteFile(filepath.Join(dir, "root.txt"), []byte("root file"), 0644)

	s := New(context.Background())
	out, err := s.toolScanDirectory(context.Background(), &ScanDirInput{Path: dir})
	if err != nil {
		t.Fatalf("toolScanDirectory error: %v", err)
	}

	if out.SubDirs != 2 {
		t.Errorf("SubDirs = %d, want 2", out.SubDirs)
	}
	if out.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1 (top-level only)", out.FileCount)
	}
	if len(out.SubDirSummary) != 2 {
		t.Fatalf("SubDirSummary length = %d, want 2", len(out.SubDirSummary))
	}

	// SubDirSummary should be sorted by TotalSize descending
	if out.SubDirSummary[0].Name != "images" {
		t.Errorf("largest subdir = %q, want 'images'", out.SubDirSummary[0].Name)
	}
	if out.SubDirSummary[0].FileCount != 1 {
		t.Errorf("images fileCount = %d, want 1", out.SubDirSummary[0].FileCount)
	}
}

// --- get_large_files edge cases ---

func TestToolGetLargeFiles_NonexistentPath(t *testing.T) {
	s := New(context.Background())
	out, err := s.toolGetLargeFiles(context.Background(), &GetLargeFilesInput{
		Path:      "/nonexistent/path",
		Threshold: 0,
	})

	if err != nil {
		t.Fatalf("toolGetLargeFiles should not error: %v", err)
	}
	if len(out.Files) != 0 {
		t.Errorf("Files length = %d, want 0", len(out.Files))
	}
}

// --- find_duplicates edge cases ---

func TestToolFindDuplicates_NoDuplicates(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "unique1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(dir, "unique2.txt"), []byte("content2"), 0644)

	s := New(context.Background())
	out, err := s.toolFindDuplicates(context.Background(), &FindDuplicatesInput{Path: dir})

	if err != nil {
		t.Fatalf("toolFindDuplicates error: %v", err)
	}
	if len(out.Groups) != 0 {
		t.Errorf("Groups length = %d, want 0 for no duplicates", len(out.Groups))
	}
}

func TestToolFindDuplicates_NonexistentPath(t *testing.T) {
	s := New(context.Background())
	out, err := s.toolFindDuplicates(context.Background(), &FindDuplicatesInput{
		Path: "/nonexistent/path",
	})

	if err != nil {
		t.Fatalf("toolFindDuplicates should not error: %v", err)
	}
	if len(out.Groups) != 0 {
		t.Errorf("Groups length = %d, want 0 for nonexistent path", len(out.Groups))
	}
}

// --- get_old_files edge cases ---

func TestToolGetOldFiles_NonexistentPath(t *testing.T) {
	s := New(context.Background())
	out, err := s.toolGetOldFiles(context.Background(), &GetOldFilesInput{
		Path:       "/nonexistent/path",
		MaxAgeDays: 365,
	})

	if err != nil {
		t.Fatalf("toolGetOldFiles should not error: %v", err)
	}
	if len(out.Files) != 0 {
		t.Errorf("Files length = %d, want 0 for nonexistent path", len(out.Files))
	}
}

func TestToolGetOldFiles_ZeroDays(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "fresh.txt"), []byte("just created"), 0644)

	s := New(context.Background())
	out, err := s.toolGetOldFiles(context.Background(), &GetOldFilesInput{
		Path:       dir,
		MaxAgeDays: 0,
	})

	if err != nil {
		t.Fatalf("toolGetOldFiles error: %v", err)
	}
	// With MaxAgeDays=0, cutoff is time.Now(), so files with zero atime
	// or atime before now should be included. Most files should match.
	if len(out.Files) == 0 {
		t.Log("No old files found with 0 days -- atime may be current")
	}
}
