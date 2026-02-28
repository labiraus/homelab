package redisutil

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"
)

type RedisConfig struct {
	Host string `yaml:"host"`
	Port string `yaml:"port"`
}

var Set func(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
var Get func(ctx context.Context, key string) *redis.StringCmd
var Scan func(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd
var Del func(ctx context.Context, keys ...string) *redis.IntCmd

func Init(ctx context.Context, config map[string]RedisConfig) error {
	var err error
	if len(config) == 1 {
		for _, v := range config {
			err = initSingle(ctx, v)
		}
	} else {
		err = initCluster(ctx, config)
	}

	if err != nil {
		return err
	}
	return nil
}

func ParseRedisConfig(config map[string]string) (map[string]RedisConfig, error) {
	redis := make(map[string]RedisConfig, len(config))
	var redisConfigValue RedisConfig
	for k, v := range config {
		err := yaml.Unmarshal([]byte(v), &redisConfigValue)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal build config %v: %v", k, err)
		}
		redis[k] = redisConfigValue
	}
	return redis, nil
}

func initCluster(ctx context.Context, config map[string]RedisConfig) error {
	addrs := make([]string, len(config))
	for _, v := range config {
		addrs = append(addrs, v.Host+":"+v.Port)
	}
	slog.Info("initializing redis", "addrs", addrs)
	rdb := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: addrs,
	})
	Set = rdb.Set
	Get = rdb.Get
	Scan = rdb.Scan
	Del = rdb.Del

	err := rdb.ForEachShard(ctx, ping)
	if err != nil {
		return err
	}
	return nil
}

func initSingle(ctx context.Context, config RedisConfig) error {
	rdb := redis.NewClient(&redis.Options{Addr: config.Host + ":" + config.Port})
	Set = rdb.Set
	Get = rdb.Get
	Scan = rdb.Scan
	Del = rdb.Del
	err := ping(ctx, rdb)
	if err != nil {
		return err
	}
	return nil
}

func ping(ctx context.Context, shard *redis.Client) error {
	if shard == nil {
		return fmt.Errorf("shard is nil")
	}
	status := shard.Ping(ctx)
	slog.Info(fmt.Sprintf("pinging shard %v: %v", shard.String(), status.String()))
	if status.Err() != nil {
		return fmt.Errorf("failed to ping shard %v: %w", shard.String(), status.Err())
	}
	return nil
}
