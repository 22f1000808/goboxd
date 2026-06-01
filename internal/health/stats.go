// internal/health/stats.go
// Package health serves /readyz and /info. Stats are atomic counters
// fed by the request path; Probe runs each language's smoke probe with a
// 30s result cache so /readyz can be polled cheaply by orchestrators.
package health

import (
	"sync/atomic"
	"time"
)

type Stats struct {
	JobsTotal           atomic.Int64
	JobsSucceeded       atomic.Int64
	JobsFailedInternal  atomic.Int64
	JobsQueueFull       atomic.Int64
	LastInternalErrorAt atomic.Pointer[time.Time]

	startedAt time.Time
}

func NewStats() *Stats {
	return &Stats{startedAt: time.Now().UTC()}
}

func (s *Stats) IncJob()       { s.JobsTotal.Add(1) }
func (s *Stats) IncSucceeded() { s.JobsSucceeded.Add(1) }
func (s *Stats) IncQueueFull() { s.JobsQueueFull.Add(1) }
func (s *Stats) IncInternal() {
	s.JobsFailedInternal.Add(1)
	now := time.Now().UTC()
	s.LastInternalErrorAt.Store(&now)
}

type Snapshot struct {
	JobsTotal           int64      `json:"jobs_total"`
	JobsSucceeded       int64      `json:"jobs_succeeded"`
	JobsFailedInternal  int64      `json:"jobs_failed_internal"`
	JobsQueueFull       int64      `json:"jobs_queue_full"`
	LastInternalErrorAt *time.Time `json:"last_internal_error_at,omitempty"`
	StartedAt           time.Time  `json:"started_at"`
	UptimeS             int64      `json:"uptime_s"`
}

func (s *Stats) Snapshot() Snapshot {
	return Snapshot{
		JobsTotal:           s.JobsTotal.Load(),
		JobsSucceeded:       s.JobsSucceeded.Load(),
		JobsFailedInternal:  s.JobsFailedInternal.Load(),
		JobsQueueFull:       s.JobsQueueFull.Load(),
		LastInternalErrorAt: s.LastInternalErrorAt.Load(),
		StartedAt:           s.startedAt,
		UptimeS:             int64(time.Since(s.startedAt).Seconds()),
	}
}
