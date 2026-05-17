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
	u.logger.Printf("[Update] checking for updates, current=%s", currentVer)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/0verL1nk/DFCleaner/releases/latest")
	if err != nil {
		u.logger.Printf("[Update] check failed: %v", err)
		return nil, fmt.Errorf("check update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		u.logger.Printf("[Update] github API returned %s", resp.Status)
		return nil, fmt.Errorf("github API returned %s", resp.Status)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		u.logger.Printf("[Update] decode failed: %v", err)
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

	switch {
	case strings.Contains(info.DownloadURL, "-installer"):
		return u.performInstallerUpdate(info)
	case strings.HasSuffix(info.DownloadURL, ".dmg"):
		return u.performDMGUpdate(info)
	default:
		return u.performPortableUpdate(info)
	}
}

// performPortableUpdate downloads a portable archive, extracts it, and
// replaces the running binary (Linux and fallback for other platforms).
func (u *Updater) performPortableUpdate(info *UpdateInfo) error {
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	selfPath, err = filepath.EvalSymlinks(selfPath)
	if err != nil {
		return fmt.Errorf("resolve symlink: %w", err)
	}

	u.logger.Printf("[Update] starting portable update: self=%s url=%s", selfPath, info.DownloadURL)

	tmpDir, err := os.MkdirTemp("", "dfcleaner-update-*")
	if err != nil {
		u.logger.Printf("[Update] ERROR create temp dir: %v", err)
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	ext := ".tar.gz"
	if strings.HasSuffix(info.DownloadURL, ".zip") {
		ext = ".zip"
	}
	archivePath := filepath.Join(tmpDir, "update-archive"+ext)
	if err := u.download(info.DownloadURL, archivePath); err != nil {
		u.logger.Printf("[Update] ERROR download: %v", err)
		return fmt.Errorf("download: %w", err)
	}

	stat, err := os.Stat(archivePath)
	if err != nil || stat.Size() == 0 {
		u.logger.Printf("[Update] ERROR downloaded file empty or missing (size=%d err=%v)", stat.Size(), err)
		return fmt.Errorf("downloaded file is empty or missing")
	}
	u.logger.Printf("[Update] download complete: %d bytes", stat.Size())

	u.emitProgress("extracting", 60, "Extracting...")
	extractedPath, err := u.extract(archivePath, tmpDir)
	if err != nil {
		u.logger.Printf("[Update] ERROR extract: %v", err)
		return fmt.Errorf("extract: %w", err)
	}

	extStat, _ := os.Stat(extractedPath)
	if extStat != nil {
		u.logger.Printf("[Update] extracted: %s (size=%d isDir=%v)", extractedPath, extStat.Size(), extStat.IsDir())
	}

	u.emitProgress("replacing", 80, "Replacing...")
	if err := u.replace(selfPath, extractedPath); err != nil {
		u.logger.Printf("[Update] ERROR replace: %v", err)
		return fmt.Errorf("replace: %w", err)
	}

	u.emitProgress("restarting", 100, "Restarting...")
	wailsrt.EventsEmit(u.ctx, "update:complete")

	time.Sleep(500 * time.Millisecond)
	u.logger.Printf("[Update] update succeeded, restarting...")
	return u.restart(selfPath)
}

// performInstallerUpdate downloads a NSIS installer and runs it silently.
// The installer handles file replacement and app restart on Windows.
func (u *Updater) performInstallerUpdate(info *UpdateInfo) error {
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	selfPath, err = filepath.EvalSymlinks(selfPath)
	if err != nil {
		return fmt.Errorf("resolve symlink: %w", err)
	}

	u.logger.Printf("[Update] starting installer update: self=%s url=%s", selfPath, info.DownloadURL)

	tmpDir, err := os.MkdirTemp("", "dfcleaner-update-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}

	installerPath := filepath.Join(tmpDir, "DFCleaner-installer.exe")
	if err := u.download(info.DownloadURL, installerPath); err != nil {
		os.RemoveAll(tmpDir)
		return fmt.Errorf("download installer: %w", err)
	}

	stat, err := os.Stat(installerPath)
	if err != nil || stat.Size() == 0 {
		os.RemoveAll(tmpDir)
		return fmt.Errorf("downloaded installer is empty or missing")
	}
	u.logger.Printf("[Update] installer downloaded: %d bytes", stat.Size())

	u.emitProgress("installing", 80, "Launching installer...")

	// Launch NSIS installer silently with elevation via PowerShell.
	// /S = silent, /D= = install directory (must be last parameter).
	installDir := filepath.Dir(selfPath)
	psScript := fmt.Sprintf(
		`Start-Process '%s' -ArgumentList '/S','/D=%s' -Verb RunAs`,
		installerPath, installDir,
	)
	cmd := exec.Command("powershell", "-Command", psScript)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(tmpDir)
		return fmt.Errorf("launch installer: %w", err)
	}

	u.logger.Printf("[Update] installer launched, exiting app")

	u.emitProgress("restarting", 100, "Installer running...")
	wailsrt.EventsEmit(u.ctx, "update:complete")
	time.Sleep(1 * time.Second)
	os.Exit(0)
	return nil
}

// performDMGUpdate downloads a macOS DMG, mounts it, replaces the .app bundle.
func (u *Updater) performDMGUpdate(info *UpdateInfo) error {
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	selfPath, err = filepath.EvalSymlinks(selfPath)
	if err != nil {
		return fmt.Errorf("resolve symlink: %w", err)
	}

	appPath := findAppBundle(selfPath)
	if appPath == "" {
		// Not running from .app bundle — fall back to portable
		u.logger.Printf("[Update] not in .app bundle, falling back to portable update")
		return u.performPortableUpdate(info)
	}

	u.logger.Printf("[Update] starting DMG update: app=%s url=%s", appPath, info.DownloadURL)

	tmpDir, err := os.MkdirTemp("", "dfcleaner-update-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	var mountPoint string
	defer func() {
		if mountPoint != "" {
			exec.Command("hdiutil", "detach", mountPoint, "-quiet").Run()
		}
		os.RemoveAll(tmpDir)
	}()

	dmgPath := filepath.Join(tmpDir, "update.dmg")
	if err := u.download(info.DownloadURL, dmgPath); err != nil {
		return fmt.Errorf("download DMG: %w", err)
	}

	stat, _ := os.Stat(dmgPath)
	if stat == nil || stat.Size() == 0 {
		return fmt.Errorf("downloaded DMG is empty or missing")
	}
	u.logger.Printf("[Update] DMG downloaded: %d bytes", stat.Size())

	u.emitProgress("mounting", 60, "Mounting DMG...")

	mountOutput, err := exec.Command("hdiutil", "attach", "-nobrowse", "-readonly", "-quiet", dmgPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount DMG: %w (%s)", err, string(mountOutput))
	}

	for _, line := range strings.Split(string(mountOutput), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "/dev/") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				mountPoint = parts[len(parts)-1]
			}
		}
	}
	if mountPoint == "" {
		mountPoint = "/Volumes/DFCleaner"
	}
	u.logger.Printf("[Update] DMG mounted at: %s", mountPoint)

	u.emitProgress("replacing", 80, "Replacing...")

	entries, _ := os.ReadDir(mountPoint)
	newAppPath := ""
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".app") && e.IsDir() {
			newAppPath = filepath.Join(mountPoint, e.Name())
			break
		}
	}
	if newAppPath == "" {
		return fmt.Errorf("no .app bundle found in DMG")
	}

	if err := u.replaceAppBundle(selfPath, newAppPath); err != nil {
		return fmt.Errorf("replace app: %w", err)
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

	// Windows: prefer NSIS installer
	if goos == "windows" {
		for _, a := range assets {
			if strings.Contains(a.Name, "-installer.") {
				u.logger.Printf("[Update] matched installer asset: %s", a.Name)
				return a.URL
			}
		}
	}

	// macOS: prefer DMG
	if goos == "darwin" {
		for _, a := range assets {
			if strings.HasSuffix(a.Name, ".dmg") {
				u.logger.Printf("[Update] matched DMG asset: %s", a.Name)
				return a.URL
			}
		}
	}

	// Fallback: portable package
	suffix := fmt.Sprintf("-%s-%s-portable.", goos, goarch)
	for _, a := range assets {
		if strings.Contains(a.Name, suffix) {
			u.logger.Printf("[Update] matched portable asset: %s", a.Name)
			return a.URL
		}
	}

	// Last resort: any asset matching the OS
	for _, a := range assets {
		if strings.Contains(a.Name, goos) {
			u.logger.Printf("[Update] fallback asset: %s", a.Name)
			return a.URL
		}
	}
	u.logger.Printf("[Update] WARNING: no platform asset found for %s/%s", goos, goarch)
	return ""
}

