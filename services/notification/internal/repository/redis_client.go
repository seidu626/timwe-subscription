package repository

import (
	"fmt"
	"github.com/redis/go-redis/v9"
)

func NewRedisClient(host string, port int, password string, db int) *redis.Client {
	addr := fmt.Sprintf("%s:%d", host, port)
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password, // no password set
		DB:       db,       // use default DB
	})
}
