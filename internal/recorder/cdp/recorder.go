package cdp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
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
	// Stderr receives non-fatal warnings (e.g. a payload that could not be
	// written to disk). Defaults to os.Stderr when nil.
	Stderr io.Writer
}

// warnf reports a non-fatal problem without aborting the recording.
func (r *Recorder) warnf(format string, args ...any) {
	w := r.Stderr
	if w == nil {
		w = os.Stderr
	}
	_, _ = fmt.Fprintf(w, "steno: "+format+"\n", args...)
}

type requestSnapshot struct {
	Method string
	URL    string
	Status int
	Start  time.Time
	End    time.Time
	Size   int
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
	// execCtx is the tab context whose CDP executor can fetch this call's body.
	// Each tab has its own executor, so bodies must be fetched against the tab
	// the request belongs to. nil for navigation events (no body).
	execCtx context.Context
}

// recordState is shared by every tab's listener so the whole browser session
// lands in one ordered timeline: a single sequence counter and a single event
// channel drained by one worker.
type recordState struct {
	sequence *atomic.Int64
	payloads chan payloadWork
	parent   context.Context
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
	var wg sync.WaitGroup
	payloads := make(chan payloadWork, 128)

	st := &recordState{sequence: &sequence, payloads: payloads, parent: parent}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case work := <-payloads:
				if work.req != nil && work.execCtx != nil {
					r.fetchBodies(&work)
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

	// Capture the initial tab.
	r.listenTab(ctx, st)

	// Follow the rest of the browser. Every tab or popup another tab opens is a
	// new CDP target; attaching a context to it makes chromedp enable the
	// Network and Page domains on that target, so its events flow into the same
	// sequence counter and channel — one ordered timeline across the whole
	// browser, not one log per tab.
	var attachedMu sync.Mutex
	attached := map[target.ID]bool{}
	chromedp.ListenBrowser(ctx, func(ev any) {
		created, ok := ev.(*target.EventTargetCreated)
		if !ok {
			return
		}
		info := created.TargetInfo
		// Only real pages opened by another tab (link or popup). The first tab
		// has no opener and is already captured above.
		if info.Type != "page" || info.OpenerID == "" {
			return
		}
		attachedMu.Lock()
		if attached[info.TargetID] {
			attachedMu.Unlock()
			return
		}
		attached[info.TargetID] = true
		attachedMu.Unlock()

		// ListenBrowser callbacks run synchronously on the event loop, so any
		// CDP round-trip (NewContext/Run) must happen on its own goroutine.
		go func(id target.ID) {
			tabCtx, _ := chromedp.NewContext(ctx, chromedp.WithTargetID(id))
			r.listenTab(tabCtx, st)
			if err := chromedp.Run(tabCtx, network.Enable(), page.Enable()); err != nil {
				r.warnf("could not attach to new tab %s: %v", id, err)
			}
		}(info.TargetID)
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

// fetchBodies fills in a captured call's side-file refs, fetching request and
// response bodies against the originating tab's CDP executor (work.execCtx).
// That executor is a descendant of the recorder's parent context, but it
// reaches the worker through a channel, so contextcheck cannot see the lineage.
//
//nolint:contextcheck // execCtx is the correct tab executor; lineage is via channel
func (r *Recorder) fetchBodies(work *payloadWork) {
	work.event.RequestRef = r.saveRequestBody(work.execCtx, work.event.Seq, work.req)
	work.event.ResponseRef = r.saveResponseBody(work.execCtx, work.event.Seq, work.req)
	work.event.RequestHeadersRef = r.saveHeaders(work.event.Seq, payload.KindRequestHeaders, work.req.resourceType, work.req.requestHeaders)
	work.event.ResponseHeadersRef = r.saveHeaders(work.event.Seq, payload.KindResponseHeaders, work.req.resourceType, work.req.responseHeaders)
}

// listenTab wires CDP Network and Page events from one target (tab or popup)
// into the shared timeline. Each tab keeps its own in-flight request map because
// request IDs are only unique within a target. Bodies are tagged with tabCtx so
// the worker fetches them against the right tab's executor.
func (r *Recorder) listenTab(tabCtx context.Context, st *recordState) {
	var mu sync.Mutex
	requests := map[network.RequestID]*trackedRequest{}

	chromedp.ListenTarget(tabCtx, func(ev any) {
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
			event.Seq = int(st.sequence.Add(1))

			select {
			case st.payloads <- payloadWork{event: event}:
			case <-st.parent.Done():
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
			if req := requests[ev.RequestID]; req != nil {
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
			event.Seq = int(st.sequence.Add(1))

			select {
			case st.payloads <- payloadWork{req: req, event: event, execCtx: tabCtx}:
			case <-st.parent.Done():
			}
		}
	})
}

func snapshotToEvent(snapshot requestSnapshot) (timeline.Event, bool) {
	parsed, err := url.Parse(snapshot.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return timeline.Event{}, false
	}

	return timeline.Event{
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
		return "" // CDP has no request body for this call — expected, stay silent.
	}
	ref, err := r.Payloads.Save(seq, payload.KindRequest, body)
	if err != nil {
		r.warnf("could not write request payload for seq %d: %v", seq, err)
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
		r.warnf("could not write %s for seq %d: %v", kind, seq, err)
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
		return "" // CDP has no response body (e.g. evicted on navigation) — expected, stay silent.
	}
	ref, err := r.Payloads.Save(seq, payload.KindResponse, body)
	if err != nil {
		r.warnf("could not write response payload for seq %d: %v", seq, err)
		return ""
	}
	return ref
}
