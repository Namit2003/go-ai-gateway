package main

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port             string
	OpenAIKey        string
	OpenAIBaseURL    string
	MaxAttempts      int
	HeaderTimeout    time.Duration
	BreakerThreshold int
	BreakerCooldown  time.Duration
	FallbackBaseURL  string
	FallbackAPIKey   string
	FallbackModel    string
	RateLimit        int
	RateBurst        int
	MaxConcurrent    int
	APIKeysFile      string
	DailyQuota       int
	RoutesFile       string
	PprofAddr        string
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

	maxAttempts := 3
	if n, err := strconv.Atoi(os.Getenv("UPSTREAM_MAX_ATTEMPTS")); err == nil && n > 0 {
		maxAttempts = n
	}

	headerTimeout := 60 * time.Second
	if n, err := strconv.Atoi(os.Getenv("UPSTREAM_HEADER_TIMEOUT_SECONDS")); err == nil && n > 0 {
		headerTimeout = time.Duration(n) * time.Second
	}

	breakerThreshold := 20
	if n, err := strconv.Atoi(os.Getenv("UPSTREAM_BREAKER_THRESHOLD")); err == nil && n > 0 {
		breakerThreshold = n
	}

	breakerCooldown := 10 * time.Second
	if n, err := strconv.Atoi(os.Getenv("UPSTREAM_BREAKER_COOLDOWN_SECONDS")); err == nil && n > 0 {
		breakerCooldown = time.Duration(n) * time.Second
	}

	rateLimit := 10
	if n, err := strconv.Atoi(os.Getenv("RATE_LIMIT_PER_SECOND")); err == nil && n > 0 {
		rateLimit = n
	}

	rateBurst := 20
	if n, err := strconv.Atoi(os.Getenv("RATE_LIMIT_BURST")); err == nil && n > 0 {
		rateBurst = n
	}

	maxConcurrent := 100
	if n, err := strconv.Atoi(os.Getenv("MAX_CONCURRENT_REQUESTS")); err == nil && n > 0 {
		maxConcurrent = n
	}

	apiKeysFile := os.Getenv("API_KEYS_FILE")
	if apiKeysFile == "" {
		apiKeysFile = "keys.json"
	}

	dailyQuota := 1000
	if n, err := strconv.Atoi(os.Getenv("QUOTA_REQUESTS_PER_DAY")); err == nil && n > 0 {
		dailyQuota = n
	}

	routesFile := os.Getenv("ROUTES_FILE")
	if routesFile == "" {
		routesFile = "routes.json"
	}

	return Config{
		Port:             port,
		OpenAIKey:        os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL:    baseURL,
		MaxAttempts:      maxAttempts,
		HeaderTimeout:    headerTimeout,
		BreakerThreshold: breakerThreshold,
		BreakerCooldown:  breakerCooldown,
		FallbackBaseURL:  os.Getenv("FALLBACK_BASE_URL"),
		FallbackAPIKey:   os.Getenv("FALLBACK_API_KEY"),
		FallbackModel:    os.Getenv("FALLBACK_MODEL"),
		RateLimit:        rateLimit,
		RateBurst:        rateBurst,
		MaxConcurrent:    maxConcurrent,
		APIKeysFile:      apiKeysFile,
		DailyQuota:       dailyQuota,
		RoutesFile:       routesFile,
		PprofAddr:        os.Getenv("PPROF_ADDR"),
	}
}