// --- Download ---

func (u *Updater) download(url, dest string) error {
	u.emitProgress("downloading", 0, "Downloading...")
	u.logger.Printf("[Update] downloading from %s", url)

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		u.logger.Printf("[Update] download request failed: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		u.logger.Printf("[Update] download HTTP %s (Content-Length=%d)", resp.Status, resp.ContentLength)
		return fmt.Errorf("download failed: HTTP %s", resp.Status)
	}

	f, err := os.Create(dest)
	if err != nil {
		u.logger.Printf("[Update] create dest file failed: %v", err)
		return err
	}
	defer f.Close()

	total := resp.ContentLength
	lastLog := time.Now()
	dp := &downloadProgress{total: total}
	dp.emitter = func(pct float64) {
		u.emitProgress("downloading", pct, fmt.Sprintf("Downloading... %.0f%%", pct))
		if time.Since(lastLog) >= 5*time.Second {
			u.logger.Printf("[Update] download progress: %.0f%% (%d/%d bytes)", pct, dp.written, total)
			lastLog = time.Now()
		}
	}

	_, err = io.Copy(f, io.TeeReader(resp.Body, dp))
	if err != nil {
		u.logger.Printf("[Update] download copy failed after %d bytes: %v", dp.written, err)
	}
	return err
}

