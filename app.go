package main

import (
	"context"

	"dfcleaner/internal/analyzer"
	"dfcleaner/internal/cleaner"
	"dfcleaner/internal/llm"
	"dfcleaner/internal/platform"
	"dfcleaner/internal/scanner"
	"dfcleaner/internal/store"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx      context.Context
	store    *store.Store
	scanner  *scanner.Scanner
	analyzer *analyzer.Analyzer
	cleaner  *cleaner.Cleaner
	llm      *llm.Provider
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	db, err := store.InitDB("dfcleaner.db")
	if err != nil {
		panic(err)
	}

	a.store = store.New(db)
	a.scanner = scanner.New(ctx)
	a.llm = llm.New(a.store)
	a.analyzer = analyzer.New(ctx, a.llm, a.scanner, a.store)
	a.cleaner = cleaner.New(a.store)
}

// Scan starts a directory scan and returns the result.
func (a *App) Scan(path string, opts scanner.ScanOptions) (*scanner.ScanResult, error) {
	return a.scanner.Scan(path, opts)
}

// CancelScan cancels the current scan operation.
func (a *App) CancelScan() {
	a.scanner.Cancel()
}

// AnalyzeFiles sends scanned file entries to the AI for risk analysis.
// Results are streamed via Wails Events.
func (a *App) AnalyzeFiles(entries []scanner.FileEntry) error {
	results, err := a.analyzer.BatchAnalyze(a.ctx, entries)
	if err != nil {
		wailsrt.EventsEmit(a.ctx, "analysis:error", err.Error())
		return err
	}
	wailsrt.EventsEmit(a.ctx, "analysis:complete", results)
	return nil
}

// CancelAnalysis cancels the current AI analysis.
func (a *App) CancelAnalysis() {
	a.analyzer.Cancel()
}

// Cleanup executes cleanup operations on the given items.
func (a *App) Cleanup(items []cleaner.CleanupItem) ([]cleaner.CleanupResult, error) {
	return a.cleaner.Cleanup(items)
}

// TestLLMConnection tests the LLM configuration including function calling support.
func (a *App) TestLLMConnection(config llm.LLMConfig) (*llm.ConnectionTestResult, error) {
	return a.llm.TestConnection(a.ctx, config)
}

// SaveLLMConfig encrypts and saves an LLM provider configuration.
func (a *App) SaveLLMConfig(config llm.LLMConfig) error {
	return a.llm.SaveConfig(&config)
}

// GetLLMConfigs returns all saved LLM configurations.
func (a *App) GetLLMConfigs() []store.LLMConfig {
	return a.store.GetLLMConfigs()
}

// GetActiveLLMConfig returns the currently active LLM configuration.
func (a *App) GetActiveLLMConfig() (*llm.LLMConfig, error) {
	return a.llm.GetActiveConfig()
}

// GetSettings returns all user settings.
func (a *App) GetSettings() map[string]string {
	return map[string]string{
		"theme":           a.store.GetSetting("theme"),
		"language":        a.store.GetSetting("language"),
		"content_preview": a.store.GetSetting("content_preview"),
	}
}

// SetSetting saves a user setting.
func (a *App) SetSetting(key, value string) error {
	return a.store.SetSetting(key, value)
}

// GetRecentCleanups returns the most recent cleanup operations.
func (a *App) GetRecentCleanups(limit int) []store.CleanupLog {
	if limit <= 0 {
		limit = 5
	}
	return a.store.GetRecentCleanups(limit)
}

// GetSystemDrives returns all available drives/volumes.
func (a *App) GetSystemDrives() []platform.DriveInfo {
	return platform.GetDrives()
}

// GetQuickTargets returns common cleanup targets for the current OS.
func (a *App) GetQuickTargets() []platform.QuickTarget {
	return platform.GetQuickTargets()
}
