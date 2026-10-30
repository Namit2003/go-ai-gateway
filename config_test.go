package main

import (
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("UPSTREAM_MAX_ATTEMPTS", "")
	t.Setenv("UPSTREAM_HEADER_TIMEOUT_SECONDS", "")
	t.Setenv("UPSTREAM_BREAKER_THRESHOLD", "")
	t.Setenv("UPSTREAM_BREAKER_COOLDOWN_SECONDS", "")
	t.Setenv("FALLBACK_BASE_URL", "")
	t.Setenv("FALLBACK_API_KEY", "")
	t.Setenv("FALLBACK_MODEL", "")
	t.Setenv("RATE_LIMIT_PER_SECOND", "")
	t.Setenv("RATE_LIMIT_BURST", "")
	t.Setenv("MAX_CONCURRENT_REQUESTS", "")
	t.Setenv("API_KEYS_FILE", "")
	t.Setenv("QUOTA_TOKENS_PER_DAY", "")
	t.Setenv("ROUTES_FILE", "")
	t.Setenv("PPROF_ADDR", "")

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
	if cfg.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", cfg.MaxAttempts)
	}
	if cfg.HeaderTimeout != 60*time.Second {
		t.Errorf("HeaderTimeout = %v, want 60s", cfg.HeaderTimeout)
	}
	if cfg.BreakerThreshold != 20 {
		t.Errorf("BreakerThreshold = %d, want 20", cfg.BreakerThreshold)
	}
	if cfg.BreakerCooldown != 10*time.Second {
		t.Errorf("BreakerCooldown = %v, want 10s", cfg.BreakerCooldown)
	}
	if cfg.FallbackBaseURL != "" || cfg.FallbackAPIKey != "" || cfg.FallbackModel != "" {
		t.Errorf("fallback = %q %q %q, want all empty", cfg.FallbackBaseURL, cfg.FallbackAPIKey, cfg.FallbackModel)
	}
	if cfg.RateLimit != 10 {
		t.Errorf("RateLimit = %d, want 10", cfg.RateLimit)
	}
	if cfg.RateBurst != 20 {
		t.Errorf("RateBurst = %d, want 20", cfg.RateBurst)
	}
	if cfg.MaxConcurrent != 100 {
		t.Errorf("MaxConcurrent = %d, want 100", cfg.MaxConcurrent)
	}
	if cfg.APIKeysFile != "keys.json" {
		t.Errorf("APIKeysFile = %q, want keys.json", cfg.APIKeysFile)
	}
	if cfg.DailyQuota != 100000 {
		t.Errorf("DailyQuota = %d, want 100000", cfg.DailyQuota)
	}
	if cfg.RoutesFile != "routes.json" {
		t.Errorf("RoutesFile = %q, want routes.json", cfg.RoutesFile)
	}
	if cfg.PprofAddr != "" {
		t.Errorf("PprofAddr = %q, want empty (pprof off)", cfg.PprofAddr)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("PORT", "9000")
	t.Setenv("OPENAI_API_KEY", "secret")
	t.Setenv("OPENAI_BASE_URL", "http://localhost:8000/v1")
	t.Setenv("UPSTREAM_MAX_ATTEMPTS", "5")
	t.Setenv("UPSTREAM_HEADER_TIMEOUT_SECONDS", "120")
	t.Setenv("UPSTREAM_BREAKER_THRESHOLD", "10")
	t.Setenv("UPSTREAM_BREAKER_COOLDOWN_SECONDS", "45")
	t.Setenv("FALLBACK_BASE_URL", "http://localhost:8000/v1")
	t.Setenv("FALLBACK_API_KEY", "fallback-secret")
	t.Setenv("FALLBACK_MODEL", "qwen")
	t.Setenv("RATE_LIMIT_PER_SECOND", "50")
	t.Setenv("RATE_LIMIT_BURST", "100")
	t.Setenv("MAX_CONCURRENT_REQUESTS", "7")
	t.Setenv("API_KEYS_FILE", "/etc/gw/keys.json")
	t.Setenv("QUOTA_TOKENS_PER_DAY", "250")
	t.Setenv("ROUTES_FILE", "/etc/gw/routes.json")
	t.Setenv("PPROF_ADDR", "localhost:6060")

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
	if cfg.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", cfg.MaxAttempts)
	}
	if cfg.HeaderTimeout != 120*time.Second {
		t.Errorf("HeaderTimeout = %v, want 120s", cfg.HeaderTimeout)
	}
	if cfg.BreakerThreshold != 10 {
		t.Errorf("BreakerThreshold = %d, want 10", cfg.BreakerThreshold)
	}
	if cfg.BreakerCooldown != 45*time.Second {
		t.Errorf("BreakerCooldown = %v, want 45s", cfg.BreakerCooldown)
	}
	if cfg.FallbackBaseURL != "http://localhost:8000/v1" {
		t.Errorf("FallbackBaseURL = %q, want %q", cfg.FallbackBaseURL, "http://localhost:8000/v1")
	}
	if cfg.FallbackAPIKey != "fallback-secret" {
		t.Errorf("FallbackAPIKey = %q, want %q", cfg.FallbackAPIKey, "fallback-secret")
	}
	if cfg.FallbackModel != "qwen" {
		t.Errorf("FallbackModel = %q, want %q", cfg.FallbackModel, "qwen")
	}
	if cfg.RateLimit != 50 {
		t.Errorf("RateLimit = %d, want 50", cfg.RateLimit)
	}
	if cfg.RateBurst != 100 {
		t.Errorf("RateBurst = %d, want 100", cfg.RateBurst)
	}
	if cfg.MaxConcurrent != 7 {
		t.Errorf("MaxConcurrent = %d, want 7", cfg.MaxConcurrent)
	}
	if cfg.APIKeysFile != "/etc/gw/keys.json" {
		t.Errorf("APIKeysFile = %q, want /etc/gw/keys.json", cfg.APIKeysFile)
	}
	if cfg.DailyQuota != 250 {
		t.Errorf("DailyQuota = %d, want 250", cfg.DailyQuota)
	}
	if cfg.RoutesFile != "/etc/gw/routes.json" {
		t.Errorf("RoutesFile = %q, want /etc/gw/routes.json", cfg.RoutesFile)
	}
	if cfg.PprofAddr != "localhost:6060" {
		t.Errorf("PprofAddr = %q, want localhost:6060", cfg.PprofAddr)
	}
}

