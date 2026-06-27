package render

import (
	"fmt"
	"io"

	"github.com/amasotti/steno/internal/timeline"
)

func Mermaid(w io.Writer, events []timeline.Event) error {
	if _, err := fmt.Fprintln(w, "sequenceDiagram"); err != nil {
		return err
	}
	for _, event := range events {
		if event.Kind != timeline.KindCall {
			continue
		}
		if _, err := fmt.Fprintf(w, "  User->>%s: %s %s (%d)\n", event.Origin, event.Method, event.Path, event.Status); err != nil {
			return err
		}
	}
	return nil
}
