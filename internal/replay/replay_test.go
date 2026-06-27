package replay

import (
	"strings"
	"testing"

	"github.com/amasotti/steno/internal/timeline"
)

func TestCurlBuildsPostWithHeadersAndBody(t *testing.T) {
	ev := timeline.Event{
		Kind:   timeline.KindCall,
		Method: "POST",
		Origin: "https://account.int.example",
		Path:   "/api/v1/profile?draft=1",
	}
	headers := map[string]string{
		"content-type": "application/json",
		"cookie":       "session=abc",
	}

	cmd, warnings := Curl(ev, headers, []byte(`{"name":"toni"}`))

	for _, want := range []string{
		"curl -X POST 'https://account.int.example/api/v1/profile?draft=1'",
		"-H 'content-type: application/json'",
		"-H 'cookie: session=abc'",
		`--data-raw '{"name":"toni"}'`,
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("curl missing %q\ngot:\n%s", want, cmd)
		}
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
}

func TestCurlGetWithoutBodyOrHeadersWarns(t *testing.T) {
	ev := timeline.Event{
		Kind:   timeline.KindCall,
		Method: "GET",
		Origin: "https://api.example",
		Path:   "/health",
	}

	cmd, warnings := Curl(ev, nil, nil)

	if want := "curl -X GET 'https://api.example/health'"; cmd != want {
		t.Fatalf("curl = %q, want %q", cmd, want)
	}
	if strings.Contains(cmd, "--data-raw") {
		t.Fatalf("GET with no body should not emit --data-raw: %s", cmd)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "include_headers") {
		t.Fatalf("want one include_headers warning, got %v", warnings)
	}
}

func TestCurlDropsPseudoAndContentLengthHeaders(t *testing.T) {
	ev := timeline.Event{
		Kind:   timeline.KindCall,
		Method: "POST",
		Origin: "https://api.example",
		Path:   "/x",
	}
	headers := map[string]string{
		":authority":     "api.example",
		":method":        "POST",
		"content-length": "15",
		"authorization":  "Bearer t",
	}

	cmd, warnings := Curl(ev, headers, []byte("body"))

	for _, banned := range []string{":authority", ":method", "content-length"} {
		if strings.Contains(cmd, banned) {
			t.Fatalf("curl must not contain %q\ngot:\n%s", banned, cmd)
		}
	}
	if !strings.Contains(cmd, "-H 'authorization: Bearer t'") {
		t.Fatalf("curl missing real header\ngot:\n%s", cmd)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
}

func TestCurlEscapesSingleQuotesInBody(t *testing.T) {
	ev := timeline.Event{
		Kind:   timeline.KindCall,
		Method: "POST",
		Origin: "https://api.example",
		Path:   "/x",
	}

	cmd, _ := Curl(ev, map[string]string{"x": "y"}, []byte(`{"q":"it's"}`))

	if !strings.Contains(cmd, `--data-raw '{"q":"it'\''s"}'`) {
		t.Fatalf("body single-quote not escaped\ngot:\n%s", cmd)
	}
}

func TestCurlWarnsWhenOnlySkippedHeadersPresent(t *testing.T) {
	ev := timeline.Event{Kind: timeline.KindCall, Method: "GET", Origin: "https://api.example", Path: "/x"}

	_, warnings := Curl(ev, map[string]string{"content-length": "0", ":authority": "api.example"}, nil)

	if len(warnings) != 1 {
		t.Fatalf("want warning when no usable headers remain, got %v", warnings)
	}
}
