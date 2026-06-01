// internal/api/info.go
package api

import "runtime"

// buildInfo is the /info.build_info object. Version/Commit are
// overridable at link time via -ldflags "-X goboxd/internal/api.Version=...
// (and likewise for Commit). GoVersion is reported from runtime.
type buildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	GoVersion string `json:"go_version"`
}

func currentBuildInfo() buildInfo {
	return buildInfo{Version: Version, Commit: Commit, GoVersion: runtime.Version()}
}

func diskFreeBytes(path string) int64 { return platformDiskFree(path) }
