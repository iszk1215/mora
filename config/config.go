package config

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type ClientConfig struct {
	ServerURL     string `toml:"server"`
	RepositoryURL string `toml:"repo"`
	Token         string `toml:"token"`
}

type ServerConfig struct {
	URL  string
	Port int
}

type RepositoryManagerConfig struct {
	Driver             string `toml:"scm"`
	URL                string
	SecretFilename     string `toml:"secret_file"`
	InsecureSkipVerify bool   `toml:"insecure_skip_verify"`
}

type MoraConfig struct {
	Server             ServerConfig
	RepositoryManagers []RepositoryManagerConfig `toml:"scm"`
	Client             ClientConfig
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

	if config.DatabaseFilename == "" {
		config.DatabaseFilename = "mora.db"
	}

	return config, nil
}

func ReadClientConfig(filename string) (ClientConfig, error) {
	config, err := ReadMoraConfig(filename)
	if err != nil {
		return ClientConfig{}, fmt.Errorf("ReadClientConfig: %w", err)
	}

	return config.Client, nil
}
