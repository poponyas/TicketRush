package core_redis_ratelimit

import "github.com/prometheus/client_golang/prometheus"

type RateLimitMetrics struct {
	Requests *prometheus.CounterVec
}

func NewRateLimitMetrics() *RateLimitMetrics {
	requests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ticketrush_rate_limit_requests_total",
			Help: "Number of requests handled by the rate limiting middleware by outcome.",
		},
		[]string{"outcome"},
	)
	for _, outcome := range []string{"allowed", "blocked", "error"} {
		requests.WithLabelValues(outcome)
	}
	return &RateLimitMetrics{Requests: requests}
}

func recordOutcome(metrics *RateLimitMetrics, outcome string) {
	if metrics != nil {
		metrics.Requests.WithLabelValues(outcome).Inc()
	}
}
