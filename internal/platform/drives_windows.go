//go:build windows

package platform

import (
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpace = kernel32.NewProc("GetDiskFreeSpaceExW")
)

func getDrives() []DriveInfo {
	return getWindowsDrives()
}

func getWindowsDrives() []DriveInfo {
	var drives []DriveInfo
	for _, letter := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		p := string(letter) + ":\\"
		if info := getDriveInfo(p); info != nil {
			info.Label = p
			info.IsSystem = (letter == 'C')
			drives = append(drives, *info)
		}
	}
	return drives
}

func getDriveInfo(path string) *DriveInfo {
	pathPtr, _ := syscall.UTF16PtrFromString(path)

	var freeBytes int64
	var totalBytes int64
	var totalFreeBytes int64

	ret, _, _ := getDiskFreeSpace.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytes)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	if ret == 0 {
		return nil
	}

	total := uint64(totalBytes)
	free := uint64(freeBytes)

	return &DriveInfo{
		Path:  path,
		Total: total,
		Used:  total - free,
		Free:  free,
	}
}
