package timeline

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

func ReadJSONL(r io.Reader) ([]Event, error) {
	var events []Event
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		var event Event
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
