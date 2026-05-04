package platform

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"syscall"
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
	switch runtime.GOOS {
	case "windows":
		return getWindowsDrives()
	case "darwin":
		return getUnixDrives()
	default: // linux
		return getUnixDrives()
	}
}

// GetQuickTargets returns common cleanup targets for the current OS.
func GetQuickTargets() []QuickTarget {
	home, _ := os.UserHomeDir()
	targets := []QuickTarget{}

	// Cross-platform targets
	if home != "" {
		targets = append(targets,
			QuickTarget{Path: filepath.Join(home, "Downloads"), Label: "Downloads", Description: "Downloaded files", Icon: "download"},
			QuickTarget{Path: filepath.Join(home, ".cache"), Label: "Cache", Description: "Application caches", Icon: "archive"},
			QuickTarget{Path: filepath.Join(home, ".local/share/Trash"), Label: "Trash", Description: "Recycle bin contents", Icon: "trash"},
			QuickTarget{Path: filepath.Join(home, ".thumbnails"), Label: "Thumbnails", Description: "Image thumbnail cache", Icon: "image"},
			QuickTarget{Path: filepath.Join(home, "tmp"), Label: "Temp", Description: "Temporary files", Icon: "file"},
		)
	}

	switch runtime.GOOS {
	case "linux":
		targets = append(targets,
			QuickTarget{Path: "/tmp", Label: "/tmp", Description: "System temp files", Icon: "file"},
			QuickTarget{Path: "/var/cache", Label: "/var/cache", Description: "System package cache", Icon: "archive"},
			QuickTarget{Path: "/var/log", Label: "/var/log", Description: "System logs", Icon: "file"},
		)
		if home != "" {
			targets = append(targets,
				QuickTarget{Path: filepath.Join(home, ".npm"), Label: "npm cache", Description: "Node.js package cache", Icon: "archive"},
				QuickTarget{Path: filepath.Join(home, ".local/share/pnpm"), Label: "pnpm store", Description: "pnpm package store", Icon: "archive"},
				QuickTarget{Path: filepath.Join(home, ".cargo/registry"), Label: "Cargo cache", Description: "Rust package cache", Icon: "archive"},
				QuickTarget{Path: filepath.Join(home, ".gradle/caches"), Label: "Gradle cache", Description: "Java build cache", Icon: "archive"},
			)
		}
	case "darwin":
		if home != "" {
			targets = append(targets,
				QuickTarget{Path: filepath.Join(home, "Library", "Caches"), Label: "Library Cache", Description: "macOS app caches", Icon: "archive"},
				QuickTarget{Path: filepath.Join(home, "Library", "Logs"), Label: "Library Logs", Description: "macOS app logs", Icon: "file"},
				QuickTarget{Path: filepath.Join(home, ".Trash"), Label: "Trash", Description: "macOS trash", Icon: "trash"},
			)
		}
	case "windows":
		if home != "" {
			targets = append(targets,
				QuickTarget{Path: filepath.Join(home, "AppData", "Local", "Temp"), Label: "Temp", Description: "Windows temp files", Icon: "file"},
				QuickTarget{Path: filepath.Join(home, "AppData", "Local", "npm-cache"), Label: "npm cache", Description: "Node.js package cache", Icon: "archive"},
			)
		}
	}

	// Filter to only existing paths
	var existing []QuickTarget
	for _, t := range targets {
		if _, err := os.Stat(t.Path); err == nil {
			existing = append(existing, t)
		}
	}
	return existing
}

func getUnixDrives() []DriveInfo {
	var drives []DriveInfo

	// Root filesystem
	if info := getDriveInfo("/"); info != nil {
		info.Label = "/ (Root)"
		info.IsSystem = true
		drives = append(drives, *info)
	}

	// Check /home mount
	if info := getDriveInfo("/home"); info != nil {
		if info.Path != "/" {
			info.Label = "/home"
			drives = append(drives, *info)
		}
	}

	// Check mounted volumes
	entries, err := os.ReadDir("/mnt")
	if err == nil {
		for _, e := range entries {
			p := filepath.Join("/mnt", e.Name())
			if info := getDriveInfo(p); info != nil {
				info.Label = p
				drives = append(drives, *info)
			}
		}
	}

	// Check /media/<user>
	if u, err := user.Current(); err == nil {
		mediaDir := filepath.Join("/media", u.Username)
		if entries, err := os.ReadDir(mediaDir); err == nil {
			for _, e := range entries {
				p := filepath.Join(mediaDir, e.Name())
				if info := getDriveInfo(p); info != nil {
					info.Label = p
					drives = append(drives, *info)
				}
			}
		}
	}

	return drives
}

func getWindowsDrives() []DriveInfo {
	var drives []DriveInfo
	for _, letter := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		p := string(letter) + ":\\"
		if info := getDriveInfo(p); info != nil {
			info.Label = p
			info.IsSystem = (letter == 'C')
			drives = append(drives, *info)
		}
	}
	return drives
}

func getDriveInfo(path string) *DriveInfo {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free

	return &DriveInfo{
		Path:  path,
		Total: total,
		Used:  used,
		Free:  free,
	}
}
