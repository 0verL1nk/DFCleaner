package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

type Updater struct {
	ctx    context.Context
	logger *log.Logger
}

type UpdateInfo struct {
	HasUpdate    bool   `json:"hasUpdate"`
	CurrentVer   string `json:"currentVer"`
	LatestVer    string `json:"latestVer"`
	DownloadURL  string `json:"downloadUrl"`
	ReleaseNotes string `json:"releaseNotes"`
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func New(ctx context.Context, logger *log.Logger) *Updater {
	return &Updater{ctx: ctx, logger: logger}
}

// CleanOldBackup removes leftover .old backup from a previous update.
// Call on startup after the new version has been confirmed running.
func CleanOldBackup() {
	selfPath, err := os.Executable()
	if err != nil {
		return
	}
	selfPath, _ = filepath.EvalSymlinks(selfPath)
	_ = os.Remove(selfPath + ".old")

	if runtime.GOOS == "darwin" {
		appPath := findAppBundle(selfPath)
		if appPath != "" {
			_ = os.RemoveAll(appPath + ".old")
		}
	}
}

func (u *Updater) CheckForUpdate(currentVer string) (*UpdateInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/0verL1nk/DFCleaner/releases/latest")
	if err != nil {
		return nil, fmt.Errorf("check update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API returned %s", resp.Status)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}

	info := &UpdateInfo{
		CurrentVer:   currentVer,
		LatestVer:    release.TagName,
		ReleaseNotes: release.Body,
	}

	current := strings.TrimPrefix(currentVer, "v")
	latest := strings.TrimPrefix(release.TagName, "v")
	info.HasUpdate = latest != "" && latest != current && semverGT(latest, current)

	info.DownloadURL = u.findPlatformAsset(release.Assets)
	if info.DownloadURL == "" {
		info.DownloadURL = release.HTMLURL
	}

	u.logger.Printf("[Update] current=%s latest=%s hasUpdate=%v url=%s", currentVer, release.TagName, info.HasUpdate, info.DownloadURL)
	return info, nil
}

func (u *Updater) PerformUpdate(info *UpdateInfo) error {
	if info.DownloadURL == "" {
		return fmt.Errorf("no download URL")
	}

	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	selfPath, err = filepath.EvalSymlinks(selfPath)
	if err != nil {
		return fmt.Errorf("resolve symlink: %w", err)
	}

	u.logger.Printf("[Update] starting update: self=%s url=%s", selfPath, info.DownloadURL)

	tmpDir, err := os.MkdirTemp("", "dfcleaner-update-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, "update-archive")
	if err := u.download(info.DownloadURL, archivePath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	stat, err := os.Stat(archivePath)
	if err != nil || stat.Size() == 0 {
		return fmt.Errorf("downloaded file is empty or missing")
	}

	u.emitProgress("extracting", 60, "Extracting...")
	extractedPath, err := u.extract(archivePath, tmpDir)
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	u.emitProgress("replacing", 80, "Replacing...")
	if err := u.replace(selfPath, extractedPath); err != nil {
		return fmt.Errorf("replace: %w", err)
	}

	u.emitProgress("restarting", 100, "Restarting...")
	wailsrt.EventsEmit(u.ctx, "update:complete")

	time.Sleep(500 * time.Millisecond)
	return u.restart(selfPath)
}

// --- Version comparison (semver) ---

func semverGT(a, b string) bool {
	aParts := parseSemver(a)
	bParts := parseSemver(b)
	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}
	for i := 0; i < maxLen; i++ {
		av, bv := 0, 0
		if i < len(aParts) {
			av = aParts[i]
		}
		if i < len(bParts) {
			bv = bParts[i]
		}
		if av > bv {
			return true
		}
		if av < bv {
			return false
		}
	}
	return false
}

func parseSemver(v string) []int {
	if idx := strings.Index(v, "-"); idx >= 0 {
		v = v[:idx]
	}
	parts := strings.Split(v, ".")
	result := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			result = append(result, 0)
		} else {
			result = append(result, n)
		}
	}
	return result
}

