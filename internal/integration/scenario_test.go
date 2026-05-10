//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"dfcleaner/internal/cleaner"
	"dfcleaner/internal/config"
	"dfcleaner/internal/scheduler"
	"dfcleaner/internal/store"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	db.AutoMigrate(&store.ScanHistory{}, &store.CleanupLog{}, &store.CleanableItemDB{})
	return db
}

// --- Scenario 1: Cleanup full flow (no scanner, simulates AI marking) ---

func TestScenarioCleanupFlow(t *testing.T) {
	dir := t.TempDir()

	// Create test structure: cleanable items + safe items
	os.MkdirAll(filepath.Join(dir, "project", "node_modules", "pkg"), 0755)
	os.WriteFile(filepath.Join(dir, "project", "node_modules", "pkg", "index.js"), make([]byte, 1024*100), 0644)
	os.MkdirAll(filepath.Join(dir, "project", "dist"), 0755)
	os.WriteFile(filepath.Join(dir, "project", "dist", "bundle.js"), make([]byte, 1024*50), 0644)
	os.MkdirAll(filepath.Join(dir, "project", ".cache"), 0755)
	os.WriteFile(filepath.Join(dir, "project", ".cache", "data.json"), make([]byte, 1024*10), 0644)
	os.MkdirAll(filepath.Join(dir, "project", "src"), 0755)
	os.WriteFile(filepath.Join(dir, "project", "src", "main.ts"), []byte("console.log('hello')"), 0644)
	os.MkdirAll(filepath.Join(dir, "logs"), 0755)
	os.WriteFile(filepath.Join(dir, "logs", "app.log"), make([]byte, 1024*200), 0644)
	os.MkdirAll(filepath.Join(dir, "temp"), 0755)
	os.WriteFile(filepath.Join(dir, "temp", "download.tmp"), make([]byte, 1024*300), 0644)

	// Step 1: Simulate AI identifying cleanable items
	cleanablePaths := []string{
		filepath.Join(dir, "project", "node_modules"),
		filepath.Join(dir, "project", "dist"),
		filepath.Join(dir, "project", ".cache"),
		filepath.Join(dir, "logs"),
		filepath.Join(dir, "temp"),
	}

	// Step 2: Cleanup with logging
	db := newTestDB(t)
	s := store.New(db)
	c := cleaner.New(s)

	items := make([]cleaner.CleanupItem, len(cleanablePaths))
	for i, p := range cleanablePaths {
		items[i] = cleaner.CleanupItem{Path: p, Operation: cleaner.OpDelete}
	}

	results, err := c.Cleanup(items)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	freed := int64(0)
	for _, r := range results {
		if !r.Success {
			t.Errorf("cleanup failed for %s: %s", r.Path, r.Error)
		} else {
			freed += r.FreedBytes
			t.Logf("cleaned %s: freed %d bytes", r.Path, r.FreedBytes)
		}
	}

	if freed == 0 {
		t.Error("expected some bytes freed")
	}

	// Step 3: Verify cleanable items are gone
	for _, p := range cleanablePaths {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expected %s to be deleted", p)
		}
	}

	// Step 4: Verify safe items still exist
	safePaths := []string{
		filepath.Join(dir, "project", "src", "main.ts"),
	}
	for _, p := range safePaths {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s to still exist: %v", p, err)
		}
	}

	// Step 5: Verify cleanup logs persisted
	logs := s.GetRecentCleanups(10)
	if len(logs) != len(cleanablePaths) {
		t.Errorf("expected %d logs, got %d", len(cleanablePaths), len(logs))
	}

	t.Logf("cleanup scenario complete: freed %d bytes, %d logs, safe items preserved", freed, len(logs))
}

// --- Scenario 2: Scheduler → Scan → Cleanup ---

