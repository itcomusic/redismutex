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

// Key returns the key value set by the lock.
func (l *Lock) Key() string {
	return l.key
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
		l.log("[ERROR] unlock %q %s: %v", l.key, l.id, err)
	}
}

func (l *Lock) refreshTTL(left time.Time) {
	defer l.cancel()

	refresh := l.updateTTL()
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
				l.log("[ERROR] refresh key %q %s: %v", l.key, l.id, err)
				continue
			}

			left = l.leftTTL()
			refresh = l.updateTTL()
			if status == 0 {
				l.log("[ERROR] refresh key %q %s already expired", l.key, l.id)
				return
			}
		}
	}
}

func (l *Lock) leftTTL() time.Time {
	return time.Now().Add(l.ttl)
}

func (l *Lock) updateTTL() time.Duration {
	return l.ttl / 2
}
