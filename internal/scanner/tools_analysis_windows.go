//go:build windows

package scanner

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpace = kernel32.NewProc("GetDiskFreeSpaceExW")
)

func getDiskInfo() (total, free, used int64) {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "C:\\"
	}

	pathPtr, _ := syscall.UTF16PtrFromString(home)

	var freeBytes int64
	var totalBytes int64
	var availableBytes int64

	getDiskFreeSpace.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytes)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&availableBytes)),
	)

	return totalBytes, freeBytes, totalBytes - freeBytes
}
