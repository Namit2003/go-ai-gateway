package main

import "context"

type Stream interface {
	Recv() (ChatCompletionChunk, error)
	Close() error
}

type Provider interface {
	ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error)
	ChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (Stream, error)
}
