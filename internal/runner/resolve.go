// internal/runner/resolve.go
package runner

import "goboxd/internal/types"

// resolveTopLevel implements the spec's precedence rule for the top-level
// status. Build precedence is wired in Phase 2; in Phase 1 buildStatus is
// always "".
func resolveTopLevel(buildStatus string, tests []types.TestResult) string {
	switch buildStatus {
	case types.BuildStatusFailed:
		return types.StatusBuildFailed
	case types.BuildStatusInternalError:
		return types.StatusInternalError
	}

	for _, t := range tests {
		if t.Status != types.StatusAccepted {
			return t.Status
		}
	}
	return types.StatusAccepted
}
