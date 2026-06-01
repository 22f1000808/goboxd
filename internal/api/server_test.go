// internal/api/server_test.go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goboxd/internal/config"
	"goboxd/internal/health"
	"goboxd/internal/limiter"
	"goboxd/internal/runner"
)

func testServer(t *testing.T, maxConcurrent, maxQueueDepth int) (*Server, *limiter.Limiter) {
	t.Helper()
	dir := t.TempDir()
	langPath := filepath.Join(dir, "languages.yaml")
	probeBin := "/bin/true"
	if _, err := os.Stat(probeBin); err != nil {
		probeBin = os.Getenv("COMSPEC")
		if probeBin == "" {
			probeBin = "echo"
		}
	}
	yml := "languages:\n" +
		"  - id: py3\n" +
		"    name: Python 3\n" +
		"    source_filename: solution.py\n" +
		"    run:\n" +
		"      cmd: " + probeBin + "\n" +
		"      args: [\"{source}\"]\n" +
		"      limits: { wall_time_s: 1, memory_kb: 1024, max_processes: 10 }\n" +
		"    smoke_probe_cmd: " + probeBin + "\n"
	if err := os.WriteFile(langPath, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}

	reg, err := config.LoadLanguages(langPath)
	if err != nil {
		t.Fatalf("LoadLanguages: %v", err)
	}

	srvCfg := &config.ServerConfig{
		HTTPAddr:          ":0",
		MaxConcurrentJobs: maxConcurrent,
		MaxQueueDepth:     maxQueueDepth,
		DrainTimeoutS:     1,
		ReadyzCacheTTLS:   1,
		MaxSourceBytes:    1024,
		MaxStdinBytes:     64,
		MaxTests:          10,
		JailRootDir:       t.TempDir(),
		NSJailBinary:      probeBin,
	}

	lim := limiter.New(maxConcurrent, maxQueueDepth)
	jobs := limiter.NewJobRegistry()
	stats := health.NewStats()
	probe := health.NewProbe(reg, probeBin, time.Second)
	run := runner.New(srvCfg, reg)
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	srv := New(Options{
		Server: srvCfg, Registry: reg, Limiter: lim, Jobs: jobs,
		Runner: run, Probe: probe, Stats: stats, Log: log,
	})
	return srv, lim
}

func TestHealthzAlways200(t *testing.T) {
	srv, _ := testServer(t, 1, 0)
	srv.SetDraining(true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d want 200", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("got status %q want ok", body["status"])
	}
}

func TestReadyzDrainingReturnsDegraded(t *testing.T) {
	srv, _ := testServer(t, 1, 0)
	srv.SetDraining(true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d want 503", rec.Code)
	}

	var body health.ReadinessReport
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != health.StatusReadyDegraded {
		t.Fatalf("status=%q want degraded", body.Status)
	}
}

func TestRunDrainingReturns503(t *testing.T) {
	srv, _ := testServer(t, 1, 0)
	srv.SetDraining(true)
	body := `{"language":"py3","source":"print(1)","tests":[{"stdin":"","expected_stdout":"1"}]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/run", strings.NewReader(body))
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d want 503", rec.Code)
	}

	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After=%q want 1", got)
	}

	if !bytes.Contains(rec.Body.Bytes(), []byte(`"code":"draining"`)) {
		t.Fatalf("body missing draining code: %s", rec.Body.String())
	}
}

func TestRunBadJSONReturns400(t *testing.T) {
	srv, _ := testServer(t, 1, 0)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/run", strings.NewReader("not-json"))
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d want 400", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"code":"bad_json"`)) {
		t.Fatalf("body missing bad_json code: %s", rec.Body.String())
	}
}

func TestRunQueueFullReturns503(t *testing.T) {
	srv, lim := testServer(t, 1, 1)
	if err := lim.Acquire(context.Background()); err != nil {
		t.Fatalf("prime acquire: %v", err)
	}
	defer lim.Release()
	waiterCtx, waiterCancel := context.WithCancel(context.Background())
	waiterDone := make(chan struct{})
	go func() {
		_ = lim.Acquire(waiterCtx)
		close(waiterDone)
	}()
	for i := 0; i < 50 && lim.Waiting() == 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if lim.Waiting() == 0 {
		waiterCancel()
		<-waiterDone
		t.Fatal("waiter never registered")
	}
	defer func() {
		waiterCancel()
		<-waiterDone
	}()
	body := `{"language":"py3","source":"print(1)","tests":[{"stdin":"","expected_stdout":"1"}]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/run", strings.NewReader(body))
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d want 503; body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After=%q want 1", got)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"code":"queue_full"`)) {
		t.Fatalf("body missing queue_full code: %s", rec.Body.String())
	}
}

func TestInfoShape(t *testing.T) {
	srv, _ := testServer(t, 2, 0)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/info", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d want 200", rec.Code)
	}

	var info InfoReport
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if info.BuildInfo.GoVersion == "" {
		t.Error("build_info.go_version empty")
	}
	if len(info.Languages) != 1 || info.Languages[0].ID != "py3" {
		t.Errorf("languages=%+v want single py3", info.Languages)
	}
	if info.Limits.MaxConcurrentJobs != 2 {
		t.Errorf("max_concurrent_jobs=%d want 2", info.Limits.MaxConcurrentJobs)
	}
	if info.NSJail.Path == "" {
		t.Error("nsjail.path empty")
	}
}

func TestRequestIDEchoed(t *testing.T) {
	srv, _ := testServer(t, 1, 0)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-Id", "test-correlation-123")
	srv.Handler().ServeHTTP(rec, req)
	if got := rec.Header().Get("X-Request-Id"); got != "test-correlation-123" {
		t.Fatalf("X-Request-Id=%q want test-correlation-123", got)
	}
}
