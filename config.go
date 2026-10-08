package main

import "os"

type Config struct {
	Port          string
	OpenAIKey     string
	OpenAIBaseURL string
}

func LoadConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return Config{
		Port:          port,
		OpenAIKey:     os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL: baseURL,
	}
}
