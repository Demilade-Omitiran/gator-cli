package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	DbUrl           string `json:"db_url"`
	CurrentUserName string `json:"current_user_name"`
}

const configFileName = ".gatorconfig.json"

func getConfigFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()

	if err != nil {
		return "", err
	}

	return homeDir + "/.gatorconfig.json", nil
}

func Read() (Config, error) {
	configFilePath, err := getConfigFilePath()

	if err != nil {
		return Config{}, err
	}

	var config Config

	data, err := os.ReadFile(configFilePath)

	if err != nil {
		return Config{}, err
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, err
	}

	return config, nil
}

func write(config Config) error {
	configFilePath, err := getConfigFilePath()

	if err != nil {
		return err
	}

	jsonConfig, err := json.Marshal(config)

	if err != nil {
		return err
	}

	err = os.WriteFile(configFilePath, jsonConfig, 0666)

	if err != nil {
		return err
	}

	return nil
}

func (c *Config) SetUser(name string) {
	c.CurrentUserName = name
	write(*c)
}
