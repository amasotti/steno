package cdp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/amasotti/steno/internal/filter"
	"github.com/amasotti/steno/internal/payload"
	"github.com/amasotti/steno/internal/timeline"
)

type Recorder struct {
	URL              string
	Filter           *filter.Filter
	Payloads         *payload.Store
	IncludeDocuments bool
	IncludeHeaders   bool
}

type requestSnapshot struct {
	Sequence    int
	Method      string
	URL         string
	Status      int
	Start       time.Time
	End         time.Time
	Size        int
	RequestRef  string
	ResponseRef string
}

type trackedRequest struct {
	method          string
	rawURL          string
	requestID       network.RequestID
	hasPostData     bool
	resourceType    network.ResourceType
	status          int
	start           time.Time
	end             time.Time
	size            int
	requestHeaders  map[string]string
	responseHeaders map[string]string
}

type payloadWork struct {
	req   *trackedRequest
	event timeline.Event
}

func New(rawURL string, f *filter.Filter, payloads *payload.Store, includeDocuments, includeHeaders bool) *Recorder {
	return &Recorder{URL: rawURL, Filter: f, Payloads: payloads, IncludeDocuments: includeDocuments, IncludeHeaders: includeHeaders}
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
	var wg sync.WaitGroup
	requests := map[network.RequestID]*trackedRequest{}
	payloads := make(chan payloadWork, 128)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case work := <-payloads:
				if work.req != nil {
					work.event.RequestRef = r.saveRequestBody(ctx, work.event.Seq, work.req)
					work.event.ResponseRef = r.saveResponseBody(ctx, work.event.Seq, work.req)
					work.event.RequestHeadersRef = r.saveHeaders(work.event.Seq, payload.KindRequestHeaders, work.req.resourceType, work.req.requestHeaders)
					work.event.ResponseHeadersRef = r.saveHeaders(work.event.Seq, payload.KindResponseHeaders, work.req.resourceType, work.req.responseHeaders)
				}

				select {
				case events <- work.event:
				case <-parent.Done():
					return
				}
			case <-parent.Done():
				return
			}
		}
	}()

	chromedp.ListenTarget(ctx, func(ev any) {
		switch ev := ev.(type) {
		case *page.EventFrameNavigated:
			// Only top-frame navigations anchor the journey; sub-frames are noise.
			if ev.Frame == nil || ev.Frame.ParentID != "" {
				return
			}
			event, ok := navigationEvent(ev.Frame.URL)
			if !ok {
				return
			}
			event.Seq = int(sequence.Add(1))

			select {
			case payloads <- payloadWork{event: event}:
			case <-parent.Done():
			}
		case *network.EventRequestWillBeSent:
			mu.Lock()
			requests[ev.RequestID] = &trackedRequest{
				method:         ev.Request.Method,
				rawURL:         ev.Request.URL,
				requestID:      ev.RequestID,
				hasPostData:    ev.Request.HasPostData,
				start:          time.Now().UTC(),
				requestHeaders: flattenHeaders(ev.Request.Headers),
			}
			mu.Unlock()
		case *network.EventRequestWillBeSentExtraInfo:
			// Raw headers include the Cookie header that Chrome redacts from the
			// main event. Overwrite the fallback captured above when present.
			mu.Lock()
			if req := requests[ev.RequestID]; req != nil {
				if h := flattenHeaders(ev.Headers); len(h) > 0 {
					req.requestHeaders = h
				}
			}
			mu.Unlock()
		case *network.EventResponseReceived:
			mu.Lock()
			req := requests[ev.RequestID]
			if req != nil {
				req.resourceType = ev.Type
				req.status = int(ev.Response.Status)
				if req.responseHeaders == nil {
					req.responseHeaders = flattenHeaders(ev.Response.Headers)
				}
			}
			mu.Unlock()
		case *network.EventResponseReceivedExtraInfo:
			// Raw headers include Set-Cookie that Chrome redacts from the main event.
			mu.Lock()
			if req := requests[ev.RequestID]; req != nil {
				if h := flattenHeaders(ev.Headers); len(h) > 0 {
					req.responseHeaders = h
				}
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

			if req == nil {
				return
			}
			event, ok := snapshotToEvent(requestSnapshot{
				Method: req.method,
				URL:    req.rawURL,
				Status: req.status,
				Start:  req.start,
				End:    req.end,
				Size:   req.size,
			})
			if !ok {
				return
			}
			if !r.Filter.Keep(string(req.resourceType), event) {
				return
			}
			event.Seq = int(sequence.Add(1))

			select {
			case payloads <- payloadWork{req: req, event: event}:
			case <-parent.Done():
			}
		}
	})

	actions := []chromedp.Action{
		network.Enable(),
		page.Enable(),
	}
	if r.URL != "" {
		actions = append(actions, chromedp.Navigate(r.URL))
	}

	if err := chromedp.Run(ctx, actions...); err != nil {
		return err
	}

	<-parent.Done()
	wg.Wait()
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
		Seq:         snapshot.Sequence,
		Time:        snapshot.Start,
		Kind:        timeline.KindCall,
		Origin:      parsed.Scheme + "://" + parsed.Host,
		Method:      snapshot.Method,
		Path:        pathWithQuery(parsed),
		Status:      snapshot.Status,
		DurationMS:  int(snapshot.End.Sub(snapshot.Start).Milliseconds()),
		SizeBytes:   snapshot.Size,
		RequestRef:  snapshot.RequestRef,
		ResponseRef: snapshot.ResponseRef,
	}, true
}

