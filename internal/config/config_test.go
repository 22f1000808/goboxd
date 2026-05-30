package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadServerDefaults(t *testing.T) {
	p := writeTemp(t, "server.yaml", "http_addr: \":9000\"\n")
	c, err := LoadServer(p)
	if err != nil {
		t.Fatal(err)
	}

	if c.HTTPAddr != ":9000" {
		t.Fatalf("addr = %q", c.HTTPAddr)
	}

	if c.MaxConcurrentJobs < 1 {
		t.Fatalf("max_concurrent_jobs should default to >=1, got %d", c.MaxConcurrentJobs)
	}
	if c.MaxSourceBytes != 256*1024 {
		t.Fatalf("max_source_bytes default wrong: %d", c.MaxSourceBytes)
	}
}

func TestLoadServerRejectsUnknownField(t *testing.T) {
	p := writeTemp(t, "server.yaml", "wat: 1\n")
	_, err := LoadServer(p)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestLoadLanguagesHappyPath(t *testing.T) {
	body := `languages:
  - id: py3
    name: Python 3
    source_filename: solution.py
    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]
      limits: { wall_time_s: 9, memory_kb: 102400, max_processes: 100 }
`
	p := writeTemp(t, "langs.yaml", body)
	r, err := LoadLanguages(p)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := r.Lookup("py3"); !ok {
		t.Fatal("py3 not registered")
	}

	if got := r.IDs(); len(got) != 1 || got[0] != "py3" {
		t.Fatalf("IDs = %v", got)
	}
}

func TestLoadLanguagesRejectsMissingRun(t *testing.T) {
	body := `languages:
  - id: py3
    source_filename: x.py
`
	p := writeTemp(t, "langs.yaml", body)
	if _, err := LoadLanguages(p); err == nil {
		t.Fatal("expected error for missing run")
	}
}

func TestLoadLanguagesRejectsDuplicateID(t *testing.T) {
	body := `languages:
  - id: py3
    source_filename: a.py
    run: { cmd: /x, limits: { wall_time_s: 1, memory_kb: 1024, max_processes: 1 } }
  - id: py3
    source_filename: b.py
    run: { cmd: /y, limits: { wall_time_s: 1, memory_kb: 1024, max_processes: 1 } }
`
	p := writeTemp(t, "langs.yaml", body)
	if _, err := LoadLanguages(p); err == nil {
		t.Fatal("expected duplicate id error")
	}
}
