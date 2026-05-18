package agent

import (
	"context"
	"strings"
	"testing"
)

func TestAutoConfirmer_AlwaysTrue(t *testing.T) {
	c := AutoConfirmer{}
	for _, tc := range []struct{ name, desc string }{
		{"run_shell", `{"cmd":"rm -rf /"}`},
		{"write_file", `{"path":"main.go"}`},
		{"delete_file", `{"path":"LICENCE"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := c.Confirm(context.Background(), tc.name, tc.desc)
			if err != nil {
				t.Fatalf("Confirm: %v", err)
			}
			if !ok {
				t.Error("AutoConfirmer should always return true")
			}
		})
	}
}

func TestTerminalConfirmer_YReturnsTrue(t *testing.T) {
	c := &TerminalConfirmer{In: strings.NewReader("y\n"), Out: &strings.Builder{}}
	ok, err := c.Confirm(context.Background(), "write_file", `{"path":"x"}`)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if !ok {
		t.Error("expected true for 'y' answer")
	}
}

func TestTerminalConfirmer_YESReturnsTrue(t *testing.T) {
	c := &TerminalConfirmer{In: strings.NewReader("YES\n"), Out: &strings.Builder{}}
	ok, err := c.Confirm(context.Background(), "write_file", `{"path":"x"}`)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if !ok {
		t.Error("expected true for 'YES' answer (case-insensitive)")
	}
}

func TestTerminalConfirmer_NReturnsFalse(t *testing.T) {
	c := &TerminalConfirmer{In: strings.NewReader("n\n"), Out: &strings.Builder{}}
	ok, err := c.Confirm(context.Background(), "delete_file", `{"path":"x"}`)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if ok {
		t.Error("expected false for 'n' answer")
	}
}

func TestTerminalConfirmer_EmptyReturnsFalse(t *testing.T) {
	c := &TerminalConfirmer{In: strings.NewReader("\n"), Out: &strings.Builder{}}
	ok, _ := c.Confirm(context.Background(), "run_shell", `{}`)
	if ok {
		t.Error("expected false for empty answer (default is N)")
	}
}

func TestTerminalConfirmer_PrintsPrompt(t *testing.T) {
	var out strings.Builder
	c := &TerminalConfirmer{In: strings.NewReader("n\n"), Out: &out}
	_, _ = c.Confirm(context.Background(), "run_shell", `{"command":"rm -rf /"}`)
	printed := out.String()
	if !strings.Contains(printed, "run_shell") {
		t.Errorf("tool name missing from prompt: %q", printed)
	}
	if !strings.Contains(printed, "[destructive]") {
		t.Errorf("[destructive] label missing from prompt: %q", printed)
	}
}
