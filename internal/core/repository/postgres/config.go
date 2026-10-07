package core_postgres_pool

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Addr string `envconfig:"DATABASE_URL" required:"true"`

	OperationTimeout time.Duration `envconfig:"DATABASE_TIMEOUT" default:"30s"`
}

func NewConfig() (Config, error) {
	var config Config

	if err := envconfig.Process("", &config); err != nil {
		return Config{}, fmt.Errorf("can't convert config: %w", err)
	}

	if config.OperationTimeout <= 0 {
		return Config{}, fmt.Errorf("DATABASE_TIMEOUT must be positive")
	}
	return config, nil
}

func NewConfigMust() Config {
	config, err := NewConfig()
	if err != nil {
		err = fmt.Errorf("init pgxpool config: %w", err)
		panic(err)
	}

	return config
}
