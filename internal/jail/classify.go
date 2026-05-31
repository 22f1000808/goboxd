package jail

import (
	"strings"

	"goboxd/internal/types"
)

const (
	sigKILL = 9
	sigXFSZ = 25
)

func Classify(o Outcome, expected string) string {
	if o.OOMKilled {
		return types.StatusMemoryExceeded
	}

	if o.PIDsExhausted {
		return types.StatusRuntimeError
	}

	if o.Signal != 0 {
		switch o.Signal {
		case sigXFSZ:
			return types.StatusRuntimeError
		case sigKILL:
			return types.StatusTimeExceeded
		}
		return types.StatusRuntimeError
	}

	if o.TimedOut {
		return types.StatusTimeExceeded
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
