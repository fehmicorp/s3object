package types

type Config struct {
	App    App    `yaml:"app"`
	Server Server `yaml:"server"`
}

type App struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Environment string `yaml:"environment"`
}

type Server struct {
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	InitialSlug string `yaml:"slug"`
}
