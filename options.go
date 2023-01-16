package redismutex

import (
	"context"
	"log"
	"os"
	"sync"
	"time"
)

const (
	lenBytesID     = 16
	refreshTimeout = time.Millisecond * 500
	defaultKeyTTL  = time.Second * 4
)

var (
	globalMx  sync.RWMutex
	globalLog = func() LogFunc {
		l := log.New(os.Stderr, "redismutex: ", log.LstdFlags)
		return func(format string, v ...any) {
			l.Printf(format, v...)
		}
	}()
)

// LogFunc type is an adapter to allow the use of ordinary functions as LogFunc.
type LogFunc func(format string, v ...any)

// NopLog logger does nothing
var NopLog = LogFunc(func(string, ...any) {})

// SetLog sets the logger.
func SetLog(l LogFunc) {
	globalMx.Lock()
	defer globalMx.Unlock()

	if l != nil {
		globalLog = l
	}
}

// MutexOption is the option for the mutex.
type MutexOption func(*mutexOptions)

type mutexOptions struct {
	name       string
	ttl        time.Duration
	lockIntent bool
	log        LogFunc
}

// WithTTL sets the TTL of the mutex.
func WithTTL(ttl time.Duration) MutexOption {
	return func(o *mutexOptions) {
		if ttl >= time.Second*2 {
			o.ttl = ttl
		}
	}
}

// WithLockIntent sets the lock intent.
func WithLockIntent() MutexOption {
	return func(o *mutexOptions) {
		o.lockIntent = true
	}
}

// LockOption is the option for the lock.
type LockOption func(*lockOptions)

type lockOptions struct {
	ctx              context.Context
	key              string
	lockIntentKey    string
	enableLockIntent int
	ttl              time.Duration
	log              LogFunc
}

func newLockOptions(m mutexOptions, opt ...LockOption) lockOptions {
	opts := lockOptions{
		ctx:              context.Background(),
		key:              m.name,
		enableLockIntent: boolToInt(m.lockIntent),
		ttl:              m.ttl,
		log:              m.log,
	}

	for _, o := range opt {
		o(&opts)
	}

	opts.lockIntentKey = lockIntentKey(opts.key)
	return opts
}

// WithKey sets the key of the lock.
func WithKey(key string) LockOption {
	return func(o *lockOptions) {
		if key != "" {
			o.key += ":" + key
		}
	}
}

// WithContext sets the context of the lock.
func WithContext(ctx context.Context) LockOption {
	return func(o *lockOptions) {
		if ctx != nil {
			o.ctx = ctx
		}
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func lockIntentKey(key string) string {
	return key + ":lock-intent"
}
