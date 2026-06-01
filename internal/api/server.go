// internal/api/server.go
// Package api is the thinnest HTTP layer that satisfies the spec.
// Routes, JSON decode, hand to the runner, encode. Business logic lives
// in runner/, validation in validator/, sandboxing in jail/. This
// package owns request-id, body caps, recovery, log emission, and the
// drain flag that flips /readyz to 503 during graceful shutdown.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	"goboxd/internal/config"
	"goboxd/internal/health"
	"goboxd/internal/limiter"
	"goboxd/internal/logging"
	"goboxd/internal/runner"
	"goboxd/internal/types"
	"goboxd/internal/validator"
)

type Server struct {
	srv     *config.ServerConfig
	reg     *config.Registry
	limiter *limiter.Limiter
	jobs    *limiter.JobRegistry
	runner  *runner.Runner
	probe   *health.Probe
	stats   *health.Stats
	log     *slog.Logger

	draining atomic.Bool
}

type Options struct {
	Server   *config.ServerConfig
	Registry *config.Registry
	Limiter  *limiter.Limiter
	Jobs     *limiter.JobRegistry
	Runner   *runner.Runner
	Probe    *health.Probe
	Stats    *health.Stats
	Log      *slog.Logger
}

func New(o Options) *Server {
	logging.RequestIDFromContext = RequestIDFrom
	return &Server{
		srv: o.Server, reg: o.Registry, limiter: o.Limiter, jobs: o.Jobs,
		runner: o.Runner, probe: o.Probe, stats: o.Stats, log: o.Log,
	}
}

func (s *Server) SetDraining(v bool) { s.draining.Store(v) }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("GET /info", s.handleInfo)
	mux.HandleFunc("POST /run", s.handleRun)
	wrap := chain(requestIDMiddleware, recoverMiddleware(s.log))
	return wrap(mux)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if s.draining.Load() {
		report := s.probe.Cached()
		report.Status = health.StatusReadyDegraded
		if report.Languages == nil {
			report.Languages = map[string]health.ProbeResult{}
		}
		if report.CachedAt.IsZero() {
			report.CachedAt = time.Now().UTC()
		}
		writeJSON(w, http.StatusServiceUnavailable, report)
		return
	}
	report := s.probe.Check(r.Context())
	status := http.StatusOK
	if !report.Ready() {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, report)
}

type InfoReport struct {
	BuildInfo buildInfo      `json:"build_info"`
	NSJail    InfoNSJail     `json:"nsjail"`
	Languages []InfoLanguage `json:"languages"`
	Limits    InfoLimits     `json:"limits"`
	Stats     InfoStats      `json:"stats"`
}

type InfoNSJail struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

type InfoLanguage struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Version          string       `json:"version,omitempty"`
	DefaultRunLimits InfoLimitSet `json:"default_run_limits"`
}

type InfoLimitSet struct {
	WallTimeS    int `json:"wall_time_s"`
	MemoryKB     int `json:"memory_kb"`
	MaxProcesses int `json:"max_processes"`
}

type InfoLimits struct {
	MaxSourceBytes    int `json:"max_source_bytes"`
	MaxStdinBytes     int `json:"max_stdin_bytes"`
	MaxTests          int `json:"max_tests"`
	MaxConcurrentJobs int `json:"max_concurrent_jobs"`
	MaxQueueDepth     int `json:"max_queue_depth"`
}

