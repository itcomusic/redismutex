package redismutex

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestMutexOptions(t *testing.T) {
	t.Parallel()

	got := mutexOptions{}
	for _, v := range []MutexOption{WithTTL(time.Second * 3), WithLockIntent()} {
		v(&got)
	}

	exp := mutexOptions{
		ttl:        time.Second * 3,
		lockIntent: true,
	}

	if !reflect.DeepEqual(exp, got) {
		t.Errorf("exp %v, got %v", exp, got)
	}
}

func TestLockOptions(t *testing.T) {
	t.Parallel()

	got := newLockOptions(mutexOptions{name: "mutex", ttl: time.Second, lockIntent: true, log: logger}, WithKey("key"), WithContext(context.Background()))
	exp := lockOptions{
		ctx:              context.Background(),
		key:              "mutex:key",
		lockIntentKey:    "mutex:key:lock_intent",
		enableLockIntent: 1,
		ttl:              time.Second,
		log:              logger,
	}

	if reflect.DeepEqual(exp, got) {
		t.Errorf("exp %v, got %v", exp, got)
	}
}
