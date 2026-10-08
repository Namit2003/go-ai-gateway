package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type UpstreamError struct {
	StatusCode int
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("openai: unexpected status %d", e.StatusCode)
}

type ErrorBody struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

func writeError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: ErrorBody{Message: message, Type: errType}})
}
