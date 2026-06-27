// Package filter applies config-driven signal-quality rules to normalized
// timeline events before they are written. It is protocol-agnostic: it knows
// nothing about CDP and operates only on plain strings and timeline.Event.
package filter

import (
	"net/url"
	"strings"

	"github.com/amasotti/steno/internal/config"
	"github.com/amasotti/steno/internal/timeline"
)

// Filter decides whether a recorded call should be kept.
type Filter struct {
	resourceTypes map[string]struct{}
	allowHosts    []string
	denyPaths     []string
	dropPreflight bool
}

// New builds a Filter from the filter section of a config.
func New(cfg config.Filter) *Filter {
	resourceTypes := make(map[string]struct{}, len(cfg.ResourceTypes))
	for _, rt := range cfg.ResourceTypes {
		resourceTypes[rt] = struct{}{}
	}
	return &Filter{
		resourceTypes: resourceTypes,
		allowHosts:    cfg.AllowHosts,
		denyPaths:     cfg.DenyPaths,
		dropPreflight: cfg.DropPreflight,
	}
}

// Keep reports whether a call with the given CDP resource type (as a string)
// and normalized event survives the filters. Rules are applied in order and the
// first rejection wins:
//
//  1. resource-type allowlist
//  2. host allowlist (empty list allows all hosts)
//  3. preflight drop (method == OPTIONS)
//  4. path denylist (prefix match)
func (f *Filter) Keep(resourceType string, ev timeline.Event) bool {
	if _, ok := f.resourceTypes[resourceType]; !ok {
		return false
	}
	if !f.hostAllowed(ev.Origin) {
		return false
	}
	if f.dropPreflight && ev.Method == "OPTIONS" {
		return false
	}
	if f.pathDenied(ev.Path) {
		return false
	}
	return true
}

// hostAllowed matches the event origin against the allowlist. An empty
// allowlist allows every host. A pattern of the form "*.suffix" matches any
// host ending in ".suffix" but not the bare suffix itself.
func (f *Filter) hostAllowed(origin string) bool {
	if len(f.allowHosts) == 0 {
		return true
	}
	host := origin
	if parsed, err := url.Parse(origin); err == nil && parsed.Host != "" {
		host = parsed.Host
	}
	for _, pattern := range f.allowHosts {
		if suffix, ok := strings.CutPrefix(pattern, "*."); ok {
			if strings.HasSuffix(host, "."+suffix) {
				return true
			}
			continue
		}
		if host == pattern {
			return true
		}
	}
	return false
}

// pathDenied reports whether the path starts with any denied prefix.
func (f *Filter) pathDenied(path string) bool {
	for _, deny := range f.denyPaths {
		if strings.HasPrefix(path, deny) {
			return true
		}
	}
	return false
}
