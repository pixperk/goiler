package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Auth     AuthConfig
	OTEL     OTELConfig
	RateLimit RateLimitConfig
}

type AppConfig struct {
	Env  string
	Port string
	Name string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
	URL      string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type AuthConfig struct {
	Type               string // "jwt" or "paseto"
	JWTSecret          string
	JWTAccessExpiry    time.Duration
	JWTRefreshExpiry   time.Duration
	PASETOSymmetricKey string
}

type OTELConfig struct {
	Enabled     bool
	ServiceName string
	Endpoint    string
}

type RateLimitConfig struct {
	Requests int
	Duration time.Duration
}

func Load() *Config {
	return &Config{
		App: AppConfig{
			Env:  getEnv("APP_ENV", "development"),
			Port: getEnv("APP_PORT", "8080"),
			Name: getEnv("APP_NAME", "goiler"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			Name:     getEnv("DB_NAME", "goiler"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
			URL:      getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/goiler?sslmode=disable"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		Auth: AuthConfig{
			Type:               getEnv("AUTH_TYPE", "jwt"),
			JWTSecret:          getEnv("JWT_SECRET", "your-super-secret-jwt-key"),
			JWTAccessExpiry:    getEnvDuration("JWT_ACCESS_EXPIRY", 15*time.Minute),
			JWTRefreshExpiry:   getEnvDuration("JWT_REFRESH_EXPIRY", 168*time.Hour),
			PASETOSymmetricKey: getEnv("PASETO_SYMMETRIC_KEY", ""),
		},
		OTEL: OTELConfig{
			Enabled:     getEnvBool("OTEL_ENABLED", true),
			ServiceName: getEnv("OTEL_SERVICE_NAME", "goiler"),
			Endpoint:    getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
		},
		RateLimit: RateLimitConfig{
			Requests: getEnvInt("RATE_LIMIT_REQUESTS", 100),
			Duration: getEnvDuration("RATE_LIMIT_DURATION", time.Minute),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
