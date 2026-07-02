package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	ListenPort int               `json:"listenPort"`
	PrivateKey string            `json:"privateKey"`
	AllowedIPs map[string]string `json:"allowedIPs"`
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
