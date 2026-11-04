package core_redis_ratelimit_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	ratelimit "github.com/poponyas/TicketRush/internal/core/ratelimit"
	response "github.com/poponyas/TicketRush/internal/core/response"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

const loginKeyPrefix = "ticketrush:ratelimit:login:"

type limiterFunc func(context.Context, string) (ratelimit.Decision, error)

func (f limiterFunc) Allow(ctx context.Context, key string) (ratelimit.Decision, error) {
	return f(ctx, key)
}

func TestLimitByIPAllowsRequest(t *testing.T) {
	for _, tc := range []struct {
		name, remoteAddr, ip string
	}{
		{"IPv4", "192.0.2.10:12345", "192.0.2.10"},
		{"IPv6", "[2001:0db8:0000::1]:12345", "2001:db8::1"},
		{"mapped IPv4", "[::ffff:192.0.2.10]:12345", "192.0.2.10"},
		{"bare IPv4", "192.0.2.10", "192.0.2.10"},
		{"bare IPv6", "2001:db8::1", "2001:db8::1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			limiterCalls, nextCalls := 0, 0
			limiter := limiterFunc(func(ctx context.Context, key string) (ratelimit.Decision, error) {
				limiterCalls++
				require.Equal(t, loginKeyPrefix+tc.ip, key)
				require.Equal(t, "test-request-id", middleware.GetReqID(ctx))
				return ratelimit.Decision{Allowed: true}, nil
			})
			handler := middleware.RequestID(ratelimit.LimitByIP(limiter, loginKeyPrefix, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalls++
				require.Equal(t, http.MethodPost, r.Method)
				w.WriteHeader(http.StatusNoContent)
			})))
			req := loginRequest(tc.remoteAddr)
			// Forwarded headers must not override the peer address without a trusted proxy setup.
			req.Header.Set("X-Forwarded-For", "203.0.113.99")
			req.Header.Set("X-Real-IP", "203.0.113.98")
			req.Header.Set("Forwarded", "for=203.0.113.97")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			require.Equal(t, http.StatusNoContent, w.Code)
			require.Equal(t, 1, limiterCalls)
			require.Equal(t, 1, nextCalls)
			require.Empty(t, w.Header().Get("Retry-After"))
		})
	}
}

func TestLimitByIPRejectsRequestWithRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		name       string
		delay      time.Duration
		wantHeader string
	}{
		{"zero", 0, "1"},
		{"negative", -time.Second, "1"},
		{"fraction of a second", 20 * time.Millisecond, "1"},
		{"exact second", time.Second, "1"},
		{"round up", 1500 * time.Millisecond, "2"},
		{"full minute", time.Minute, "60"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			limiter := limiterFunc(func(_ context.Context, key string) (ratelimit.Decision, error) {
				require.Equal(t, loginKeyPrefix+"192.0.2.10", key)
				return ratelimit.Decision{RetryAfter: tc.delay}, nil
			})
			handler := middleware.RequestID(ratelimit.LimitByIP(limiter, loginKeyPrefix, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("blocked request reached login handler")
			})))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, loginRequest("192.0.2.10:12345"))

			require.Equal(t, http.StatusTooManyRequests, w.Code)
			require.Equal(t, tc.wantHeader, w.Header().Get("Retry-After"))
			require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			requireHTTPError(t, w, "rate_limited")
		})
	}
}

func TestLimitByIPUnavailable(t *testing.T) {
	internalError := errors.New("redis connection failed: internal detail")
	limiter := limiterFunc(func(context.Context, string) (ratelimit.Decision, error) {
		return ratelimit.Decision{Allowed: true}, internalError
	})
	handler := middleware.RequestID(ratelimit.LimitByIP(limiter, loginKeyPrefix, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("request reached login handler after limiter error")
	})))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, loginRequest("192.0.2.10:12345"))

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Empty(t, w.Header().Get("Retry-After"))
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	require.NotContains(t, w.Body.String(), internalError.Error())
	requireHTTPError(t, w, "service_unavailable")
}

func TestLimitByIPRejectsInvalidAddress(t *testing.T) {
	for _, remoteAddr := range []string{"", "invalid-address", "example.test:12345"} {
		t.Run(remoteAddr, func(t *testing.T) {
			limiter := limiterFunc(func(context.Context, string) (ratelimit.Decision, error) {
				t.Fatal("invalid address reached limiter")
				return ratelimit.Decision{}, nil
			})
			handler := middleware.RequestID(ratelimit.LimitByIP(limiter, loginKeyPrefix, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("invalid address reached login handler")
			})))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, loginRequest(remoteAddr))

			require.Equal(t, http.StatusBadRequest, w.Code)
			requireHTTPError(t, w, "invalid_argument")
		})
	}
}

