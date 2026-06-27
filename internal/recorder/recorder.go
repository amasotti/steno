package recorder

import (
	"context"

	"github.com/amasotti/steno/internal/timeline"
)

type Recorder interface {
	Record(ctx context.Context) (<-chan timeline.Event, <-chan error)
}
