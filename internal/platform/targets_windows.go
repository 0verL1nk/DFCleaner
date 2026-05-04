//go:build windows

package platform

import "path/filepath"

func getPlatformTargets(home string) []QuickTarget {
	if home == "" {
		return nil
	}
	return []QuickTarget{
		{Path: filepath.Join(home, "AppData", "Local", "Temp"), Label: "Temp", Description: "Windows temp files", Icon: "file"},
		{Path: filepath.Join(home, "AppData", "Local", "npm-cache"), Label: "npm cache", Description: "Node.js package cache", Icon: "archive"},
	}
}
