package ratelimit

import (
	"context"
	"time"

	"ProxyaService/internal/storage"
)

type Limiter struct {
	store    *storage.Store
	free     int
	prem     int
	admin    int
	throttle time.Duration
}

func New(store *storage.Store, free, prem, admin, throttleSec int) *Limiter {
	return &Limiter{store: store, free: free, prem: prem, admin: admin, throttle: time.Duration(throttleSec) * time.Second}
}

// Allow checks per-minute limits by role and adds an event if allowed.
func (l *Limiter) Allow(ctx context.Context, user storage.User, kind string) (bool, error) {
	var limit int
	switch user.Role {
	case storage.RoleAdmin:
		limit = l.admin
	case storage.RolePremium:
		limit = l.prem
	default:
		limit = l.free
	}
	// Throttle: simple delay to avoid bursts
	if l.throttle > 0 {
		time.Sleep(l.throttle)
	}
	since := time.Now().Add(-1 * time.Minute)
	cnt, err := l.store.CountEventsSince(ctx, user.ID, since)
	if err != nil {
		return false, err
	}
	if cnt >= limit {
		return false, nil
	}
	if err := l.store.InsertRateEvent(ctx, user.ID, kind); err != nil {
		return false, err
	}
	return true, nil
}
