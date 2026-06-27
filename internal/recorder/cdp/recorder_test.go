package cdp

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/amasotti/steno/internal/timeline"
)

func TestSnapshotToEventNormalizesURLAndDuration(t *testing.T) {
	start := time.Date(2026, 6, 27, 14, 0, 0, 0, time.UTC)
	end := start.Add(150 * time.Millisecond)

	event, ok := snapshotToEvent(requestSnapshot{
		Sequence:        7,
		Method:          "POST",
		URL:             "https://example.test/api/users?debug=true",
		Status:          201,
		Start:           start,
		End:             end,
		Size:            1234,
		RequestPayload:  json.RawMessage(`{"username":"admin"}`),
		ResponsePayload: json.RawMessage(`{"ok":true}`),
	})

	if !ok {
		t.Fatal("snapshotToEvent returned ok=false")
	}
	if event.Kind != timeline.KindCall {
		t.Fatalf("Kind = %q, want %q", event.Kind, timeline.KindCall)
	}
	if event.Origin != "https://example.test" {
		t.Fatalf("Origin = %q, want https://example.test", event.Origin)
	}
	if event.Path != "/api/users?debug=true" {
		t.Fatalf("Path = %q, want /api/users?debug=true", event.Path)
	}
	if event.DurationMS != 150 {
		t.Fatalf("DurationMS = %d, want 150", event.DurationMS)
	}
	if event.SizeBytes != 1234 {
		t.Fatalf("SizeBytes = %d, want 1234", event.SizeBytes)
	}
	if string(event.RequestPayload) != `{"username":"admin"}` {
		t.Fatalf("RequestPayload = %s, want request JSON", event.RequestPayload)
	}
	if string(event.ResponsePayload) != `{"ok":true}` {
		t.Fatalf("ResponsePayload = %s, want response JSON", event.ResponsePayload)
	}
}

func TestInlineJSONPayloadKeepsOnlyJSON(t *testing.T) {
	if got := inlineJSONPayload([]byte(`{"ok":true}`)); string(got) != `{"ok":true}` {
		t.Fatalf("inlineJSONPayload(JSON) = %s, want original JSON", got)
	}
	if got := inlineJSONPayload([]byte(`<html></html>`)); got != nil {
		t.Fatalf("inlineJSONPayload(HTML) = %s, want nil", got)
	}
}

func TestShouldCapturePayloadSkipsDocumentsByDefault(t *testing.T) {
	recorder := New("", nil, false)

	if recorder.shouldCapturePayload("Document") {
		t.Fatal("document payload should be skipped by default")
	}
	if !recorder.shouldCapturePayload("Fetch") {
		t.Fatal("fetch payload should be captured by default")
	}
}

func TestSnapshotToEventRejectsInvalidURL(t *testing.T) {
	_, ok := snapshotToEvent(requestSnapshot{URL: "://not-a-url"})
	if ok {
		t.Fatal("snapshotToEvent returned ok=true for invalid URL")
	}
}

func TestNavigationEventNormalizesURL(t *testing.T) {
	event, ok := navigationEvent("https://example.test/dashboard?tab=1")

	if !ok {
		t.Fatal("navigationEvent returned ok=false")
	}
	if event.Kind != timeline.KindNavigation {
		t.Fatalf("Kind = %q, want %q", event.Kind, timeline.KindNavigation)
	}
	if event.Origin != "https://example.test" {
		t.Fatalf("Origin = %q, want https://example.test", event.Origin)
	}
	if event.Path != "/dashboard?tab=1" {
		t.Fatalf("Path = %q, want /dashboard?tab=1", event.Path)
	}
}

func TestNavigationEventRejectsURLsWithoutHost(t *testing.T) {
	for _, raw := range []string{"about:blank", "://not-a-url", ""} {
		if _, ok := navigationEvent(raw); ok {
			t.Fatalf("navigationEvent(%q) returned ok=true, want false", raw)
		}
	}
}
