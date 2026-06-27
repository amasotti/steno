package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/amasotti/steno/internal/filter"
	"github.com/amasotti/steno/internal/payload"
	"github.com/amasotti/steno/internal/timeline"
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
	restore := stubRecorderFactory(t, []timeline.Event{})
	defer restore()

	var stdout, stderr bytes.Buffer

	err := Run(context.Background(), []string{"record", "--url", "https://example.test", "--out", "./runs/demo"}, &stdout, &stderr)

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	err = Run(context.Background(), []string{"record", "--out", "./runs/demo"}, &stdout, &stderr)

	if err != nil {
		t.Fatalf("Run returned error for missing optonal arg: %v", err)
	}
}

func stubRecorderFactory(t *testing.T, events []timeline.Event) func() {
	t.Helper()

	previous := newRecorder
	newRecorder = func(rawURL string, _ *filter.Filter, _ *payload.Store, _ bool) eventRecorder {
		return stubRecorder{events: events}
	}
	return func() {
		newRecorder = previous
	}
}

type stubRecorder struct {
	events []timeline.Event
}

func (s stubRecorder) Record(context.Context) (<-chan timeline.Event, <-chan error) {
	events := make(chan timeline.Event, len(s.events))
	errs := make(chan error, 1)
	for _, event := range s.events {
		events <- event
	}
	close(events)
	close(errs)
	return events, errs
}
