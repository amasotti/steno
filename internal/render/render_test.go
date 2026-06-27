package render

import (
	"strings"
	"testing"

	"github.com/amasotti/steno/internal/timeline"
)

func TestMermaidRendersCallEventsByOrigin(t *testing.T) {
	events := []timeline.Event{
		{Kind: timeline.KindCall, Origin: "https://example.test", Method: "GET", Path: "/", Status: 200},
		{Kind: timeline.KindNavigation, Origin: "https://example.test", Path: "/login"},
		{Kind: timeline.KindCall, Origin: "https://api.example.test", Method: "POST", Path: "/users", Status: 201},
	}
	var out strings.Builder

	err := Mermaid(&out, events)

	if err != nil {
		t.Fatalf("Mermaid returned error: %v", err)
	}

	want := strings.Join([]string{
		"sequenceDiagram",
		"  actor User",
		"  participant example.test",
		"  participant api.example.test",
		"  User->>example.test: GET / (200)",
		"  Note over User: navigate https://example.test/login",
		"  User->>api.example.test: POST /users (201)",
		"",
	}, "\n")
	if got := out.String(); got != want {
		t.Fatalf("Mermaid output = %q, want %q", got, want)
	}
}

func TestMermaidHandlesEmptyTimeline(t *testing.T) {
	var out strings.Builder

	err := Mermaid(&out, nil)

	if err != nil {
		t.Fatalf("Mermaid returned error: %v", err)
	}

	want := strings.Join([]string{
		"sequenceDiagram",
		"  actor User",
		"",
	}, "\n")
	if got := out.String(); got != want {
		t.Fatalf("Mermaid output = %q, want %q", got, want)
	}
}
