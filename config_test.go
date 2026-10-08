package main

import "testing"

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")

	cfg := LoadConfig()

	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want %q", cfg.Port, "8080")
	}
	if cfg.OpenAIKey != "" {
		t.Errorf("OpenAIKey = %q, want empty", cfg.OpenAIKey)
	}
	if cfg.OpenAIBaseURL != "https://api.openai.com/v1" {
		t.Errorf("OpenAIBaseURL = %q, want %q", cfg.OpenAIBaseURL, "https://api.openai.com/v1")
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("PORT", "9000")
	t.Setenv("OPENAI_API_KEY", "secret")
	t.Setenv("OPENAI_BASE_URL", "http://localhost:8000/v1")

	cfg := LoadConfig()

	if cfg.Port != "9000" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9000")
	}
	if cfg.OpenAIKey != "secret" {
		t.Errorf("OpenAIKey = %q, want %q", cfg.OpenAIKey, "secret")
	}
	if cfg.OpenAIBaseURL != "http://localhost:8000/v1" {
		t.Errorf("OpenAIBaseURL = %q, want %q", cfg.OpenAIBaseURL, "http://localhost:8000/v1")
	}
}
