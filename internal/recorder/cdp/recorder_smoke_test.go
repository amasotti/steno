package cdp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"

	"github.com/amasotti/steno/internal/config"
	"github.com/amasotti/steno/internal/filter"
	"github.com/amasotti/steno/internal/timeline"
)

// TestMultiTabSmoke launches a real (headless) Chrome and verifies that a tab
// Steno did not open itself — discovered only through followNewTabs — lands in
// the same ordered timeline as the first tab. It is env-gated so the normal
// `just test` run does not require a browser.
//
// Run with: STENO_SMOKE=1 go test ./internal/recorder/cdp -run MultiTabSmoke -v
//
// The second tab fires its API call after a short delay, mirroring a human
// acting in a freshly-opened tab (i.e. after Steno has attached). A tab whose
// only requests fire in the first few milliseconds, before attach, is a known
// limitation documented in .pairing/PLAN.md.
func TestMultiTabSmoke(t *testing.T) {
	if os.Getenv("STENO_SMOKE") == "" {
		t.Skip("set STENO_SMOKE=1 to run the headless-browser multi-tab smoke test")
	}

	page := func(body string) string {
		return `<!doctype html><meta charset="utf-8"><title>t</title>` + body
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/a", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page(`A<script>fetch('/api/a')</script>`)))
	})
	mux.HandleFunc("/b", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page(`B<script>setTimeout(()=>fetch('/api/ping'),1500)</script>`)))
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), chromedp.DefaultExecAllocatorOptions[:]...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()
	if err := chromedp.Run(browserCtx); err != nil {
		t.Skipf("cannot launch Chrome in this environment: %v", err)
	}

	// Collect every event the listeners emit into one slice (the shared timeline).
	testCtx := t.Context()
	var mu sync.Mutex
	var got []timeline.Event
	payloads := make(chan payloadWork, 256)
	go func() {
		for {
			select {
			case w := <-payloads:
				mu.Lock()
				got = append(got, w.event)
				mu.Unlock()
			case <-testCtx.Done():
				return
			}
		}
	}()

	r := &Recorder{Filter: filter.New(config.Default().Filter)}
	st := &recordState{sequence: new(atomic.Int64), payloads: payloads, parent: testCtx}

	firstID := chromedp.FromContext(browserCtx).Target.TargetID
	r.listenTab(browserCtx, st)
	r.followNewTabs(browserCtx, st, firstID)

	// Drive the first tab.
	if err := chromedp.Run(browserCtx, chromedp.Navigate(srv.URL+"/a")); err != nil {
		t.Fatalf("navigate first tab: %v", err)
	}

	// Open a SECOND tab at the protocol level — no chromedp context for it — so
	// the only thing that can capture it is Steno's followNewTabs discovery.
	browser := chromedp.FromContext(browserCtx).Browser
	if _, err := target.CreateTarget(srv.URL + "/b").Do(cdp.WithExecutor(browserCtx, browser)); err != nil {
		t.Fatalf("create second tab: %v", err)
	}

	has := func(path string) bool {
		mu.Lock()
		defer mu.Unlock()
		for _, e := range got {
			if e.Origin == srv.URL && strings.Contains(e.Path, path) {
				return true
			}
		}
		return false
	}

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if has("/api/a") && has("/api/ping") {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	mu.Lock()
	for _, e := range got {
		t.Logf("seq=%d kind=%s %s%s status=%d", e.Seq, e.Kind, e.Origin, e.Path, e.Status)
	}
	mu.Unlock()

	if !has("/api/a") {
		t.Error("first-tab call /api/a not captured")
	}
	if !has("/api/ping") {
		t.Error("second-tab call /api/ping not captured — multi-tab capture failed")
	}
}
