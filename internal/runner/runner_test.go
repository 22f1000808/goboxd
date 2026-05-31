package runner

import (
	"testing"

	"goboxd/internal/types"
)

func TestResolveTopLevel(t *testing.T) {
	cases := []struct {
		name  string
		build string
		tests []types.TestResult
		want  string
	}{
		{"all accepted", "", []types.TestResult{{Status: types.StatusAccepted}, {Status: types.StatusAccepted}}, types.StatusAccepted},
		{"build failed wins", types.BuildStatusFailed, []types.TestResult{{Status: types.StatusNotExecuted}}, types.StatusBuildFailed},
		{"build internal -> internal", types.BuildStatusInternalError, []types.TestResult{{Status: types.StatusNotExecuted}}, types.StatusInternalError},
		{"build ok lets tests decide", types.BuildStatusOK, []types.TestResult{{Status: types.StatusAccepted}, {Status: types.StatusAccepted}}, types.StatusAccepted},
		{"runtime beats accepted", "", []types.TestResult{{Status: types.StatusAccepted}, {Status: types.StatusRuntimeError}}, types.StatusRuntimeError},
		{"memory exceeded ranks with time", "", []types.TestResult{{Status: types.StatusAccepted}, {Status: types.StatusMemoryExceeded}, {Status: types.StatusTimeExceeded}}, types.StatusMemoryExceeded},
		{"whitespace beats accepted", "", []types.TestResult{{Status: types.StatusAccepted}, {Status: types.StatusOutputWhitespaceDiff}}, types.StatusOutputWhitespaceDiff},
		{"internal beats all", "", []types.TestResult{{Status: types.StatusAccepted}, {Status: types.StatusInternalError}, {Status: types.StatusRuntimeError}}, types.StatusInternalError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveTopLevel(c.build, c.tests); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}

func TestExpandArgsFlagsSplice(t *testing.T) {
	got := expandArgs(
		[]string{"{{flags}}", "-o", "{{artifact}}", "{{source}}"},
		"/work/x.cpp", "solution",
		[]string{"-O2", "-Wall"},
	)
	want := []string{"-O2", "-Wall", "-o", "solution", "/work/x.cpp"}
	eqStrings(t, got, want)
}

func TestExpandArgsZeroFlags(t *testing.T) {
	got := expandArgs([]string{"{{flags}}", "{{source}}"}, "/work/s.py", "", nil)
	eqStrings(t, got, []string{"/work/s.py"})
}

func TestExpandArgsThreeFlags(t *testing.T) {
	got := expandArgs([]string{"{{flags}}", "-a", "-b"}, "", "", []string{"-a", "-b", "-c"})
	eqStrings(t, got, []string{"-a", "-b", "-c", "-a", "-b"})
}

func TestExpandArgsNoFlagsPlaceholder(t *testing.T) {
	got := expandArgs([]string{"{{source}}"}, "/work/x.py", "", nil)
	eqStrings(t, got, []string{"/work/x.py"})
}

func TestExpandCmd(t *testing.T) {
	if got := expandCmd("./{{artifact}}", "", "solution"); got != "./solution" {
		t.Fatalf("got %q, want ./solution", got)
	}

	if got := expandCmd("/usr/bin/python3", "", ""); got != "/usr/bin/python3" {
		t.Fatalf("got %q, want /usr/bin/python3", got)
	}
}

func eqStrings(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len: got %v want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("at %d: got %q want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}
