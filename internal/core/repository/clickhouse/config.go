package core_clickhouse

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Addr     string        `envconfig:"CLICKHOUSE_ADDR" required:"true"`
	Database string        `envconfig:"CLICKHOUSE_DB" required:"true"`
	Username string        `envconfig:"CLICKHOUSE_USER" required:"true"`
	Password string        `envconfig:"CLICKHOUSE_PASSWORD" required:"true"`
	Timeout  time.Duration `envconfig:"CLICKHOUSE_TIMEOUT" default:"5s"`
}

func NewConfig() (Config, error) {
	var config Config
	if err := envconfig.Process("", &config); err != nil {
		return Config{}, fmt.Errorf("parse clickhouse config: %w", err)
	}

	if config.Timeout <= 0 {
		return Config{}, fmt.Errorf("CLICKHOUSE_TIMEOUT must be positive")
	}

	return config, nil
}

func NewConfigMust() Config {
	config, err := NewConfig()
	if err != nil {
		panic(fmt.Errorf("init clickhouse config: %w", err))
	}

	return config
}
