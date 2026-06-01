//go:build linux

// internal/api/diskfree_linux.go
package api

import "syscall"

func platformDiskFree(path string) int64 {
	if path == "" {
		return 0
	}

	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0
	}

	return int64(st.Bavail) * int64(st.Bsize)
}
