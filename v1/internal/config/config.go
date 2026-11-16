package config

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

func LoadConfig(file string) (*Config, error) {
	c := defaultConfig()
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read config file: %w", err)
		}
		if err = yaml.Unmarshal(b, c); err != nil {
			return nil, fmt.Errorf("unmarshal yaml: %w", err)
		}
	}
	cfg, err := GetEnvConfig(c)
	if err != nil {
		return nil, fmt.Errorf("Error in Env: %w", err)
	}
	return cfg, nil
}

func GetEnvConfig(c *Config) (*Config, error) {
	EnvStr("S3_ENV", &c.App.Environment)
	EnvInt("S3_PORT", &c.Server.Port)
	EnvStr("S3_SLUG", &c.Server.InitialSlug)
	EnvStr("S3_PATH", &c.Storage.Path)
	EnvInt("S3_PORT", &c.Server.Port)
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 9000
	}
	if c.Storage.MaxUploadSize == 0 {
		c.Storage.MaxUploadSize = 10 << 30
	}
	if c.Storage.Path == "" {
		return nil, fmt.Errorf("storage.path are required")
	}
	if c.Storage.Driver != "" && c.Storage.Driver != "filesystem" {
		return nil, fmt.Errorf("only filesystem storage driver is supported")
	}
	return c, nil
}
