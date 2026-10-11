package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"
	"unicode/utf8"
)

const autoModel = "auto"

var ErrNoRoute = errors.New("no route matches this request")

type When struct {
	MinChars int    `json:"min_chars"`
	MaxChars int    `json:"max_chars"`
	Contains string `json:"contains"`
}

type Rule struct {
	Model    string `json:"model"`
	When     When   `json:"when"`
	Provider string `json:"provider"`
	UseModel string `json:"use_model"`
}

type providerConfig struct {
	BaseURL   string `json:"base_url"`
	APIKeyEnv string `json:"api_key_env"`
}

type routesFile struct {
	Providers map[string]providerConfig `json:"providers"`
	Routes    []Rule                    `json:"routes"`
}

type route struct {
	rule     Rule
	provider Provider
}

type Router struct {
	Default Provider
	routes  []route
}

func matchModel(pattern, model string) bool {
	if prefix, ok := strings.CutSuffix(pattern, "*"); ok {
		return strings.HasPrefix(model, prefix)
	}
	return pattern == model
}

func (r Rule) matches(req ChatCompletionRequest) bool {
	if !matchModel(r.Model, req.Model) {
		return false
	}
	chars := 0
	hasText := false
	for _, m := range req.Messages {
		chars += utf8.RuneCountInString(m.Content)
		if r.When.Contains != "" && strings.Contains(m.Content, r.When.Contains) {
			hasText = true
		}
	}
	if r.When.MinChars > 0 && chars < r.When.MinChars {
		return false
	}
	if r.When.MaxChars > 0 && chars > r.When.MaxChars {
		return false
	}
	return r.When.Contains == "" || hasText
}

func (r *Router) pick(req ChatCompletionRequest) (Provider, ChatCompletionRequest, error) {
	for _, rt := range r.routes {
		if !rt.rule.matches(req) {
			continue
		}
		if rt.rule.UseModel != "" {
			req.Model = rt.rule.UseModel
		}
		if rt.rule.Model == autoModel {
			log.Printf("auto routed to provider %q, model %q", rt.rule.Provider, req.Model)
		}
		return rt.provider, req, nil
	}
	if req.Model == autoModel {
		return nil, req, ErrNoRoute
	}
	return r.Default, req, nil
}

func (r *Router) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	p, req, err := r.pick(req)
	if err != nil {
		return ChatCompletionResponse{}, err
	}
	return p.ChatCompletion(ctx, req)
}

func (r *Router) ChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (Stream, error) {
	p, req, err := r.pick(req)
	if err != nil {
		return nil, err
	}
	return p.ChatCompletionStream(ctx, req)
}

func LoadRouter(path string, def Provider, cfg Config) (*Router, error) {
	router := &Router{Default: def}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return router, nil
	}
	if err != nil {
		return nil, err
	}
	var file routesFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	providers := map[string]Provider{"default": def}
	for name, pc := range file.Providers {
		if name == "default" {
			return nil, fmt.Errorf("%s: provider name %q is reserved", path, name)
		}
		if pc.BaseURL == "" {
			return nil, fmt.Errorf("%s: provider %q needs a base_url", path, name)
		}
		providers[name] = OpenAI{
			APIKey:  os.Getenv(pc.APIKeyEnv),
			BaseURL: pc.BaseURL,
			Breaker: NewCircuitBreaker(cfg.BreakerThreshold, cfg.BreakerCooldown),
		}
	}

	for i, rule := range file.Routes {
		p, ok := providers[rule.Provider]
		if !ok {
			return nil, fmt.Errorf("%s: route %d uses unknown provider %q", path, i+1, rule.Provider)
		}
		if rule.Model == "" {
			return nil, fmt.Errorf("%s: route %d needs a model", path, i+1)
		}
		if rule.Model == autoModel && rule.UseModel == "" {
			return nil, fmt.Errorf("%s: route %d matches %q so it needs a use_model", path, i+1, autoModel)
		}
		router.routes = append(router.routes, route{rule: rule, provider: p})
	}
	return router, nil
}
