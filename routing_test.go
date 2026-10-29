package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func userMsg(s string) []Message {
	return []Message{{Role: "user", Content: s}}
}

func TestMatchModel(t *testing.T) {
	tests := []struct {
		pattern, model string
		want           bool
	}{
		{"gpt-4o", "gpt-4o", true},
		{"gpt-4o", "gpt-4o-mini", false},
		{"qwen*", "qwen-9b", true},
		{"qwen*", "qwen", true},
		{"qwen*", "my-qwen", false},
		{"meta/*", "meta/llama-3", true},
		{"*", "anything", true},
	}
	for _, tt := range tests {
		if got := matchModel(tt.pattern, tt.model); got != tt.want {
			t.Errorf("matchModel(%q, %q) = %v, want %v", tt.pattern, tt.model, got, tt.want)
		}
	}
}

func TestRuleMatchesWhen(t *testing.T) {
	tests := []struct {
		name string
		when When
		text string
		want bool
	}{
		{"no conditions", When{}, "anything", true},
		{"under max", When{MaxChars: 10}, "short", true},
		{"over max", When{MaxChars: 3}, "short", false},
		{"over min", When{MinChars: 3}, "short", true},
		{"under min", When{MinChars: 10}, "short", false},
		{"contains", When{Contains: "```"}, "see ```go\nx```", true},
		{"does not contain", When{Contains: "```"}, "plain text", false},
		{"counts characters not bytes", When{MaxChars: 3}, "héé", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := Rule{Model: "auto", When: tt.when}
			got := rule.matches(ChatCompletionRequest{Model: "auto", Messages: userMsg(tt.text)})
			if got != tt.want {
				t.Errorf("matches = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRouterPick(t *testing.T) {
	def := &recordingProvider{}
	local := &recordingProvider{}
	r := &Router{Default: def, routes: []route{
		{Rule{Model: "qwen*", Provider: "local"}, local},
		{Rule{Model: "auto", When: When{Contains: "```"}, Provider: "local", UseModel: "qwen-coder"}, local},
		{Rule{Model: "auto", When: When{MaxChars: 20}, Provider: "local", UseModel: "qwen-small"}, local},
		{Rule{Model: "auto", Provider: "openai", UseModel: "big"}, def},
	}}

	tests := []struct {
		name      string
		model     string
		text      string
		wantLocal bool
		wantModel string
	}{
		{"unmatched model goes to default, unchanged", "gpt-4o", "hi", false, "gpt-4o"},
		{"model rule", "qwen-9b", "hi", true, "qwen-9b"},
		{"auto with code", "auto", "fix ```x``` please, this is a longer prompt", true, "qwen-coder"},
		{"auto short", "auto", "hello", true, "qwen-small"},
		{"auto long falls through to catch-all", "auto", strings.Repeat("a", 100), false, "big"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			*def, *local = recordingProvider{}, recordingProvider{}
			_, err := r.ChatCompletion(context.Background(), ChatCompletionRequest{Model: tt.model, Messages: userMsg(tt.text)})
			if err != nil {
				t.Fatal(err)
			}
			used, other := def, local
			if tt.wantLocal {
				used, other = local, def
			}
			if !used.called || other.called {
				t.Errorf("used.called = %v, other.called = %v, want true, false", used.called, other.called)
			}
			if used.gotModel != tt.wantModel {
				t.Errorf("model sent upstream = %q, want %q", used.gotModel, tt.wantModel)
			}
		})
	}
}

func TestRouterPickStream(t *testing.T) {
	local := &recordingProvider{}
	r := &Router{Default: &recordingProvider{}, routes: []route{
		{Rule{Model: "auto", Provider: "local", UseModel: "qwen"}, local},
	}}
	if _, err := r.ChatCompletionStream(context.Background(), ChatCompletionRequest{Model: "auto", Messages: userMsg("hi")}); err != nil {
		t.Fatal(err)
	}
	if local.gotModel != "qwen" {
		t.Errorf("model = %q, want qwen", local.gotModel)
	}
}

func TestRouterAutoWithoutRoute(t *testing.T) {
	def := &recordingProvider{}
	r := &Router{Default: def}

	_, err := r.ChatCompletion(context.Background(), ChatCompletionRequest{Model: "auto", Messages: userMsg("hi")})
	if !errors.Is(err, ErrNoRoute) {
		t.Errorf("err = %v, want ErrNoRoute", err)
	}
	if def.called {
		t.Error("auto must not be sent upstream as a literal model name")
	}
}

func TestNoRouteDoesNotTriggerFallback(t *testing.T) {
	fallback := &recordingProvider{}
	p := FallbackProvider{Primary: &Router{Default: &recordingProvider{}}, Fallback: fallback}

	_, err := p.ChatCompletion(context.Background(), ChatCompletionRequest{Model: "auto", Messages: userMsg("hi")})
	if !errors.Is(err, ErrNoRoute) || fallback.called {
		t.Errorf("err = %v, fallback.called = %v, want ErrNoRoute and no fallback", err, fallback.called)
	}
}

func TestLoadRouter(t *testing.T) {
	dir := t.TempDir()
	def := &recordingProvider{}
	write := func(content string) string {
		path := filepath.Join(dir, "routes.json")
		os.WriteFile(path, []byte(content), 0o600)
		return path
	}

	t.Run("missing file means no rules", func(t *testing.T) {
		r, err := LoadRouter(filepath.Join(dir, "nope.json"), def, Config{})
		if err != nil || len(r.routes) != 0 || r.Default != Provider(def) {
			t.Errorf("router = %+v, err = %v", r, err)
		}
	})

	t.Run("no default in file keeps the env provider", func(t *testing.T) {
		r, err := LoadRouter(write(`{"providers": {"local": {"base_url": "http://x"}}, "routes": []}`), def, Config{})
		if err != nil || r.Default != Provider(def) {
			t.Errorf("Default = %#v, err = %v, want the env provider", r.Default, err)
		}
	})

	t.Run("valid file", func(t *testing.T) {
		path := write(`{
			"default": "openai",
			"providers": {
				"openai": {"base_url": "https://api.openai.com/v1"},
				"local": {"base_url": "http://localhost:8000/v1"}
			},
			"routes": [
				{"model": "qwen*", "provider": "local"},
				{"model": "auto", "provider": "openai", "use_model": "gpt-4o-mini"}
			]}`)
		r, err := LoadRouter(path, def, Config{BreakerThreshold: 1})
		if err != nil {
			t.Fatal(err)
		}
		if len(r.routes) != 2 {
			t.Fatalf("routes = %d, want 2", len(r.routes))
		}
		if o, ok := r.routes[0].provider.(OpenAI); !ok || o.BaseURL != "http://localhost:8000/v1" {
			t.Errorf("route 1 provider = %#v, want the local OpenAI provider", r.routes[0].provider)
		}
		if o, ok := r.Default.(OpenAI); !ok || o.BaseURL != "https://api.openai.com/v1" {
			t.Errorf("Default = %#v, want the openai provider from the file", r.Default)
		}
	})

	bad := map[string]string{
		"invalid JSON":           `nope`,
		"unknown provider":       `{"routes": [{"model": "x", "provider": "ghost"}]}`,
		"missing model":          `{"routes": [{"provider": "openai"}]}`,
		"auto without use_model": `{"routes": [{"model": "auto", "provider": "openai"}]}`,
		"provider without url":   `{"providers": {"local": {}}, "routes": []}`,
		"unknown default":        `{"default": "ghost", "routes": []}`,
	}
	for name, content := range bad {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadRouter(write(content), def, Config{}); err == nil {
				t.Error("want an error")
			}
		})
	}
}

func BenchmarkRouterPick(b *testing.B) {
	r, err := LoadRouter("routes.example.json", &recordingProvider{}, Config{})
	if err != nil {
		b.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		text string
	}{
		{"short prompt", "hi"},
		{"long prompt", strings.Repeat("a", 20000)},
	} {
		b.Run(tt.name, func(b *testing.B) {
			req := ChatCompletionRequest{Model: "auto", Messages: userMsg(tt.text)}
			for b.Loop() {
				r.pick(context.Background(), req)
			}
		})
	}
}
