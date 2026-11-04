package events_cache

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	"github.com/redis/go-redis/v9"
)

type EventCache struct {
	RedisCl      *redis.Client
	Timeout      time.Duration
	TTL          time.Duration
	EventMetrics *CacheMetrics
}

func NewEventCache(
	redisCl *redis.Client,
	timeout time.Duration,
	ttl time.Duration,
	metrics *CacheMetrics,
) (*EventCache, error) {
	if ttl <= 0 {
		return nil, fmt.Errorf("ttl must be bigger than 0")
	}

	if timeout <= 0 {
		return nil, fmt.Errorf("timeout can't be a zero")
	}
	return &EventCache{
		RedisCl:      redisCl,
		Timeout:      timeout,
		TTL:          ttl,
		EventMetrics: metrics,
	}, nil
}

func (c *EventCache) Get(
	ctx context.Context,
	eventID uuid.UUID,
) (domain.Event, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	key := eventKey(eventID)
	data, err := c.RedisCl.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			c.recordOperation("get", "miss")
			return domain.Event{}, false, nil
		}
		c.recordOperation("get", "error")
		return domain.Event{}, false, fmt.Errorf("redis get event: %w", err)
	}

	var event domain.Event
	if err = json.Unmarshal(data, &event); err != nil {
		c.recordOperation("get", "error")
		return domain.Event{}, false, fmt.Errorf("decode cached event: %w", err)
	}

	c.recordOperation("get", "hit")
	return event, true, nil
}

func (c *EventCache) Set(
	ctx context.Context,
	event domain.Event,
) error {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	key := eventKey(event.ID)
	data, err := json.Marshal(event)
	if err != nil {
		c.recordOperation("set", "error")
		return fmt.Errorf("serialize error: %w", err)
	}

	if err := c.RedisCl.Set(ctx, key, data, c.TTL).Err(); err != nil {
		c.recordOperation("set", "error")
		return fmt.Errorf("redis set error: %w", err)
	}
	c.recordOperation("set", "success")
	return nil
}

func eventKey(id uuid.UUID) string {
	return "ticketrush:public:event:" + id.String()
}
