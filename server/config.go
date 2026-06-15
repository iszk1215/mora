package server

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type ServerConfig struct {
	URL  string
	Port int
}

type RepositoryManagerConfig struct {
	Driver         string `toml:"scm"`
	URL            string
	SecretFilename string `toml:"secret_file"`
}

type MoraConfig struct {
	Server             ServerConfig
	RepositoryManagers []RepositoryManagerConfig `toml:"scm"`
	Debug              bool
	DatabaseFilename   string
}

func ReadMoraConfig(filename string) (MoraConfig, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return MoraConfig{}, fmt.Errorf("ReadMoraConfig(%s): %w", filename, err)
	}

	var config MoraConfig
	if err := toml.Unmarshal(b, &config); err != nil {
		return MoraConfig{}, fmt.Errorf("ReadMoraConfig Unmarshal: %w", err)
	}

	config.DatabaseFilename = "mora.db"

	return config, nil
}
