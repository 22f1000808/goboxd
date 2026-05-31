package runner

import "goboxd/internal/types"

func resolveTopLevel(buildStatus string, tests []types.TestResult) string {
	switch buildStatus {
	case types.BuildStatusFailed:
		return types.StatusBuildFailed
	case types.BuildStatusInternalError:
		return types.StatusInternalError
	}

	rank := map[string]int{
		types.StatusAccepted:             0,
		types.StatusOutputWhitespaceDiff: 1,
		types.StatusWrongOutput:          2,
		types.StatusTimeExceeded:         3,
		types.StatusMemoryExceeded:       3,
		types.StatusRuntimeError:         4,
		types.StatusNotExecuted:          5,
		types.StatusInternalError:        6,
	}

	worst := types.StatusAccepted
	for _, t := range tests {
		if rank[t.Status] > rank[worst] {
			worst = t.Status
		}
	}
	return worst
}
