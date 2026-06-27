package cdp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"

	"github.com/amasotti/steno/internal/payload"
	"github.com/amasotti/steno/internal/timeline"
)

func TestSnapshotToEventNormalizesURLAndDuration(t *testing.T) {
	start := time.Date(2026, 6, 27, 14, 0, 0, 0, time.UTC)
	end := start.Add(150 * time.Millisecond)

	event, ok := snapshotToEvent(requestSnapshot{
		Method: "POST",
		URL:    "https://example.test/api/users?debug=true",
		Status: 201,
		Start:  start,
		End:    end,
		Size:   1234,
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
}

func TestShouldCapturePayloadSkipsDocumentsByDefault(t *testing.T) {
	recorder := New("", nil, payload.NewStore(t.TempDir()), false, false)

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

func TestFlattenHeadersStringifiesValues(t *testing.T) {
	got := flattenHeaders(network.Headers{
		"content-type": "application/json",
		"cookie":       "session=abc; csrf=xyz",
		"x-count":      42,
	})

	if got["content-type"] != "application/json" {
		t.Fatalf("content-type = %q", got["content-type"])
	}
	if got["cookie"] != "session=abc; csrf=xyz" {
		t.Fatalf("cookie = %q", got["cookie"])
	}
	if got["x-count"] != "42" {
		t.Fatalf("x-count = %q, want stringified 42", got["x-count"])
	}
}

func TestFlattenHeadersEmptyReturnsNil(t *testing.T) {
	if got := flattenHeaders(nil); got != nil {
		t.Fatalf("flattenHeaders(nil) = %v, want nil", got)
	}
}

func TestSaveHeadersWarnsOnWriteFailure(t *testing.T) {
	// A regular file as the store root makes creating payloads/ under it fail,
	// so the write error must surface as a warning rather than vanish.
	notADir := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var stderr bytes.Buffer
	r := &Recorder{Payloads: payload.NewStore(notADir), IncludeHeaders: true, Stderr: &stderr}

	ref := r.saveHeaders(7, payload.KindRequestHeaders, network.ResourceTypeXHR, map[string]string{"a": "b"})

	if ref != "" {
		t.Fatalf("ref = %q, want empty on write failure", ref)
	}
	if !strings.Contains(stderr.String(), "could not write") {
		t.Fatalf("stderr = %q, want a write-failure warning", stderr.String())
	}
}
