package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeReplayRun(t *testing.T) string {
	t.Helper()
	runDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(runDir, "payloads"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "payloads", "0005-req-headers.json"), []byte(`{"content-type":"application/json","cookie":"s=1"}`), 0o600); err != nil {
		t.Fatalf("write headers: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "payloads", "0005-req.txt"), []byte(`{"name":"toni"}`), 0o600); err != nil {
		t.Fatalf("write body: %v", err)
	}
	lines := []string{
		`{"seq":4,"kind":"navigation","origin":"https://api.example","path":"/login"}`,
		`{"seq":5,"kind":"call","origin":"https://api.example","method":"POST","path":"/profile?draft=1","status":201,"request_headers_ref":"payloads/0005-req-headers.json","request_ref":"payloads/0005-req.txt"}`,
		`{"seq":6,"kind":"call","origin":"https://api.example","method":"GET","path":"/health","status":200}`,
	}
	if err := os.WriteFile(filepath.Join(runDir, "timeline.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("write timeline: %v", err)
	}
	return runDir
}

func TestReplayEmitsCurlWithHeadersAndBody(t *testing.T) {
	runDir := writeReplayRun(t)

	var stdout, stderr bytes.Buffer
	if err := Run(context.Background(), []string{"replay", runDir, "5"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{
		"curl -X POST 'https://api.example/profile?draft=1'",
		"-H 'content-type: application/json'",
		"-H 'cookie: s=1'",
		`--data-raw '{"name":"toni"}'`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("curl missing %q\ngot:\n%s", want, out)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestReplayWarnsWhenNoHeadersCaptured(t *testing.T) {
	runDir := writeReplayRun(t)

	var stdout, stderr bytes.Buffer
	if err := Run(context.Background(), []string{"replay", runDir, "6"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if got, want := stdout.String(), "curl -X GET 'https://api.example/health'\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if !strings.Contains(stderr.String(), "include_headers") {
		t.Fatalf("stderr missing include_headers warning, got: %s", stderr.String())
	}
}

func TestReplayRejectsNonCallSeq(t *testing.T) {
	runDir := writeReplayRun(t)

	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), []string{"replay", runDir, "4"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "not a call") {
		t.Fatalf("error = %v, want not a call", err)
	}
}

func TestReplaySeqNotFound(t *testing.T) {
	runDir := writeReplayRun(t)

	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), []string{"replay", runDir, "99"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "no event with seq 99") {
		t.Fatalf("error = %v, want no event with seq 99", err)
	}
}

func TestReplayRequiresSeq(t *testing.T) {
	runDir := writeReplayRun(t)

	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), []string{"replay", runDir}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "missing seq") {
		t.Fatalf("error = %v, want missing seq", err)
	}
}

func TestReplayInvalidSeq(t *testing.T) {
	runDir := writeReplayRun(t)

	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), []string{"replay", runDir, "abc"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "invalid seq") {
		t.Fatalf("error = %v, want invalid seq", err)
	}
}
