package render

import (
	"fmt"
	"io"
	"net/url"

	"github.com/amasotti/steno/internal/timeline"
)

func Mermaid(w io.Writer, events []timeline.Event) error {
	if _, err := fmt.Fprintln(w, "sequenceDiagram"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "  actor User"); err != nil {
		return err
	}

	participants := orderedParticipants(events)
	for _, participant := range participants {
		if _, err := fmt.Fprintf(w, "  participant %s\n", participant); err != nil {
			return err
		}
	}

	for _, event := range events {
		if event.Kind != timeline.KindCall {
			continue
		}
		if _, err := fmt.Fprintf(w, "  User->>%s: %s %s (%d)\n", participantName(event.Origin), event.Method, event.Path, event.Status); err != nil {
			return err
		}
	}
	return nil
}

func orderedParticipants(events []timeline.Event) []string {
	seen := map[string]struct{}{}
	var participants []string
	for _, event := range events {
		if event.Kind != timeline.KindCall {
			continue
		}
		participant := participantName(event.Origin)
		if participant == "" {
			continue
		}
		if _, ok := seen[participant]; ok {
			continue
		}
		seen[participant] = struct{}{}
		participants = append(participants, participant)
	}
	return participants
}

func participantName(origin string) string {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return origin
	}
	return parsed.Host
}
