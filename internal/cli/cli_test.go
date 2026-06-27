package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRecordRequiresOut(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := Run(context.Background(), []string{"record"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("Run return no error, but --out was missing")
	}

	if !strings.Contains(err.Error(), "missing --out") {
		t.Fatalf("error = %q, want missing --out", err)
	}
}

func TestRecordAcceptsOptionalURL(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := Run(context.Background(), []string{"record", "--url", "https://example.test", "--out", "./runs/demo"}, &stdout, &stderr)

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}
