package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"dfcleaner/internal/analyzer"
	"dfcleaner/internal/cleaner"
	"dfcleaner/internal/config"
	"dfcleaner/internal/llm"
	"dfcleaner/internal/platform"
	"dfcleaner/internal/scanner"
	"dfcleaner/internal/scheduler"
	"dfcleaner/internal/store"
	"dfcleaner/internal/updater"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Version is set via -ldflags at build time. Defaults to "dev".
var Version = "dev"

type App struct {
	ctx      context.Context
	store    *store.Store
	cfg      *config.Manager
	scanner  *scanner.Scanner
	analyzer *analyzer.Analyzer
	cleaner   *cleaner.Cleaner
	llm       *llm.Provider
	updater   *updater.Updater
	sched     *scheduler.Scheduler
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

	updater.CleanOldBackup()

	// Initialize TOML config (persists across reinstalls)
	cfg, err := config.NewManager()
	if err != nil {
		a.logger.Printf("WARN: config init failed: %v", err)
	} else {
		a.cfg = cfg
	}

	// Initialize SQLite for operational data (scan results, cleanup logs)
	db, err := store.InitDB("dfcleaner.db")
	if err != nil {
		a.logger.Printf("FATAL: failed to init db: %v", err)
		panic(err)
	}

	a.store = store.New(db)

	// Migrate settings from SQLite to TOML if config file was just created
	if a.cfg != nil {
		a.migrateFromStore()
	}

	a.scanner = scanner.New(ctx)
	a.llm = llm.New(a.cfg)
	a.analyzer = analyzer.New(ctx, a.llm, a.scanner, a.store)
	a.cleaner = cleaner.New(a.store)
	a.updater = updater.New(ctx, a.logger)

	// Initialize scheduler
	if err := scheduler.MigrateDB(db); err != nil {
		a.logger.Printf("WARN: scheduler migration failed: %v", err)
	}
	schedStore := scheduler.NewDBStore(db)
	a.sched = scheduler.New(schedStore, a.logger)
	a.sched.SetScanHandler(func(scanCtx context.Context, scanPath string, maxAutoRisk scheduler.RiskLevel) error {
		return a.scheduledScan(scanCtx, scanPath, maxAutoRisk)
	})
	if err := a.sched.Start(); err != nil {
		a.logger.Printf("WARN: scheduler start failed: %v", err)
	}

	a.logger.Println("startup complete")

	go a.startTray()
	go a.autoCheckUpdate()
}

// migrateFromStore copies settings from SQLite to TOML on first run with new config.
// Only runs if TOML config has no LLM entries and SQLite has some.
func (a *App) migrateFromStore() {
	if len(a.cfg.GetAllLLMs()) > 0 {
		return
	}

	llmCfg := a.store.GetActiveLLMConfig()
	if llmCfg == nil {
		// Also migrate theme/language
		theme := a.store.GetSetting("theme")
		lang := a.store.GetSetting("language")
		if theme != "" || lang != "" {
			_ = a.cfg.ImportFromStore(
				a.store.GetSetting("theme"),
				a.store.GetSetting("language"),
				nil,
			)
		}
		return
	}

	entries := []config.LLMEntry{
		{
			Provider:  llmCfg.Provider,
			Endpoint:  llmCfg.Endpoint,
			APIKey:    llmCfg.APIKey, // already encrypted in store
			ModelName: llmCfg.ModelName,
			IsActive:  llmCfg.IsActive,
		},
	}

	_ = a.cfg.ImportFromStore(
		a.store.GetSetting("theme"),
		a.store.GetSetting("language"),
		entries,
	)
	a.logger.Println("migrated settings from SQLite to TOML")
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
	total := len(items)
	results := make([]cleaner.CleanupResult, 0, total)
	for i, item := range items {
		wailsrt.EventsEmit(a.ctx, "cleanup:progress", map[string]any{
			"current": i + 1,
			"total":   total,
			"path":    item.Path,
		})
		result := a.cleaner.CleanupOne(item)
		results = append(results, result)
	}
	return results, nil
}

func (a *App) TestLLMConnection(config llm.LLMConfig) (*llm.ConnectionTestResult, error) {
	return a.llm.TestConnection(a.ctx, config)
}

func (a *App) SaveLLMConfig(config llm.LLMConfig) error {
	return a.llm.SaveConfig(&config)
}

func (a *App) GetLLMConfigs() []config.LLMEntry {
	return a.cfg.GetAllLLMs()
}

func (a *App) GetActiveLLMConfig() (*llm.LLMConfig, error) {
	return a.llm.GetActiveConfig()
}

func (a *App) GetSettings() map[string]string {
	return map[string]string{
		"theme":     a.cfg.GetTheme(),
		"language":  a.cfg.GetLanguage(),
		"safe_mode": fmt.Sprintf("%v", a.cfg.GetSafeMode()),
	}
}