// navigationEvent normalizes a top-frame navigation URL into a navigation
// event. URLs without a scheme and host (e.g. "about:blank") are rejected so
// the initial blank page does not pollute the journey.
func navigationEvent(rawURL string) (timeline.Event, bool) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return timeline.Event{}, false
	}

	return timeline.Event{
		Time:   time.Now().UTC(),
		Kind:   timeline.KindNavigation,
		Origin: parsed.Scheme + "://" + parsed.Host,
		Path:   pathWithQuery(parsed),
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

func (r *Recorder) shouldCapturePayload(resourceType string) bool {
	return resourceType != string(network.ResourceTypeDocument) || r.IncludeDocuments
}

func (r *Recorder) saveRequestBody(ctx context.Context, seq int, req *trackedRequest) string {
	if r.Payloads == nil || !req.hasPostData || !r.shouldCapturePayload(string(req.resourceType)) {
		return ""
	}
	body, err := network.GetRequestPostData(req.requestID).Do(cdp.WithExecutor(ctx, chromedp.FromContext(ctx).Target))
	if err != nil {
		return ""
	}
	ref, err := r.Payloads.Save(seq, payload.KindRequest, body)
	if err != nil {
		return ""
	}
	return ref
}

func (r *Recorder) saveHeaders(seq int, kind payload.Kind, resourceType network.ResourceType, headers map[string]string) string {
	if r.Payloads == nil || !r.IncludeHeaders || len(headers) == 0 || !r.shouldCapturePayload(string(resourceType)) {
		return ""
	}
	ref, err := r.Payloads.SaveHeaders(seq, kind, headers)
	if err != nil {
		return ""
	}
	return ref
}

// flattenHeaders converts CDP's map[string]any header bag into a plain
// string map so no CDP type leaks past the recorder seam. Values are already
// strings in practice; anything else is stringified defensively.
func flattenHeaders(h network.Headers) map[string]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		switch val := v.(type) {
		case string:
			out[k] = val
		default:
			out[k] = fmt.Sprintf("%v", val)
		}
	}
	return out
}

func (r *Recorder) saveResponseBody(ctx context.Context, seq int, req *trackedRequest) string {
	if r.Payloads == nil || !r.shouldCapturePayload(string(req.resourceType)) {
		return ""
	}
	body, err := network.GetResponseBody(req.requestID).Do(cdp.WithExecutor(ctx, chromedp.FromContext(ctx).Target))
	if err != nil {
		return ""
	}
	ref, err := r.Payloads.Save(seq, payload.KindResponse, body)
	if err != nil {
		return ""
	}
	return ref
}
