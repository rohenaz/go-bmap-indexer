// cache/cache.go
package cache

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/ttacon/chalk"
)

var ctx = context.Background()
var rdb *redis.Client
var mu sync.Mutex
var Connected = false
var cacheChalk = chalk.Red.NewStyle().WithBackground(chalk.Black)

func onRedisConnect(ctx context.Context, cn *redis.Conn) error {
	mu.Lock()
	defer mu.Unlock()

	fmt.Printf("%sRedis cache connected%s\n", cacheChalk, chalk.Reset)
	Connected = true
	return nil
}

func Connect() {

	url := os.Getenv("REDIS_PRIVATE_URL")
	opts, err := redis.ParseURL(url)
	if err != nil {
		panic(err)
	})

	opts.OnConnect = onRedisConnect
	rdb = redis.NewClient(opts)
	fmt.Printf("%sConnecting to Redis cache%s\n", cacheChalk, chalk.Reset)

}

// Set a value in Redis
func Set(key string, value string) error {
	return rdb.Set(ctx, key, value, 0).Err()
}

// Get a value from Redis
func Get(key string) (string, error) {
	return rdb.Get(ctx, key).Result()
}
