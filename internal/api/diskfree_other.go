//go:build !linux

// internal/api/diskfree_other.go
package api

func platformDiskFree(_ string) int64 { return 0 }
