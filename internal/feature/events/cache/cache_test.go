package events_cache_test

import (
	"context"
	"encoding/json/v2"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	cache "github.com/poponyas/TicketRush/internal/feature/events/cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// Intercept commands to exercise EventCache without a Redis server.
type commandHook struct {
	process redis.ProcessHook
}

func (h commandHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h commandHook) ProcessHook(redis.ProcessHook) redis.ProcessHook {
	return h.process
}

func (h commandHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func TestEventCacheMetrics(t *testing.T) {
	event := domain.NewEvent(uuid.New(), "Go Live", "Description", "conference")
	event.Status = domain.EventPublished
	encoded, err := json.Marshal(event)
	require.NoError(t, err)

	for _, tc := range []struct {
		name        string
		operation   string
		outcome     string
		payload     string
		redisErr    error
		invalidTime bool
	}{
		{name: "cache hit", operation: "get", outcome: "hit", payload: string(encoded)},
		{name: "cache miss", operation: "get", outcome: "miss", redisErr: redis.Nil},
		{name: "read error", operation: "get", outcome: "error", redisErr: context.DeadlineExceeded},
		{name: "invalid cached JSON", operation: "get", outcome: "error", payload: "{"},
		{name: "successful write", operation: "set", outcome: "success"},
		{name: "write error", operation: "set", outcome: "error", redisErr: context.DeadlineExceeded},
		{name: "serialization error", operation: "set", outcome: "error", invalidTime: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, enabled := range []bool{true, false} {
				name := "without metrics"
				if enabled {
					name = "with metrics"
				}
				t.Run(name, func(t *testing.T) {
					redisCalls := 0
					client := redis.NewClient(&redis.Options{Addr: "unused:6379"})
					t.Cleanup(func() { _ = client.Close() })
					client.AddHook(commandHook{process: func(_ context.Context, cmd redis.Cmder) error {
						redisCalls++
						require.Equal(t, tc.operation, cmd.Name())
						require.Equal(t, "ticketrush:public:event:"+event.ID.String(), cmd.Args()[1])
						switch command := cmd.(type) {
						case *redis.StringCmd:
							command.SetVal(tc.payload)
						case *redis.StatusCmd:
							command.SetVal("OK")
						default:
							t.Fatalf("unexpected command type %T", cmd)
						}
						cmd.SetErr(tc.redisErr)
						return tc.redisErr
					}})

					var metrics *cache.CacheMetrics
					registry := prometheus.NewRegistry()
					if enabled {
						metrics = cache.NewCacheMetrics()
						require.NoError(t, registry.Register(metrics.Operations))
					}
					eventCache, err := cache.NewEventCache(client, time.Second, time.Minute, metrics)
					require.NoError(t, err)
					require.Same(t, metrics, eventCache.EventMetrics)

					if tc.operation == "get" {
						var result domain.Event
						var found bool
						result, found, err = eventCache.Get(context.Background(), event.ID)
						if tc.outcome == "hit" {
							require.True(t, found)
							require.Equal(t, event, result)
						} else {
							require.False(t, found)
							require.Empty(t, result)
						}
					} else {
						input := event
						if tc.invalidTime {
							input.CreatedAt = time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC)
						}
						err = eventCache.Set(context.Background(), input)
					}
					if tc.outcome == "error" {
						require.Error(t, err)
						if tc.redisErr != nil {
							require.ErrorIs(t, err, tc.redisErr)
						}
					} else {
						require.NoError(t, err)
					}
					if tc.invalidTime {
						require.Zero(t, redisCalls)
					} else {
						require.Equal(t, 1, redisCalls)
					}

					families, err := registry.Gather()
					require.NoError(t, err)
					if !enabled {
						require.Empty(t, families)
						return
					}
					require.Len(t, families, 1)
					require.Equal(t, "ticketrush_event_cache_operations_total", families[0].GetName())
					samples := families[0].GetMetric()
					require.Len(t, samples, 1)
					require.Equal(t, 1.0, samples[0].GetCounter().GetValue())
					labels := make(map[string]string)
					for _, label := range samples[0].GetLabel() {
						labels[label.GetName()] = label.GetValue()
					}
					require.Equal(t, map[string]string{"operation": tc.operation, "outcome": tc.outcome}, labels)
				})
			}
		})
	}
}
