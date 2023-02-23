package redismutex

import (
	"context"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func init() {
	SetLog(func(format string, a ...any) {
		if strings.HasPrefix(format, "[ERROR]") {
			log.Fatalf(format, a...)
		}
	})
}

func TestMutex(t *testing.T) {
	t.Parallel()

	const lockKey = "mutex"
	rc := redis.NewClient(redisOpts())
	prep(t, rc, lockKey)

	mx := NewMutex(rc, lockKey)
	lock, ok := mx.Lock()
	if exp, got := true, ok; exp != got {
		t.Fatalf("exp %v, got %v", exp, got)
	}
	defer lock.Unlock()

	assertTTL(t, rc, lockKey, defaultKeyTTL)

	// try again
	_, ok = mx.TryLock()
	if exp, got := false, ok; exp != got {
		t.Fatalf("exp %v, got %v", exp, got)
	}

	_, ok = mx.TryRLock()
	if exp, got := false, ok; exp != got {
		t.Fatalf("exp %v, got %v", exp, got)
	}

	// manually unlock
	lock.Unlock()

	// lock again
	lock, ok = mx.Lock()
	if exp, got := true, ok; exp != got {
		t.Fatalf("exp %v, got %v", exp, got)
	}
	defer lock.Unlock()
}

func TestRWMutex(t *testing.T) {
	t.Parallel()

	const lockKey = "rw_mutex"
	rc := redis.NewClient(redisOpts())
	prep(t, rc, lockKey)

	mx := NewMutex(rc, lockKey)
	lock, ok := mx.RLock()
	if exp, got := true, ok; exp != got {
		t.Fatalf("exp %v, got %v", exp, got)
	}
	defer lock.Unlock()

	assertTTL(t, rc, lockKey, defaultKeyTTL)

	// try again
	_, ok = mx.TryLock()
	if exp, got := false, ok; exp != got {
		t.Fatalf("exp %v, got %v", exp, got)
	}

	// try rlock
	rlock, ok := mx.TryRLock()
	if exp, got := true, ok; exp != got {
		t.Fatalf("exp %v, got %v", exp, got)
	}
	rlock.Unlock()

	// manually unlock
	lock.Unlock()

	// lock again
	lock, ok = mx.Lock()
	if exp, got := true, ok; exp != got {
		t.Fatalf("exp %v, got %v", exp, got)
	}
	defer lock.Unlock()
}

func TestRWMutex_LockIntent(t *testing.T) {
	t.Parallel()

	const lockKey = "lock_intent_mutex"
	rc := redis.NewClient(redisOpts())
	prep(t, rc, lockKey)

	mx := NewMutex(rc, lockKey, WithLockIntent())
	lock, ok := mx.RLock()
	if exp, got := true, ok; exp != got {
		t.Fatalf("exp %v, got %v", exp, got)
	}
	defer lock.Unlock()

	// mark lock intent
	_, _, err := mx.lock(newLockOptions(mx.opts))
	if exp, got := ErrLock, err; !errors.Is(got, exp) {
		t.Fatalf("exp %v, got %v", exp, got)
	}

	// try rlock
	_, ok = mx.TryRLock()
	if exp, got := false, ok; exp != got {
		t.Fatalf("exp %v, got %v", exp, got)
	}

	// manually unlock
	lock.Unlock()

	// lock write
	lock, ok = mx.Lock()
	if exp, got := true, ok; exp != got {
		t.Fatalf("exp %v, got %v", exp, got)
	}
	lock.Unlock() // remove lock intent

	// lock again
	lock, ok = mx.RLock()
	if exp, got := true, ok; exp != got {
		t.Fatalf("exp %v, got %v", exp, got)
	}
	defer lock.Unlock()
}

func TestRWMutex_ID(t *testing.T) {
	t.Parallel()

	rw := &RWMutex{}
	rw.id.buf = make([]byte, lenBytesID)
	id, _ := rw.randomID()
	if exp, got := 32, len(id); exp != got {
		t.Fatalf("exp %v, got %v", exp, got)
	}
}

func prep(t *testing.T, rc *redis.Client, key string) {
	t.Cleanup(func() {
		for _, v := range []string{key, lockIntentKey(key)} {
			if err := rc.Del(context.Background(), v).Err(); err != nil {
				t.Fatal(err)
			}
		}

		if err := rc.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

func assertTTL(t *testing.T, rc *redis.Client, key string, exp time.Duration) {
	t.Helper()

	got, err := rc.TTL(context.Background(), key).Result()
	if exp, got := (any)(nil), err; exp != got {
		t.Fatalf("exp %v, got %v", exp, got)
	}

	delta := got - exp
	if delta < 0 {
		delta = 1 - delta
	}

	if delta > time.Second {
		t.Fatalf("exp ~%v, got %v", exp, got)
	}
}

func redisOpts() *redis.Options {
	return &redis.Options{
		Network: "tcp",
		Addr:    "0.0.0.0:6379",
		DB:      9,
	}
}
