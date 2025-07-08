package infra

import (
	"context"
	"fmt"
	"time"
	"sync"
	"github.com/redis/go-redis/v9"
)

var (
	redisClient *redis.Client
	once sync.Once
)
func RDB() *redis.Client {
	once.Do(func() {
		addr := getenv("REDIS_ADDR", "redis:6379")
		rdb = redis.NewClient(&redis.Options{Addr:addr})
	})
	return rdb
}
const lockLua = `if redis.call('SETNX',KEYS[1],ARGV[1]) == 1 
	then redis.call('PEXPIRE',KEYS[1],ARGV[2] ); return 1 else return 0 end`

const unlockLua = `if redis.call('GET',KEYS[1]) == ARGV[1] then 
	return redis.call('DEL',KEYS[1]); else return 0 end`

func Acquire(ctx context.Context, key string, ttl time.Duration) (token string, ok bool, err error) {
	token = uuid.New().String()
	ok, err = RDB().Eval(ctx, lockLua, []string{key}, token, ttl.Milliseconds()).Bool()
	return
}

func Release(ctx context.Context, key string, token string) (ok bool, err error) {
	ok, err = RDB().Eval(ctx, unlockLua, []string{key}, token).Bool()
	return
}