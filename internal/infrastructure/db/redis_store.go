package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	domain "subtitle-delivery/internal/domain"

	redis "github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Addr      string
	Password  string
	DB        int
	KeyPrefix string
	TTL       time.Duration
}

type RedisStore struct {
	client    *redis.Client
	keyPrefix string
	ttl       time.Duration
}

func NewRedisStore(config RedisConfig) (*RedisStore, error) {
	if config.Addr == "" {
		return nil, errors.New("redis address is required")
	}
	if config.KeyPrefix == "" {
		config.KeyPrefix = "subtitle-delivery"
	}
	if config.TTL == 0 {
		config.TTL = 10 * time.Minute
	}

	client := redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Password: config.Password,
		DB:       config.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisStore{
		client:    client,
		keyPrefix: config.KeyPrefix,
		ttl:       config.TTL,
	}, nil
}

func (store *RedisStore) Close() error {
	if store == nil || store.client == nil {
		return nil
	}
	return store.client.Close()
}

func (store *RedisStore) Save(ctx context.Context, record domain.Subtitle) error {
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}

	pipe := store.client.TxPipeline()
	pipe.Set(ctx, store.subtitleKey(record.ID), encoded, store.ttl)
	pipe.Set(ctx, store.latestKey(), encoded, store.ttl)
	_, err = pipe.Exec(ctx)
	return err
}

func (store *RedisStore) Latest(ctx context.Context) (domain.Subtitle, error) {
	value, err := store.client.Get(ctx, store.latestKey()).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return domain.Subtitle{}, errors.New("subtitle not found")
		}
		return domain.Subtitle{}, err
	}

	var subtitle domain.Subtitle
	if err := json.Unmarshal([]byte(value), &subtitle); err != nil {
		return domain.Subtitle{}, err
	}
	return subtitle, nil
}

func (store *RedisStore) subtitleKey(id string) string {
	return fmt.Sprintf("%s:subtitle:%s", store.keyPrefix, id)
}

func (store *RedisStore) latestKey() string {
	return fmt.Sprintf("%s:latest", store.keyPrefix)
}
