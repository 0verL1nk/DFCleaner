//go:build windows

package scanner

import "time"

func getAccessTime(sys any) time.Time {
	// Windows: use win32 file info via syscall
	// Fallback to zero time, caller should use ModTime
	return time.Time{}
}
