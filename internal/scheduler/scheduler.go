package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

type Scheduler struct {
	interval time.Duration
	tickFn   func(context.Context)

	running atomic.Bool

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func New(interval time.Duration, tickFn func(context.Context)) (*Scheduler, error) {
	if interval <= 0 {
		return nil, errors.New("interval must be > 0")
	}
	if tickFn == nil {
		return nil, errors.New("tickFn must not be nil")
	}
	return &Scheduler{
		interval: interval,
		tickFn:   tickFn,
		done:     make(chan struct{}),
	}, nil
}

func (s *Scheduler) Start() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running.Load() {
		return false
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.done = make(chan struct{})
	s.running.Store(true)

	go func() {
		defer close(s.done)

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		slog.Info("scheduler started", "interval", s.interval.String())

		s.safeTick(ctx)

		for {
			select {
			case <-ctx.Done():
				slog.Info("scheduler stopping")
				return
			case <-ticker.C:
				s.safeTick(ctx)
			}
		}
	}()

	return true
}

func (s *Scheduler) Stop() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running.Load() {
		return false
	}

	s.cancel()
	<-s.done
	s.running.Store(false)

	slog.Info("scheduler stopped")
	return true
}

func (s *Scheduler) IsRunning() bool {
	return s.running.Load()
}

func (s *Scheduler) safeTick(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("scheduler tick panic recovered", "panic", r)
		}
	}()

	start := time.Now()
	s.tickFn(ctx)
	slog.Info("scheduler tick completed", "duration_ms", time.Since(start).Milliseconds())
}
