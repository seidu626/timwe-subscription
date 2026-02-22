package repository

import (
	"fmt"
	"github.com/redis/go-redis/v9"
	cached "github.com/seidu626/subscription-manager/common/cache"
)

func NewRedisClient(host string, port int, password string, db int) cached.RedisClient {
	addr := fmt.Sprintf("%s:%d", host, port)
	return cached.NewFailoverRedisClient(&redis.Options{
		Addr:     addr,
		Password: password, // no password set
		DB:       db,       // use default DB
	})
}
