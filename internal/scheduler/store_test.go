package scheduler

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *DBStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS schedule_d_bs") })

	if err := MigrateDB(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewDBStore(db)
}

func TestDBStoreSaveAndLoad(t *testing.T) {
	s := newTestDB(t)

	cfg := &ScheduleConfig{
		ID:          "db-1",
		CronExpr:    "0 3 * * *",
		ScanPath:    "/tmp",
		MaxAutoRisk: RiskSafe,
		Enabled:     true,
	}

	if err := s.SaveSchedule(cfg); err != nil {
		t.Fatalf("SaveSchedule: %v", err)
	}

	schedules, err := s.LoadSchedules()
	if err != nil {
		t.Fatalf("LoadSchedules: %v", err)
	}
	if len(schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(schedules))
	}
	if schedules[0].ID != "db-1" {
		t.Errorf("expected ID db-1, got %s", schedules[0].ID)
	}
	if schedules[0].MaxAutoRisk != RiskSafe {
		t.Errorf("expected risk safe, got %s", schedules[0].MaxAutoRisk)
	}
}

func TestDBStoreUpdate(t *testing.T) {
	s := newTestDB(t)

	cfg := &ScheduleConfig{ID: "db-2", CronExpr: "0 3 * * *", ScanPath: "/tmp", Enabled: true}
	s.SaveSchedule(cfg)

	cfg.ScanPath = "/var"
	cfg.Enabled = false
	if err := s.SaveSchedule(cfg); err != nil {
		t.Fatalf("SaveSchedule update: %v", err)
	}

	schedules, _ := s.LoadSchedules()
	if len(schedules) != 1 {
		t.Fatalf("expected 1 schedule after update, got %d", len(schedules))
	}
	if schedules[0].ScanPath != "/var" {
		t.Errorf("expected scan path /var, got %s", schedules[0].ScanPath)
	}
	if schedules[0].Enabled {
		t.Error("expected schedule to be disabled")
	}
}

func TestDBStoreDelete(t *testing.T) {
	s := newTestDB(t)

	cfg := &ScheduleConfig{ID: "db-3", CronExpr: "0 3 * * *", ScanPath: "/tmp", Enabled: true}
	s.SaveSchedule(cfg)

	if err := s.DeleteSchedule("db-3"); err != nil {
		t.Fatalf("DeleteSchedule: %v", err)
	}

	schedules, _ := s.LoadSchedules()
	if len(schedules) != 0 {
		t.Errorf("expected 0 schedules after delete, got %d", len(schedules))
	}
}

func TestDBStoreDeleteNonexistent(t *testing.T) {
	s := newTestDB(t)
	if err := s.DeleteSchedule("nope"); err != nil {
		t.Fatalf("DeleteSchedule nonexistent: %v", err)
	}
}

func TestDBStoreLoadEmpty(t *testing.T) {
	s := newTestDB(t)
	schedules, err := s.LoadSchedules()
	if err != nil {
		t.Fatalf("LoadSchedules empty: %v", err)
	}
	if len(schedules) != 0 {
		t.Errorf("expected 0 schedules from empty db, got %d", len(schedules))
	}
}

func TestMigrateDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := MigrateDB(db); err != nil {
		t.Fatalf("MigrateDB: %v", err)
	}
	if err := MigrateDB(db); err != nil {
		t.Fatalf("MigrateDB idempotent: %v", err)
	}
}

func TestExecuteJob(t *testing.T) {
	called := false
	var gotPath string
	var gotRisk RiskLevel

	store := &memStore{}
	s := New(store, log.New(os.Stderr, "[test] ", 0))
	s.SetScanHandler(func(ctx context.Context, scanPath string, maxAutoRisk RiskLevel) error {
		called = true
		gotPath = scanPath
		gotRisk = maxAutoRisk
		return nil
	})

	cfg := &ScheduleConfig{
		ID:          "exec-1",
		CronExpr:    "0 3 * * *",
		ScanPath:    "/tmp",
		MaxAutoRisk: RiskCaution,
		Enabled:     true,
	}
	store.SaveSchedule(cfg)

	s.executeJob(cfg)

	if !called {
		t.Fatal("expected scan handler to be called")
	}
	if gotPath != "/tmp" {
		t.Errorf("expected path /tmp, got %s", gotPath)
	}
	if gotRisk != RiskCaution {
		t.Errorf("expected risk caution, got %s", gotRisk)
	}

	schedules, _ := store.LoadSchedules()
	if schedules[0].LastRun.IsZero() {
		t.Error("expected LastRun to be set")
	}
}

func TestExecuteJobWithScanError(t *testing.T) {
	store := &memStore{}
	s := New(store, log.New(os.Stderr, "[test] ", 0))
	s.SetScanHandler(func(ctx context.Context, scanPath string, maxAutoRisk RiskLevel) error {
		return fmt.Errorf("scan failed")
	})

	cfg := &ScheduleConfig{ID: "exec-err", CronExpr: "0 3 * * *", ScanPath: "/tmp", Enabled: true}
	store.SaveSchedule(cfg)

	s.executeJob(cfg)

	schedules, _ := store.LoadSchedules()
	if schedules[0].LastRun.IsZero() {
		t.Error("expected LastRun to be set even on error")
	}
}

func TestExecuteJobNoHandler(t *testing.T) {
	store := &memStore{}
	s := New(store, log.New(os.Stderr, "[test] ", 0))

	cfg := &ScheduleConfig{ID: "exec-no", CronExpr: "0 3 * * *", ScanPath: "/tmp", Enabled: true}
	store.SaveSchedule(cfg)

	s.executeJob(cfg)
}

func TestNewWithNilLogger(t *testing.T) {
	s := New(&memStore{}, nil)
	if s.logger == nil {
		t.Error("expected default logger when nil passed")
	}
}