type InfoStats struct {
	InFlightJobs         int    `json:"in_flight_jobs"`
	WaitingJobs          int    `json:"waiting_jobs"`
	JobsTotal            int64  `json:"jobs_total"`
	JobsSucceeded        int64  `json:"jobs_succeeded"`
	JobsFailedInternal   int64  `json:"jobs_failed_internal"`
	JobsQueueFull        int64  `json:"jobs_queue_full"`
	LastInternalErrorAt  string `json:"last_internal_error_at,omitempty"`
	DiskFreeBytesJailDir int64  `json:"disk_free_bytes_jail_dir"`
	NumGoroutine         int    `json:"num_goroutine"`
	StartedAt            string `json:"started_at"`
	UptimeS              int64  `json:"uptime_s"`
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	probe := s.probe.Check(r.Context())

	langs := make([]InfoLanguage, 0, len(s.reg.IDs()))
	for _, l := range s.reg.All() {
		v := ""
		if pr, ok := probe.Languages[l.ID]; ok {
			v = pr.Version
		}
		langs = append(langs, InfoLanguage{
			ID: l.ID, Name: l.Name, Version: v,
			DefaultRunLimits: InfoLimitSet{
				WallTimeS:    l.Run.Limits.WallTimeS,
				MemoryKB:     l.Run.Limits.MemoryKB,
				MaxProcesses: l.Run.Limits.MaxProcesses,
			},
		})
	}

	snap := s.stats.Snapshot()
	lastErr := ""
	if snap.LastInternalErrorAt != nil {
		lastErr = snap.LastInternalErrorAt.Format(time.RFC3339)
	}

	info := InfoReport{
		BuildInfo: currentBuildInfo(),
		NSJail: InfoNSJail{
			Path:    s.srv.NSJailBinary,
			Version: probe.NSJail.Version,
		},
		Languages: langs,
		Limits: InfoLimits{
			MaxSourceBytes:    s.srv.MaxSourceBytes,
			MaxStdinBytes:     s.srv.MaxStdinBytes,
			MaxTests:          s.srv.MaxTests,
			MaxConcurrentJobs: s.srv.MaxConcurrentJobs,
			MaxQueueDepth:     s.srv.MaxQueueDepth,
		},
		Stats: InfoStats{
			InFlightJobs:         s.limiter.InFlight(),
			WaitingJobs:          s.limiter.Waiting(),
			JobsTotal:            snap.JobsTotal,
			JobsSucceeded:        snap.JobsSucceeded,
			JobsFailedInternal:   snap.JobsFailedInternal,
			JobsQueueFull:        snap.JobsQueueFull,
			LastInternalErrorAt:  lastErr,
			DiskFreeBytesJailDir: diskFreeBytes(s.srv.JailRootDir),
			NumGoroutine:         runtime.NumGoroutine(),
			StartedAt:            snap.StartedAt.Format(time.RFC3339),
			UptimeS:              snap.UptimeS,
		},
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if s.draining.Load() {
		w.Header().Set("Retry-After", "1")
		writeError(w, http.StatusServiceUnavailable, "draining", "server is shutting down")
		logging.Emit(r.Context(), s.log, &types.RunRequest{}, "draining", time.Since(start), http.StatusServiceUnavailable, "draining")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req types.RunRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, validator.CodeBadJSON, "malformed json")
		logging.Emit(r.Context(), s.log, &req, "bad_json", time.Since(start), http.StatusBadRequest, validator.CodeBadJSON)
		return
	}

	if vErr := validator.Validate(&req, s.reg, s.srv); vErr != nil {
		writeError(w, http.StatusBadRequest, vErr.Code, vErr.Message)
		logging.Emit(r.Context(), s.log, &req, "rejected", time.Since(start), http.StatusBadRequest, vErr.Code)
		return
	}

	if err := s.limiter.Acquire(r.Context()); err != nil {
		if errors.Is(err, limiter.ErrQueueFull) {
			s.stats.IncQueueFull()
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusServiceUnavailable, "queue_full", "server busy")
			logging.Emit(r.Context(), s.log, &req, "queue_full", time.Since(start), http.StatusServiceUnavailable, "queue_full")
			return
		}
		if errors.Is(err, context.Canceled) {
			return
		}
		s.stats.IncInternal()
		writeError(w, http.StatusInternalServerError, "internal_error", "acquire failed")
		logging.Emit(r.Context(), s.log, &req, "internal_error", time.Since(start), http.StatusInternalServerError, "internal_error")
		return
	}
	defer s.limiter.Release()

	s.stats.IncJob()

	jobCtx, cancel := context.WithCancel(r.Context())
	defer cancel()

	jobID := RequestIDFrom(r.Context())
	if jobID == "" {
		jobID = newRequestID()
	}
	s.jobs.Track(jobID, cancel)
	defer s.jobs.Untrack(jobID)

	resp, err := s.runner.Run(jobCtx, &req)
	if err != nil {
		s.stats.IncInternal()
		s.log.Error("runner failed", "request_id", jobID, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "execution failed")
		logging.Emit(r.Context(), s.log, &req, "internal_error", time.Since(start), http.StatusInternalServerError, "internal_error")
		return
	}
	s.stats.IncSucceeded()
	writeJSON(w, http.StatusOK, resp)
	logging.Emit(r.Context(), s.log, &req, resp.Status, time.Since(start), http.StatusOK, "")
}
