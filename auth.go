package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

type clientKey struct{}

type apiKeyEntry struct {
	Name string `json:"name"`
	Hash string `json:"hash"`
}

func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func readKeyEntries(path string) ([]apiKeyEntry, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []apiKeyEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return entries, nil
}

func LoadAPIKeys(path string) (map[string]string, error) {
	entries, err := readKeyEntries(path)
	if err != nil {
		return nil, err
	}
	keys := make(map[string]string, len(entries))
	for _, e := range entries {
		keys[e.Hash] = e.Name
	}
	return keys, nil
}

func newKey(path, name string) (string, error) {
	if name == "" {
		return "", errors.New("name is required")
	}
	entries, err := readKeyEntries(path)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.Name == name {
			return "", fmt.Errorf("a key named %q already exists", name)
		}
	}
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	key := "sk-gw-" + hex.EncodeToString(buf)
	entries = append(entries, apiKeyEntry{Name: name, Hash: hashKey(key)})
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return key, nil
}

func requireAPIKey(keys map[string]string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		name, found := keys[hashKey(token)]
		if !ok || !found {
			writeError(w, http.StatusUnauthorized, "invalid_request_error", "invalid API key")
			return
		}
		requestInfoFrom(r.Context()).Client = name
		ctx := context.WithValue(r.Context(), clientKey{}, name)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
