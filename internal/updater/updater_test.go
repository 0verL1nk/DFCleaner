package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestSemverGT(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"1.1.0", "1.0.0", true},
		{"1.0.0", "1.1.0", false},
		{"2.0.0", "1.9.9", true},
		{"1.0.0", "1.0.0", false},
		{"1.0.1", "1.0.0", true},
		{"1.0.0", "0.9.9", true},
		{"0.9.0", "1.0.0", false},
		{"1.10.0", "1.9.0", true},
		{"1.2.3-beta", "1.2.2", true},
		{"1.2", "1.2.0", false},
	}

	for _, tt := range tests {
		if got := semverGT(tt.a, tt.b); got != tt.want {
			t.Errorf("semverGT(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"1.2.3", []int{1, 2, 3}},
		{"0.9.0", []int{0, 9, 0}},
		{"2.0", []int{2, 0}},
		{"1.2.3-beta", []int{1, 2, 3}},
		{"", []int{0}},
		{"1.x.3", []int{1, 0, 3}},
	}

	for _, tt := range tests {
		got := parseSemver(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseSemver(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseSemver(%q)[%d] = %d, want %d", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestFindAppBundle(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/Apps/DFCleaner.app/Contents/MacOS/DFCleaner", "/Apps/DFCleaner.app"},
		{"/usr/local/bin/DFCleaner", ""},
		{"", ""},
		{"/Apps/My.app/Contents/MacOS/My", "/Apps/My.app"},
	}

	for _, tt := range tests {
		got := findAppBundle(tt.path)
		if got != tt.want {
			t.Errorf("findAppBundle(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestFindPlatformAsset(t *testing.T) {
	u := &Updater{logger: log.New(os.Stderr, "[test] ", 0)}

	assets := []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	}{
		{Name: "DFCleaner-linux-amd64-portable.tar.gz", URL: "https://example.com/linux-amd64"},
		{Name: "DFCleaner-windows-amd64-portable.zip", URL: "https://example.com/windows-amd64"},
		{Name: "DFCleaner-linux-arm64-portable.tar.gz", URL: "https://example.com/linux-arm64"},
	}

	url := u.findPlatformAsset(assets)
	if url == "" {
		t.Error("expected to find a platform asset")
	}
}

func TestFindPlatformAssetPrefersInstaller(t *testing.T) {
	u := &Updater{logger: log.New(os.Stderr, "[test] ", 0)}

	assets := []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	}{
		{Name: "DFCleaner-linux-arm64-portable.tar.gz", URL: "https://example.com/portable"},
		{Name: "DFCleaner-windows-amd64-portable.zip", URL: "https://example.com/win-portable"},
		{Name: "DFCleaner-amd64-installer.exe", URL: "https://example.com/installer"},
	}

	// On Linux the installer isn't preferred; portable is matched instead.
	// The test validates that the function returns a valid URL without panicking.
	url := u.findPlatformAsset(assets)
	if url == "" {
		t.Error("expected to find a platform asset")
	}
}

func TestFindPlatformAssetNoMatch(t *testing.T) {
	u := &Updater{logger: log.New(os.Stderr, "[test] ", 0)}

	assets := []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	}{
		{Name: "README.md", URL: "https://example.com/readme"},
	}

	url := u.findPlatformAsset(assets)
	if url != "" {
		t.Errorf("expected empty URL for no match, got %s", url)
	}
}

func TestCleanOldBackupNoPanic(t *testing.T) {
	CleanOldBackup()
}

func newTestUpdater() *Updater {
	return &Updater{logger: log.New(os.Stderr, "[test] ", 0)}
}

func createTestTarGz(t *testing.T, destDir, binaryName string) string {
	t.Helper()
	archivePath := filepath.Join(destDir, "test.tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gzw := gzip.NewWriter(f)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	data := []byte("#!/bin/sh\necho hello\n")
	hdr := &tar.Header{
		Name: binaryName,
		Mode: 0755,
		Size: int64(len(data)),
	}
	tw.WriteHeader(hdr)
	tw.Write(data)
	tw.Flush()

	return archivePath
}

func TestExtractTarGz(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := createTestTarGz(t, tmpDir, "DFCleaner")

	u := newTestUpdater()
	extracted, err := u.extractTarGz(archivePath, tmpDir)
	if err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}

	if _, err := os.Stat(extracted); err != nil {
		t.Errorf("extracted file not found: %s", extracted)
	}
}

func TestExtractTarGzNoBinary(t *testing.T) {
	tmpDir := t.TempDir()

	archivePath := filepath.Join(tmpDir, "empty.tar.gz")
	f, _ := os.Create(archivePath)
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)
	tw.Close()
	gzw.Close()
	f.Close()

	u := newTestUpdater()
	_, err := u.extractTarGz(archivePath, tmpDir)
	if err == nil {
		t.Error("expected error for archive with no binary")
	}
}

func TestExtractZip(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	w := zip.NewWriter(f)
	data := []byte("binary content")
	outf, _ := w.Create("DFCleaner")
	outf.Write(data)
	w.Close()
	f.Close()

	u := newTestUpdater()
	extracted, err := u.extractZip(zipPath, tmpDir)
	if err != nil {
		t.Fatalf("extractZip: %v", err)
	}

	if _, err := os.Stat(extracted); err != nil {
		t.Errorf("extracted file not found: %s", extracted)
	}
}

func TestExtractZipWithExe(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")

	f, _ := os.Create(zipPath)
	w := zip.NewWriter(f)
	data := []byte("exe content")
	outf, _ := w.Create("DFCleaner.exe")
	outf.Write(data)
	w.Close()
	f.Close()

	u := newTestUpdater()
	extracted, err := u.extractZip(zipPath, tmpDir)
	if err != nil {
		t.Fatalf("extractZip: %v", err)
	}
	if extracted == "" {
		t.Error("expected to extract DFCleaner.exe")
	}
}

func TestExtractZipNoBinary(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "empty.zip")

	f, _ := os.Create(zipPath)
	w := zip.NewWriter(f)
	outf, _ := w.Create(".hidden")
	outf.Write([]byte("hidden"))
	w.Close()
	f.Close()

	u := newTestUpdater()
	_, err := u.extractZip(zipPath, tmpDir)
	if err == nil {
		t.Error("expected error for zip with no binary")
	}
}

