// internal/health/probe_test.go
package health

import (
	"context"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"goboxd/internal/config"
)

func TestProbeTargetResolutionOrder(t *testing.T) {
	cases := []struct {
		name    string
		lang    config.LanguageSpec
		wantCmd string
	}{
		{
			name:    "smoke_probe_cmd wins",
			lang:    config.LanguageSpec{SmokeProbeCmd: "/usr/bin/gcc", Run: &config.PhaseSpec{Cmd: "./{{artifact}}"}},
			wantCmd: "/usr/bin/gcc",
		},
		{
			name:    "build cmd when smoke_probe_cmd unset",
			lang:    config.LanguageSpec{Build: &config.PhaseSpec{Cmd: "/usr/bin/g++"}, Run: &config.PhaseSpec{Cmd: "./{{artifact}}"}},
			wantCmd: "/usr/bin/g++",
		},
		{
			name:    "run cmd for interpreted",
			lang:    config.LanguageSpec{Run: &config.PhaseSpec{Cmd: "/usr/bin/python3"}},
			wantCmd: "/usr/bin/python3",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, args := probeTarget(&tc.lang)
			if got != tc.wantCmd {
				t.Fatalf("cmd = %q, want %q", got, tc.wantCmd)
			}
			if len(args) != 1 || args[0] != "--version" {
				t.Fatalf("args = %v, want [--version]", args)
			}
		})
	}
}

func TestProbeTargetCustomArgs(t *testing.T) {
	lang := config.LanguageSpec{
		Run:        &config.PhaseSpec{Cmd: "/usr/bin/node"},
		SmokeProbe: []string{"-v"},
	}
	_, args := probeTarget(&lang)
	if len(args) != 1 || args[0] != "-v" {
		t.Fatalf("custom smoke probe args ignored: %v", args)
	}
}

func TestCheckCachesUntilTTL(t *testing.T) {
	reg := newSingleLangRegistry(t, "noop", "/bin/true")
	p := NewProbe(reg, "/bin/true", 500*time.Millisecond)

	first := p.Check(context.Background())
	second := p.Check(context.Background())
	if !first.CachedAt.Equal(second.CachedAt) {
		t.Fatalf("cache served a fresh value: %v vs %v", first.CachedAt, second.CachedAt)
	}

	time.Sleep(550 * time.Millisecond)
	third := p.Check(context.Background())
	if !third.CachedAt.After(first.CachedAt) {
		t.Fatalf("cache did not refresh after TTL")
	}
}

func TestCheckSingleflightUnderConcurrentCallers(t *testing.T) {
	reg := newSingleLangRegistry(t, "noop", "/bin/true")
	p := NewProbe(reg, "/bin/true", time.Hour)

	const N = 50
	var done atomic.Int32
	start := make(chan struct{})
	for i := 0; i < N; i++ {
		go func() {
			<-start
			_ = p.Check(context.Background())
			done.Add(1)
		}()
	}
	close(start)
	deadline := time.Now().Add(5 * time.Second)
	for done.Load() < N && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	if done.Load() != N {
		t.Fatalf("only %d/%d callers returned", done.Load(), N)
	}
}

func newSingleLangRegistry(t *testing.T, id, cmd string) *config.Registry {
	t.Helper()
	yml := []byte("languages:\n" +
		"  - id: \"" + id + "\"\n" +
		"    name: \"" + id + "\"\n" +
		"    source_filename: x\n" +
		"    source_filename_strategy: fixed\n" +
		"    run:\n" +
		"      cmd: \"" + cmd + "\"\n" +
		"      limits:\n" +
		"        wall_time_s: 1\n" +
		"        memory_kb: 1024\n" +
		"        max_processes: 4\n")
	path := t.TempDir() + "/langs.yaml"
	if err := os.WriteFile(path, yml, 0644); err != nil {
		t.Fatal(err)
	}
	r, err := config.LoadLanguages(path)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
