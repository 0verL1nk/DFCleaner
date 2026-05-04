//go:build linux || darwin

package scanner

import (
	"syscall"
	"time"
)

func getAccessTime(sys any) time.Time {
	if stat, ok := sys.(*syscall.Stat_t); ok {
		return time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
	}
	return time.Time{}
}
