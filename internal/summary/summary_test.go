package summary

import (
	"strings"
	"testing"

	"github.com/amasotti/steno/internal/timeline"
)

func TestFromEventsEmpty(t *testing.T) {
	got := FromEvents(nil)

	if got.TotalCalls != 0 {
		t.Fatalf("TotalCalls = %d, want 0", got.TotalCalls)
	}
	if len(got.Origins) != 0 {
		t.Fatalf("Origins = %v, want empty", got.Origins)
	}
	if got.HasDurations {
		t.Fatal("HasDurations = true, want false")
	}
}

func TestFromEventsSummarizesCalls(t *testing.T) {
	events := []timeline.Event{
		{Kind: timeline.KindCall, Origin: "https://b.example", Status: 201, DurationMS: 100},
		{Kind: timeline.KindCall, Origin: "https://a.example", Status: 302, DurationMS: 50},
		{Kind: timeline.KindCall, Origin: "https://b.example", Status: 500, DurationMS: 250},
		{Kind: timeline.KindCall, Origin: "https://a.example", Status: 404, DurationMS: 0},
	}

	got := FromEvents(events)

	if got.TotalCalls != 4 {
		t.Fatalf("TotalCalls = %d, want 4", got.TotalCalls)
	}
	if strings.Join(got.Origins, ",") != "https://a.example,https://b.example" {
		t.Fatalf("Origins = %v, want sorted unique origins", got.Origins)
	}
	if got.Status2xx != 1 || got.Status3xx != 1 || got.Status4xx != 1 || got.Status5xx != 1 {
		t.Fatalf("status counts = %+v, want one in each class", got)
	}
	if got.AverageDurationMS != 100 {
		t.Fatalf("AverageDurationMS = %d, want 100", got.AverageDurationMS)
	}
	if got.MinDurationMS != 0 {
		t.Fatalf("MinDurationMS = %d, want 0", got.MinDurationMS)
	}
	if got.MaxDurationMS != 250 {
		t.Fatalf("MaxDurationMS = %d, want 250", got.MaxDurationMS)
	}
}

func TestFromEventsIgnoresNonCalls(t *testing.T) {
	events := []timeline.Event{
		{Kind: timeline.KindNavigation, Origin: "https://ignored.example", Status: 200, DurationMS: 999},
		{Kind: timeline.KindCall, Origin: "https://kept.example", Status: 200, DurationMS: 10},
	}

	got := FromEvents(events)

	if got.TotalCalls != 1 {
		t.Fatalf("TotalCalls = %d, want 1", got.TotalCalls)
	}
	if strings.Join(got.Origins, ",") != "https://kept.example" {
		t.Fatalf("Origins = %v, want kept call origin only", got.Origins)
	}
}
