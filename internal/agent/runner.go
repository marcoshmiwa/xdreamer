package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/xdreamer/xdreamer/internal/memory"
	"github.com/xdreamer/xdreamer/internal/model"
	"github.com/xdreamer/xdreamer/internal/rag"
	"github.com/xdreamer/xdreamer/internal/tools"
)

// ErrMaxStepsExceeded is returned when the agent loop exceeds the configured step limit.
var ErrMaxStepsExceeded = errors.New("max steps exceeded")

// Retriever is the narrow interface the Runner needs from the RAG engine.
// *rag.Engine satisfies this automatically.
type Retriever interface {
	Retrieve(ctx context.Context, query string) ([]rag.Chunk, error)
}

// RunnerConfig holds runtime settings for a single agent run.
type RunnerConfig struct {
	MaxSteps     int
	DryRun       bool
	WorktreePath string
}

// Runner orchestrates the step loop: prompt → tool call → result → repeat.
// All dependencies are interfaces; concrete types are wired in the CLI layer.
type Runner struct {
	cfg        RunnerConfig
	provider   model.Provider
	registry   *tools.Registry
	retriever  Retriever
	memory     *memory.Memory
	confirmer  Confirmer
	transcript *Transcript
}

// NewRunner constructs a Runner. ragEngine may be nil if RAG is disabled.
func NewRunner(
	cfg RunnerConfig,
	provider model.Provider,
	registry *tools.Registry,
	retriever Retriever,
	mem *memory.Memory,
	confirmer Confirmer,
) *Runner {
	return &Runner{
		cfg:       cfg,
		provider:  provider,
		registry:  registry,
		retriever: retriever,
		memory:    mem,
		confirmer: confirmer,
	}
}

// Run executes the agent loop for the given task and returns the full transcript.
func (r *Runner) Run(ctx context.Context, task string) (*Transcript, error) {
	r.transcript = &Transcript{Task: task}

	systemPrompt, err := renderSystemPrompt(r.cfg.WorktreePath, r.memory.Content())
	if err != nil {
		return nil, fmt.Errorf("render prompt: %w", err)
	}

	history := []model.Message{
		{Role: model.RoleSystem, Content: systemPrompt},
		{Role: model.RoleUser, Content: task},
	}

	stepCount := 0
	for {
		stepCount++
		if stepCount > r.cfg.MaxSteps {
			return nil, ErrMaxStepsExceeded
		}

		ragChunks := r.retrieveContext(ctx, history)
		promptHistory := appendRAGContext(history, ragChunks)

		response, err := r.provider.Chat(ctx, promptHistory, r.registry.Definitions())
		if err != nil {
			return nil, fmt.Errorf("step %d: chat: %w", stepCount, err)
		}

		history = append(history, model.Message{
			Role:      model.RoleAssistant,
			Content:   response.Content,
			ToolCalls: response.ToolCalls,
		})

		if response.Done {
			break
		}

		if len(response.ToolCalls) == 0 {
			history = append(history, model.Message{Role: model.RoleUser, Content: "continue"})
			continue
		}

		step := Step{
			Number:      stepCount,
			UserMessage: lastUserMessage(history),
			LLMResponse: response.Content,
		}
		for _, call := range response.ToolCalls {
			tr, msg := r.executeToolCall(ctx, call)
			step.ToolResults = append(step.ToolResults, tr)
			history = append(history, msg)
		}
		r.transcript.AddStep(step)
	}

	return r.transcript, nil
}

// executeToolCall dispatches one tool call and returns its result and the history message.
func (r *Runner) executeToolCall(ctx context.Context, call model.ToolCall) (ToolResult, model.Message) {
	tool, ok := r.registry.Get(call.Name)
	if !ok {
		errMsg := fmt.Sprintf("tool %q not found", call.Name)
		return ToolResult{ToolName: call.Name, Params: call.Arguments, Output: errMsg, IsError: true},
			model.Message{Role: model.RoleTool, ToolCallID: call.ID, Content: errMsg}
	}

	if r.cfg.DryRun {
		const dryRunMsg = "[dry-run] skipped"
		return ToolResult{ToolName: call.Name, Params: call.Arguments, Skipped: true},
			model.Message{Role: model.RoleTool, ToolCallID: call.ID, Content: dryRunMsg}
	}

	if tool.Destructive() {
		allowed, err := r.confirmer.Confirm(ctx, call.Name, call.Arguments)
		if err != nil || !allowed {
			const deniedMsg = "[denied] user denied"
			return ToolResult{ToolName: call.Name, Params: call.Arguments, Skipped: true},
				model.Message{Role: model.RoleTool, ToolCallID: call.ID, Content: deniedMsg}
		}
	}

	output, execErr := tool.Execute(ctx, json.RawMessage(call.Arguments))
	resultContent := output
	if execErr != nil && resultContent == "" {
		resultContent = "error: " + execErr.Error()
	}

	return ToolResult{
		ToolName: call.Name,
		Params:   call.Arguments,
		Output:   resultContent,
		IsError:  execErr != nil,
	}, model.Message{Role: model.RoleTool, ToolCallID: call.ID, Content: resultContent}
}

// retrieveContext fetches RAG chunks for the last user message in history.
func (r *Runner) retrieveContext(ctx context.Context, history []model.Message) []rag.Chunk {
	if r.retriever == nil {
		return nil
	}
	chunks, _ := r.retriever.Retrieve(ctx, lastUserMessage(history))
	return chunks
}

// appendRAGContext builds a new history slice with a RAG context message appended.
// The original history slice is never mutated.
func appendRAGContext(history []model.Message, chunks []rag.Chunk) []model.Message {
	if len(chunks) == 0 {
		return history
	}
	msg := buildRAGMessage(chunks)
	result := make([]model.Message, len(history)+1)
	copy(result, history)
	result[len(history)] = msg
	return result
}

func buildRAGMessage(chunks []rag.Chunk) model.Message {
	var sb strings.Builder
	sb.WriteString("Relevant context:\n\n")
	for _, ch := range chunks {
		fmt.Fprintf(&sb, "%s:%d\n%s\n---\n", ch.FilePath, ch.StartLine, ch.Content)
	}
	return model.Message{Role: model.RoleSystem, Content: sb.String()}
}

// lastUserMessage returns the content of the most recent user message in history.
func lastUserMessage(history []model.Message) string {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == model.RoleUser {
			return history[i].Content
		}
	}
	return ""
}
