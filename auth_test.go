package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequireAPIKey(t *testing.T) {
	keys := map[string]string{hashKey("sk-good"): "alice"}
	var gotClient any
	h := requireAPIKey(keys, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClient = r.Context().Value(clientKey{})
	}))

	tests := []struct {
		name   string
		header string
		want   int
	}{
		{"valid key", "Bearer sk-good", http.StatusOK},
		{"unknown key", "Bearer sk-bad", http.StatusUnauthorized},
		{"missing header", "", http.StatusUnauthorized},
		{"wrong scheme", "Basic sk-good", http.StatusUnauthorized},
		{"empty token", "Bearer ", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotClient = nil
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
			if tt.want == http.StatusOK && gotClient != "alice" {
				t.Errorf("client in context = %v, want alice", gotClient)
			}
			if tt.want != http.StatusOK && gotClient != nil {
				t.Error("handler ran for a rejected request")
			}
		})
	}
}

func TestRequireAPIKeyNoKeysRejectsEverything(t *testing.T) {
	h := requireAPIKey(map[string]string{}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler ran with no keys configured")
	}))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer ")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLoadAPIKeys(t *testing.T) {
	dir := t.TempDir()

	keys, err := LoadAPIKeys(filepath.Join(dir, "missing.json"))
	if err != nil || len(keys) != 0 {
		t.Errorf("missing file: keys = %v, err = %v, want empty and no error", keys, err)
	}

	path := filepath.Join(dir, "keys.json")
	os.WriteFile(path, []byte(`[{"name":"alice","hash":"abc"},{"name":"bob","hash":"def"}]`), 0o600)
	keys, err = LoadAPIKeys(path)
	if err != nil {
		t.Fatal(err)
	}
	if keys["abc"] != "alice" || keys["def"] != "bob" || len(keys) != 2 {
		t.Errorf("keys = %v", keys)
	}

	os.WriteFile(path, []byte(`not json`), 0o600)
	if _, err := LoadAPIKeys(path); err == nil {
		t.Error("invalid JSON: want an error")
	}
}

func TestNewKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.json")

	key, err := newKey(path, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key, "sk-gw-") {
		t.Errorf("key = %q, want sk-gw- prefix", key)
	}
	if _, err := newKey(path, "bob"); err != nil {
		t.Fatal(err)
	}

	keys, err := LoadAPIKeys(path)
	if err != nil {
		t.Fatal(err)
	}
	if keys[hashKey(key)] != "alice" || len(keys) != 2 {
		t.Errorf("keys = %v, want alice's hash and bob's hash", keys)
	}

	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), key) {
		t.Error("the file contains the plaintext key")
	}

	if _, err := newKey(path, "alice"); err == nil {
		t.Error("duplicate name: want an error")
	}
	if _, err := newKey(path, ""); err == nil {
		t.Error("empty name: want an error")
	}
}

func BenchmarkRequireAPIKey(b *testing.B) {
	keys := map[string]string{hashKey("sk-good"): "alice"}
	h := requireAPIKey(keys, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer sk-good")
	rec := httptest.NewRecorder()
	for b.Loop() {
		h.ServeHTTP(rec, req)
	}
}