func TestLimitByIPGroupsDifferentPorts(t *testing.T) {
	counts := make(map[string]int)
	limiter := limiterFunc(func(_ context.Context, key string) (ratelimit.Decision, error) {
		counts[key]++
		return ratelimit.Decision{Allowed: counts[key] <= 5, RetryAfter: time.Minute}, nil
	})
	nextCalls := 0
	handler := ratelimit.LimitByIP(limiter, loginKeyPrefix, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalls++
		w.WriteHeader(http.StatusNoContent)
	}))
	for i := range 6 {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, loginRequest(fmt.Sprintf("192.0.2.10:%d", 12000+i)))
		if i < 5 {
			require.Equal(t, http.StatusNoContent, w.Code)
		} else {
			require.Equal(t, http.StatusTooManyRequests, w.Code)
		}
	}
	require.Equal(t, 5, nextCalls)
	require.Equal(t, map[string]int{loginKeyPrefix + "192.0.2.10": 6}, counts)
}

func TestLimitByIPRecordsOneOutcomePerRequest(t *testing.T) {
	for _, tc := range []struct {
		name       string
		decision   ratelimit.Decision
		limiterErr error
		remoteAddr string
		nextStatus int
		wantStatus int
		outcome    string
	}{
		{
			name: "allowed", decision: ratelimit.Decision{Allowed: true},
			nextStatus: http.StatusNoContent, wantStatus: http.StatusNoContent, outcome: "allowed",
		},
		{
			name: "invalid credentials still count as allowed", decision: ratelimit.Decision{Allowed: true},
			nextStatus: http.StatusUnauthorized, wantStatus: http.StatusUnauthorized, outcome: "allowed",
		},
		{
			name: "login error still counts as allowed", decision: ratelimit.Decision{Allowed: true},
			nextStatus: http.StatusInternalServerError, wantStatus: http.StatusInternalServerError, outcome: "allowed",
		},
		{
			name: "blocked", decision: ratelimit.Decision{RetryAfter: time.Minute},
			wantStatus: http.StatusTooManyRequests, outcome: "blocked",
		},
		{
			name: "limiter error", limiterErr: errors.New("redis unavailable"),
			wantStatus: http.StatusServiceUnavailable, outcome: "error",
		},
		{
			name: "invalid peer address", remoteAddr: "invalid-address",
			wantStatus: http.StatusBadRequest, outcome: "error",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			metrics := ratelimit.NewRateLimitMetrics()
			registry := prometheus.NewRegistry()
			require.NoError(t, registry.Register(metrics.Requests))
			limiterCalls, nextCalls := 0, 0
			limiter := limiterFunc(func(context.Context, string) (ratelimit.Decision, error) {
				limiterCalls++
				return tc.decision, tc.limiterErr
			})
			handler := ratelimit.LimitByIP(limiter, loginKeyPrefix, metrics)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				nextCalls++
				w.WriteHeader(tc.nextStatus)
			}))
			remoteAddr := tc.remoteAddr
			if remoteAddr == "" {
				remoteAddr = "192.0.2.10:12345"
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, loginRequest(remoteAddr))
			require.Equal(t, tc.wantStatus, w.Code)
			if tc.outcome == "allowed" {
				require.Equal(t, 1, nextCalls)
			} else {
				require.Zero(t, nextCalls)
			}
			if tc.remoteAddr == "invalid-address" {
				require.Zero(t, limiterCalls)
			} else {
				require.Equal(t, 1, limiterCalls)
			}

			families, err := registry.Gather()
			require.NoError(t, err)
			require.Len(t, families, 1)
			require.Equal(t, "ticketrush_rate_limit_requests_total", families[0].GetName())
			counts := make(map[string]float64)
			for _, sample := range families[0].GetMetric() {
				require.Len(t, sample.GetLabel(), 1)
				require.Equal(t, "outcome", sample.GetLabel()[0].GetName())
				counts[sample.GetLabel()[0].GetValue()] = sample.GetCounter().GetValue()
			}
			want := map[string]float64{"allowed": 0, "blocked": 0, "error": 0}
			want[tc.outcome] = 1
			require.Equal(t, want, counts)
		})
	}
}

func loginRequest(remoteAddr string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = remoteAddr
	req.Header.Set(middleware.RequestIDHeader, "test-request-id")
	return req
}

func requireHTTPError(t *testing.T, w *httptest.ResponseRecorder, code string) {
	t.Helper()
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	var body response.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, code, body.Code)
	require.NotEmpty(t, body.Message)
	require.Equal(t, "test-request-id", body.RequestID)
}
