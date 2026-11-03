// Package config reads and validates the environment at start-up; missing values stop the process.
package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	AppEnv             string `env:"APP_ENV" envDefault:"development"` // development | test | production
	HTTPAddr           string `env:"HTTP_ADDR" envDefault:"127.0.0.1:8080"`
	DatabaseURL        string `env:"DATABASE_URL,notEmpty"`
	MigrateDatabaseURL string `env:"MIGRATE_DATABASE_URL"`
	WebOrigin          string `env:"WEB_ORIGIN" envDefault:"http://localhost:5173"`
	StorageDir         string `env:"STORAGE_DIR" envDefault:"./data/photos"`
	SMTPHost           string `env:"SMTP_HOST"`
	SMTPPort           int    `env:"SMTP_PORT" envDefault:"1025"`
	SMTPFrom           string `env:"SMTP_FROM" envDefault:"gatepass@localhost"`
	SMTPUser           string `env:"SMTP_USER"`
	SMTPPass           string `env:"SMTP_PASS"`
	BcryptCost         int    `env:"BCRYPT_COST" envDefault:"12"`
	SeedPassword       string `env:"SEED_PASSWORD" envDefault:"Gatepass@123"`
}

func Load() (Config, error) {
	var c Config
	if err := env.Parse(&c); err != nil {
		return c, fmt.Errorf("configuration: %w", err)
	}
	switch c.AppEnv {
	case "development", "test", "production":
	default:
		return c, fmt.Errorf("configuration: APP_ENV must be development, test or production")
	}
	return c, nil
}

func (c Config) Production() bool { return c.AppEnv == "production" }
