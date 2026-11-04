package booking_worker_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	worker "github.com/poponyas/TicketRush/internal/feature/booking/worker"
	"github.com/stretchr/testify/require"
)

type expirerFunc func(context.Context, int) (int, error)

func (f expirerFunc) ExpireBookings(ctx context.Context, limit int) (int, error) {
	return f(ctx, limit)
}

func newWorker(t *testing.T, f expirerFunc, interval time.Duration, batch int) *worker.ExpirationWorker {
	t.Helper()
	w, err := worker.NewExpirationWorker(f, corelogger.FromContext(context.Background()), worker.ExpirationConfig{Interval: interval, BatchSize: batch}, worker.NewExpirationMetrics())
	require.NoError(t, err)
	return w
}
func waitStopped(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("expiration worker did not stop")
	}
}

func TestExpirationWorkerDrainsImmediately(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	w := newWorker(t, func(ctx context.Context, limit int) (int, error) {
		if limit != 2 {
			t.Errorf("unexpected batch size: %d", limit)
		}
		switch calls.Add(1) {
		case 1:
			return 2, nil
		case 2:
			cancel()
			return 1, nil
		default:
			t.Error("unexpected extra batch")
			return 0, nil
		}
	}, time.Hour, 2)
	done := make(chan struct{})
	go func() { defer close(done); w.Run(ctx) }()
	waitStopped(t, done)
	require.EqualValues(t, 2, calls.Load(), "full batch must trigger another pass without waiting for a tick")
}

func TestExpirationWorkerRetriesAfterError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	w := newWorker(t, func(context.Context, int) (int, error) {
		if calls.Add(1) == 1 {
			return 0, errors.New("temporary database failure")
		}
		cancel()
		return 0, nil
	}, time.Millisecond, 10)
	done := make(chan struct{})
	go func() { defer close(done); w.Run(ctx) }()
	waitStopped(t, done)
	require.EqualValues(t, 2, calls.Load())
}

func TestExpirationWorkerPollsAfterEmptyBatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	w := newWorker(t, func(context.Context, int) (int, error) {
		if calls.Add(1) == 2 {
			cancel()
		}
		return 0, nil
	}, time.Millisecond, 10)
	done := make(chan struct{})
	go func() { defer close(done); w.Run(ctx) }()
	waitStopped(t, done)
	require.EqualValues(t, 2, calls.Load())
}

func TestExpirationWorkerCancelsInFlightCall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan struct{})
	w := newWorker(t, func(ctx context.Context, limit int) (int, error) {
		close(entered)
		<-ctx.Done()
		return 0, ctx.Err()
	}, time.Hour, 10)
	done := make(chan struct{})
	go func() { defer close(done); w.Run(ctx) }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not start")
	}
	cancel()
	waitStopped(t, done)
}

func TestExpirationWorkerAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var calls atomic.Int32
	w := newWorker(t, func(context.Context, int) (int, error) { calls.Add(1); return 0, nil }, time.Hour, 10)
	w.Run(ctx)
	require.Zero(t, calls.Load())
}

func TestExpirationWorkerConfiguration(t *testing.T) {
	f := expirerFunc(func(context.Context, int) (int, error) { return 0, nil })
	log := corelogger.FromContext(context.Background())
	for _, cfg := range []worker.ExpirationConfig{
		{Interval: 0, BatchSize: 100}, {Interval: -time.Second, BatchSize: 100},
		{Interval: time.Second, BatchSize: 0}, {Interval: time.Second, BatchSize: 1001},
	} {
		_, err := worker.NewExpirationWorker(f, log, cfg, worker.NewExpirationMetrics())
		require.Error(t, err)
	}
	_, err := worker.NewExpirationWorker(nil, log, worker.DefaultExpirationConfig(), worker.NewExpirationMetrics())
	require.Error(t, err)
	_, err = worker.NewExpirationWorker(f, nil, worker.DefaultExpirationConfig(), worker.NewExpirationMetrics())
	require.Error(t, err)
	_, err = worker.NewExpirationWorker(f, log, worker.DefaultExpirationConfig(), nil)
	require.Error(t, err)
}