func TestScenarioSchedulerAutoCleanup(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "cache"), 0755)
	os.WriteFile(filepath.Join(dir, "cache", "stale.dat"), make([]byte, 1024), 0644)

	// Setup scheduler with real DB store
	db := newTestDB(t)
	scheduler.MigrateDB(db)
	dbStore := scheduler.NewDBStore(db)

	s := scheduler.New(dbStore, nil)

	var scanCalled bool
	var capturedPath string
	s.SetScanHandler(func(_ context.Context, scanPath string, _ scheduler.RiskLevel) error {
		scanCalled = true
		capturedPath = scanPath

		// Simulate scan + cleanup
		c := cleaner.New(nil)
		cachePath := filepath.Join(scanPath, "cache")
		if _, err := os.Stat(cachePath); err == nil {
			c.Cleanup([]cleaner.CleanupItem{{Path: cachePath, Operation: cleaner.OpDelete}})
		}
		return nil
	})

	// Create schedule
	cfg := &scheduler.ScheduleConfig{
		ID:          "auto-test",
		CronExpr:    "0 3 * * *",
		ScanPath:    dir,
		MaxAutoRisk: scheduler.RiskSafe,
		Enabled:     true,
	}
	if err := s.CreateSchedule(cfg); err != nil {
		t.Fatalf("create schedule: %v", err)
	}

	// Start scheduler
	s.Start()
	defer s.Stop()

	// Manually trigger the job by calling executeJob via exported helper
	s.ExecuteJobForTest(cfg)

	if !scanCalled {
		t.Fatal("expected scan handler to be called")
	}
	if capturedPath != dir {
		t.Errorf("expected scan path %s, got %s", dir, capturedPath)
	}

	// Verify cleanup happened
	cachePath := filepath.Join(dir, "cache")
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Error("expected cache dir to be cleaned")
	}

	// Verify schedule persisted
	schedules, _ := s.GetSchedules()
	if len(schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(schedules))
	}
	if schedules[0].LastRun.IsZero() {
		t.Error("expected LastRun to be set")
	}

	t.Log("scheduler scenario complete: auto-cleanup executed")
}

// --- Scenario 3: Settings persistence across restarts ---

func TestScenarioSettingsPersistence(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// First session: set settings
	m1, err := config.NewManager()
	if err != nil {
		t.Fatalf("new manager 1: %v", err)
	}
	m1.SetTheme("dark")
	m1.SetLanguage("zh")
	m1.SetSafeMode(true)
	m1.SaveLLM(config.LLMEntry{
		Provider:  "openai",
		Endpoint:  "https://api.openai.com/v1",
		APIKey:    "test-key-encrypted",
		ModelName: "gpt-4o",
		IsActive:  true,
	})

	// Second session: reload and verify
	m2, err := config.NewManager()
	if err != nil {
		t.Fatalf("new manager 2: %v", err)
	}
	if m2.GetTheme() != "dark" {
		t.Errorf("theme = %q, want dark", m2.GetTheme())
	}
	if m2.GetLanguage() != "zh" {
		t.Errorf("language = %q, want zh", m2.GetLanguage())
	}
	if !m2.GetSafeMode() {
		t.Error("safe mode should be true")
	}
	active := m2.GetActiveLLM()
	if active == nil {
		t.Fatal("expected active LLM config")
	}
	if active.ModelName != "gpt-4o" {
		t.Errorf("model = %q, want gpt-4o", active.ModelName)
	}

	// Third session: update and verify
	m2.SetTheme("light")
	m2.SaveLLM(config.LLMEntry{
		Provider:  "ollama",
		Endpoint:  "http://localhost:11434",
		APIKey:    "",
		ModelName: "llama3",
		IsActive:  true,
	})

	m3, err := config.NewManager()
	if err != nil {
		t.Fatalf("new manager 3: %v", err)
	}
	if m3.GetTheme() != "light" {
		t.Errorf("theme after update = %q, want light", m3.GetTheme())
	}
	active3 := m3.GetActiveLLM()
	if active3 == nil || active3.Provider != "ollama" {
		t.Errorf("expected ollama as active, got %v", active3)
	}

	t.Log("settings persistence scenario complete")
}

// --- Scenario 4: Cleanup log persistence ---

func TestScenarioCleanupLogging(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "junk.tmp"), make([]byte, 1024), 0644)
	os.WriteFile(filepath.Join(dir, "cache.dat"), make([]byte, 2048), 0644)

	// Setup store with real DB
	db := newTestDB(t)
	s := store.New(db)

	c := cleaner.New(s)

	items := []cleaner.CleanupItem{
		{Path: filepath.Join(dir, "junk.tmp"), Operation: cleaner.OpDelete},
		{Path: filepath.Join(dir, "cache.dat"), Operation: cleaner.OpDelete},
	}

	results, _ := c.Cleanup(items)
	for _, r := range results {
		if !r.Success {
			t.Errorf("cleanup failed: %s", r.Error)
		}
	}

	// Verify cleanup logs
	logs := s.GetRecentCleanups(10)
	if len(logs) != 2 {
		t.Fatalf("expected 2 cleanup logs, got %d", len(logs))
	}

	totalFreed := int64(0)
	for _, l := range logs {
		totalFreed += l.FreedBytes
		t.Logf("log: %s op=%s freed=%d", l.FilePath, l.Operation, l.FreedBytes)
	}
	if totalFreed == 0 {
		t.Error("expected bytes freed in logs")
	}

	// Verify files are gone
	for _, item := range items {
		if _, err := os.Stat(item.Path); !os.IsNotExist(err) {
			t.Errorf("expected %s deleted", item.Path)
		}
	}

	t.Logf("cleanup logging scenario complete: %d bytes freed, %d logs", totalFreed, len(logs))
}

