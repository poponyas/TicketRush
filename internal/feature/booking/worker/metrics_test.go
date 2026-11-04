package booking_worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	worker "github.com/poponyas/TicketRush/internal/feature/booking/worker"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func TestExpirationWorkerMetrics(t *testing.T) {
	for _, tc := range []struct {
		name      string
		count     int
		err       error
		cancelled bool
		outcome   string
		expired   float64
	}{
		{name: "expired bookings", count: 3, outcome: "success", expired: 3},
		{name: "empty batch", outcome: "success"},
		{name: "database error", count: 7, err: errors.New("database unavailable"), outcome: "error"},
		{name: "shutdown during batch", cancelled: true, outcome: "cancelled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			metrics := worker.NewExpirationMetrics()
			const previousSuccess = 123.0
			metrics.LastSuccess.Set(previousSuccess)
			registry := prometheus.NewRegistry()
			require.NoError(t, registry.Register(metrics.Batches))
			require.NoError(t, registry.Register(metrics.ExpiredBookings))
			require.NoError(t, registry.Register(metrics.Duration))
			require.NoError(t, registry.Register(metrics.LastSuccess))

			entered := make(chan struct{})
			svc := expirerFunc(func(ctx context.Context, _ int) (int, error) {
				if tc.cancelled {
					close(entered)
					<-ctx.Done()
					return 0, ctx.Err()
				}
				return tc.count, tc.err
			})
			w, err := worker.NewExpirationWorker(svc, corelogger.FromContext(ctx), worker.ExpirationConfig{
				Interval: time.Hour, BatchSize: 10,
			}, metrics)
			require.NoError(t, err)
			startedAt := time.Now()
			done := make(chan struct{})
			go func() { defer close(done); w.Run(ctx) }()
			t.Cleanup(func() { cancel(); waitStopped(t, done) })

			if tc.cancelled {
				select {
				case <-entered:
				case <-time.After(time.Second):
					t.Fatal("expiration batch did not start")
				}
			} else {
				counter := metrics.Batches.WithLabelValues(tc.outcome)
				require.Eventually(t, func() bool {
					var sample dto.Metric
					return counter.Write(&sample) == nil && sample.GetCounter().GetValue() == 1
				}, time.Second, time.Millisecond)
			}
			cancel()
			waitStopped(t, done)

			families, err := registry.Gather()
			require.NoError(t, err)
			require.Len(t, families, 4)
			byName := make(map[string]*dto.MetricFamily, len(families))
			for _, family := range families {
				byName[family.GetName()] = family
			}

			batches := byName["ticketrush_booking_expiration_batches_total"].GetMetric()
			require.Len(t, batches, 1)
			require.Equal(t, 1.0, batches[0].GetCounter().GetValue())
			require.Len(t, batches[0].GetLabel(), 1)
			require.Equal(t, "outcome", batches[0].GetLabel()[0].GetName())
			require.Equal(t, tc.outcome, batches[0].GetLabel()[0].GetValue())

			durations := byName["ticketrush_booking_expiration_batch_duration_seconds"].GetMetric()
			require.Len(t, durations, 1)
			require.EqualValues(t, 1, durations[0].GetHistogram().GetSampleCount())
			require.GreaterOrEqual(t, durations[0].GetHistogram().GetSampleSum(), 0.0)
			require.Len(t, durations[0].GetLabel(), 1)
			require.Equal(t, "outcome", durations[0].GetLabel()[0].GetName())
			require.Equal(t, tc.outcome, durations[0].GetLabel()[0].GetValue())

			expired := byName["ticketrush_bookings_expired_total"].GetMetric()
			require.Len(t, expired, 1)
			require.Equal(t, tc.expired, expired[0].GetCounter().GetValue())

			lastSuccess := byName["ticketrush_booking_expiration_last_success_timestamp_seconds"].GetMetric()
			require.Len(t, lastSuccess, 1)
			if tc.outcome == "success" {
				require.GreaterOrEqual(t, lastSuccess[0].GetGauge().GetValue(), float64(startedAt.Unix()))
			} else {
				require.Equal(t, previousSuccess, lastSuccess[0].GetGauge().GetValue())
			}
		})
	}
}
