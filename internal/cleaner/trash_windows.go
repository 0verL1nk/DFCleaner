//go:build windows

package cleaner

import (
	"fmt"
	"os"
	"path/filepath"
)

func moveToTrash(path string) error {
	// On Windows, use syscall to invoke SHFileOperation for recycle bin
	// For now, fall back to moving to a .trash directory in user home
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	trashDir := filepath.Join(home, ".dfcleaner-trash")
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return fmt.Errorf("create trash dir: %w", err)
	}

	dest := filepath.Join(trashDir, filepath.Base(path))
	if _, err := os.Stat(dest); err == nil {
		// Name collision: append timestamp
		dest = fmt.Sprintf("%s_%d", dest, os.Getpid())
	}

	return os.Rename(path, dest)
}
