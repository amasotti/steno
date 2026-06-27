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
		Seq:    1,
		Kind:   timeline.KindCall,
		Origin: "https://example.test",
		Method: "GET",
		Path:   "/api",
		Status: 200,
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
	if got := string(content); !strings.Contains(got, `"kind":"call"`) || !strings.Contains(got, `"path":"/api"`) {
		t.Fatalf("timeline content = %q, want call event with /api path", got)
	}
}
