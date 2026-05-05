package store

import (
	"fmt"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type Store struct {
	db *gorm.DB
}

func InitDB(dbPath string) (*gorm.DB, error) {
	fullPath, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, fmt.Errorf("resolve db path: %w", err)
	}

	db, err := gorm.Open(sqlite.Open(fullPath+"?_journal_mode=WAL"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	err = db.AutoMigrate(
		&LLMConfig{},
		&ScanHistory{},
		&CleanupLog{},
		&UserSetting{},
		&CleanableItemDB{},
	)
	if err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	return db, nil
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// --- LLM Config ---

func (s *Store) GetActiveLLMConfig() *LLMConfig {
	var cfg LLMConfig
	if err := s.db.Where("is_active = ?", true).First(&cfg).Error; err != nil {
		return nil
	}
	return &cfg
}

func (s *Store) GetLLMConfigs() []LLMConfig {
	var configs []LLMConfig
	s.db.Find(&configs)
	return configs
}

func (s *Store) SaveLLMConfig(cfg *LLMConfig) error {
	if cfg.IsActive {
		s.db.Model(&LLMConfig{}).Where("is_active = ?", true).Update("is_active", false)
	}

	if cfg.ID > 0 {
		return s.db.Save(cfg).Error
	}

	// Check if a config with same provider+endpoint+modelName exists
	var existing LLMConfig
	if err := s.db.Where("provider = ? AND endpoint = ? AND model_name = ?",
		cfg.Provider, cfg.Endpoint, cfg.ModelName).First(&existing).Error; err == nil {
		cfg.ID = existing.ID
		return s.db.Save(cfg).Error
	}

	return s.db.Save(cfg).Error
}

func (s *Store) DeleteLLMConfig(id uint) error {
	return s.db.Delete(&LLMConfig{}, id).Error
}

// --- User Settings ---

func (s *Store) GetSetting(key string) string {
	var setting UserSetting
	if err := s.db.Where("`key` = ?", key).First(&setting).Error; err != nil {
		return ""
	}
	return setting.Value
}

func (s *Store) SetSetting(key, value string) error {
	var existing UserSetting
	if err := s.db.Where("`key` = ?", key).First(&existing).Error; err == nil {
		return s.db.Model(&existing).Update("value", value).Error
	}
	return s.db.Save(&UserSetting{Key: key, Value: value}).Error
}

// --- Cleanup Logs ---

func (s *Store) LogCleanup(log *CleanupLog) error {
	return s.db.Create(log).Error
}

func (s *Store) GetRecentCleanups(limit int) []CleanupLog {
	var logs []CleanupLog
	s.db.Order("created_at DESC").Limit(limit).Find(&logs)
	return logs
}

// --- Cleanable Items ---

func (s *Store) SaveCleanableItem(item *CleanableItemDB) error {
	return s.db.Save(item).Error
}

func (s *Store) GetCleanableItems(scanPath string) []CleanableItemDB {
	var items []CleanableItemDB
	if scanPath != "" {
		s.db.Where("scan_path = ?", scanPath).Order("created_at DESC").Find(&items)
	} else {
		s.db.Order("created_at DESC").Find(&items)
	}
	return items
}

func (s *Store) DeleteCleanableItem(path string) error {
	return s.db.Where("path = ?", path).Delete(&CleanableItemDB{}).Error
}

func (s *Store) GetCleanableSize(scanPath string) int64 {
	var total int64
	q := s.db.Model(&CleanableItemDB{})
	if scanPath != "" {
		q = q.Where("scan_path = ?", scanPath)
	}
	q.Select("COALESCE(SUM(size), 0)").Scan(&total)
	return total
}

func (s *Store) ClearCleanableItems(scanPath string) error {
	if scanPath != "" {
		return s.db.Where("scan_path = ?", scanPath).Delete(&CleanableItemDB{}).Error
	}
	return s.db.Exec("DELETE FROM cleanable_item_d_bs").Error
}
