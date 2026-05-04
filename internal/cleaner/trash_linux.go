//go:build linux

package cleaner

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func moveToTrash(path string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	var baseDir string
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		baseDir = xdg
	} else {
		baseDir = filepath.Join(home, ".local", "share")
	}

	trashFilesDir := filepath.Join(baseDir, "Trash", "files")
	trashInfoDir := filepath.Join(baseDir, "Trash", "info")

	if err := os.MkdirAll(trashFilesDir, 0700); err != nil {
		return fmt.Errorf("create trash files dir: %w", err)
	}
	if err := os.MkdirAll(trashInfoDir, 0700); err != nil {
		return fmt.Errorf("create trash info dir: %w", err)
	}

	dest := uniquePath(filepath.Join(trashFilesDir, filepath.Base(path)))

	if err := os.Rename(path, dest); err != nil {
		return fmt.Errorf("rename to trash: %w", err)
	}

	// Write .trashinfo per FreeDesktop.org Trash specification
	infoPath := filepath.Join(trashInfoDir, filepath.Base(dest)+".trashinfo")
	infoContent := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		path, time.Now().Format("2006-01-02T15:04:05"))
	_ = os.WriteFile(infoPath, []byte(infoContent), 0600)

	return nil
}

func uniquePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	base := filepath.Base(path)
	dir := filepath.Dir(path)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]

	for i := 1; i < 1000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", name, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	return path
}
