package booking_worker

import "github.com/prometheus/client_golang/prometheus"

type ExpirationMetrics struct {
	Batches         *prometheus.CounterVec
	ExpiredBookings prometheus.Counter
	Duration        *prometheus.HistogramVec
	LastSuccess     prometheus.Gauge
}

func NewExpirationMetrics() *ExpirationMetrics {
	batchesCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ticketrush_booking_expiration_batches_total",
			Help: "Counter of total expiration batches",
		},
		[]string{"outcome"},
	)

	expiredBookingsCounter := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "ticketrush_bookings_expired_total",
			Help: "Counter of total expiration bookings",
		},
	)

	duration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "ticketrush_booking_expiration_batch_duration_seconds",
			Help: "Worker duration",
		},
		[]string{"outcome"},
	)

	lastSuccess := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ticketrush_booking_expiration_last_success_timestamp_seconds",
			Help: "Unix timestamp of the last successful booking expiration batch, including empty batches.",
		},
	)

	return &ExpirationMetrics{
		Batches:         batchesCounter,
		ExpiredBookings: expiredBookingsCounter,
		Duration:        duration,
		LastSuccess:     lastSuccess,
	}
}
