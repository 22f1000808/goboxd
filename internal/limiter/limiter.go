package limiter

import "context"

type Limiter struct {
	sem chan struct{}
}

func New(max int) *Limiter {
	if max < 1 {
		max = 1
	}
	return &Limiter{sem: make(chan struct{}, max)}
}

func (l *Limiter) Acquire(ctx context.Context) error {
	select {
	case l.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *Limiter) Release() {
	<-l.sem
}

func (l *Limiter) InFlight() int {
	return len(l.sem)
}

func (l *Limiter) Capacity() int {
	return cap(l.sem)
}
