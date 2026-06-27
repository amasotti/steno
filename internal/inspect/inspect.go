package inspect

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/amasotti/steno/internal/timeline"
)

func ReadTimeline(r io.Reader) ([]timeline.Event, error) {
	events, err := timeline.ReadJSONL(r)
	if err != nil {
		return nil, fmt.Errorf("read timeline: %w", err)
	}
	return events, nil
}

// statusToken matches a status class ("4xx") or an exact code ("404").
var statusToken = regexp.MustCompile(`^([1-5]xx|[1-5][0-9]{2})$`)

// Filter narrows a timeline for inspection. A zero Filter keeps everything.
type Filter struct {
	Origin   string   // case-insensitive substring of Origin; "" matches any
	Statuses []string // status tokens ("4xx", "404"); nil matches any
}

// Apply returns the events that satisfy every set condition (AND). A status
// filter implies "call" events only, since navigation events have no status.
func (f Filter) Apply(events []timeline.Event) []timeline.Event {
	var out []timeline.Event
	origin := strings.ToLower(f.Origin)
	for _, e := range events {
		if origin != "" && !strings.Contains(strings.ToLower(e.Origin), origin) {
			continue
		}
		if len(f.Statuses) > 0 {
			if e.Kind != timeline.KindCall || !matchStatus(e.Status, f.Statuses) {
				continue
			}
		}
		out = append(out, e)
	}
	return out
}

func matchStatus(status int, tokens []string) bool {
	for _, tok := range tokens {
		if strings.HasSuffix(tok, "xx") {
			if int(tok[0]-'0') == status/100 {
				return true
			}
			continue
		}
		if fmt.Sprintf("%d", status) == tok {
			return true
		}
	}
	return false
}

// ParseStatuses splits a comma-separated --status value ("4xx,5xx,404") into
// validated tokens, rejecting anything that is neither a class nor a 3-digit code.
func ParseStatuses(raw string) ([]string, error) {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		tok := strings.TrimSpace(part)
		if tok == "" {
			continue
		}
		if !statusToken.MatchString(tok) {
			return nil, fmt.Errorf("invalid status %q (want a class like 4xx or a code like 404)", tok)
		}
		out = append(out, tok)
	}
	return out, nil
}

// Find returns the event with the given sequence id.
func Find(events []timeline.Event, seq int) (timeline.Event, bool) {
	for _, e := range events {
		if e.Seq == seq {
			return e, true
		}
	}
	return timeline.Event{}, false
}

// ReadArtifact reads a payload or header side-file referenced from the timeline
// (e.g. "payloads/0005-res.txt"), resolved relative to the run directory.
//
// The ref comes from timeline.jsonl, which may be attacker-supplied (recordings
// are a sharing format). A ref like "../secret" must not escape the run dir, so
// the resolved path is confined to runDir before any read.
func ReadArtifact(runDir, ref string) ([]byte, error) {
	resolved := filepath.Join(runDir, filepath.FromSlash(ref))
	rel, err := filepath.Rel(runDir, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("artifact %q escapes the run directory", ref)
	}
	body, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("read artifact %q: %w", ref, err)
	}
	return body, nil
}
