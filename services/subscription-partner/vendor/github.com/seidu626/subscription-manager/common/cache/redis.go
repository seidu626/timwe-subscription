package cached

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClientWrapper defines the interface for our RedisConfig operations
type RedisClientWrapper interface {
	Set(ctx context.Context, key string, value interface{}, ttl int) error
	Get(ctx context.Context, key string) (string, error)
	GetBytes(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, keys ...string) error
	BeginTx(ctx context.Context) (RedisClientWrapper, error)
	ExecutePipeline(ctx context.Context, actions []func(redis.Pipeliner) error) ([]redis.Cmder, error)
	GetAllKeys(ctx context.Context, pattern string) ([]string, error)
	GetMultiple(ctx context.Context, keys []string) ([]interface{}, error)
}

// Client wraps a go-redis/v9 client
type Client struct {
	client    *redis.Client
	pipeliner redis.Pipeliner
}

// NewClient creates a new redis client wrapper
func NewClient(redisOptions *redis.Options, options ...Option) *Client {
	opts := redisOptions

	for _, opt := range options {
		opt(opts)
	}

	client := redis.NewClient(opts)
	ctx := context.Background()
	ping, err := client.Ping(ctx).Result()
	if err != nil {
		log.Printf("ERROR: Redis Connection failed: %v", err)
	} else {
		log.Printf("INFO: Redis Connection successful: %s", ping)
	}

	return &Client{client: client}
}

// Option configures the RedisConfig client
type Option func(*redis.Options)

// WithAddress is an option to set the address of the RedisConfig server
func WithAddress(addr string) Option {
	return func(opts *redis.Options) {
		opts.Addr = addr
	}
}

// WithConfig initialize with an existing config
func WithConfig(options *redis.Options) Option {
	return func(opts *redis.Options) {
		opts = options
	}
}

// Set sets a key with a value and an expiration in seconds
func (c *Client) Set(ctx context.Context, key string, value interface{}, ttl int) error {
	return c.client.Set(ctx, key, value, time.Duration(ttl)*time.Second).Err()
}

// Get retrieves a value for a key
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

// GetBytes retrieves a value for a key
func (c *Client) GetBytes(ctx context.Context, key string) ([]byte, error) {
	return c.client.Get(ctx, key).Bytes()
}

// Delete removes keys
func (c *Client) Delete(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

// GetAllKeys retrieves all keys matching a specific pattern
func (c *Client) GetAllKeys(ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	var cursor uint64
	var err error
	for {
		var ks []string
		ks, cursor, err = c.client.Scan(ctx, cursor, pattern, 0).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, ks...)
		if cursor == 0 { // No more keys
			break
		}
	}
	return keys, nil
}

// GetMultiple retrieves multiple values by keys
func (c *Client) GetMultiple(ctx context.Context, keys []string) ([]interface{}, error) {
	return c.client.MGet(ctx, keys...).Result()
}

// BeginTx begins a new transaction
func (c *Client) BeginTx(ctx context.Context) (RedisClientWrapper, error) {
	tx := c.client.TxPipeline()
	return &Client{pipeliner: tx}, nil
}

// ExecutePipeline executes multiple commands in a pipeline
func (c *Client) ExecutePipeline(ctx context.Context, actions []func(redis.Pipeliner) error) ([]redis.Cmder, error) {
	pipeline := c.client.Pipeline()
	for _, action := range actions {
		if err := action(pipeline); err != nil {
			//pipeline.Close()
			return nil, err
		}
	}
	return pipeline.Exec(ctx)
}

func (c *Client) Close() error {
	return c.client.Close()
}
