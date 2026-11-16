package config

import "time"

type Config struct {
	App     App     `yaml:"app"`
	Server  Server  `yaml:"server"`
	Storage Storage `yaml:"storage"`
}

type App struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Environment string `yaml:"environment"`
}

type Server struct {
	Host           string        `yaml:"host"`
	Port           int           `yaml:"port"`
	InitialSlug    string        `yaml:"slug"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
	IdleTimeout    time.Duration `yaml:"idle_timeout"`
	MaxHeaderBytes int           `yaml:"max_header_bytes"`
}

type Storage struct {
	Driver        string `yaml:"driver"`
	Path          string `yaml:"path"`
	MaxUploadSize int64  `yaml:"max_upload_size"`
	ChunkSize     int64  `yaml:"chunk_size"`
}
