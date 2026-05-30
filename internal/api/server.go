package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"goboxd/internal/config"
	"goboxd/internal/limiter"
	"goboxd/internal/runner"
	"goboxd/internal/types"
	"goboxd/internal/validator"
)

type Server struct {
	srv     *config.ServerConfig
	reg     *config.Registry
	limiter *limiter.Limiter
	runner  *runner.Runner
	log     *slog.Logger
}

func NewServer(srv *config.ServerConfig, reg *config.Registry, lim *limiter.Limiter, run *runner.Runner, log *slog.Logger) *Server {
	return &Server{srv: srv, reg: reg, limiter: lim, runner: run, log: log}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("POST /run", s.handleRun)
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req types.RunRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, validator.CodeBadJSON, "malformed JSON request")
		return
	}

	vErr := validator.Validate(&req, s.reg, s.srv)
	if vErr != nil {
		writeError(w, http.StatusBadRequest, vErr.Code, vErr.Message)
		return
	}

	if err := s.limiter.Acquire(r.Context()); err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		writeError(w, http.StatusServiceUnavailable, "queue_full", err.Error())
		return
	}
	defer s.limiter.Release()

	resp, err := s.runner.Run(r.Context(), &req)
	if err != nil {
		s.log.Error("runner failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "execution failed")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
