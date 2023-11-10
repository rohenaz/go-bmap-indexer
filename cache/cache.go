// cache/cache.go
package cache

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/go-redis/redis/v8"
)

var ctx = context.Background()
var rdb *redis.Client
var mu sync.Mutex
var Connected = false

func onRedisConnect(ctx context.Context, cn *redis.Conn) error {
	mu.Lock()
	defer mu.Unlock()

	log.Println("Redis cache connected")
	Connected = true
	return nil
}

func Connect() {
	rdb = redis.NewClient(&redis.Options{
		Addr:      os.Getenv("REDIS_PRIVATE_URL"),
		OnConnect: onRedisConnect,
	})
	log.Println("Redis cache initialized")

}

// Set a value in Redis
func Set(key string, value string) error {
	return rdb.Set(ctx, key, value, 0).Err()
}

// Get a value from Redis
func Get(key string) (string, error) {
	return rdb.Get(ctx, key).Result()
}
