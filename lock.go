package redismutex

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
)

var _ context.Context = (*Lock)(nil)

// Lock represents a lock with context.
type Lock struct {
	redis  redis.Scripter
	id     string
	ttl    time.Duration
	key    string
	log    LogFunc
	ctx    context.Context
	cancel context.CancelFunc
}

// ID returns the id value set by the lock.
func (l *Lock) ID() string {
	return l.id
}

func (l *Lock) Deadline() (deadline time.Time, ok bool) {
	return l.ctx.Deadline()
}

func (l *Lock) Done() <-chan struct{} {
	return l.ctx.Done()
}

func (l *Lock) Err() error {
	return l.ctx.Err()
}

func (l *Lock) Value(key any) any {
	return l.ctx.Value(key)
}

// Unlock unlocks.
func (l *Lock) Unlock() {
	l.cancel()
	_, err := scriptUnlock.Run(context.Background(), l.redis, []string{l.key}, l.id).Result()
	if err != nil {
		l.log("[ERROR] %s unlock %q: %v", l.id, l.key, err)
	}
}

func (l *Lock) refreshTTL() {
	defer l.cancel()

	refresh := updateTTL(l.ttl)
	left := time.Now().Add(l.ttl)
	for {
		diff := time.Since(left)
		select {

		case <-l.ctx.Done():
			return

		case <-time.After(-diff): // cant refresh
			return

		case <-time.After(refresh):
			status, err := scriptRefresh.Run(l.ctx, l.redis, []string{l.key}, l.id, l.ttl.Milliseconds()).Int()
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}

				refresh = refreshTimeout
				l.log("[ERROR] %s refresh key %q: %v", l.id, l.key, err)
				continue
			}

			refresh = updateTTL(l.ttl)
			left = time.Now().Add(l.ttl)
			if status == 0 {
				l.log("[ERROR] %s refresh key already expired %q", l.id, l.key)
				return
			}
		}
	}
}

func updateTTL(d time.Duration) time.Duration {
	return d / 2
}
