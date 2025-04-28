package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const fileName = "config.json"

type Config struct {
	FlacExePath string `json:"flacExePath"`
}

func LoadConfig() (Config, error) {
	exeFilePath, err := os.Executable()
	if err != nil {
		return Config{}, err
	}
	binary, err := os.ReadFile(filepath.Join(filepath.Dir(exeFilePath), fileName))
	if err != nil {
		return Config{}, err
	}

	config := Config{}
	if err := json.Unmarshal([]byte(binary), &config); err != nil {
		return Config{}, err
	}

	return config, nil
}