func (a *App) SetSetting(key, value string) error {
	switch key {
	case "theme":
		return a.cfg.SetTheme(value)
	case "language":
		return a.cfg.SetLanguage(value)
	case "safe_mode":
		return a.cfg.SetSafeMode(value == "true")
	default:
		return nil
	}
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

// --- Scheduler bindings ---

func (a *App) GetSchedules() []scheduler.ScheduleConfig {
	if a.sched == nil {
		return nil
	}
	cfgs, err := a.sched.GetSchedules()
	if err != nil {
		return nil
	}
	return cfgs
}

func (a *App) SaveSchedule(cfg scheduler.ScheduleConfig) error {
	if a.sched == nil {
		return fmt.Errorf("scheduler not initialized")
	}
	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("sched_%d", time.Now().UnixMilli())
	}
	if cfg.Enabled {
		return a.sched.CreateSchedule(&cfg)
	}
	return a.sched.UpdateSchedule(&cfg)
}

func (a *App) DeleteSchedule(id string) error {
	if a.sched == nil {
		return fmt.Errorf("scheduler not initialized")
	}
	return a.sched.DeleteSchedule(id)
}

func (a *App) ToggleSchedule(id string, enabled bool) error {
	if a.sched == nil {
		return fmt.Errorf("scheduler not initialized")
	}
	return a.sched.ToggleSchedule(id, enabled)
}

// scheduledScan runs a Smart Scan triggered by the scheduler.
func (a *App) scheduledScan(ctx context.Context, scanPath string, maxAutoRisk scheduler.RiskLevel) error {
	a.logger.Printf("scheduled scan: path=%s maxRisk=%s", scanPath, maxAutoRisk)

	// Run SmartScan synchronously for scheduled runs
	a.SmartScan(scanPath, scanner.ScanOptions{})

	// Wait for scan to complete (poll)
	for i := 0; i < 120; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		time.Sleep(5 * time.Second)
		items := a.store.GetCleanableItems(scanPath)
		if len(items) > 0 {
			break
		}
	}

	// Auto-clean items at or below the risk threshold
	items := a.store.GetCleanableItems(scanPath)
	var toClean []cleaner.CleanupItem
	for _, item := range items {
		if item.RiskLevel == "safe" || (maxAutoRisk == scheduler.RiskCaution && item.RiskLevel == "caution") {
			op := cleaner.OpDelete
			if a.cfg.GetSafeMode() {
				op = cleaner.OpTrash
			}
			toClean = append(toClean, cleaner.CleanupItem{
				Path:      item.Path,
				Operation: op,
			})
		}
	}

	var freed int64
	var failed int
	if len(toClean) > 0 {
		results, err := a.cleaner.Cleanup(toClean)
		if err != nil {
			return fmt.Errorf("scheduled cleanup: %w", err)
		}
		for _, r := range results {
			if r.Success {
				freed += r.FreedBytes
			} else {
				failed++
			}
		}
		a.logger.Printf("scheduled scan complete: cleaned=%d freed=%d failed=%d", len(toClean)-failed, freed, failed)
	}

	wailsrt.EventsEmit(a.ctx, "scheduler:complete", map[string]any{
		"scanPath": scanPath,
		"cleaned":  len(toClean) - failed,
		"freed":    freed,
		"failed":   failed,
	})
	return nil
}

func (a *App) GetVersion() string {
	return Version
}

func (a *App) GetCleanableItems(scanPath string) []store.CleanableItemDB {
	return a.store.GetCleanableItems(scanPath)
}

func (a *App) GetCleanableSize(scanPath string) int64 {
	return a.store.GetCleanableSize(scanPath)
}

func (a *App) ClearCleanableItems(scanPath string) error {
	return a.store.ClearCleanableItems(scanPath)
}

func (a *App) CheckForUpdate() (*updater.UpdateInfo, error) {
	return a.updater.CheckForUpdate(Version)
}

func (a *App) PerformUpdate(info updater.UpdateInfo) error {
	return a.updater.PerformUpdate(&info)
}

func (a *App) autoCheckUpdate() {
	info, err := a.updater.CheckForUpdate(Version)
	if err != nil {
		a.logger.Printf("[AutoUpdate] check failed: %v", err)
		return
	}
	if info.HasUpdate {
		a.logger.Printf("[AutoUpdate] new version available: %s", info.LatestVer)
		wailsrt.EventsEmit(a.ctx, "update:available", map[string]any{
			"hasUpdate":   true,
			"latestVer":  info.LatestVer,
			"downloadUrl": info.DownloadURL,
		})
	} else {
		a.logger.Printf("[AutoUpdate] up to date")
	}
}
