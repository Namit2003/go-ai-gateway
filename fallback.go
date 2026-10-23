package main

import (
	"context"
	"log"
)

type FallbackProvider struct {
	Primary       Provider
	Fallback      Provider
	FallbackModel string
}

func (f FallbackProvider) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	resp, err := f.Primary.ChatCompletion(ctx, req)
	if !upstreamFailed(ctx, err) {
		return resp, err
	}
	log.Printf("primary failed, using fallback: %v", err)
	if f.FallbackModel != "" {
		req.Model = f.FallbackModel
	}
	info := requestInfoFrom(ctx)
	info.Provider, info.UpstreamModel = "fallback", req.Model
	return f.Fallback.ChatCompletion(ctx, req)
}

func (f FallbackProvider) ChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (Stream, error) {
	stream, err := f.Primary.ChatCompletionStream(ctx, req)
	if !upstreamFailed(ctx, err) {
		return stream, err
	}
	log.Printf("primary failed, using fallback: %v", err)
	if f.FallbackModel != "" {
		req.Model = f.FallbackModel
	}
	info := requestInfoFrom(ctx)
	info.Provider, info.UpstreamModel = "fallback", req.Model
	return f.Fallback.ChatCompletionStream(ctx, req)
}
