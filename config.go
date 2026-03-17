package main

import "os"

type Config struct {
	Port      string
	OllamaURL string
	AdminKey  string
}

func LoadConfig() Config {
	cfg := Config{
		Port:      os.Getenv("PORT"),
		OllamaURL: os.Getenv("OLLAMA_URL"),
		AdminKey:  os.Getenv("ADMIN_KEY"),
	}
	if cfg.Port == "" {
		cfg.Port = "8090"
	}
	if cfg.OllamaURL == "" {
		cfg.OllamaURL = "http://192.168.70.130:8080"
	}
	if cfg.AdminKey == "" {
		cfg.AdminKey = "changeme"
	}
	return cfg
}
