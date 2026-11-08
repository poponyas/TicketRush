package core_metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

type HTTPMetrics struct {
	Requests *prometheus.CounterVec
	Duration *prometheus.HistogramVec
}

func NewHTTPMetrics() *HTTPMetrics {
	counterReq := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ticketrush_http_requests_total",
			Help: "Number of total http requests",
		},
		[]string{
			"method",
			"route",
			"status",
		},
	)

	duration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "ticketrush_http_request_duration_seconds",
			Help: "Duration of requests",
			Buckets: []float64{
				0.0001, 0.00025, 0.0005,
				0.001, 0.0025, 0.005,
				0.01, 0.025, 0.05,
				0.1, 0.25, 0.5,
				1, 2.5, 5, 10,
			},
		},
		[]string{
			"method",
			"route",
			"status",
		},
	)

	return &HTTPMetrics{
		Requests: counterReq,
		Duration: duration,
	}
}

func (m *HTTPMetrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			before := time.Now()
			wNew := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(wNew, r)
			duration := time.Since(before).Seconds()
			status := wNew.Status()
			if status == 0 {
				status = http.StatusOK
			}
			statusString := strconv.Itoa(status)

			route := chi.RouteContext(r.Context()).RoutePattern()

			if route == "/metrics" {
				return
			}

			if route == "" {
				route = "unmatched"
			}

			m.Requests.WithLabelValues(r.Method, route, statusString).Inc()
			m.Duration.WithLabelValues(r.Method, route, statusString).Observe(duration)
		},
	)
}
