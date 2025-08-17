package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App struct {
		LogLevel string `yaml:"log_level"`
	} `yaml:"app"`

	WS struct {
		URL            string `yaml:"url"`
		Token          string `yaml:"token"`
		HeartbeatSec   int    `yaml:"heartbeat_sec"`
		ReadTimeoutSec int    `yaml:"read_timeout_sec"`
		Reconnect      struct {
			Enabled     bool `yaml:"enabled"`
			MaxRetries  int  `yaml:"max_retries"`
			BaseSeconds int  `yaml:"base_seconds"`
			MaxSeconds  int  `yaml:"max_seconds"`
		} `yaml:"reconnect"`
	} `yaml:"ws"`

	Store struct {
		Path string `yaml:"path"`
	} `yaml:"store"`
}

func (c Config) Heartbeat() time.Duration   { return time.Duration(c.WS.HeartbeatSec) * time.Second }
func (c Config) ReadTimeout() time.Duration { return time.Duration(c.WS.ReadTimeoutSec) * time.Second }

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("yaml: %w", err)
	}
	return cfg, nil
}
