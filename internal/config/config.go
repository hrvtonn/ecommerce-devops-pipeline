package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Env       string
	Port      int
	DB        DBConfig
	Redis     RedisConfig
	RateLimit RateLimitConfig
}

type DBConfig struct {
	Host     string
	Port     int
	Name     string
	User     string
	Password string
	PoolMin  int32
	PoolMax  int32
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

type RateLimitConfig struct {
	Window time.Duration
	Max    int
}

// Load reads the configuration from environment variables (and from a .env
// file, when present), applying the same defaults as .env.example.
func Load() Config {
	_ = godotenv.Load()

	return Config{
		Env:  getString("APP_ENV", "development"),
		Port: getInt("PORT", 3000),

		DB: DBConfig{
			Host:     getString("DB_HOST", "localhost"),
			Port:     getInt("DB_PORT", 5432),
			Name:     getString("DB_NAME", "ecommerce"),
			User:     getString("DB_USER", "ecommerce"),
			Password: getString("DB_PASSWORD", "ecommerce123"),
			PoolMin:  int32(getInt("DB_POOL_MIN", 2)),
			PoolMax:  int32(getInt("DB_POOL_MAX", 10)),
		},

		Redis: RedisConfig{
			Host:     getString("REDIS_HOST", "localhost"),
			Port:     getInt("REDIS_PORT", 6379),
			Password: getString("REDIS_PASSWORD", ""),
			DB:       getInt("REDIS_DB", 0),
		},

		RateLimit: RateLimitConfig{
			Window: time.Duration(getInt("RATE_LIMIT_WINDOW_MS", 900000)) * time.Millisecond,
			Max:    getInt("RATE_LIMIT_MAX_REQUESTS", 100),
		},
	}
}

func (c Config) IsProduction() bool { return c.Env == "production" }

func (c Config) IsTest() bool { return c.Env == "test" }

// DSN builds a PostgreSQL connection URL.
func (d DBConfig) DSN() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(d.User, d.Password),
		Host:     fmt.Sprintf("%s:%d", d.Host, d.Port),
		Path:     d.Name,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

func (r RedisConfig) Addr() string { return fmt.Sprintf("%s:%d", r.Host, r.Port) }

func getString(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil || v == 0 {
		return fallback
	}
	return v
}
