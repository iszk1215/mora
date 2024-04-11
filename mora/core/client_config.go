package core

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

type ClientConfig struct {
	ServerURL     string `toml:"server"`
	RepositoryURL string `toml:"repo"`
	Token         string `toml:"token"`
}

func ReadClientConfig(filename string) (ClientConfig, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return ClientConfig{}, err
	}

	var config ClientConfig
	if err := toml.Unmarshal(b, &config); err != nil {
		return ClientConfig{}, err
	}

	return config, nil
}
