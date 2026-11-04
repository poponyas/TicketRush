package booking_worker

import (
	"context"
	"fmt"
	"time"

	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	"go.uber.org/zap"
)

type ExpirationService interface {
	ExpireBookings(ctx context.Context, limit int) (int, error)
}

type ExpirationConfig struct {
	Interval  time.Duration
	BatchSize int
}

func DefaultExpirationConfig() ExpirationConfig {
	return ExpirationConfig{Interval: 5 * time.Second, BatchSize: 100}
}

type ExpirationWorker struct {
	service ExpirationService
	logger  *corelogger.Logger
	config  ExpirationConfig
	metrics *ExpirationMetrics
}

func NewExpirationWorker(service ExpirationService, logger *corelogger.Logger, config ExpirationConfig, metrics *ExpirationMetrics) (*ExpirationWorker, error) {
	if service == nil || logger == nil || logger.Logger == nil {
		return nil, fmt.Errorf("expiration service and logger are required")
	}
	if config.Interval <= 0 {
		return nil, fmt.Errorf("expiration interval must be positive")
	}
	if config.BatchSize < 1 || config.BatchSize > 1000 {
		return nil, fmt.Errorf("expiration batch size must be between 1 and 1000")
	}

	if metrics == nil {
		return nil, fmt.Errorf("expiration metrics are required")
	}
	return &ExpirationWorker{
		service: service,
		logger:  logger.With(zap.String("component", "booking_expiration")),
		config:  config,
		metrics: metrics,
	}, nil
}

// Run processes overdue bookings immediately, then polls on each tick. Calls
// are sequential: a slow database never starts overlapping work in this process.
func (w *ExpirationWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.config.Interval)
	defer ticker.Stop()
	for {
		// Drain full batches so a backlog does not wait one interval per batch.
		for {
			if ctx.Err() != nil {
				return
			}
			startedAt := time.Now()
			count, err := w.service.ExpireBookings(ctx, w.config.BatchSize)
			duration := time.Since(startedAt).Seconds()

			outcome := "success"
			if err != nil {
				outcome = "error"
				if ctx.Err() != nil {
					outcome = "cancelled"
				}
			}
			w.metrics.Batches.WithLabelValues(outcome).Inc()
			w.metrics.Duration.WithLabelValues(outcome).Observe(duration)

			if err != nil {
				if ctx.Err() != nil {
					return
				}
				w.logger.Error("expire bookings", zap.Error(err))
				break // Retry on the next tick, without spinning on a persistent failure.
			}
			w.metrics.ExpiredBookings.Add(float64(count))
			w.metrics.LastSuccess.SetToCurrentTime()
			if count > 0 {
				w.logger.Info("expired bookings", zap.Int("count", count))
			}
			if count < w.config.BatchSize {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
