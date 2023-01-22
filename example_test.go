package redismutex_test

import (
	"context"
	"fmt"
	"log"

	"github.com/go-redis/redis/v8"

	"github.com/itcomusic/redismutex"
)

func Example() {
	// connect to redis
	rc := redis.NewClient(&redis.Options{
		Network: "tcp",
		Addr:    "127.0.0.1:6379",
	})
	defer rc.Close()

	// init mutex
	mx := redismutex.NewMutex(rc, "mutex_name")

	// lock, optionally add key and lock_key will be "mutex_name:key"
	lock, ok := mx.Lock(redismutex.WithKey("key")) // also available RLock, TryLock, TryRLock
	if !ok {
		log.Fatalln("could not obtain lock, disconnected from redis?")
	}
	defer lock.Unlock()

	// lock implements interface context.Context
	handler := func(ctx context.Context) {
		select {
		case <-ctx.Done():
			fmt.Println("lock expired")

		default:
			fmt.Println("lock active")
		}
	}

	fmt.Printf("lock %q!\n", lock.Key())
	handler(lock)

	// Output:
	// lock "mutex_name:key"!
	// lock active
}
