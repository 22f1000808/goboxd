// internal/logging/logging.go
// Package logging is the only place outside cmd/ that calls slog. One
// structured line per request keeps logs greppable and machine-friendly.
package logging

import (
	"context"
	"log/slog"
	"time"

	"goboxd/internal/types"
)

// RequestIDFromContext is supplied by the api package via Init so the
// logging package does not import api (which would be a cycle).
var RequestIDFromContext = func(ctx context.Context) string { return "" }

// Emit logs one JSON line summarising a completed request. status carries
// the final outcome string; httpStatus and errCode are included for
// structured log queries.
func Emit(ctx context.Context, log *slog.Logger, req *types.RunRequest, status string, dur time.Duration, httpStatus int, errCode string) {
	attrs := []slog.Attr{
		slog.String("request_id", RequestIDFromContext(ctx)),
		slog.String("language", req.Language),
		slog.Int("source_bytes", len(req.Source)),
		slog.Int("tests", len(req.Tests)),
		slog.String("status", status),
		slog.Int("http_status", httpStatus),
		slog.Int64("duration_ms", dur.Milliseconds()),
	}
	if errCode != "" {
		attrs = append(attrs, slog.String("error_code", errCode))
	}
	log.LogAttrs(ctx, slog.LevelInfo, "request_complete", attrs...)
}