// --- Extraction ---

func (u *Updater) extract(archivePath, destDir string) (string, error) {
	if strings.HasSuffix(archivePath, ".zip") {
		u.logger.Printf("[Update] extracting zip: %s", archivePath)
		return u.extractZip(archivePath, destDir)
	}
	u.logger.Printf("[Update] extracting tar.gz: %s", archivePath)
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

	if runtime.GOOS == "darwin" {
		return u.extractMacOSAppBundle(tr, destDir)
	}

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
			u.logger.Printf("[Update] ERROR write tar file %s: %v", outPath, err)
			return "", err
		}

		u.logger.Printf("[Update] extracted binary: %s", outPath)
		return outPath, nil
	}

	return "", fmt.Errorf("no binary found in archive")
}

func (u *Updater) extractMacOSAppBundle(tr *tar.Reader, destDir string) (string, error) {
	var binaryPath string
	fileCount := 0

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
			fileCount++
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
		return "", fmt.Errorf("no DFCleaner binary found in .app bundle (%d files extracted)", fileCount)
	}

	appDir := filepath.Join(destDir, "DFCleaner.app")
	u.logger.Printf("[Update] extracted macOS bundle: %s (%d files, binary: %s)", appDir, fileCount, binaryPath)
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

	u.logger.Printf("[Update] zip contains %d files", len(r.File))

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
			u.logger.Printf("[Update] extracted binary: %s (%d bytes)", outPath, f.FileInfo().Size())
			return outPath, nil
		}
	}

	return "", fmt.Errorf("no binary found in archive (%d entries checked)", len(r.File))
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

	u.logger.Printf("[Update] read new binary: %d bytes", len(newData))

	backupPath := selfPath + ".old"
	_ = os.Remove(backupPath)

	if err := os.Rename(selfPath, backupPath); err != nil {
		u.logger.Printf("[Update] ERROR rename %s -> %s: %v", selfPath, backupPath, err)
		return fmt.Errorf("rename old binary: %w", err)
	}
	u.logger.Printf("[Update] renamed old binary to %s", backupPath)

	if err := os.WriteFile(selfPath, newData, 0755); err != nil {
		u.logger.Printf("[Update] ERROR write new binary, rolling back: %v", err)
		if rollbackErr := os.Rename(backupPath, selfPath); rollbackErr != nil {
			u.logger.Printf("[Update] CRITICAL rollback also failed: %v", rollbackErr)
		}
		return fmt.Errorf("write new binary: %w", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(selfPath, 0755); err != nil {
			return fmt.Errorf("chmod: %w", err)
		}
	}

	u.logger.Printf("[Update] binary replaced successfully (backup at %s)", backupPath)
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
		u.logger.Printf("[Update] ERROR rename %s -> %s: %v", appPath, backupPath, err)
		return fmt.Errorf("rename old .app: %w", err)
	}

	if err := copyDir(newAppBundle, appPath); err != nil {
		u.logger.Printf("[Update] ERROR copy new .app, rolling back: %v", err)
		_ = os.RemoveAll(appPath)
		_ = os.Rename(backupPath, appPath)
		return fmt.Errorf("copy new .app: %w", err)
	}

	u.logger.Printf("[Update] .app bundle replaced successfully (backup at %s)", backupPath)
	return nil
}

// --- Restart ---

func (u *Updater) restart(selfPath string) error {
	u.logger.Printf("[Update] restarting: %s", selfPath)

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
		u.logger.Printf("[Update] ERROR start new process: %v", err)
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
