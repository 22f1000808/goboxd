// internal/limiter/limiter_test.go
package limiter

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAcquireRelease(t *testing.T) {
	l := New(2, 4)
	ctx := context.Background()
	if err := l.Acquire(ctx); err != nil {
		t.Fatal(err)
	}
	if err := l.Acquire(ctx); err != nil {
		t.Fatal(err)
	}
	if l.InFlight() != 2 {
		t.Fatalf("inflight = %d", l.InFlight())
	}
	l.Release()
	if l.InFlight() != 1 {
		t.Fatalf("after release inflight = %d", l.InFlight())
	}
}

func TestAcquireCancelDoesNotConsumeSlot(t *testing.T) {
	l := New(1, 4)
	_ = l.Acquire(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := l.Acquire(ctx); err == nil {
		t.Fatal("expected ctx error")
	}
	if l.InFlight() != 1 {
		t.Fatalf("cancel leaked a slot: inflight=%d", l.InFlight())
	}
	if l.Waiting() != 0 {
		t.Fatalf("waiting not drained: %d", l.Waiting())
	}
}

func TestQueueFullAtCap(t *testing.T) {
	l := New(1, 2)
	_ = l.Acquire(context.Background())

	ready := make(chan struct{}, 2)
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			ready <- struct{}{}
			done <- l.Acquire(context.Background())
		}()
	}
	<-ready
	<-ready
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := l.Acquire(ctx); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}

	l.Release()
	if err := <-done; err != nil {
		t.Fatalf("waiter 1: %v", err)
	}
	l.Release()
	if err := <-done; err != nil {
		t.Fatalf("waiter 2: %v", err)
	}
}

func TestUnboundedQueueByDefault(t *testing.T) {
	l := New(1, 0)
	_ = l.Acquire(context.Background())

	const N = 32
	done := make(chan struct{}, N)
	for i := 0; i < N; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			_ = l.Acquire(ctx)
			done <- struct{}{}
		}()
	}

	time.Sleep(20 * time.Millisecond)
	if w := l.Waiting(); w != N {
		t.Fatalf("waiting = %d, want %d (no waiter should have been refused)", w, N)
	}

	for i := 0; i < N; i++ {
		<-done
	}
}

func TestInFlightUnderRace(t *testing.T) {
	l := New(8, 64)
	const N = 200
	var wg sync.WaitGroup
	wg.Add(N)
	var seenOver atomic.Int32
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			if err := l.Acquire(context.Background()); err != nil {
				return
			}
			if l.InFlight() > l.Capacity() {
				seenOver.Add(1)
			}
			l.Release()
		}()
	}
	wg.Wait()
	if seenOver.Load() != 0 {
		t.Fatalf("inflight exceeded capacity %d times", seenOver.Load())
	}
}

func TestJobRegistryForceKillAll(t *testing.T) {
	reg := NewJobRegistry()
	ctxA, cancelA := context.WithCancel(context.Background())
	ctxB, cancelB := context.WithCancel(context.Background())
	reg.Track("a", cancelA)
	reg.Track("b", cancelB)
	if reg.Count() != 2 {
		t.Fatalf("count = %d", reg.Count())
	}
	if n := reg.ForceKillAll(); n != 2 {
		t.Fatalf("ForceKillAll returned %d", n)
	}
	select {
	case <-ctxA.Done():
	default:
		t.Fatal("ctxA not cancelled")
	}
	select {
	case <-ctxB.Done():
	default:
		t.Fatal("ctxB not cancelled")
	}
	if reg.Count() != 0 {
		t.Fatalf("registry not drained: %d", reg.Count())
	}
}
