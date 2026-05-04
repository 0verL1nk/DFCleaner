package platform

import (
	"os"
	"path/filepath"
)

type DriveInfo struct {
	Path      string `json:"path"`
	Label     string `json:"label"`
	Total     uint64 `json:"total"`
	Used      uint64 `json:"used"`
	Free      uint64 `json:"free"`
	IsSystem  bool   `json:"isSystem"`
}

type QuickTarget struct {
	Path        string `json:"path"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

// GetDrives returns all available drives/volumes on the system.
func GetDrives() []DriveInfo {
	return getDrives()
}

// GetQuickTargets returns common cleanup targets for the current OS.
func GetQuickTargets() []QuickTarget {
	home, _ := os.UserHomeDir()
	targets := getCommonTargets(home)
	targets = append(targets, getPlatformTargets(home)...)

	var existing []QuickTarget
	for _, t := range targets {
		if _, err := os.Stat(t.Path); err == nil {
			existing = append(existing, t)
		}
	}
	return existing
}

func getCommonTargets(home string) []QuickTarget {
	if home == "" {
		return nil
	}
	return []QuickTarget{
		{Path: filepath.Join(home, "Downloads"), Label: "Downloads", Description: "Downloaded files", Icon: "download"},
		{Path: filepath.Join(home, ".cache"), Label: "Cache", Description: "Application caches", Icon: "archive"},
		{Path: filepath.Join(home, ".thumbnails"), Label: "Thumbnails", Description: "Image thumbnail cache", Icon: "image"},
	}
}