func TestLoadConfigInvalidValuesUseDefaults(t *testing.T) {
	t.Setenv("UPSTREAM_MAX_ATTEMPTS", "abc")
	t.Setenv("UPSTREAM_HEADER_TIMEOUT_SECONDS", "-5")
	t.Setenv("UPSTREAM_BREAKER_THRESHOLD", "0")
	t.Setenv("UPSTREAM_BREAKER_COOLDOWN_SECONDS", "abc")
	t.Setenv("RATE_LIMIT_PER_SECOND", "0")
	t.Setenv("RATE_LIMIT_BURST", "-1")
	t.Setenv("MAX_CONCURRENT_REQUESTS", "abc")
	t.Setenv("QUOTA_TOKENS_PER_DAY", "0")

	cfg := LoadConfig()

	if cfg.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", cfg.MaxAttempts)
	}
	if cfg.HeaderTimeout != 60*time.Second {
		t.Errorf("HeaderTimeout = %v, want 60s", cfg.HeaderTimeout)
	}
	if cfg.BreakerThreshold != 20 {
		t.Errorf("BreakerThreshold = %d, want 20", cfg.BreakerThreshold)
	}
	if cfg.BreakerCooldown != 10*time.Second {
		t.Errorf("BreakerCooldown = %v, want 10s", cfg.BreakerCooldown)
	}
	if cfg.RateLimit != 10 {
		t.Errorf("RateLimit = %d, want 10", cfg.RateLimit)
	}
	if cfg.RateBurst != 20 {
		t.Errorf("RateBurst = %d, want 20", cfg.RateBurst)
	}
	if cfg.MaxConcurrent != 100 {
		t.Errorf("MaxConcurrent = %d, want 100", cfg.MaxConcurrent)
	}
	if cfg.DailyQuota != 100000 {
		t.Errorf("DailyQuota = %d, want 100000", cfg.DailyQuota)
	}
}
