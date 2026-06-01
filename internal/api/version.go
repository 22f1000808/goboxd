// internal/api/version.go
package api

// Version and Commit are overridable at link time via:
//
//	-ldflags "-X goboxd/internal/api.Version=v1.2.3 -X goboxd/internal/api.Commit=abc1234"
var (
	Version = "0.1.0"
	Commit  = "unknown"
)
