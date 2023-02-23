// Package redismutex provides a distributed rw mutex.
package redismutex

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrLock = errors.New("redismutex: lock not obtained")

var (
	//go:embed lua
	lua embed.FS

	scriptRLock   *redis.Script
	scriptLock    *redis.Script
	scriptRefresh *redis.Script
	scriptUnlock  *redis.Script
)

func init() {
	scriptRLock = redis.NewScript(mustReadFile("rlock.lua"))
	scriptLock = redis.NewScript(mustReadFile("lock.lua"))
	scriptRefresh = redis.NewScript(mustReadFile("refresh.lua"))
	scriptUnlock = redis.NewScript(mustReadFile("unlock.lua"))
}

// A RWMutex is a distributed mutual exclusion lock.
type RWMutex struct {
	redis redis.Scripter
	opts  mutexOptions

	id struct {
		sync.Mutex
		buf []byte
	}
}

// NewMutex creates a new distributed mutex.
func NewMutex(rc redis.Scripter, name string, opt ...MutexOption) *RWMutex {
	globalMx.RLock()
	defer globalMx.RUnlock()

	opts := mutexOptions{
		name: name,
		ttl:  defaultKeyTTL,
		log:  globalLog,
	}

	for _, o := range opt {
		o(&opts)
	}

	rw := &RWMutex{
		redis: rc,
		opts:  opts,
	}
	rw.id.buf = make([]byte, lenBytesID)
	return rw
}

// TryRLock tries to lock for reading and reports whether it succeeded.
func (m *RWMutex) TryRLock(opt ...LockOption) (*Lock, bool) {
	opts := newLockOptions(m.opts, opt...)
	ctx, _, err := m.rlock(opts)
	if err != nil {
		if !errors.Is(err, ErrLock) {
			m.opts.log("[ERROR] try-read-lock key %q: %v", opts.key, err)
		}
		return nil, false
	}
	return ctx, true
}

// RLock locks for reading.
func (m *RWMutex) RLock(opt ...LockOption) (*Lock, bool) {
	opts := newLockOptions(m.opts, opt...)
	ctx, ttl, err := m.rlock(opts)
	if err == nil {
		return ctx, true
	}

	if !errors.Is(err, ErrLock) {
		m.opts.log("[ERROR] read-lock key %q: %v", opts.key, err)
		return nil, false
	}

	for {
		select {
		case <-opts.ctx.Done():
			m.opts.log("[ERROR] read-lock key %q: %v", opts.key, opts.ctx.Err())
			return nil, false

		case <-time.After(ttl):
			ctx, ttl, err = m.rlock(opts)
			if err == nil {
				return ctx, true
			}

			if !errors.Is(err, ErrLock) {
				m.opts.log("[ERROR] read-lock key %q: %v", opts.key, err)
				return nil, false
			}
			continue
		}
	}
}

// TryLock tries to lock for writing and reports whether it succeeded.
func (m *RWMutex) TryLock(opt ...LockOption) (*Lock, bool) {
	opts := newLockOptions(m.opts, opt...)
	opts.enableLockIntent = 0 // force disable lock intent

	ctx, _, err := m.lock(opts)
	if err != nil {
		if !errors.Is(err, ErrLock) {
			m.opts.log("[ERROR] try-lock key %q: %v", opts.key, err)
		}
		return nil, false
	}
	return ctx, true
}

// Lock locks for writing.
func (m *RWMutex) Lock(opt ...LockOption) (*Lock, bool) {
	opts := newLockOptions(m.opts, opt...)
	ctx, ttl, err := m.lock(opts)

	if err == nil {
		return ctx, true
	}

	if !errors.Is(err, ErrLock) {
		m.opts.log("[ERROR] lock key %q: %v", opts.key, err)
		return nil, false
	}

	for {
		select {
		case <-opts.ctx.Done():
			m.opts.log("[ERROR] lock key %q: %v", opts.key, opts.ctx.Err())
			return nil, false

		case <-time.After(ttl):
			ctx, ttl, err = m.lock(opts)
			if err == nil {
				return ctx, true
			}

			if !errors.Is(err, ErrLock) {
				m.opts.log("[ERROR] lock key %q: %v", opts.key, err)
				return nil, false
			}
			continue
		}
	}
}

func (m *RWMutex) lock(opts lockOptions) (*Lock, time.Duration, error) {
	id, err := m.randomID()
	if err != nil {
		return nil, 0, err
	}

	pTTL, err := scriptLock.Run(opts.ctx, m.redis, []string{opts.key, opts.lockIntentKey}, id, opts.ttl.Milliseconds(), opts.enableLockIntent).Result()
	leftTTL := time.Now().Add(opts.ttl)
	if err == nil {
		return nil, time.Duration(pTTL.(int64)) * time.Millisecond, ErrLock
	}

	if err != redis.Nil {
		return nil, 0, err
	}

	ctx, cancel := context.WithCancel(opts.ctx)
	lock := &Lock{
		redis:  m.redis,
		id:     id,
		ttl:    opts.ttl,
		key:    opts.key,
		log:    opts.log,
		ctx:    ctx,
		cancel: cancel,
	}
	go lock.refreshTTL(leftTTL)
	return lock, 0, nil
}

func (m *RWMutex) rlock(opts lockOptions) (*Lock, time.Duration, error) {
	id, err := m.randomID()
	if err != nil {
		return nil, 0, err
	}

	pTTL, err := scriptRLock.Run(opts.ctx, m.redis, []string{opts.key, opts.lockIntentKey}, id, opts.ttl.Milliseconds()).Result()
	leftTTL := time.Now().Add(opts.ttl)
	if err == nil {
		return nil, time.Duration(pTTL.(int64)) * time.Millisecond, ErrLock
	}

	if err != redis.Nil {
		return nil, 0, err
	}

	ctx, cancel := context.WithCancel(opts.ctx)
	lock := &Lock{
		redis:  m.redis,
		id:     id,
		ttl:    opts.ttl,
		key:    opts.key,
		log:    opts.log,
		ctx:    ctx,
		cancel: cancel,
	}
	go lock.refreshTTL(leftTTL)
	return lock, 0, nil
}

// randomID generates a random hex string with 16 bytes.
func (m *RWMutex) randomID() (string, error) {
	m.id.Lock()
	defer m.id.Unlock()

	_, err := rand.Read(m.id.buf)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(m.id.buf), nil
}

func mustReadFile(filename string) string {
	b, err := lua.ReadFile("lua/" + filename)
	if err != nil {
		panic(err)
	}
	return string(b)
}
