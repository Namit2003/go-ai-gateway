package main

import (
	"context"
	"io"
)

type Provider interface {
	ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error)
	ChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (io.ReadCloser, error)
}
