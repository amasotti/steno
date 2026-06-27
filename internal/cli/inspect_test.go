package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectPrintsCallEvents(t *testing.T) {
	runDir := t.TempDir()
	timelinePath := filepath.Join(runDir, "timeline.jsonl")
	err := os.WriteFile(timelinePath, []byte(`{"seq":1,"kind":"call","origin":"https://example.test","method":"GET","path":"/api","status":200}`+"\n"), 0o600)
	if err != nil {
		t.Fatalf("write timeline fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer

	err = Run(context.Background(), []string{"inspect", runDir}, &stdout, &stderr)

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got, want := stdout.String(), "0001  200  GET  https://example.test/api\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestInspectPrintsNavigationAndCallEventsInOrder(t *testing.T) {
	runDir := t.TempDir()
	timelinePath := filepath.Join(runDir, "timeline.jsonl")
	lines := []string{
		`{"seq":1,"kind":"navigation","origin":"https://example.test","path":"/login"}`,
		`{"seq":2,"kind":"call","origin":"https://example.test","method":"GET","path":"/api","status":200}`,
	}
	err := os.WriteFile(timelinePath, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
	if err != nil {
		t.Fatalf("write timeline fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer

	err = Run(context.Background(), []string{"inspect", runDir}, &stdout, &stderr)

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	want := "0001  nav  https://example.test/login\n" +
		"0002  200  GET  https://example.test/api\n"
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestInspectRequiresRunDirectory(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := Run(context.Background(), []string{"inspect"}, &stdout, &stderr)

	if err == nil {
		t.Fatal("Run returned nil error, want missing run directory")
	}
	if !strings.Contains(err.Error(), "missing run directory") {
		t.Fatalf("error = %q, want missing run directory", err)
	}
}
