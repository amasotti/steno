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

func TestInspectFiltersByOriginAndStatus(t *testing.T) {
	runDir := t.TempDir()
	lines := []string{
		`{"seq":1,"kind":"navigation","origin":"https://portal.example","path":"/login"}`,
		`{"seq":2,"kind":"call","origin":"https://portal.example","method":"GET","path":"/a","status":200}`,
		`{"seq":3,"kind":"call","origin":"https://api.example","method":"POST","path":"/b","status":404}`,
		`{"seq":4,"kind":"call","origin":"https://api.example","method":"GET","path":"/c","status":503}`,
	}
	if err := os.WriteFile(filepath.Join(runDir, "timeline.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("write timeline fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), []string{"inspect", runDir, "--origin", "api", "--status", "5xx"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got, want := stdout.String(), "0004  503  GET  https://api.example/c\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestInspectShowsDetailBySeqWithArtifacts(t *testing.T) {
	runDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(runDir, "payloads"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "payloads", "0005-req-headers.json"), []byte(`{"cookie":"s=1"}`), 0o600); err != nil {
		t.Fatalf("write headers: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "payloads", "0005-res.txt"), []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatalf("write body: %v", err)
	}
	line := `{"seq":5,"kind":"call","origin":"https://api.example","method":"POST","path":"/login","status":200,"duration_ms":42,"size_bytes":11,"request_headers_ref":"payloads/0005-req-headers.json","response_ref":"payloads/0005-res.txt"}`
	if err := os.WriteFile(filepath.Join(runDir, "timeline.jsonl"), []byte(line+"\n"), 0o600); err != nil {
		t.Fatalf("write timeline fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	if err := Run(context.Background(), []string{"inspect", runDir, "--seq", "5"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	out := stdout.String()
	for _, want := range []string{
		"seq 0005  POST  200  https://api.example/login",
		"duration 42ms  size 11B",
		"request headers (payloads/0005-req-headers.json):",
		`{"cookie":"s=1"}`,
		"response body (payloads/0005-res.txt):",
		`{"ok":true}`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("detail output missing %q\ngot:\n%s", want, out)
		}
	}
}

func TestInspectSeqNotFound(t *testing.T) {
	runDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(runDir, "timeline.jsonl"), []byte(`{"seq":1,"kind":"call","origin":"https://x","method":"GET","path":"/","status":200}`+"\n"), 0o600); err != nil {
		t.Fatalf("write timeline fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), []string{"inspect", runDir, "--seq", "99"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "no event with seq 99") {
		t.Fatalf("error = %v, want no event with seq 99", err)
	}
}

func TestInspectRejectsInvalidStatus(t *testing.T) {
	runDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(runDir, "timeline.jsonl"), []byte("\n"), 0o600); err != nil {
		t.Fatalf("write timeline fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), []string{"inspect", runDir, "--status", "nope"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("Run returned nil error, want invalid status")
	}
	if !strings.Contains(err.Error(), "invalid status") {
		t.Fatalf("error = %q, want invalid status", err)
	}
}

func TestInspectAcceptsTimelineFilePath(t *testing.T) {
	runDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(runDir, "payloads"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "payloads", "0002-res.txt"), []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatalf("write body: %v", err)
	}
	line := `{"seq":2,"kind":"call","origin":"https://x","method":"GET","path":"/","status":200,"response_ref":"payloads/0002-res.txt"}`
	timelinePath := filepath.Join(runDir, "timeline.jsonl")
	if err := os.WriteFile(timelinePath, []byte(line+"\n"), 0o600); err != nil {
		t.Fatalf("write timeline fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	// Pass the timeline file directly, not the directory.
	if err := Run(context.Background(), []string{"inspect", timelinePath, "--seq", "2"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), `{"ok":true}`) {
		t.Fatalf("detail output missing body, got:\n%s", stdout.String())
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
