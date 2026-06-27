package cdp

import (
	"testing"
	"time"

	"github.com/amasotti/steno/internal/timeline"
)

func TestSnapshotToEventNormalizesURLAndDuration(t *testing.T) {
	start := time.Date(2026, 6, 27, 14, 0, 0, 0, time.UTC)
	end := start.Add(150 * time.Millisecond)

	event, ok := snapshotToEvent(requestSnapshot{
		Sequence: 7,
		Method:   "POST",
		URL:      "https://example.test/api/users?debug=true",
		Status:   201,
		Start:    start,
		End:      end,
		Size:     1234,
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

func TestSnapshotToEventRejectsInvalidURL(t *testing.T) {
	_, ok := snapshotToEvent(requestSnapshot{URL: "://not-a-url"})
	if ok {
		t.Fatal("snapshotToEvent returned ok=true for invalid URL")
	}
}
