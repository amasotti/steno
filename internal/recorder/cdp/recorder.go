package cdp

import (
	"context"
	"errors"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

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

type trackedRequest struct {
	method       string
	rawURL       string
	resourceType network.ResourceType
	status       int
	start        time.Time
	end          time.Time
	size         int
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

		errs <- r.record(ctx, events)
	}()
	return events, errs
}

func (r *Recorder) record(parent context.Context, events chan<- timeline.Event) error {
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(parent, append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", false),
	)...)
	defer cancelAlloc()

	ctx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	var sequence atomic.Int64
	var mu sync.Mutex
	requests := map[network.RequestID]*trackedRequest{}

	chromedp.ListenTarget(ctx, func(ev any) {
		switch ev := ev.(type) {
		case *network.EventRequestWillBeSent:
			mu.Lock()
			requests[ev.RequestID] = &trackedRequest{
				method: ev.Request.Method,
				rawURL: ev.Request.URL,
				start:  time.Now().UTC(),
			}
			mu.Unlock()
		case *network.EventResponseReceived:
			mu.Lock()
			req := requests[ev.RequestID]
			if req != nil {
				req.resourceType = ev.Type
				req.status = int(ev.Response.Status)
			}
			mu.Unlock()
		case *network.EventLoadingFinished:
			mu.Lock()
			req := requests[ev.RequestID]
			if req != nil {
				req.end = time.Now().UTC()
				req.size = int(ev.EncodedDataLength)
				delete(requests, ev.RequestID)
			}
			mu.Unlock()

			if req == nil || !keepResourceType(req.resourceType) {
				return
			}
			event, ok := snapshotToEvent(requestSnapshot{
				Sequence: int(sequence.Add(1)),
				Method:   req.method,
				URL:      req.rawURL,
				Status:   req.status,
				Start:    req.start,
				End:      req.end,
				Size:     req.size,
			})
			if !ok {
				return
			}

			select {
			case events <- event:
			case <-parent.Done():
			}
		}
	})

	actions := []chromedp.Action{
		network.Enable(),
	}
	if r.URL != "" {
		actions = append(actions, chromedp.Navigate(r.URL))
	}

	if err := chromedp.Run(ctx, actions...); err != nil {
		return err
	}

	<-parent.Done()
	if errors.Is(parent.Err(), context.Canceled) {
		return nil
	}
	return parent.Err()
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

func keepResourceType(resourceType network.ResourceType) bool {
	switch resourceType {
	case network.ResourceTypeDocument, network.ResourceTypeXHR, network.ResourceTypeFetch:
		return true
	default:
		return false
	}
}
