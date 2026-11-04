package core_redis_ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisLimiter struct {
	redisClient *redis.Client
	limit       int64
	timeout     time.Duration
	timeWindow  time.Duration
}

var allowScript = redis.NewScript(`
	local count = redis.call("INCR", KEYS[1])

	if count == 1 then
		redis.call("PEXPIRE", KEYS[1], ARGV[1])
	end

	return {count, redis.call("PTTL", KEYS[1])}
`)

type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

func NewRedisLimiter(
	rCl *redis.Client,
	limit int64,
	timeout time.Duration,
	timeWindow time.Duration,
) (*RedisLimiter, error) {
	if rCl == nil {
		return nil, fmt.Errorf("redis client must be not a nil pointer")
	}

	if timeout <= 0 {
		return nil, fmt.Errorf("timeout can't be a zero")
	}

	if limit <= 0 {
		return nil, fmt.Errorf("limit can't be a zero")
	}

	if timeWindow.Milliseconds() <= 0 {
		return nil, fmt.Errorf("time window can't be a zero")
	}

	return &RedisLimiter{
		redisClient: rCl,
		limit:       limit,
		timeout:     timeout,
		timeWindow:  timeWindow,
	}, nil
}

func (l *RedisLimiter) Allow(
	ctx context.Context,
	key string,
) (Decision, error) {
	if key == "" {
		return Decision{}, fmt.Errorf("key can't be zero")
	}

	ctx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()

	values, err := allowScript.Run(
		ctx,
		l.redisClient,
		[]string{key},
		l.timeWindow.Milliseconds(),
	).Int64Slice()
	if err != nil {
		return Decision{}, fmt.Errorf("lua script failed: %w", err)
	}

	if len(values) != 2 {
		return Decision{}, fmt.Errorf("must be len(values) == 2")
	}

	count := values[0]
	ttlMilliseconds := values[1]

	if count <= 0 || ttlMilliseconds < 0 {
		return Decision{}, fmt.Errorf(
			"invalid rate limiter response: count=%d ttl_ms=%d",
			count,
			ttlMilliseconds,
		)
	}

	if count <= l.limit {
		return Decision{
			Allowed:    true,
			RetryAfter: 0,
		}, nil
	} else {
		return Decision{
			Allowed:    false,
			RetryAfter: time.Duration(ttlMilliseconds) * time.Millisecond,
		}, nil
	}
}
