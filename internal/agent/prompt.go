package agent

import (
	"bytes"
	"fmt"
	"text/template"
)

const systemPromptTemplate = `You are xdreamer, an autonomous coding agent. Work step by step using the tools provided.
After each tool result, select the next action. When the task is complete, write a short
summary of what you did and end your message with the single word DONE on its own line.

Rules:
- Call exactly one tool per turn.
- Never guess file contents — always use read_file before modifying a file.
- Always use write_file to persist changes — never output code in prose.
- Run shell commands in the worktree directory: {{.WorktreePath}}
- Project memory:
{{.MemoryContent}}`

type promptData struct {
	WorktreePath  string
	MemoryContent string
}

var parsedPromptTmpl = template.Must(template.New("system").Parse(systemPromptTemplate))

// renderSystemPrompt renders the system prompt with the given worktree path and memory content.
func renderSystemPrompt(worktreePath, memoryContent string) (string, error) {
	var buf bytes.Buffer
	data := promptData{WorktreePath: worktreePath, MemoryContent: memoryContent}
	if err := parsedPromptTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render system prompt: %w", err)
	}
	return buf.String(), nil
}
