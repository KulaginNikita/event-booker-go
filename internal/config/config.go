package config

import (
	"fmt"
	"os"
	"time"

	wbfconfig "github.com/wb-go/wbf/config"
)

type Config struct {
	App       AppConfig       `mapstructure:"app"`
	HTTP      HTTPConfig      `mapstructure:"http"`
	Postgres  PostgresConfig  `mapstructure:"postgres"`
	Booking   BookingConfig   `mapstructure:"booking"`
	Scheduler SchedulerConfig `mapstructure:"scheduler"`
	Email     EmailConfig     `mapstructure:"email"`
	Telegram  TelegramConfig  `mapstructure:"telegram"`
	Logger    LoggerConfig    `mapstructure:"logger"`
}

type AppConfig struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
}

type HTTPConfig struct {
	Port string `mapstructure:"port"`
}

type PostgresConfig struct {
	DSN         string `mapstructure:"dsn"`
	MaxPoolSize int32  `mapstructure:"max_pool_size"`
}

type BookingConfig struct {
	PaymentDeadline time.Duration `mapstructure:"payment_deadline"`
}

type SchedulerConfig struct {
	Interval time.Duration `mapstructure:"interval"`
}

type EmailConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	SMTPHost string `mapstructure:"smtp_host"`
	SMTPPort int    `mapstructure:"smtp_port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
}

type TelegramConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Token   string `mapstructure:"token"`
}

type LoggerConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

func Load(path string) (*Config, error) {
	cfg := wbfconfig.New()
	if err := cfg.LoadConfigFiles(path); err != nil {
		return nil, fmt.Errorf("load config file: %w", err)
	}
	cfg.EnableEnv("APP")

	var c Config
	if err := cfg.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if c.App.Name == "" {
		c.App.Name = "event-booker"
	}
	if c.App.Env == "" {
		c.App.Env = "local"
	}
	if c.HTTP.Port == "" {
		c.HTTP.Port = "8080"
	}
	if c.Postgres.MaxPoolSize == 0 {
		c.Postgres.MaxPoolSize = 10
	}
	if c.Booking.PaymentDeadline == 0 {
		c.Booking.PaymentDeadline = 2 * time.Minute
	}
	if c.Scheduler.Interval == 0 {
		c.Scheduler.Interval = 10 * time.Second
	}
	applyNotificationEnv(&c)
	if c.Email.SMTPHost == "" {
		c.Email.SMTPHost = "smtp.gmail.com"
	}
	if c.Email.SMTPPort == 0 {
		c.Email.SMTPPort = 587
	}
	if c.Email.From == "" {
		c.Email.From = c.Email.Username
	}
	if c.Logger.Level == "" {
		c.Logger.Level = "info"
	}
	if c.Logger.Format == "" {
		c.Logger.Format = "console"
	}

	return &c, nil
}

func applyNotificationEnv(c *Config) {
	c.Email.Enabled = envBool("EMAIL_ENABLED", c.Email.Enabled)
	c.Email.SMTPHost = envString("EMAIL_SMTP_HOST", c.Email.SMTPHost)
	c.Email.SMTPPort = envInt("EMAIL_SMTP_PORT", c.Email.SMTPPort)
	c.Email.Username = envString("EMAIL_USERNAME", c.Email.Username)
	c.Email.Password = envString("EMAIL_PASSWORD", c.Email.Password)
	c.Email.From = envString("EMAIL_FROM", c.Email.From)

	c.Telegram.Enabled = envBool("TG_ENABLED", c.Telegram.Enabled)
	c.Telegram.Token = envString("TG_TOKEN", c.Telegram.Token)
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
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return fallback
	}
	return parsed
}
