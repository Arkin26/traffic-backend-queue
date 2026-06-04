package antibot

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Limiter {
	return &Limiter{rdb: rdb}
}

func key(kind, id string) string {
	return fmt.Sprintf("ratelimit:%s:%s", kind, id)
}

func (l *Limiter) Allow(ctx context.Context, kind, id string, limit int, window time.Duration) (bool, error) {
	k := key(kind, id)
	pipe := l.rdb.Pipeline()
	incr := pipe.Incr(ctx, k)
	pipe.Expire(ctx, k, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, err
	}
	n, err := incr.Result()
	if err != nil {
		return false, err
	}
	return n <= int64(limit), nil
}

func (l *Limiter) AllowJoin(ctx context.Context, userID, ip string) (bool, error) {
	okIP, err := l.Allow(ctx, "ip_join", ip, 30, time.Minute)
	if err != nil || !okIP {
		return false, err
	}
	okUser, err := l.Allow(ctx, "user_join", userID, 10, time.Minute)
	if err != nil || !okUser {
		return false, err
	}
	return true, nil
}

func (l *Limiter) AllowCheckout(ctx context.Context, userID, ip string) (bool, error) {
	okIP, err := l.Allow(ctx, "ip_checkout", ip, 3, time.Minute)
	if err != nil || !okIP {
		return false, err
	}
	okUser, err := l.Allow(ctx, "user_checkout", userID, 5, time.Minute)
	return okUser, err
}
