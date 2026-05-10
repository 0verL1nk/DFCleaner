package cleaner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dfcleaner/internal/store"
)

const (
	OpTrash  = "trash"  // Move to system trash
	OpDelete = "delete" // Permanent delete (irreversible)
	OpMove   = "move"   // Move to a specified directory
)

func (c *Cleaner) Cleanup(items []CleanupItem) ([]CleanupResult, error) {
	if len(items) == 0 {
		return nil, nil
	}

	results := make([]CleanupResult, len(items))
	for i, item := range items {
		results[i] = c.CleanupOne(item)
	}
	return results, nil
}

func (c *Cleaner) CleanupOne(item CleanupItem) CleanupResult {
	path := filepath.Clean(item.Path)

	if !filepath.IsAbs(path) {
		return CleanupResult{Path: item.Path, Error: "path must be absolute"}
	}

	if isDangerousPath(path) {
		return CleanupResult{Path: item.Path, Error: "refused: path is too broad"}
	}

	size := getPathSize(path)

	var opErr error
	switch item.Operation {
	case OpTrash:
		opErr = moveToTrash(path)
	case OpDelete:
		opErr = permanentDelete(path)
	case OpMove:
		if item.Target == "" {
			return CleanupResult{Path: item.Path, Error: "move requires a target path"}
		}
		opErr = moveToDir(path, filepath.Clean(item.Target))
	default:
		return CleanupResult{Path: item.Path, Error: fmt.Sprintf("unknown operation: %s", item.Operation)}
	}

	if opErr != nil {
		c.logOperation(item.Path, item.Operation, "failed", opErr.Error(), 0)
		return CleanupResult{Path: item.Path, Error: opErr.Error()}
	}

	c.logOperation(item.Path, item.Operation, "success", "", size)
	return CleanupResult{Path: item.Path, Success: true, FreedBytes: size}
}

func (c *Cleaner) logOperation(path, operation, result, errMsg string, freedBytes int64) {
	if c.store == nil {
		return
	}
	_ = c.store.LogCleanup(&store.CleanupLog{
		FilePath:     path,
		Operation:    operation,
		Result:       result,
		ErrorMessage: errMsg,
		FreedBytes:   freedBytes,
	})
}

// isDangerousPath only blocks root-level paths — all other judgments are left to AI.
func isDangerousPath(path string) bool {
	clean := strings.TrimRight(path, string(filepath.Separator))
	if clean == "/" || len(clean) <= 3 {
		return true
	}
	depth := len(strings.Split(clean, string(filepath.Separator)))
	return depth < 2
}

func getPathSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	if !info.IsDir() {
		return info.Size()
	}
	var total int64
	filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			if fi, e := d.Info(); e == nil {
				total += fi.Size()
			}
		}
		return nil
	})
	return total
}

func permanentDelete(path string) error {
	return os.RemoveAll(path)
}

func moveToDir(src, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return moveDirToDir(src, destDir)
	}
	return os.Rename(src, filepath.Join(destDir, filepath.Base(src)))
}

func moveDirToDir(src, destDir string) error {
	targetDir := filepath.Join(destDir, filepath.Base(src))
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		if err := moveToDir(srcPath, targetDir); err != nil {
			return err
		}
	}

	return os.Remove(src)
}
