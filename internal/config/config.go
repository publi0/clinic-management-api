package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Port                  string        `env:"PORT" envDefault:"8080"`
	DatabaseURL           string        `env:"DATABASE_URL,required"`
	OTelEnabled           bool          `env:"OTEL_ENABLED" envDefault:"true"`
	OTelServiceName       string        `env:"OTEL_SERVICE_NAME" envDefault:"capim-test-api"`
	JWTSecret             string        `env:"JWT_SECRET,required"`
	JWTIssuer             string        `env:"JWT_ISSUER" envDefault:"capim-test-api"`
	JWTAccessTokenTTL     time.Duration `env:"JWT_ACCESS_TOKEN_TTL" envDefault:"15m"`
	BootstrapUserEmail    string        `env:"AUTH_BOOTSTRAP_EMAIL"`
	BootstrapUserPassword string        `env:"AUTH_BOOTSTRAP_PASSWORD"`
}

func Load() (Config, error) {
	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
