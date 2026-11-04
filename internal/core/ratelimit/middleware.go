package core_redis_ratelimit

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	response "github.com/poponyas/TicketRush/internal/core/response"
	"go.uber.org/zap"
)

type Limiter interface {
	Allow(ctx context.Context, key string) (Decision, error)
}

func LimitByIP(limiter Limiter, keyPrefix string, metrics *RateLimitMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			log := corelogger.FromContext(ctx)
			handler := response.NewHTTPResponseHandler(log, w)
			requestID := middleware.GetReqID(ctx)

			ip, err := remoteIP(r.RemoteAddr)
			if err != nil {
				recordOutcome(metrics, "error")
				log.Warn("parse client IP for rate limit", zap.Error(err))
				handler.JSONResponse(response.ErrorResponse{
					Code: "invalid_argument", Message: "invalid client address", RequestID: requestID,
				}, http.StatusBadRequest)
				return
			}

			decision, err := limiter.Allow(ctx, keyPrefix+ip)
			if err != nil {
				recordOutcome(metrics, "error")
				log.Error("check request rate limit", zap.Error(err))
				w.Header().Set("Cache-Control", "no-store")
				handler.JSONResponse(response.ErrorResponse{
					Code: "service_unavailable", Message: "service temporarily unavailable", RequestID: requestID,
				}, http.StatusServiceUnavailable)
				return
			}
			if !decision.Allowed {
				recordOutcome(metrics, "blocked")
				w.Header().Set("Retry-After", strconv.FormatInt(retryAfterSeconds(decision.RetryAfter), 10))
				w.Header().Set("Cache-Control", "no-store")
				handler.JSONResponse(response.ErrorResponse{
					Code: "rate_limited", Message: "too many requests", RequestID: requestID,
				}, http.StatusTooManyRequests)
				return
			}

			recordOutcome(metrics, "allowed")
			next.ServeHTTP(w, r)
		})
	}
}

func remoteIP(remoteAddr string) (string, error) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return "", fmt.Errorf("parse remote address: %w", err)
	}
	return ip.Unmap().String(), nil
}

func retryAfterSeconds(delay time.Duration) int64 {
	seconds := delay / time.Second
	if delay%time.Second > 0 {
		seconds++
	}
	if seconds < 1 {
		seconds = 1
	}
	return int64(seconds)
}
