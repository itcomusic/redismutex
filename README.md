# RWMutex

[![build-img]][build-url]
[![pkg-img]][pkg-url]
[![coverage-img]][coverage-url]

Distributed rw locking implementation using [Redis](https://redis.io/docs/manual/patterns/distributed-locks/) with auto extended lock expiration. 
RWMutex can work in write prefer mode, when readers do not acquire lock if writer has declared intention to lock.

## Example

```go
package main

import (
	"context"
	"fmt"
	"log"
    
	"github.com/go-redis/redis/v8"
	"github.com/itcomusic/redismutex"
)

func main() {
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

    fmt.Printf("lock %q! \n", lock.Key())
    handler(lock) 
}
```

## License
[MIT License](LICENSE)

[build-img]: https://github.com/itcomusic/redismutex/workflows/test/badge.svg
[build-url]: https://github.com/itcomusic/redismutex/actions
[pkg-img]: https://pkg.go.dev/badge/github.com/itcomusic/redismutex.svg
[pkg-url]: https://pkg.go.dev/github.com/itcomusic/redismutex
[coverage-img]: https://codecov.io/gh/itcomusic/redismutex/branch/main/graph/badge.svg
[coverage-url]: https://codecov.io/gh/itcomusic/redismutex