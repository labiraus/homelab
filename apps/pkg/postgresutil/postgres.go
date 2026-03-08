package postgresutil

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/yaml.v3"
)

type PostgresConfig struct {
	Host                     string `yaml:"host"`
	Port                     string `yaml:"port"`
	User                     string `yaml:"user"`
	Password                 string `yaml:"password"`
	Database                 string `yaml:"database"`
	SSLMode                  string `yaml:"sslmode"`
	MaxConns                 int32  `yaml:"maxConns"`
	MinConns                 int32  `yaml:"minConns"`
	MaxConnLifetimeMinutes   int    `yaml:"maxConnLifetimeMinutes"`
	MaxConnIdleTimeMinutes   int    `yaml:"maxConnIdleTimeMinutes"`
	HealthCheckPeriodSeconds int    `yaml:"healthCheckPeriodSeconds"`
	ConnectTimeoutSeconds    int    `yaml:"connectTimeoutSeconds"`
}

var (
	Exec     func(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query    func(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow func(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Begin    func(ctx context.Context) (pgx.Tx, error)

	pools   = map[string]*pgxpool.Pool{}
	poolsMu = sync.RWMutex{}
)

func Init(ctx context.Context, config map[string]PostgresConfig) error {
	if len(config) == 0 {
		return fmt.Errorf("postgres config is empty")
	}

	var defaultPool *pgxpool.Pool
	for name, cfg := range config {
		pool, err := initPool(ctx, name, cfg)
		if err != nil {
			return err
		}
		if defaultPool == nil {
			defaultPool = pool
		}
	}

	Exec = defaultPool.Exec
	Query = defaultPool.Query
	QueryRow = defaultPool.QueryRow
	Begin = defaultPool.Begin

	return nil
}

func ParsePostgresConfig(config map[string]string) (map[string]PostgresConfig, error) {
	postgres := make(map[string]PostgresConfig, len(config))
	var postgresConfigValue PostgresConfig
	for k, v := range config {
		err := yaml.Unmarshal([]byte(v), &postgresConfigValue)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal build config %v: %v", k, err)
		}
		postgres[k] = postgresConfigValue
	}
	return postgres, nil
}

func GetPool(name string) (*pgxpool.Pool, error) {
	poolsMu.RLock()
	defer poolsMu.RUnlock()

	pool, ok := pools[name]
	if !ok {
		return nil, fmt.Errorf("postgres pool %s not initialized", name)
	}
	return pool, nil
}

func initPool(ctx context.Context, name string, config PostgresConfig) (*pgxpool.Pool, error) {
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		config.User,
		config.Password,
		config.Host,
		defaultString(config.Port, "5432"),
		config.Database,
		defaultString(config.SSLMode, "disable"),
	)
	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("could not parse postgres config %s: %v", name, err)
	}

	if config.MaxConns > 0 {
		poolConfig.MaxConns = config.MaxConns
	}
	if config.MinConns > 0 {
		poolConfig.MinConns = config.MinConns
	}
	if config.MaxConnLifetimeMinutes > 0 {
		poolConfig.MaxConnLifetime = time.Duration(config.MaxConnLifetimeMinutes) * time.Minute
	}
	if config.MaxConnIdleTimeMinutes > 0 {
		poolConfig.MaxConnIdleTime = time.Duration(config.MaxConnIdleTimeMinutes) * time.Minute
	}
	if config.HealthCheckPeriodSeconds > 0 {
		poolConfig.HealthCheckPeriod = time.Duration(config.HealthCheckPeriodSeconds) * time.Second
	}
	if config.ConnectTimeoutSeconds > 0 {
		poolConfig.ConnConfig.ConnectTimeout = time.Duration(config.ConnectTimeoutSeconds) * time.Second
	}

	slog.Info("initializing postgres", "name", name, "host", config.Host, "port", defaultString(config.Port, "5432"), "database", config.Database)
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("could not create postgres pool %s: %v", name, err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("could not ping postgres %s: %v", name, err)
	}

	poolsMu.Lock()
	pools[name] = pool
	poolsMu.Unlock()

	go func() {
		<-ctx.Done()
		pool.Close()
	}()

	return pool, nil
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
