package cdp

import (
	"context"
	"net/url"
	"time"

	"github.com/amasotti/steno/internal/timeline"
)

type Recorder struct {
	URL string
}

type requestSnapshot struct {
	Sequence int
	Method   string
	URL      string
	Status   int
	Start    time.Time
	End      time.Time
	Size     int
}

func New(rawURL string) *Recorder {
	return &Recorder{URL: rawURL}
}

func (r *Recorder) Record(ctx context.Context) (<-chan timeline.Event, <-chan error) {
	events := make(chan timeline.Event)
	errs := make(chan error, 1)
	go func() {
		defer close(events)
		defer close(errs)
		<-ctx.Done()
		errs <- ctx.Err()
	}()
	return events, errs
}

func snapshotToEvent(snapshot requestSnapshot) (timeline.Event, bool) {
	parsed, err := url.Parse(snapshot.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return timeline.Event{}, false
	}

	return timeline.Event{
		Seq:        snapshot.Sequence,
		Time:       snapshot.Start,
		Kind:       timeline.KindCall,
		Origin:     parsed.Scheme + "://" + parsed.Host,
		Method:     snapshot.Method,
		Path:       pathWithQuery(parsed),
		Status:     snapshot.Status,
		DurationMS: int(snapshot.End.Sub(snapshot.Start).Milliseconds()),
		SizeBytes:  snapshot.Size,
	}, true
}

func pathWithQuery(parsed *url.URL) string {
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	if parsed.RawQuery != "" {
		path += "?" + parsed.RawQuery
	}
	return path
}
