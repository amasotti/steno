package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderWritesFlowMermaid(t *testing.T) {
	runDir := t.TempDir()
	timelinePath := filepath.Join(runDir, "timeline.jsonl")
	err := os.WriteFile(timelinePath, []byte(`{"seq":1,"kind":"call","origin":"https://example.test","method":"GET","path":"/","status":200}`+"\n"), 0o600)
	if err != nil {
		t.Fatalf("write timeline fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer

	err = Run(context.Background(), []string{"render", runDir}, &stdout, &stderr)

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(runDir, "flow.mmd"))
	if err != nil {
		t.Fatalf("read flow.mmd: %v", err)
	}
	if got := string(content); !strings.Contains(got, "User->>example.test: GET / (200)") {
		t.Fatalf("flow.mmd = %q, want rendered call", got)
	}
	if got, want := stdout.String(), "Rendered: "+filepath.Join(runDir, "flow.mmd")+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRenderRequiresRunDirectory(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := Run(context.Background(), []string{"render"}, &stdout, &stderr)

	if err == nil {
		t.Fatal("Run returned nil error, want missing run directory")
	}
	if !strings.Contains(err.Error(), "missing run directory") {
		t.Fatalf("error = %q, want missing run directory", err)
	}
}
