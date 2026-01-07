package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	RateLimit RateLimitConfig
	URL      URLConfig
}

type AppConfig struct {
	Env     string
	Port    string
	BaseURL string
}

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
	MaxConns int
	MinConns int
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
	PoolSize int
}

type RateLimitConfig struct {
	Requests int
	Duration time.Duration
}

type URLConfig struct {
	DefaultExpiry   time.Duration
	ShortCodeLength int
}

func Load() (*Config, error) {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	// Set defaults
	setDefaults()

	// Read config file (optional, env vars take precedence)
	_ = viper.ReadInConfig()

	cfg := &Config{
		App: AppConfig{
			Env:     viper.GetString("APP_ENV"),
			Port:    viper.GetString("APP_PORT"),
			BaseURL: viper.GetString("APP_BASE_URL"),
		},
		Postgres: PostgresConfig{
			Host:     viper.GetString("POSTGRES_HOST"),
			Port:     viper.GetString("POSTGRES_PORT"),
			User:     viper.GetString("POSTGRES_USER"),
			Password: viper.GetString("POSTGRES_PASSWORD"),
			DBName:   viper.GetString("POSTGRES_DB"),
			SSLMode:  viper.GetString("POSTGRES_SSLMODE"),
			MaxConns: viper.GetInt("POSTGRES_MAX_CONNS"),
			MinConns: viper.GetInt("POSTGRES_MIN_CONNS"),
		},
		Redis: RedisConfig{
			Host:     viper.GetString("REDIS_HOST"),
			Port:     viper.GetString("REDIS_PORT"),
			Password: viper.GetString("REDIS_PASSWORD"),
			DB:       viper.GetInt("REDIS_DB"),
			PoolSize: viper.GetInt("REDIS_POOL_SIZE"),
		},
		RateLimit: RateLimitConfig{
			Requests: viper.GetInt("RATE_LIMIT_REQUESTS"),
			Duration: viper.GetDuration("RATE_LIMIT_DURATION"),
		},
		URL: URLConfig{
			DefaultExpiry:   viper.GetDuration("URL_DEFAULT_EXPIRY"),
			ShortCodeLength: viper.GetInt("SHORT_CODE_LENGTH"),
		},
	}

	return cfg, nil
}

func setDefaults() {
	viper.SetDefault("APP_ENV", "production")
	viper.SetDefault("APP_PORT", "8080")
	viper.SetDefault("APP_BASE_URL", "http://localhost")

	viper.SetDefault("POSTGRES_HOST", "localhost")
	viper.SetDefault("POSTGRES_PORT", "5432")
	viper.SetDefault("POSTGRES_USER", "shorturl")
	viper.SetDefault("POSTGRES_PASSWORD", "shorturl")
	viper.SetDefault("POSTGRES_DB", "shorturl")
	viper.SetDefault("POSTGRES_SSLMODE", "disable")
	viper.SetDefault("POSTGRES_MAX_CONNS", 25)
	viper.SetDefault("POSTGRES_MIN_CONNS", 5)

	viper.SetDefault("REDIS_HOST", "localhost")
	viper.SetDefault("REDIS_PORT", "6379")
	viper.SetDefault("REDIS_PASSWORD", "")
	viper.SetDefault("REDIS_DB", 0)
	viper.SetDefault("REDIS_POOL_SIZE", 10)

	viper.SetDefault("RATE_LIMIT_REQUESTS", 100)
	viper.SetDefault("RATE_LIMIT_DURATION", "1m")

	viper.SetDefault("URL_DEFAULT_EXPIRY", "0")
	viper.SetDefault("SHORT_CODE_LENGTH", 6)
}

func (c *PostgresConfig) DSN() string {
	return "postgres://" + c.User + ":" + c.Password + "@" + c.Host + ":" + c.Port + "/" + c.DBName + "?sslmode=" + c.SSLMode
}

func (c *RedisConfig) Addr() string {
	return c.Host + ":" + c.Port
}

