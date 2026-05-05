package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dfcleaner/internal/analyzer"
	"dfcleaner/internal/cleaner"
	"dfcleaner/internal/llm"
	"dfcleaner/internal/platform"
	"dfcleaner/internal/scanner"
	"dfcleaner/internal/store"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Version is set via -ldflags at build time. Defaults to "dev".
var Version = "dev"

type App struct {
	ctx      context.Context
	store    *store.Store
	scanner  *scanner.Scanner
	analyzer *analyzer.Analyzer
	cleaner  *cleaner.Cleaner
	llm      *llm.Provider
	logger   *log.Logger
	logFile  *os.File

	smartScanCancel context.CancelFunc
	quitting       bool
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	execPath, _ := os.Executable()
	logDir := filepath.Join(filepath.Dir(execPath), "logs")
	os.MkdirAll(logDir, 0755)

	logPath := filepath.Join(logDir, "dfcleaner.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		a.logFile = f
		a.logger = log.New(f, "", log.LstdFlags|log.Lshortfile)
	} else {
		a.logger = log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile)
	}

	a.logger.Println("=== DFCleaner starting ===")

	db, err := store.InitDB("dfcleaner.db")
	if err != nil {
		a.logger.Printf("FATAL: failed to init db: %v", err)
		panic(err)
	}

	a.store = store.New(db)
	a.scanner = scanner.New(ctx)
	a.llm = llm.New(a.store)
	a.analyzer = analyzer.New(ctx, a.llm, a.scanner, a.store)
	a.cleaner = cleaner.New(a.store)

	a.logger.Println("startup complete")

	go a.startTray()
}

func (a *App) shutdown(ctx context.Context) {
	a.logger.Println("=== DFCleaner shutting down ===")
	if a.logFile != nil {
		a.logFile.Close()
	}
}

func (a *App) Scan(path string, opts scanner.ScanOptions) (*scanner.ScanResult, error) {
	a.logger.Printf("Scan: path=%s maxDepth=%d", path, opts.MaxDepth)
	result, err := a.scanner.Scan(path, opts)
	if err != nil {
		a.logger.Printf("Scan error: %v", err)
	}
	return result, err
}

func (a *App) CancelScan() {
	a.scanner.Cancel()
}

func (a *App) AnalyzeFiles(entries []scanner.FileEntry) error {
	a.logger.Printf("AnalyzeFiles: %d entries", len(entries))
	results, err := a.analyzer.BatchAnalyze(a.ctx, entries)
	if err != nil {
		a.logger.Printf("AnalyzeFiles error: %v", err)
		wailsrt.EventsEmit(a.ctx, "analysis:error", err.Error())
		return err
	}
	wailsrt.EventsEmit(a.ctx, "analysis:complete", results)
	return nil
}

func (a *App) CancelAnalysis() {
	a.analyzer.Cancel()
}

func (a *App) Cleanup(items []cleaner.CleanupItem) ([]cleaner.CleanupResult, error) {
	a.logger.Printf("Cleanup: %d items", len(items))
	results, err := a.cleaner.Cleanup(items)
	if err != nil {
		a.logger.Printf("Cleanup error: %v", err)
	}
	return results, err
}

func (a *App) TestLLMConnection(config llm.LLMConfig) (*llm.ConnectionTestResult, error) {
	return a.llm.TestConnection(a.ctx, config)
}

func (a *App) SaveLLMConfig(config llm.LLMConfig) error {
	return a.llm.SaveConfig(&config)
}

func (a *App) GetLLMConfigs() []store.LLMConfig {
	return a.store.GetLLMConfigs()
}

func (a *App) GetActiveLLMConfig() (*llm.LLMConfig, error) {
	return a.llm.GetActiveConfig()
}

func (a *App) GetSettings() map[string]string {
	return map[string]string{
		"theme":           a.store.GetSetting("theme"),
		"language":        a.store.GetSetting("language"),
		"content_preview": a.store.GetSetting("content_preview"),
	}
}

func (a *App) SetSetting(key, value string) error {
	return a.store.SetSetting(key, value)
}

func (a *App) GetRecentCleanups(limit int) []store.CleanupLog {
	if limit <= 0 {
		limit = 5
	}
	return a.store.GetRecentCleanups(limit)
}

func (a *App) GetSystemDrives() []platform.DriveInfo {
	return platform.GetDrives()
}

func (a *App) GetQuickTargets() []platform.QuickTarget {
	return platform.GetQuickTargets()
}

func (a *App) GetVersion() string {
	return Version
}

func (a *App) GetCleanableItems(scanPath string) []store.CleanableItemDB {
	return a.store.GetCleanableItems(scanPath)
}

func (a *App) ClearCleanableItems(scanPath string) error {
	return a.store.ClearCleanableItems(scanPath)
}

type UpdateInfo struct {
	HasUpdate    bool   `json:"hasUpdate"`
	CurrentVer   string `json:"currentVer"`
	LatestVer    string `json:"latestVer"`
	DownloadURL  string `json:"downloadUrl"`
	ReleaseNotes string `json:"releaseNotes"`
}

func (a *App) CheckForUpdate() (*UpdateInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/0verL1nk/DFCleaner/releases/latest")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	info := &UpdateInfo{
		CurrentVer:   Version,
		LatestVer:    release.TagName,
		ReleaseNotes: release.Body,
	}

	// Compare versions (strip 'v' prefix)
	current := strings.TrimPrefix(Version, "v")
	latest := strings.TrimPrefix(release.TagName, "v")
	info.HasUpdate = latest != "" && latest != current && latest > current

	// Find matching download URL for current platform
	for _, asset := range release.Assets {
		info.DownloadURL = asset.URL
		break
	}
	if info.DownloadURL == "" {
		info.DownloadURL = release.HTMLURL
	}

	a.logger.Printf("[Update] current=%s latest=%s hasUpdate=%v", Version, release.TagName, info.HasUpdate)
	return info, nil
}
