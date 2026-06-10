package jail

import (
	"strings"
	"testing"

	"goboxd/internal/types"
)

func TestClassify(t *testing.T) {
	const (
		sigKILL = 9
		sigSEGV = 11
		sigXFSZ = 25
	)
	cases := []struct {
		name     string
		o        Outcome
		expected string
		want     string
	}{
		{"accepted exact", Outcome{Stdout: []byte("hi\n")}, "hi\n", types.StatusAccepted},
		{"whitespace diff", Outcome{Stdout: []byte("hi \n")}, "hi", types.StatusOutputWhitespaceMismatch},
		{"wrong output", Outcome{Stdout: []byte("bye\n")}, "hi\n", types.StatusWrongOutput},
		{"runtime err exit 1", Outcome{ExitCode: 1}, "", types.StatusRuntimeError},
		{"runtime err exit 139", Outcome{ExitCode: 139}, "", types.StatusRuntimeError},
		{"sigkill no cgroup -> time", Outcome{Signal: sigKILL}, "", types.StatusTimeExceeded},
		{"sigsegv -> runtime", Outcome{Signal: sigSEGV}, "", types.StatusRuntimeError},
		{"sigxfsz -> runtime", Outcome{Signal: sigXFSZ}, "", types.StatusRuntimeError},
		{"oom -> memory", Outcome{Signal: sigKILL, OOMKilled: true}, "", types.StatusMemoryExceeded},
		{"pids exhausted -> runtime", Outcome{Signal: sigKILL, PIDsExhausted: true}, "", types.StatusRuntimeError},
		{"timed out no signal", Outcome{TimedOut: true}, "x", types.StatusTimeExceeded},
		{"timed out with sigkill", Outcome{TimedOut: true, Signal: sigKILL}, "x", types.StatusTimeExceeded},
		{"oom beats timed out", Outcome{TimedOut: true, OOMKilled: true, Signal: sigKILL}, "x", types.StatusMemoryExceeded},
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

// TestCapBufferBoundary verifies that capBuffer returns len(p) (not n<len(p))
// even when truncation occurs. Returning n<len(p) causes os/exec io.Copy to
// fail with ErrShortWrite -> spurious 500. This is §7 bug #1.
func TestCapBufferBoundary(t *testing.T) {
	const cap = 64 * 1024

	t.Run("write exactly at cap", func(t *testing.T) {
		cb := &capBuffer{cap: cap}
		data := strings.Repeat("x", cap)
		n, err := cb.Write([]byte(data))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != len(data) {
			t.Fatalf("Write returned %d; want %d", n, len(data))
		}
		if cb.truncated {
			t.Fatal("should not be truncated at exactly cap")
		}
	})

	t.Run("write over cap truncates and returns len(p)", func(t *testing.T) {
		cb := &capBuffer{cap: cap}
		// First fill to cap.
		first := strings.Repeat("a", cap)
		cb.Write([]byte(first)) //nolint:errcheck

		// Now write more — must return len(p), not 0 or partial.
		overflow := []byte("this should be truncated")
		n, err := cb.Write(overflow)
		if err != nil {
			t.Fatalf("truncated write returned error: %v", err)
		}
		if n != len(overflow) {
			t.Fatalf("truncated Write returned %d; want %d (ErrShortWrite avoidance)", n, len(overflow))
		}
		if !cb.truncated {
			t.Fatal("should be marked truncated")
		}
	})

	t.Run("write spanning boundary returns len(p)", func(t *testing.T) {
		cb := &capBuffer{cap: 10}
		data := []byte("123456789012345") // 15 bytes, cap=10
		n, err := cb.Write(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != len(data) {
			t.Fatalf("boundary Write returned %d; want %d", n, len(data))
		}
		if !cb.truncated {
			t.Fatal("should be truncated after spanning cap")
		}
	})
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
