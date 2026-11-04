package booking_transport

import "github.com/prometheus/client_golang/prometheus"

type BookingMetrics struct {
	Attempts *prometheus.CounterVec
	Duration *prometheus.HistogramVec
}

func NewBookingMetrics() *BookingMetrics {
	comletedBookings := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ticketrush_booking_attempts_total",
			Help: "Number of completed booking attempts.",
		},
		[]string{"outcome"},
	)

	duration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "ticketrush_booking_duration_seconds",
			Help: "Duration of completed booking attempts in seconds.",
			Buckets: []float64{
				0.0001, 0.00025, 0.0005,
				0.001, 0.0025, 0.005,
				0.01, 0.025, 0.05,
				0.1, 0.25, 0.5,
				1, 2.5, 5, 10,
			},
		},
		[]string{"outcome"},
	)

	return &BookingMetrics{
		Attempts: comletedBookings,
		Duration: duration,
	}
}
