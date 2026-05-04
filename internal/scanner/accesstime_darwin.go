//go:build darwin

package scanner

import (
	"syscall"
	"time"
)

func getAccessTime(sys any) time.Time {
	if stat, ok := sys.(*syscall.Stat_t); ok {
		return time.Unix(stat.Atimespec.Sec, stat.Atimespec.Nsec)
	}
	return time.Time{}
}
