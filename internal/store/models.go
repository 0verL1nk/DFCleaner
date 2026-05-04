package store

import (
	"time"
)

type LLMConfig struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Provider   string    `gorm:"not null" json:"provider"`
	Endpoint   string    `gorm:"not null" json:"endpoint"`
	APIKey     string    `gorm:"not null" json:"apiKey"`
	ModelName  string    `gorm:"not null" json:"modelName"`
	IsActive   bool      `gorm:"default:false" json:"isActive"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type ScanHistory struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Path       string    `gorm:"not null" json:"path"`
	ScannedAt  time.Time `gorm:"not null" json:"scannedAt"`
	FileCount  int       `gorm:"default:0" json:"fileCount"`
	TotalSize  int64     `gorm:"default:0" json:"totalSize"`
	DurationMs int64     `gorm:"default:0" json:"durationMs"`
	CreatedAt  time.Time `json:"createdAt"`
}

type CleanupLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ScanID       uint      `json:"scanId"`
	FilePath     string    `gorm:"not null" json:"filePath"`
	Operation    string    `gorm:"not null" json:"operation"`
	TargetPath   string    `json:"targetPath"`
	Result       string    `gorm:"not null" json:"result"`
	ErrorMessage string    `json:"errorMessage"`
	FreedBytes   int64     `gorm:"default:0" json:"freedBytes"`
	CreatedAt    time.Time `json:"createdAt"`
}

type UserSetting struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"uniqueIndex;not null" json:"key"`
	Value     string    `gorm:"not null" json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}
