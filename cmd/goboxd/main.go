package main

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"goboxd/internal/api"
	"goboxd/internal/config"
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

	lim := limiter.New(srvCfg.MaxConcurrentJobs)
	run := runner.New(srvCfg, reg)
	srv := api.NewServer(srvCfg, reg, lim, run, log)

	log.Info("starting", "addr", srvCfg.HTTPAddr, "languages", reg.IDs(), "max_concurrent", srvCfg.MaxConcurrentJobs)
	if err := http.ListenAndServe(srvCfg.HTTPAddr, srv.Handler()); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
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
}
