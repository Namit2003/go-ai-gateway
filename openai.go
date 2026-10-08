package main

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"`
}

type Choice struct {
	Index   int     `json:"index"`
	Message Message `json:"message"`
}

type ChatCompletionResponse struct {
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

type ChunkChoice struct {
	Index int     `json:"index"`
	Delta Message `json:"delta"`
}

type ChatCompletionChunk struct {
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
}
