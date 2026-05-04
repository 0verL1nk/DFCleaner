//go:build darwin

package cleaner

import (
	"fmt"
	"os"
	"path/filepath"
)

func moveToTrash(path string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	trashDir := filepath.Join(home, ".Trash")
	if err := os.MkdirAll(trashDir, 0700); err != nil {
		return fmt.Errorf("create trash dir: %w", err)
	}

	dest := uniquePath(filepath.Join(trashDir, filepath.Base(path)))
	return os.Rename(path, dest)
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
