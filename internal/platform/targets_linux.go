//go:build linux

package platform

import (
	"os"
	"path/filepath"
)

func getPlatformTargets(home string) []QuickTarget {
	targets := []QuickTarget{
		{Path: "/tmp", Label: "/tmp", Description: "System temp files", Icon: "file"},
		{Path: "/var/cache", Label: "/var/cache", Description: "System package cache", Icon: "archive"},
		{Path: "/var/log", Label: "/var/log", Description: "System logs", Icon: "file"},
	}
	if home != "" {
		targets = append(targets,
			QuickTarget{Path: filepath.Join(home, ".local/share/Trash"), Label: "Trash", Description: "Recycle bin contents", Icon: "trash"},
			QuickTarget{Path: filepath.Join(home, ".npm"), Label: "npm cache", Description: "Node.js package cache", Icon: "archive"},
			QuickTarget{Path: filepath.Join(home, ".local/share/pnpm"), Label: "pnpm store", Description: "pnpm package store", Icon: "archive"},
			QuickTarget{Path: filepath.Join(home, ".cargo/registry"), Label: "Cargo cache", Description: "Rust package cache", Icon: "archive"},
			QuickTarget{Path: filepath.Join(home, ".gradle/caches"), Label: "Gradle cache", Description: "Java build cache", Icon: "archive"},
		)
	}

	xdgCache := os.Getenv("XDG_CACHE_HOME")
	if xdgCache == "" && home != "" {
		xdgCache = filepath.Join(home, ".cache")
	}
	if xdgCache != "" {
		targets = append(targets, QuickTarget{Path: xdgCache, Label: "XDG Cache", Description: "XDG cache directory", Icon: "archive"})
	}

	return targets
}
