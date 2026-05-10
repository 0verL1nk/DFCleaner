package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type RiskLevel string

const (
	RiskSafe    RiskLevel = "safe"
	RiskCaution RiskLevel = "caution"
)

type ScheduleConfig struct {
	ID          string    `json:"id"`
	CronExpr    string    `json:"cronExpr"`
	ScanPath    string    `json:"scanPath"`
	MaxAutoRisk RiskLevel `json:"maxAutoRisk"`
	Enabled     bool      `json:"enabled"`
	LastRun     time.Time `json:"lastRun"`
	NextRun     time.Time `json:"nextRun,omitempty"`
}

type Scheduler struct {
	mu      sync.Mutex
	cron    *cron.Cron
	store   Store
	jobs    map[string]cron.EntryID
	logger  *log.Logger
	onScan  func(ctx context.Context, scanPath string, maxAutoRisk RiskLevel) error
	running bool
}

type Store interface {
	LoadSchedules() ([]ScheduleConfig, error)
	SaveSchedule(cfg *ScheduleConfig) error
	DeleteSchedule(id string) error
}

func New(s Store, logger *log.Logger) *Scheduler {
	if logger == nil {
		logger = log.Default()
	}
	return &Scheduler{
		cron:   cron.New(cron.WithSeconds()),
		store:  s,
		jobs:   make(map[string]cron.EntryID),
		logger: logger,
	}
}

func (s *Scheduler) SetScanHandler(fn func(ctx context.Context, scanPath string, maxAutoRisk RiskLevel) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onScan = fn
}

func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	schedules, err := s.store.LoadSchedules()
	if err != nil {
		return fmt.Errorf("load schedules: %w", err)
	}

	for i := range schedules {
		cfg := &schedules[i]
		if !cfg.Enabled {
			continue
		}
		if err := s.addJob(cfg); err != nil {
			s.logger.Printf("scheduler: skip schedule %s: %v", cfg.ID, err)
		}
	}

	s.cron.Start()
	s.running = true
	s.logger.Printf("scheduler: started with %d active jobs", len(s.jobs))
	return nil
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		ctx := s.cron.Stop()
		<-ctx.Done()
		s.running = false
		s.logger.Println("scheduler: stopped")
	}
}

func (s *Scheduler) CreateSchedule(cfg *ScheduleConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateCron(cfg.CronExpr); err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	if err := s.store.SaveSchedule(cfg); err != nil {
		return err
	}

	if cfg.Enabled {
		if err := s.addJob(cfg); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) UpdateSchedule(cfg *ScheduleConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateCron(cfg.CronExpr); err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	s.removeJob(cfg.ID)

	if err := s.store.SaveSchedule(cfg); err != nil {
		return err
	}

	if cfg.Enabled {
		if err := s.addJob(cfg); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) DeleteSchedule(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.removeJob(id)
	return s.store.DeleteSchedule(id)
}

func (s *Scheduler) GetSchedules() ([]ScheduleConfig, error) {
	return s.store.LoadSchedules()
}

func (s *Scheduler) ToggleSchedule(id string, enabled bool) error {
	schedules, err := s.store.LoadSchedules()
	if err != nil {
		return err
	}

	for i := range schedules {
		if schedules[i].ID == id {
			schedules[i].Enabled = enabled
			return s.UpdateSchedule(&schedules[i])
		}
	}
	return fmt.Errorf("schedule %s not found", id)
}

func (s *Scheduler) addJob(cfg *ScheduleConfig) error {
	schedule, err := cron.ParseStandard(cfg.CronExpr)
	if err != nil {
		return err
	}

	entryID := s.cron.Schedule(schedule, cron.FuncJob(func() {
		s.executeJob(cfg)
	}))

	s.jobs[cfg.ID] = entryID
	cfg.NextRun = s.cron.Entry(entryID).Next
	return nil
}

func (s *Scheduler) removeJob(id string) {
	if entryID, ok := s.jobs[id]; ok {
		s.cron.Remove(entryID)
		delete(s.jobs, id)
	}
}

// ExecuteJobForTest triggers a scheduled job manually. Used by integration tests.
func (s *Scheduler) ExecuteJobForTest(cfg *ScheduleConfig) {
	s.executeJob(cfg)
}

func (s *Scheduler) executeJob(cfg *ScheduleConfig) {
	s.logger.Printf("scheduler: executing schedule %s (path=%s, maxRisk=%s)", cfg.ID, cfg.ScanPath, cfg.MaxAutoRisk)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	if s.onScan != nil {
		if err := s.onScan(ctx, cfg.ScanPath, cfg.MaxAutoRisk); err != nil {
			s.logger.Printf("scheduler: schedule %s failed: %v", cfg.ID, err)
		}
	}

	cfg.LastRun = time.Now()
	_ = s.store.SaveSchedule(cfg)
}

func validateCron(expr string) error {
	_, err := cron.ParseStandard(expr)
	return err
}
