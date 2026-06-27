package inspect

import (
	"fmt"
	"io"

	"github.com/amasotti/steno/internal/timeline"
)

func ReadTimeline(r io.Reader) ([]timeline.Event, error) {
	events, err := timeline.ReadJSONL(r)
	if err != nil {
		return nil, fmt.Errorf("read timeline: %w", err)
	}
	return events, nil
}