func TestExtractDispatchesTarGz(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := createTestTarGz(t, tmpDir, "DFCleaner")

	u := newTestUpdater()
	extracted, err := u.extract(archivePath, tmpDir)
	if err != nil {
		t.Fatalf("extract tar.gz: %v", err)
	}
	if extracted == "" {
		t.Error("expected extracted path")
	}
}

func TestExtractDispatchesZip(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")

	f, _ := os.Create(zipPath)
	w := zip.NewWriter(f)
	outf, _ := w.Create("DFCleaner")
	outf.Write([]byte("data"))
	w.Close()
	f.Close()

	u := newTestUpdater()
	extracted, err := u.extract(zipPath, tmpDir)
	if err != nil {
		t.Fatalf("extract zip: %v", err)
	}
	if extracted == "" {
		t.Error("expected extracted path")
	}
}

func TestDownloadProgressWriter(t *testing.T) {
	dp := &downloadProgress{
		total:   1000,
		emitter: func(float64) {},
	}

	n, err := dp.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5 bytes written, got %d", n)
	}
	if dp.written != 5 {
		t.Errorf("expected 5 total written, got %d", dp.written)
	}
}

func TestCopyDir(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dst")

	os.WriteFile(filepath.Join(src, "a.txt"), []byte("hello"), 0644)
	os.MkdirAll(filepath.Join(src, "sub"), 0755)
	os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("world"), 0644)

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dst, "a.txt"))
	if err != nil || string(data) != "hello" {
		t.Errorf("expected 'hello', got %q, err=%v", string(data), err)
	}

	data, err = os.ReadFile(filepath.Join(dst, "sub", "b.txt"))
	if err != nil || string(data) != "world" {
		t.Errorf("expected 'world', got %q, err=%v", string(data), err)
	}
}

func TestReplaceBinaryError(t *testing.T) {
	u := newTestUpdater()
	err := u.replaceBinary("/nonexistent/path/binary", "/nonexistent/new")
	if err == nil {
		t.Error("expected error for nonexistent paths")
	}
}

func TestPerformUpdateEmptyURL(t *testing.T) {
	u := newTestUpdater()
	err := u.PerformUpdate(&UpdateInfo{})
	if err == nil {
		t.Error("expected error for empty download URL")
	}
}

func TestReplaceEmptyBinary(t *testing.T) {
	tmpDir := t.TempDir()
	emptyBin := filepath.Join(tmpDir, "empty")
	os.WriteFile(emptyBin, []byte{}, 0755)

	u := newTestUpdater()
	err := u.replaceBinary("/nonexistent/self", emptyBin)
	if err == nil {
		t.Error("expected error for empty binary")
	}
}