// --- Scenario 5: Scheduler CRUD lifecycle ---

func TestScenarioSchedulerLifecycle(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	scheduler.MigrateDB(db)
	dbStore := scheduler.NewDBStore(db)

	s := scheduler.New(dbStore, nil)
	s.SetScanHandler(func(_ context.Context, _ string, _ scheduler.RiskLevel) error {
		return nil
	})

	// Create
	s1 := &scheduler.ScheduleConfig{
		ID: "life-1", CronExpr: "0 3 * * *", ScanPath: "/tmp", MaxAutoRisk: scheduler.RiskSafe, Enabled: true,
	}
	s.CreateSchedule(s1)

	s2 := &scheduler.ScheduleConfig{
		ID: "life-2", CronExpr: "@daily", ScanPath: "/var", MaxAutoRisk: scheduler.RiskCaution, Enabled: true,
	}
	s.CreateSchedule(s2)

	// Start
	s.Start()
	defer s.Stop()

	schedules, _ := s.GetSchedules()
	if len(schedules) != 2 {
		t.Fatalf("expected 2 schedules, got %d", len(schedules))
	}

	// Toggle
	s.ToggleSchedule("life-1", false)
	schedules, _ = s.GetSchedules()
	for _, sc := range schedules {
		if sc.ID == "life-1" && sc.Enabled {
			t.Error("life-1 should be disabled")
		}
	}

	// Update
	s2.ScanPath = "/home"
	s2.CronExpr = "0 */6 * * *"
	s.UpdateSchedule(s2)
	schedules, _ = s.GetSchedules()
	for _, sc := range schedules {
		if sc.ID == "life-2" && sc.ScanPath != "/home" {
			t.Errorf("life-2 scan path should be /home, got %s", sc.ScanPath)
		}
	}

	// Delete
	s.DeleteSchedule("life-1")
	schedules, _ = s.GetSchedules()
	if len(schedules) != 1 || schedules[0].ID != "life-2" {
		t.Errorf("expected 1 schedule (life-2), got %v", schedules)
	}

	t.Log("scheduler lifecycle scenario complete")
}

// --- Scenario 6: Deep nested cleanup ---

func TestScenarioDeepNestedCleanup(t *testing.T) {
	dir := t.TempDir()

	// Create deep structure: 5 levels deep
	base := filepath.Join(dir, "a", "b", "c", "d", "e")
	os.MkdirAll(base, 0755)
	os.WriteFile(filepath.Join(base, "deep.log"), make([]byte, 1024), 0644)
	os.MkdirAll(filepath.Join(base, "node_modules", "lodash"), 0755)
	os.WriteFile(filepath.Join(base, "node_modules", "lodash", "index.js"), make([]byte, 1024*100), 0644)

	// Cleanup nested node_modules
	c := cleaner.New(nil)
	results, _ := c.Cleanup([]cleaner.CleanupItem{
		{Path: filepath.Join(base, "node_modules"), Operation: cleaner.OpDelete},
	})

	if !results[0].Success {
		t.Errorf("cleanup failed: %s", results[0].Error)
	}
	if results[0].FreedBytes < 1024*100 {
		t.Errorf("expected at least 100KB freed, got %d", results[0].FreedBytes)
	}

	// Verify deep.log still exists (wasn't cleaned)
	if _, err := os.Stat(filepath.Join(base, "deep.log")); err != nil {
		t.Error("deep.log should still exist")
	}
	// Verify node_modules is gone
	if _, err := os.Stat(filepath.Join(base, "node_modules")); !os.IsNotExist(err) {
		t.Error("node_modules should be deleted")
	}

	t.Logf("deep nested cleanup: cleaned %d bytes", results[0].FreedBytes)
}
