package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App       AppConfig
	HTTP      HTTPConfig
	Postgres  PostgresConfig
	Migration MigrationConfig
	Booking   BookingConfig
	Auth      AuthConfig
	Scheduler SchedulerConfig
	Email     EmailConfig
	Telegram  TelegramConfig
	Logger    LoggerConfig
}

type AppConfig struct {
	Name string
	Env  string
}

type HTTPConfig struct {
	Port              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

type PostgresConfig struct {
	DSN         string
	MaxPoolSize int32
}

type MigrationConfig struct {
	Dir string
}

type BookingConfig struct {
	PaymentDeadline time.Duration
}

type AuthConfig struct {
	JWTSecret string
	TokenTTL  time.Duration
	Issuer    string
	Audience  string
}

type SchedulerConfig struct {
	Interval time.Duration
}

type EmailConfig struct {
	Enabled  bool
	SMTPHost string
	SMTPPort int
	Username string
	Password string
	From     string
}

type TelegramConfig struct {
	Enabled bool
	Token   string
}

type LoggerConfig struct {
	Level  string
	Format string
}

func Load() (*Config, error) {
	_ = godotenv.Load(".env")

	c := Config{
		App: AppConfig{
			Name: envString("APP_NAME", "event-booker"),
			Env:  envString("APP_ENV", "local"),
		},
		HTTP: HTTPConfig{
			Port:              envString("APP_HTTP_PORT", "8080"),
			ReadHeaderTimeout: envDuration("APP_HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			ReadTimeout:       envDuration("APP_HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:      envDuration("APP_HTTP_WRITE_TIMEOUT", 10*time.Second),
			IdleTimeout:       envDuration("APP_HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout:   envDuration("APP_HTTP_SHUTDOWN_TIMEOUT", 10*time.Second),
		},
		Postgres: PostgresConfig{
			DSN:         envString("APP_POSTGRES_DSN", ""),
			MaxPoolSize: int32Env("APP_POSTGRES_MAX_POOL_SIZE", 10),
		},
		Migration: MigrationConfig{
			Dir: envString("APP_MIGRATION_DIR", "migrations"),
		},
		Booking: BookingConfig{
			PaymentDeadline: envDuration("APP_BOOKING_PAYMENT_DEADLINE", 2*time.Minute),
		},
		Auth: AuthConfig{
			JWTSecret: envString("APP_AUTH_JWT_SECRET", ""),
			TokenTTL:  envDuration("APP_AUTH_TOKEN_TTL", 12*time.Hour),
			Issuer:    envString("APP_AUTH_ISSUER", "event-booker"),
			Audience:  envString("APP_AUTH_AUDIENCE", "event-booker-api"),
		},
		Scheduler: SchedulerConfig{
			Interval: envDuration("APP_SCHEDULER_INTERVAL", 10*time.Second),
		},
		Email: EmailConfig{
			Enabled:  envBool("EMAIL_ENABLED", false),
			SMTPHost: envString("EMAIL_SMTP_HOST", "smtp.gmail.com"),
			SMTPPort: envInt("EMAIL_SMTP_PORT", 587),
			Username: envString("EMAIL_USERNAME", ""),
			Password: envString("EMAIL_PASSWORD", ""),
			From:     envString("EMAIL_FROM", ""),
		},
		Telegram: TelegramConfig{
			Enabled: envBool("TG_ENABLED", false),
			Token:   envString("TG_TOKEN", ""),
		},
		Logger: LoggerConfig{
			Level:  envString("APP_LOGGER_LEVEL", "info"),
			Format: envString("APP_LOGGER_FORMAT", "console"),
		},
	}
	if c.Email.From == "" {
		c.Email.From = c.Email.Username
	}
	if c.Postgres.DSN == "" {
		return nil, fmt.Errorf("APP_POSTGRES_DSN is required")
	}
	if c.Auth.JWTSecret == "" {
		return nil, fmt.Errorf("APP_AUTH_JWT_SECRET is required")
	}

	return &c, nil
}

func envString(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	if value := os.Getenv("APP_" + key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := envString(key, "")
	switch value {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return fallback
	}
}

func envInt(key string, fallback int) int {
	value := envString(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func int32Env(key string, fallback int32) int32 {
	value := envString(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return fallback
	}
	return int32(parsed)
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := envString(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
