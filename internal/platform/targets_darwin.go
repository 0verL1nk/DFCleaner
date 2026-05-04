//go:build darwin

package platform

import "path/filepath"

func getPlatformTargets(home string) []QuickTarget {
	if home == "" {
		return nil
	}
	return []QuickTarget{
		{Path: filepath.Join(home, "Library", "Caches"), Label: "Library Cache", Description: "macOS app caches", Icon: "archive"},
		{Path: filepath.Join(home, "Library", "Logs"), Label: "Library Logs", Description: "macOS app logs", Icon: "file"},
		{Path: filepath.Join(home, ".Trash"), Label: "Trash", Description: "macOS trash", Icon: "trash"},
		{Path: filepath.Join(home, ".npm"), Label: "npm cache", Description: "Node.js package cache", Icon: "archive"},
		{Path: filepath.Join(home, ".cargo/registry"), Label: "Cargo cache", Description: "Rust package cache", Icon: "archive"},
	}
}
