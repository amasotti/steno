package inspect

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	"github.com/amasotti/steno/internal/timeline"
)

func ReadTimeline(r io.Reader) ([]timeline.Event, error) {
	var events []timeline.Event
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		var event timeline.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, fmt.Errorf("decode timeline event: %w", err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read timeline: %w", err)
	}
	return events, nil
}
