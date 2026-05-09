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

func (u *Updater) CheckForUpdate(currentVer string) (*UpdateInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/0verL1nk/DFCleaner/releases/latest")
	if err != nil {
		return nil, fmt.Errorf("check update: %w", err)
	}
	defer resp.Body.Close()

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
	info.HasUpdate = latest != "" && latest != current && latest > current

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

	u.emitProgress("extracting", 60, "Extracting...")
	binaryPath, err := u.extract(archivePath, tmpDir)
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	u.emitProgress("replacing", 80, "Replacing binary...")
	if err := u.replace(selfPath, binaryPath); err != nil {
		return fmt.Errorf("replace: %w", err)
	}

	u.emitProgress("restarting", 100, "Restarting...")
	wailsrt.EventsEmit(u.ctx, "update:complete")

	time.Sleep(500 * time.Millisecond)
	return u.restart(selfPath)
}

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

func (u *Updater) download(url, dest string) error {
	u.emitProgress("downloading", 0, "Downloading...")

	resp, err := http.Get(url)
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
		outF, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(outF, tr); err != nil {
			outF.Close()
			return "", err
		}
		outF.Close()

		if !strings.Contains(hdr.Name, ".app/") || strings.HasSuffix(hdr.Name, "DFCleaner") {
			u.logger.Printf("[Update] extracted binary: %s", outPath)
			return outPath, nil
		}
	}

	return "", fmt.Errorf("no binary found in archive")
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
		outF, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
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

func (u *Updater) replace(selfPath, newBinary string) error {
	newData, err := os.ReadFile(newBinary)
	if err != nil {
		return fmt.Errorf("read new binary: %w", err)
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

	_ = os.Remove(backupPath)
	u.logger.Printf("[Update] binary replaced successfully")
	return nil
}

func (u *Updater) restart(selfPath string) error {
	u.logger.Printf("[Update] restarting: %s", selfPath)

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
