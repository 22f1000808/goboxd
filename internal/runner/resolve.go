package runner

import "goboxd/internal/types"

func resolveTopLevel(buildStatus string, tests []types.TestResult) string {
	if buildStatus == types.BuildStatusFailed {
		return types.StatusBuildFailed
	}

	rank := map[string]int{
		types.StatusAccepted:             0,
		types.StatusOutputWhitespaceDiff: 1,
		types.StatusWrongOutput:          2,
		types.StatusTimeExceeded:         3,
		types.StatusMemoryExceeded:       4,
		types.StatusRuntimeError:         5,
		types.StatusNotExecuted:          6,
		types.StatusInternalError:        7,
	}

	worst := types.StatusAccepted
	for _, t := range tests {
		if rank[t.Status] > rank[worst] {
			worst = t.Status
		}
	}

	return worst
}
