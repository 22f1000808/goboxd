// internal/limiter/limiter.go
// Package limiter is a context-aware counting semaphore with an
// optional waiting-queue cap. Spec §07 says "When the limit is reached,
// requests queue rather than fail" - so the default queue depth is
// unbounded (cap == 0 means no cap). MaxQueueDepth > 0 is an operator
// safety valve to prevent an adversarial burst from pinning unbounded
// goroutines, each holding a decoded 1 MiB RunRequest.
package limiter

import (
	"context"
	"errors"
	"sync/atomic"
)

// ErrQueueFull is returned by Acquire when the number of goroutines
// already waiting on the semaphore is at MaxQueueDepth. The HTTP layer
// maps this to 503 with Retry-After: 1. Only reachable when an operator
// explicitly sets max_queue_depth > 0; the spec default is unbounded.
var ErrQueueFull = errors.New("queue full")

type Limiter struct {
	sem           chan struct{}
	maxQueueDepth int
	waiting       atomic.Int32
}

// New returns a Limiter that admits at most max concurrent slots. If
// maxQueueDepth > 0, waiting callers past the cap get ErrQueueFull
// instead of blocking. maxQueueDepth <= 0 disables the cap (spec
// default).
func New(max, maxQueueDepth int) *Limiter {
	if max < 1 {
		max = 1
	}
	if maxQueueDepth < 0 {
		maxQueueDepth = 0
	}
	return &Limiter{sem: make(chan struct{}, max), maxQueueDepth: maxQueueDepth}
}

// Acquire blocks until a slot is free or ctx is cancelled. If the queue
// of already-waiting callers is at the cap, returns ErrQueueFull
// without blocking.
func (l *Limiter) Acquire(ctx context.Context) error {
	select {
	case l.sem <- struct{}{}:
		return nil
	default:
	}

	if l.maxQueueDepth > 0 {
		if l.waiting.Add(1) > int32(l.maxQueueDepth) {
			l.waiting.Add(-1)
			return ErrQueueFull
		}
		defer l.waiting.Add(-1)
	} else {
		l.waiting.Add(1)
		defer l.waiting.Add(-1)
	}

	select {
	case l.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *Limiter) Release()           { <-l.sem }
func (l *Limiter) InFlight() int      { return len(l.sem) }
func (l *Limiter) Waiting() int       { return int(l.waiting.Load()) }
func (l *Limiter) Capacity() int      { return cap(l.sem) }
func (l *Limiter) MaxQueueDepth() int { return l.maxQueueDepth }
