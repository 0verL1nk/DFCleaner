package scheduler

import (
	"context"
	"log"
	"os"
	"testing"
)

type memStore struct {
	schedules []ScheduleConfig
}

func (m *memStore) LoadSchedules() ([]ScheduleConfig, error) {
	return m.schedules, nil
}

func (m *memStore) SaveSchedule(cfg *ScheduleConfig) error {
	for i := range m.schedules {
		if m.schedules[i].ID == cfg.ID {
			m.schedules[i] = *cfg
			return nil
		}
	}
	m.schedules = append(m.schedules, *cfg)
	return nil
}

func (m *memStore) DeleteSchedule(id string) error {
	for i := range m.schedules {
		if m.schedules[i].ID == id {
			m.schedules = append(m.schedules[:i], m.schedules[i+1:]...)
			return nil
		}
	}
	return nil
}

func newTestScheduler() *Scheduler {
	return New(&memStore{}, log.New(os.Stderr, "[test-sched] ", 0))
}

func TestCreateSchedule(t *testing.T) {
	s := newTestScheduler()

	cfg := &ScheduleConfig{
		ID:          "test-1",
		CronExpr:    "0 3 * * *",
		ScanPath:    "/tmp",
		MaxAutoRisk: RiskSafe,
		Enabled:     true,
	}

	if err := s.CreateSchedule(cfg); err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}

	schedules, _ := s.GetSchedules()
	if len(schedules) != 1 {
		t.Errorf("expected 1 schedule, got %d", len(schedules))
	}
}

func TestCreateScheduleInvalidCron(t *testing.T) {
	s := newTestScheduler()
	cfg := &ScheduleConfig{
		ID:       "bad",
		CronExpr: "not a cron",
		ScanPath: "/tmp",
		Enabled:  true,
	}
	if err := s.CreateSchedule(cfg); err == nil {
		t.Fatal("expected error for invalid cron")
	}
}

func TestDeleteSchedule(t *testing.T) {
	s := newTestScheduler()
	cfg := &ScheduleConfig{ID: "del-1", CronExpr: "0 3 * * *", ScanPath: "/tmp", Enabled: true}
	s.CreateSchedule(cfg)

	if err := s.DeleteSchedule("del-1"); err != nil {
		t.Fatalf("DeleteSchedule: %v", err)
	}

	schedules, _ := s.GetSchedules()
	if len(schedules) != 0 {
		t.Errorf("expected 0 schedules after delete, got %d", len(schedules))
	}
}

func TestToggleSchedule(t *testing.T) {
	s := newTestScheduler()
	cfg := &ScheduleConfig{ID: "tog-1", CronExpr: "0 3 * * *", ScanPath: "/tmp", Enabled: true}
	s.CreateSchedule(cfg)

	if err := s.ToggleSchedule("tog-1", false); err != nil {
		t.Fatalf("ToggleSchedule: %v", err)
	}

	schedules, _ := s.GetSchedules()
	if schedules[0].Enabled {
		t.Error("expected schedule to be disabled")
	}
}

func TestToggleScheduleNotFound(t *testing.T) {
	s := newTestScheduler()
	err := s.ToggleSchedule("nonexistent", true)
	if err == nil {
		t.Fatal("expected error for nonexistent schedule")
	}
}

func TestUpdateSchedule(t *testing.T) {
	s := newTestScheduler()
	cfg := &ScheduleConfig{ID: "upd-1", CronExpr: "0 3 * * *", ScanPath: "/tmp", Enabled: true}
	s.CreateSchedule(cfg)

	cfg.ScanPath = "/var"
	cfg.CronExpr = "0 4 * * *"
	if err := s.UpdateSchedule(cfg); err != nil {
		t.Fatalf("UpdateSchedule: %v", err)
	}

	schedules, _ := s.GetSchedules()
	if schedules[0].ScanPath != "/var" {
		t.Errorf("expected scan path /var, got %s", schedules[0].ScanPath)
	}
}

func TestStartStop(t *testing.T) {
	s := newTestScheduler()
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s.Stop()
}

func TestStartIdempotent(t *testing.T) {
	s := newTestScheduler()
	s.Start()
	s.Start()
	s.Stop()
}

func TestAddJobRegistersInCron(t *testing.T) {
	s := newTestScheduler()
	s.Start()
	defer s.Stop()

	cfg := &ScheduleConfig{
		ID:          "job-1",
		CronExpr:    "0 3 * * *",
		ScanPath:    "/tmp",
		MaxAutoRisk: RiskSafe,
		Enabled:     true,
	}

	if err := s.CreateSchedule(cfg); err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}

	if len(s.jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(s.jobs))
	}

	// Verify job has a next run time
	entryID, ok := s.jobs["job-1"]
	if !ok {
		t.Fatal("job-1 not in jobs map")
	}
	next := s.cron.Entry(entryID).Next
	if next.IsZero() {
		t.Error("expected non-zero next run time")
	}
}

func TestStartLoadsPersistedSchedules(t *testing.T) {
	store := &memStore{
		schedules: []ScheduleConfig{
			{ID: "p-1", CronExpr: "0 3 * * *", ScanPath: "/tmp", Enabled: true},
		},
	}
	s := New(store, log.New(os.Stderr, "[test] ", 0))

	s.SetScanHandler(func(ctx context.Context, scanPath string, maxAutoRisk RiskLevel) error {
		return nil
	})

	s.Start()
	defer s.Stop()

	if len(s.jobs) != 1 {
		t.Errorf("expected 1 job loaded, got %d", len(s.jobs))
	}
}

func TestValidateCron(t *testing.T) {
	tests := []struct {
		expr string
		ok   bool
	}{
		{"0 3 * * *", true},
		{"0 */6 * * *", true},
		{"@daily", true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		err := validateCron(tt.expr)
		if (err == nil) != tt.ok {
			t.Errorf("validateCron(%q) = %v, want ok=%v", tt.expr, err, tt.ok)
		}
	}
}
