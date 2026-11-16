package config

import "time"

func defaultConfig() *Config {
	c := &Config{}
	c.App.Name = "fehmi-s3"
	c.App.Version = "0.1.0"
	c.App.Environment = "production"
	c.Server.Host = "0.0.0.0"
	c.Server.Port = 9000
	c.Server.InitialSlug = "/"
	c.Server.ReadTimeout = 300 * time.Second
	c.Server.WriteTimeout = 300 * time.Second
	c.Server.IdleTimeout = 120 * time.Second
	c.Server.MaxHeaderBytes = 1 << 20 // 1 MB
	c.Storage.Driver = "filesystem"
	c.Storage.Path = "/data"
	c.Storage.MaxUploadSize = 10 << 30 // 10 GB
	c.Storage.ChunkSize = 8 << 20      // 8 MB
	return c
}
