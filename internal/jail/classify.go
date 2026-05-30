package jail

import (
	"strings"

	"goboxd/internal/types"
)

func Classify(o Outcome, expected string) string {
	if o.TimedOut {
		return types.StatusTimeExceeded
	}

	if o.Signal != 0 {
		return types.StatusRuntimeError
	}

	if o.ExitCode != 0 {
		return types.StatusRuntimeError
	}

	got := string(o.Stdout)
	if got == expected {
		return types.StatusAccepted
	}

	if strings.TrimSpace(got) == strings.TrimSpace(expected) {
		return types.StatusOutputWhitespaceDiff
	}

	return types.StatusWrongOutput
}
