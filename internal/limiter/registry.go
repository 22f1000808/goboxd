// internal/limiter/registry.go
package limiter

import (
	"context"
	"sync"
)

// JobRegistry tracks in-flight requests by id and exposes ForceKillAll,
// which cancels every registered context. The runner wraps each request
// in a cancellable context, registers it on Track, and Untracks in defer.
// On graceful-shutdown overrun the server calls ForceKillAll; jail's
// exec.CommandContext cancel hook then SIGKILLs each payload process
// group.
type JobRegistry struct {
	mu   sync.Mutex
	jobs map[string]context.CancelFunc
}

func NewJobRegistry() *JobRegistry {
	return &JobRegistry{jobs: make(map[string]context.CancelFunc)}
}

// Track records cancel under id. Calling Track twice with the same id
// replaces the prior entry (callers should pick unique ids).
func (r *JobRegistry) Track(id string, cancel context.CancelFunc) {
	r.mu.Lock()
	r.jobs[id] = cancel
	r.mu.Unlock()
}

func (r *JobRegistry) Untrack(id string) {
	r.mu.Lock()
	delete(r.jobs, id)
	r.mu.Unlock()
}

func (r *JobRegistry) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.jobs)
}

// ForceKillAll cancels every tracked context. It does not wait for the
// goroutines that owned those contexts to exit; the caller already gave
// them their drain window.
func (r *JobRegistry) ForceKillAll() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := len(r.jobs)
	for id, c := range r.jobs {
		c()
		delete(r.jobs, id)
	}
	return n
}
