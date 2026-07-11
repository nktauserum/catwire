package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	PrivateKey string `json:"private_key"`
	ServerAddr string `json:"server_addr"`
	PeerAddr   string `json:"peer_addr"`
	ForwardAll bool   `json:"forward_all"`
}

func LoadConfig(path string) (*Config, error) {
	configFile, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := new(Config)
	err = json.Unmarshal(configFile, config)

	return config, err
}
