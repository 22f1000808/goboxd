// cmd/goboxd/main.go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"goboxd/internal/api"
	"goboxd/internal/config"
	"goboxd/internal/health"
	"goboxd/internal/limiter"
	"goboxd/internal/runner"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	srvCfgPath := envOr("GOBOXD_SERVER_CONFIG", "configs/server.yaml")
	langsCfgPath := envOr("GOBOXD_LANGUAGES_CONFIG", "configs/languages.yaml")

	srvCfg, err := config.LoadServer(srvCfgPath)
	if err != nil {
		log.Error("load server config", "err", err)
		os.Exit(1)
	}
	applyServerEnv(srvCfg)
	reg, err := config.LoadLanguages(langsCfgPath)
	if err != nil {
		log.Error("load languages", "err", err)
		os.Exit(1)
	}

	lim := limiter.New(srvCfg.MaxConcurrentJobs, srvCfg.MaxQueueDepth)
	jobs := limiter.NewJobRegistry()
	run := runner.New(srvCfg, reg)
	stats := health.NewStats()
	probe := health.NewProbe(reg, srvCfg.NSJailBinary, time.Duration(srvCfg.ReadyzCacheTTLS)*time.Second)

	srv := api.New(api.Options{
		Server: srvCfg, Registry: reg, Limiter: lim, Jobs: jobs,
		Runner: run, Probe: probe, Stats: stats, Log: log,
	})

	httpSrv := &http.Server{
		Addr:              srvCfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 14,
	}

	rootCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()

	sw := runner.NewSweeper(srvCfg.JailRootDir, log)
	go sw.Run(rootCtx)

	log.Info("starting",
		"addr", srvCfg.HTTPAddr,
		"languages", reg.IDs(),
		"max_concurrent", srvCfg.MaxConcurrentJobs,
		"max_queue_depth", srvCfg.MaxQueueDepth,
		"drain_timeout_s", srvCfg.DrainTimeoutS,
	)

	serverErr := make(chan error, 1)
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			log.Error("listener error", "err", err)
			os.Exit(1)
		}
	case <-rootCtx.Done():
		log.Info("shutdown signal received; draining")
	}

	srv.SetDraining(true)
	drainCtx, cancel := context.WithTimeout(context.Background(), time.Duration(srvCfg.DrainTimeoutS)*time.Second)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
loop:
	for jobs.Count() > 0 {
		select {
		case <-ticker.C:
		case <-drainCtx.Done():
			break loop
		}
	}

	if drainCtx.Err() != nil {
		log.Warn("graceful shutdown overran; force-killing in-flight jobs")
		n := jobs.ForceKillAll()
		log.Info("force-killed jobs", "count", n)
		_ = httpSrv.Close()
	} else {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutdownCancel()
		_ = httpSrv.Shutdown(shutdownCtx)
		log.Info("graceful shutdown complete")
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func applyServerEnv(c *config.ServerConfig) {
	if v := os.Getenv("GOBOXD_HTTP_ADDR"); v != "" {
		c.HTTPAddr = v
	}
	if v := os.Getenv("GOBOXD_MAX_CONCURRENT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.MaxConcurrentJobs = n
		}
	}
	if v := os.Getenv("GOBOXD_MAX_QUEUE_DEPTH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			c.MaxQueueDepth = n
		}
	}
	if v := os.Getenv("GOBOXD_DRAIN_TIMEOUT_S"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			c.DrainTimeoutS = n
		}
	}
}
