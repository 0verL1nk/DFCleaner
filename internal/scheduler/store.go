package scheduler

import (
	"time"

	"gorm.io/gorm"
)

type ScheduleDB struct {
	ID          string    `gorm:"primaryKey"`
	CronExpr    string    `gorm:"not null"`
	ScanPath    string    `gorm:"not null"`
	MaxAutoRisk string    `gorm:"not null;default:safe"`
	Enabled     bool      `gorm:"default:true"`
	LastRun     time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func MigrateDB(db *gorm.DB) error {
	return db.AutoMigrate(&ScheduleDB{})
}

type DBStore struct {
	db *gorm.DB
}

func NewDBStore(db *gorm.DB) *DBStore {
	return &DBStore{db: db}
}

func (s *DBStore) LoadSchedules() ([]ScheduleConfig, error) {
	var rows []ScheduleDB
	if err := s.db.Find(&rows).Error; err != nil {
		return nil, err
	}

	configs := make([]ScheduleConfig, len(rows))
	for i, r := range rows {
		configs[i] = ScheduleConfig{
			ID:          r.ID,
			CronExpr:    r.CronExpr,
			ScanPath:    r.ScanPath,
			MaxAutoRisk: RiskLevel(r.MaxAutoRisk),
			Enabled:     r.Enabled,
			LastRun:     r.LastRun,
		}
	}
	return configs, nil
}

func (s *DBStore) SaveSchedule(cfg *ScheduleConfig) error {
	row := ScheduleDB{
		ID:          cfg.ID,
		CronExpr:    cfg.CronExpr,
		ScanPath:    cfg.ScanPath,
		MaxAutoRisk: string(cfg.MaxAutoRisk),
		Enabled:     cfg.Enabled,
		LastRun:     cfg.LastRun,
	}
	return s.db.Save(&row).Error
}

func (s *DBStore) DeleteSchedule(id string) error {
	return s.db.Delete(&ScheduleDB{}, "id = ?", id).Error
}
