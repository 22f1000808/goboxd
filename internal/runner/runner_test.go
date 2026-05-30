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
		{"all accepted", "", []types.TestResult{{Status: types.StatusAccepted}}, types.StatusAccepted},
		{"build failed wins", types.BuildStatusFailed, []types.TestResult{{Status: types.StatusAccepted}}, types.StatusBuildFailed},
		{"runtime beats accepted", "", []types.TestResult{{Status: types.StatusRuntimeError}, {Status: types.StatusAccepted}}, types.StatusRuntimeError},
		{"whitespace beats accepted", "", []types.TestResult{{Status: types.StatusOutputWhitespaceDiff}, {Status: types.StatusAccepted}}, types.StatusOutputWhitespaceDiff},
		{"internal beats all", "", []types.TestResult{{Status: types.StatusInternalError}, {Status: types.StatusRuntimeError}}, types.StatusInternalError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveTopLevel(c.build, c.tests); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}

func TestExpandArgs(t *testing.T) {
	got := expandArgs([]string{"{{flags}}", "-o", "{{artifact}}", "{{source}}"}, "/work/x.cpp", "/work/x", []string{"-O2", "-Wall"})
	want := []string{"-O2", "-Wall", "-o", "/work/x", "/work/x.cpp"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("at %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestExpandArgsNoFlags(t *testing.T) {
	got := expandArgs([]string{"{{source}}"}, "/work/s.py", "", nil)
	if len(got) != 1 || got[0] != "/work/s.py" {
		t.Fatalf("got %v", got)
	}
}
