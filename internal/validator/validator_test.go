package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goboxd/internal/config"
	"goboxd/internal/types"
)

func loadFixture(t *testing.T) (*config.ServerConfig, *config.Registry) {
	t.Helper()
	dir := t.TempDir()
	srv := filepath.Join(dir, "server.yaml")
	if err := os.WriteFile(srv, []byte(`max_source_bytes: 100
max_stdin_bytes: 50
max_tests: 50`), 0644); err != nil {
		t.Fatal(err)
	}
	langs := filepath.Join(dir, "langs.yaml")
	if err := os.WriteFile(langs, []byte(`languages:
  - id: py3
    source_filename: solution.py
    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]
      limits: { wall_time_s: 9, memory_kb: 102400, max_processes: 100 }
      flag_allowlist: ["-O", "-W*"]
`), 0644); err != nil {
		t.Fatal(err)
	}
	sc, err := config.LoadServer(srv)
	if err != nil {
		t.Fatal(err)
	}
	reg, err := config.LoadLanguages(langs)
	if err != nil {
		t.Fatal(err)
	}
	return sc, reg
}

func base() *types.RunRequest {
	return &types.RunRequest{
		Language: "py3",
		Source:   "print(1)\n",
		Tests:    []types.Test{{Stdin: "", ExpectedStdout: "1\n"}},
	}
}

func TestValidateHappy(t *testing.T) {
	sc, reg := loadFixture(t)
	if err := Validate(base(), reg, sc); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestValidateCases(t *testing.T) {
	sc, reg := loadFixture(t)
	cases := []struct {
		name string
		mut  func(r *types.RunRequest)
		code string
	}{
		{"unknown lang", func(r *types.RunRequest) { r.Language = "ml" }, CodeUnknownLanguage},
		{"source too large", func(r *types.RunRequest) { r.Source = strings.Repeat("x", 101) }, CodeSourceTooLarge},
		{"no tests", func(r *types.RunRequest) { r.Tests = nil }, CodeNoTests},
		{"too many tests", func(r *types.RunRequest) {
			r.Tests = make([]types.Test, 51)
			for i := range r.Tests {
				r.Tests[i] = types.Test{ExpectedStdout: "x"}
			}
		}, CodeTooManyTests},
		{"stdin too large", func(r *types.RunRequest) {
			r.Tests[0].Stdin = strings.Repeat("x", 51)
		}, CodeStdinTooLarge},
		{"filename with slash", func(r *types.RunRequest) {
			r.SourceFilename = "../etc/passwd"
		}, CodeInvalidFilename},
		{"wrong fixed filename", func(r *types.RunRequest) {
			r.SourceFilename = "other.py"
		}, CodeInvalidFilename},
		{"disallowed flag", func(r *types.RunRequest) {
			r.Run = &types.Phase{Flags: []string{"-Xevil"}}
		}, CodeFlagNotAllowed},
		{"prefix allowlist accepts -Wall", func(r *types.RunRequest) {
			r.Run = &types.Phase{Flags: []string{"-Wall"}}
		}, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := base()
			c.mut(r)
			err := Validate(r, reg, sc)
			if c.code == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected %s, got nil", c.code)
			}
			if err.Code != c.code {
				t.Fatalf("got code %q; want %q (msg=%q)", err.Code, c.code, err.Message)
			}
		})
	}
}
