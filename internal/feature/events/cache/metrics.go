package events_cache

import "github.com/prometheus/client_golang/prometheus"

type CacheMetrics struct {
	Operations *prometheus.CounterVec
}

func NewCacheMetrics() *CacheMetrics {
	operationCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ticketrush_event_cache_operations_total",
			Help: "Number of event cache operations by operation and outcome",
		},
		[]string{"operation", "outcome"},
	)

	return &CacheMetrics{
		Operations: operationCounter,
	}
}

func (c *EventCache) recordOperation(operation, outcome string) {
	if c.EventMetrics == nil {
		return
	}
	c.EventMetrics.Operations.WithLabelValues(operation, outcome).Inc()
}
