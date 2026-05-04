//go:build linux || darwin

package platform

import (
	"os"
	"os/user"
	"path/filepath"
	"syscall"
)

func getDrives() []DriveInfo {
	return getUnixDrives()
}

func getUnixDrives() []DriveInfo {
	var drives []DriveInfo

	// Root filesystem
	if info := getDriveInfo("/"); info != nil {
		info.Label = "/ (Root)"
		info.IsSystem = true
		drives = append(drives, *info)
	}

	// Check /home mount
	if info := getDriveInfo("/home"); info != nil {
		if info.Path != "/" {
			info.Label = "/home"
			drives = append(drives, *info)
		}
	}

	// Check mounted volumes
	entries, err := os.ReadDir("/mnt")
	if err == nil {
		for _, e := range entries {
			p := filepath.Join("/mnt", e.Name())
			if info := getDriveInfo(p); info != nil {
				info.Label = p
				drives = append(drives, *info)
			}
		}
	}

	// Check /media/<user>
	if u, err := user.Current(); err == nil {
		mediaDir := filepath.Join("/media", u.Username)
		if entries, err := os.ReadDir(mediaDir); err == nil {
			for _, e := range entries {
				p := filepath.Join(mediaDir, e.Name())
				if info := getDriveInfo(p); info != nil {
					info.Label = p
					drives = append(drives, *info)
				}
			}
		}
	}

	return drives
}

func getDriveInfo(path string) *DriveInfo {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free

	return &DriveInfo{
		Path:  path,
		Total: total,
		Used:  used,
		Free:  free,
	}
}
