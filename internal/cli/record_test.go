package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amasotti/steno/internal/timeline"
)

func TestRecordWritesTimelineJSONL(t *testing.T) {
	restore := stubRecorderFactory(t, []timeline.Event{{
		Seq:         1,
		Kind:        timeline.KindCall,
		Origin:      "https://example.test",
		Method:      "GET",
		Path:        "/api",
		Status:      200,
		RequestRef:  "payloads/0001-req.txt",
		ResponseRef: "payloads/0001-res.txt",
	}})
	defer restore()

	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := Run(context.Background(), []string{"record", "--out", outDir}, &stdout, &stderr)

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "timeline.jsonl"))
	if err != nil {
		t.Fatalf("read timeline: %v", err)
	}
	got := string(content)
	for _, want := range []string{`"kind":"call"`, `"path":"/api"`, `"request_ref":"payloads/0001-req.txt"`, `"response_ref":"payloads/0001-res.txt"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("timeline content = %q, want it to contain %q", got, want)
		}
	}
}

func TestRecordPrintsSessionSummary(t *testing.T) {
	restore := stubRecorderFactory(t, []timeline.Event{
		{Seq: 1, Kind: timeline.KindCall, Origin: "https://b.example", Method: "GET", Path: "/", Status: 200, DurationMS: 100},
		{Seq: 2, Kind: timeline.KindCall, Origin: "https://a.example", Method: "POST", Path: "/api", Status: 500, DurationMS: 300},
	})
	defer restore()

	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := Run(context.Background(), []string{"record", "--out", outDir}, &stdout, &stderr)

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	got := stdout.String()
	for _, want := range []string{
		"Session summary\n",
		"Calls: 2\n",
		"  - https://a.example\n",
		"  - https://b.example\n",
		"Status: 2xx=1 3xx=0 4xx=0 5xx=1\n",
		"Duration: avg=200ms min=100ms max=300ms\n",
		"Timeline: " + filepath.Join(outDir, "timeline.jsonl") + "\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout = %q, want it to contain %q", got, want)
		}
	}
}

func TestRecordPrintsEmptySessionSummary(t *testing.T) {
	restore := stubRecorderFactory(t, nil)
	defer restore()

	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := Run(context.Background(), []string{"record", "--out", outDir}, &stdout, &stderr)

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	got := stdout.String()
	for _, want := range []string{
		"Session summary\n",
		"Calls: 0\n",
		"Origins: none\n",
		"Status: 2xx=0 3xx=0 4xx=0 5xx=0\n",
		"Duration: no requests captured\n",
		"Timeline: " + filepath.Join(outDir, "timeline.jsonl") + "\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout = %q, want it to contain %q", got, want)
		}
	}
}