// --- Platform asset matching ---

func (u *Updater) findPlatformAsset(assets []struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}) string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	suffix := fmt.Sprintf("-%s-%s-portable.", goos, goarch)
	for _, a := range assets {
		if strings.Contains(a.Name, suffix) {
			return a.URL
		}
	}

	for _, a := range assets {
		if strings.Contains(a.Name, goos) {
			return a.URL
		}
	}
	return ""
}

// --- Download ---

func (u *Updater) download(url, dest string) error {
	u.emitProgress("downloading", 0, "Downloading...")

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	total := resp.ContentLength
	progress := &downloadProgress{
		total:   total,
		written: 0,
		emitter: func(pct float64) {
			u.emitProgress("downloading", pct, fmt.Sprintf("Downloading... %.0f%%", pct))
		},
	}

	_, err = io.Copy(f, io.TeeReader(resp.Body, progress))
	return err
}

// --- Extraction ---

func (u *Updater) extract(archivePath, destDir string) (string, error) {
	if strings.HasSuffix(archivePath, ".zip") {
		return u.extractZip(archivePath, destDir)
	}
	return u.extractTarGz(archivePath, destDir)
}

func (u *Updater) extractTarGz(archivePath, destDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	// macOS: extract entire .app bundle
	if runtime.GOOS == "darwin" {
		return u.extractMacOSAppBundle(tr, destDir)
	}

	// Linux/Windows: extract single binary
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		name := filepath.Base(hdr.Name)
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".txt") || strings.HasSuffix(name, ".md") {
			continue
		}

		outPath := filepath.Join(destDir, name)
		if err := writeTarFile(tr, outPath, hdr.FileInfo().Mode()); err != nil {
			return "", err
		}

		u.logger.Printf("[Update] extracted binary: %s", outPath)
		return outPath, nil
	}

	return "", fmt.Errorf("no binary found in archive")
}

func (u *Updater) extractMacOSAppBundle(tr *tar.Reader, destDir string) (string, error) {
	var binaryPath string

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		targetPath := filepath.Join(destDir, hdr.Name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, hdr.FileInfo().Mode()); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return "", err
			}
			if err := writeTarFile(tr, targetPath, hdr.FileInfo().Mode()); err != nil {
				return "", err
			}
			if strings.HasSuffix(hdr.Name, "Contents/MacOS/DFCleaner") {
				binaryPath = targetPath
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return "", err
			}
			_ = os.Symlink(hdr.Linkname, targetPath)
		}
	}

	if binaryPath == "" {
		return "", fmt.Errorf("no DFCleaner binary found in .app bundle")
	}

	appDir := filepath.Join(destDir, "DFCleaner.app")
	u.logger.Printf("[Update] extracted macOS bundle: %s (binary: %s)", appDir, binaryPath)
	return appDir, nil
}

func writeTarFile(tr *tar.Reader, outPath string, mode os.FileMode) error {
	outF, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(outF, tr)
	outF.Close()
	return err
}

func (u *Updater) extractZip(archivePath, destDir string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		name := filepath.Base(f.Name)
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".txt") || strings.HasSuffix(name, ".md") {
			continue
		}

		outPath := filepath.Join(destDir, name)
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return "", err
		}

		outF, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.FileInfo().Mode())
		if err != nil {
			return "", err
		}

		rc, err := f.Open()
		if err != nil {
			outF.Close()
			return "", err
		}

		_, err = io.Copy(outF, rc)
		rc.Close()
		outF.Close()
		if err != nil {
			return "", err
		}

		if strings.HasSuffix(strings.ToLower(name), ".exe") || name == "DFCleaner" {
			u.logger.Printf("[Update] extracted binary: %s", outPath)
			return outPath, nil
		}
	}

	return "", fmt.Errorf("no binary found in archive")
}

