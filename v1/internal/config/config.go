package config

import (
	"fmt"
	"os"

	"github.com/fehmicorp/s3object/v1/types"
	"go.yaml.in/yaml/v2"
)

func LoadConfig(file string) (*types.Config, error) {
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

func GetEnvConfig(c *types.Config) (*types.Config, error) {
	EnvStr("S3_ENV", &c.App.Environment)
	EnvInt("S3_PORT", &c.Server.Port)
	EnvStr("S3_SLUG", &c.Server.InitialSlug)
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 9000
	}
	return c, nil
}
