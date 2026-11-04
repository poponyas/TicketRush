package core_cache_redis

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Addr    string        `envconfig:"ADDR"`
	Timeout time.Duration `envconfig:"TIMEOUT" default:"1s"`
}

func NewConfig() (Config, error) {
	var config Config
	if err := envconfig.Process("REDIS", &config); err != nil {
		return Config{}, fmt.Errorf("redis config error: %w", err)
	}

	if config.Addr == "" {
		config.Addr = "127.0.0.1:6380"
	}

	return config, nil
}

func NewConfigMust() Config {
	config, err := NewConfig()
	if err != nil {
		panic(err)
	}

	return config
}