// --- Replace ---

func (u *Updater) replace(selfPath, newArtifact string) error {
	info, err := os.Stat(newArtifact)
	if err != nil {
		return fmt.Errorf("stat new artifact: %w", err)
	}

	if info.IsDir() {
		return u.replaceAppBundle(selfPath, newArtifact)
	}
	return u.replaceBinary(selfPath, newArtifact)
}

func (u *Updater) replaceBinary(selfPath, newBinary string) error {
	newData, err := os.ReadFile(newBinary)
	if err != nil {
		return fmt.Errorf("read new binary: %w", err)
	}

	if len(newData) == 0 {
		return fmt.Errorf("new binary is empty")
	}

	backupPath := selfPath + ".old"
	_ = os.Remove(backupPath)

	if err := os.Rename(selfPath, backupPath); err != nil {
		return fmt.Errorf("rename old binary: %w", err)
	}

	if err := os.WriteFile(selfPath, newData, 0755); err != nil {
		_ = os.Rename(backupPath, selfPath)
		return fmt.Errorf("write new binary: %w", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(selfPath, 0755); err != nil {
			return fmt.Errorf("chmod: %w", err)
		}
	}

	// Keep .old — cleaned on next startup via CleanOldBackup()
	u.logger.Printf("[Update] binary replaced (backup at %s)", backupPath)
	return nil
}

func (u *Updater) replaceAppBundle(selfPath, newAppBundle string) error {
	appPath := findAppBundle(selfPath)
	if appPath == "" {
		return fmt.Errorf("running outside .app bundle, cannot replace bundle")
	}

	backupPath := appPath + ".old"
	_ = os.RemoveAll(backupPath)

	if err := os.Rename(appPath, backupPath); err != nil {
		return fmt.Errorf("rename old .app: %w", err)
	}

	if err := copyDir(newAppBundle, appPath); err != nil {
		_ = os.RemoveAll(appPath)
		_ = os.Rename(backupPath, appPath)
		return fmt.Errorf("copy new .app: %w", err)
	}

	u.logger.Printf("[Update] .app bundle replaced (backup at %s)", backupPath)
	return nil
}

// --- Restart ---

func (u *Updater) restart(selfPath string) error {
	u.logger.Printf("[Update] restarting: %s", selfPath)

	// macOS: use `open` to launch .app through LaunchServices
	if runtime.GOOS == "darwin" {
		appPath := findAppBundle(selfPath)
		if appPath != "" {
			u.logger.Printf("[Update] restarting via open: %s", appPath)
			cmd := exec.Command("open", appPath)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				u.logger.Printf("[Update] open failed, falling back to direct exec: %v", err)
				return u.restartDirect(selfPath)
			}
			os.Exit(0)
		}
	}

	return u.restartDirect(selfPath)
}

func (u *Updater) restartDirect(selfPath string) error {
	cmd := exec.Command(selfPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start new process: %w", err)
	}

	os.Exit(0)
	return nil
}

// --- Helpers ---

func findAppBundle(exePath string) string {
	idx := strings.Index(exePath, ".app/Contents/MacOS/")
	if idx < 0 {
		return ""
	}
	return exePath[:idx+4]
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, info.Mode())
	})
}

func (u *Updater) emitProgress(phase string, progress float64, message string) {
	wailsrt.EventsEmit(u.ctx, "update:progress", map[string]interface{}{
		"phase":    phase,
		"progress": progress,
		"message":  message,
	})
}

type downloadProgress struct {
	total   int64
	written int64
	emitter func(float64)
}

func (p *downloadProgress) Write(data []byte) (int, error) {
	n := len(data)
	p.written += int64(n)
	if p.total > 0 {
		pct := float64(p.written) / float64(p.total) * 50
		p.emitter(pct)
	}
	return n, nil
}
