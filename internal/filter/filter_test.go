package filter

import (
	"testing"

	"github.com/amasotti/steno/internal/config"
	"github.com/amasotti/steno/internal/timeline"
)

func call(origin, method, path string) timeline.Event {
	return timeline.Event{
		Kind:   timeline.KindCall,
		Origin: origin,
		Method: method,
		Path:   path,
	}
}

func TestKeep(t *testing.T) {
	cfg := config.Filter{
		ResourceTypes: []string{"Document", "XHR", "Fetch"},
		AllowHosts:    []string{"auth.example.test", "*.int.example.test"},
		DenyPaths:     []string{"/health", "/favicon.ico"},
		DropPreflight: true,
	}

	tests := []struct {
		name     string
		rtype    string
		event    timeline.Event
		expected bool
	}{
		{
			name:     "kept resource type, allowed exact host",
			rtype:    "XHR",
			event:    call("https://auth.example.test", "GET", "/api/login"),
			expected: true,
		},
		{
			name:     "kept resource type, allowed wildcard host",
			rtype:    "Fetch",
			event:    call("https://api.int.example.test", "POST", "/users"),
			expected: true,
		},
		{
			name:     "dropped resource type",
			rtype:    "Image",
			event:    call("https://auth.example.test", "GET", "/logo.png"),
			expected: false,
		},
		{
			name:     "host not in allowlist",
			rtype:    "Fetch",
			event:    call("https://analytics.thirdparty.test", "POST", "/collect"),
			expected: false,
		},
		{
			name:     "wildcard does not match bare base domain",
			rtype:    "Fetch",
			event:    call("https://int.example.test", "GET", "/"),
			expected: false,
		},
		{
			name:     "preflight OPTIONS dropped",
			rtype:    "Fetch",
			event:    call("https://auth.example.test", "OPTIONS", "/api/login"),
			expected: false,
		},
		{
			name:     "denied path by prefix",
			rtype:    "Document",
			event:    call("https://auth.example.test", "GET", "/healthz"),
			expected: false,
		},
		{
			name:     "denied exact path",
			rtype:    "Document",
			event:    call("https://auth.example.test", "GET", "/favicon.ico"),
			expected: false,
		},
	}

	f := New(cfg)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := f.Keep(tt.rtype, tt.event); got != tt.expected {
				t.Fatalf("Keep(%q, %s %s%s) = %v, want %v", tt.rtype, tt.event.Method, tt.event.Origin, tt.event.Path, got, tt.expected)
			}
		})
	}
}

func TestEmptyAllowHostsAllowsAll(t *testing.T) {
	f := New(config.Filter{ResourceTypes: []string{"Fetch"}})

	if !f.Keep("Fetch", call("https://anything.test", "GET", "/x")) {
		t.Fatal("empty AllowHosts should allow all hosts")
	}
}

func TestDropPreflightDisabledKeepsOptions(t *testing.T) {
	f := New(config.Filter{ResourceTypes: []string{"Fetch"}, DropPreflight: false})

	if !f.Keep("Fetch", call("https://anything.test", "OPTIONS", "/x")) {
		t.Fatal("OPTIONS should be kept when DropPreflight is false")
	}
}

func TestEmptyResourceTypesDropsEverything(t *testing.T) {
	f := New(config.Filter{})

	if f.Keep("Fetch", call("https://anything.test", "GET", "/x")) {
		t.Fatal("empty ResourceTypes should drop all calls")
	}
}
