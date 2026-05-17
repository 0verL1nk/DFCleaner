//go:build linux || darwin

package scanner

import (
	"os"
	"syscall"
)

func getDiskInfo() (total, free, used int64) {
	var stat syscall.Statfs_t
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/"
	}
	syscall.Statfs(home, &stat)

	total = int64(stat.Blocks) * int64(stat.Bsize)
	free = int64(stat.Bavail) * int64(stat.Bsize)
	used = (int64(stat.Blocks) - int64(stat.Bfree)) * int64(stat.Bsize)
	return
}
