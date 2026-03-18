package main

import "os"

type Config struct {
	Port          string
	OllamaURL     string
	AdminKey      string
	DiscordWebhook string
}

func LoadConfig() Config {
	cfg := Config{
		Port:          os.Getenv("PORT"),
		OllamaURL:     os.Getenv("OLLAMA_URL"),
		AdminKey:      os.Getenv("ADMIN_KEY"),
		DiscordWebhook: os.Getenv("DISCORD_WEBHOOK"),
	}
	if cfg.Port == "" {
		cfg.Port = "8090"
	}
	if cfg.OllamaURL == "" {
		cfg.OllamaURL = "http://192.168.70.223:8080"
	}
	if cfg.AdminKey == "" {
		cfg.AdminKey = "changeme"
	}
	return cfg
}
