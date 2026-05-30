package jail

import (
	"testing"

	"goboxd/internal/types"
)

func TestClassifyPhase1(t *testing.T) {
	cases := []struct {
		name     string
		o        Outcome
		expected string
		want     string
	}{
		{"accepted exact", Outcome{Stdout: []byte("hi\n")}, "hi\n", types.StatusAccepted},
		{"whitespace diff", Outcome{Stdout: []byte("hi \n")}, "hi\n", types.StatusOutputWhitespaceDiff},
		{"whitespace diff2", Outcome{Stdout: []byte("hi\n")}, "hi", types.StatusOutputWhitespaceDiff},
		{"wrong output", Outcome{Stdout: []byte("bye\n")}, "hi\n", types.StatusWrongOutput},
		{"runtime err exit 1", Outcome{ExitCode: 1, Stdout: []byte("")}, "", types.StatusRuntimeError},
		{"runtime err signal", Outcome{Signal: 11}, "", types.StatusRuntimeError},
		{"time exceeded", Outcome{TimedOut: true}, "x", types.StatusTimeExceeded},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Classify(c.o, c.expected); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}

func TestPbEscape(t *testing.T) {
	cases := map[string]string{
		"abc":        "abc",
		"a\"b":       "a\\\"b",
		"a\nb":       "a\\nb",
		"a\\b":       "a\\\\b",
		"with\rline": "with\\rline",
	}

	for in, want := range cases {
		if got := pbEscape(in); got != want {
			t.Fatalf("pbEscape(%q) = %q want %q", in, got, want)
		}
	}
}

func TestRenderConfigEscapes(t *testing.T) {
	j := Job{
		Name:      "nasty\"name",
		Workspace: Workspace{CgroupPath: "/cg"},
		Cmd:       "/usr/bin/python3",
		Args:      []string{"/work/solution.py"},
		Mounts:    []Mount{{HostPath: "/host/dir", JailPath: "/work", ReadOnly: true}},
		WallTimeS: 9,
		MemoryKB:  102400,
	}
	cfg, err := renderConfig(j)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(cfg, "name: \"nasty\\\"name\"") {
		t.Fatalf("expected escaped name in:\n%s", cfg)
	}
	if !contains(cfg, "clone_newnet: true") {
		t.Fatalf("expected default-deny network in:\n%s", cfg)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
