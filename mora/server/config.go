package server

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

type ServerConfig struct {
	URL  string
	Port int
}

type SCMConfig struct {
	Driver         string `toml:"scm"`
	Name           string
	URL            string
	SecretFilename string `toml:"secret_file"`
}

type MoraConfig struct {
	Server ServerConfig
	SCMs   []SCMConfig `toml:"scm"`
	Debug  bool
}

func ReadMoraConfig(filename string) (MoraConfig, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return MoraConfig{}, err
	}

	var config MoraConfig
	if err := toml.Unmarshal(b, &config); err != nil {
		return MoraConfig{}, err
	}

	return config, nil
}
