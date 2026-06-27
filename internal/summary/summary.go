package summary

import (
	"sort"

	"github.com/amasotti/steno/internal/timeline"
)

type Summary struct {
	TotalCalls        int
	Origins           []string
	Status2xx         int
	Status3xx         int
	Status4xx         int
	Status5xx         int
	HasDurations      bool
	AverageDurationMS int
	MinDurationMS     int
	MaxDurationMS     int
}

func FromEvents(events []timeline.Event) Summary {
	var summary Summary
	origins := map[string]struct{}{}
	totalDuration := 0

	for _, event := range events {
		if event.Kind != timeline.KindCall {
			continue
		}

		summary.TotalCalls++
		if event.Origin != "" {
			origins[event.Origin] = struct{}{}
		}
		countStatus(&summary, event.Status)

		if !summary.HasDurations {
			summary.HasDurations = true
			summary.MinDurationMS = event.DurationMS
			summary.MaxDurationMS = event.DurationMS
		}

		if event.DurationMS < summary.MinDurationMS {
			summary.MinDurationMS = event.DurationMS
		}

		if event.DurationMS > summary.MaxDurationMS {
			summary.MaxDurationMS = event.DurationMS
		}
		totalDuration += event.DurationMS
	}

	for origin := range origins {
		summary.Origins = append(summary.Origins, origin)
	}
	sort.Strings(summary.Origins)

	if summary.TotalCalls > 0 {
		summary.AverageDurationMS = totalDuration / summary.TotalCalls
	}

	return summary
}

func countStatus(summary *Summary, status int) {
	switch {
	case status >= 200 && status < 300:
		summary.Status2xx++
	case status >= 300 && status < 400:
		summary.Status3xx++
	case status >= 400 && status < 500:
		summary.Status4xx++
	case status >= 500 && status < 600:
		summary.Status5xx++
	}
}
