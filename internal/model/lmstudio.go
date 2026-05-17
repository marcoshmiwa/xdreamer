package model

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const doneSignal = "DONE"

// LMStudio implements Provider using LM Studio's OpenAI-compatible local API.
type LMStudio struct {
	client      *openai.Client
	model       string
	embedModel  string
	temperature float32
	maxRetries  int
	retryDelay  time.Duration
}

// NewLMStudio creates a Provider connected to a local LM Studio server.
// LM Studio does not require an API key; pass an empty string for baseURL's auth.
func NewLMStudio(baseURL, model, embedModel string, temperature float32, maxRetries, retryDelayMS int) *LMStudio {
	cfg := openai.DefaultConfig("")
	cfg.BaseURL = baseURL
	return &LMStudio{
		client:      openai.NewClientWithConfig(cfg),
		model:       model,
		embedModel:  embedModel,
		temperature: temperature,
		maxRetries:  maxRetries,
		retryDelay:  time.Duration(retryDelayMS) * time.Millisecond,
	}
}

// Chat sends a conversation to LM Studio and returns the model's response.
func (l *LMStudio) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (Response, error) {
	req := openai.ChatCompletionRequest{
		Model:       l.model,
		Messages:    toOpenAIMessages(messages),
		Tools:       toOpenAITools(tools),
		Temperature: l.temperature,
	}

	resp, err := retryChat(ctx, l.maxRetries, l.retryDelay, func() (openai.ChatCompletionResponse, error) {
		return l.client.CreateChatCompletion(ctx, req)
	})
	if err != nil {
		return Response{}, fmt.Errorf("chat completion: %w", err)
	}
	return fromOpenAIChatResponse(resp), nil
}

// Embed returns a vector embedding for the given text using the configured embed model.
func (l *LMStudio) Embed(ctx context.Context, text string) ([]float32, error) {
	req := openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(l.embedModel),
		Input: []string{text},
	}

	resp, err := retryEmbed(ctx, l.maxRetries, l.retryDelay, func() (openai.EmbeddingResponse, error) {
		return l.client.CreateEmbeddings(ctx, req)
	})
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("embed: empty response from server")
	}
	return resp.Data[0].Embedding, nil
}

// --- conversion helpers ---

func toOpenAIMessages(msgs []Message) []openai.ChatCompletionMessage {
	result := make([]openai.ChatCompletionMessage, len(msgs))
	for i, m := range msgs {
		result[i] = openai.ChatCompletionMessage{
			Role:       string(m.Role),
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
			ToolCalls:  toOpenAIToolCalls(m.ToolCalls),
		}
	}
	return result
}

func toOpenAIToolCalls(calls []ToolCall) []openai.ToolCall {
	if len(calls) == 0 {
		return nil
	}
	result := make([]openai.ToolCall, len(calls))
	for i, c := range calls {
		result[i] = openai.ToolCall{
			ID:   c.ID,
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      c.Name,
				Arguments: c.Arguments,
			},
		}
	}
	return result
}

func toOpenAITools(defs []ToolDefinition) []openai.Tool {
	if len(defs) == 0 {
		return nil
	}
	result := make([]openai.Tool, len(defs))
	for i, d := range defs {
		params := d.Parameters
		if params == nil {
			params = map[string]any{}
		}
		result[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        d.Name,
				Description: d.Description,
				Parameters: map[string]any{
					"type":       "object",
					"properties": params,
				},
			},
		}
	}
	return result
}

func fromOpenAIChatResponse(resp openai.ChatCompletionResponse) Response {
	if len(resp.Choices) == 0 {
		return Response{}
	}
	msg := resp.Choices[0].Message
	toolCalls := fromOpenAIToolCalls(msg.ToolCalls)
	return Response{
		Content:   msg.Content,
		ToolCalls: toolCalls,
		Done:      len(toolCalls) == 0 && isDone(msg.Content),
	}
}

func fromOpenAIToolCalls(calls []openai.ToolCall) []ToolCall {
	if len(calls) == 0 {
		return nil
	}
	result := make([]ToolCall, len(calls))
	for i, c := range calls {
		result[i] = ToolCall{
			ID:        c.ID,
			Name:      c.Function.Name,
			Arguments: c.Function.Arguments,
		}
	}
	return result
}

// isDone returns true if the last non-empty line of content is exactly "DONE".
func isDone(content string) bool {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line == doneSignal
		}
	}
	return false
}

// --- retry helpers ---

// shouldRetry reports whether an error from the OpenAI client warrants a retry.
// Only 429 (rate limited) and 5xx (server error) responses are retryable.
func shouldRetry(err error) bool {
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		return apiErr.HTTPStatusCode == http.StatusTooManyRequests ||
			apiErr.HTTPStatusCode >= http.StatusInternalServerError
	}
	return false
}

// sleepOrCancel waits for d or returns ctx.Err() if the context is cancelled first.
func sleepOrCancel(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

func retryChat(ctx context.Context, maxRetries int, delay time.Duration, fn func() (openai.ChatCompletionResponse, error)) (openai.ChatCompletionResponse, error) {
	var (
		resp openai.ChatCompletionResponse
		err  error
	)
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if ctxErr := sleepOrCancel(ctx, delay); ctxErr != nil {
				return resp, ctxErr
			}
		}
		resp, err = fn()
		if err == nil || !shouldRetry(err) {
			return resp, err
		}
	}
	return resp, err
}

func retryEmbed(ctx context.Context, maxRetries int, delay time.Duration, fn func() (openai.EmbeddingResponse, error)) (openai.EmbeddingResponse, error) {
	var (
		resp openai.EmbeddingResponse
		err  error
	)
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if ctxErr := sleepOrCancel(ctx, delay); ctxErr != nil {
				return resp, ctxErr
			}
		}
		resp, err = fn()
		if err == nil || !shouldRetry(err) {
			return resp, err
		}
	}
	return resp, err
}
