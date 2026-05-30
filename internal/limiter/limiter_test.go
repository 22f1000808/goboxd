package limiter

import (
	"context"
	"testing"
	"time"
)

func TestAcquireRelease(t *testing.T) {
	l := New(2)
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

func TestAcquireCancels(t *testing.T) {
	l := New(1)
	_ = l.Acquire(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := l.Acquire(ctx); err == nil {
		t.Fatal("expected ctx error")
	}
}
