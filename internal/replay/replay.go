// Package replay turns one recorded call event into a copy-paste-ready curl command.
package replay

import (
	"sort"
	"strings"

	"github.com/amasotti/steno/internal/timeline"
)

func Curl(ev timeline.Event, headers map[string]string, body []byte) (string, []string) {
	var warnings []string

	method := ev.Method
	if method == "" {
		method = "GET"
	}
	url := ev.Origin + ev.Path

	parts := []string{"curl -X " + method + " " + shellQuote(url)}

	names := make([]string, 0, len(headers))
	for name := range headers {
		if skipHeader(name) {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		parts = append(parts, "-H "+shellQuote(name+": "+headers[name]))
	}

	if len(body) > 0 {
		parts = append(parts, "--data-raw "+shellQuote(string(body)))
	}

	if len(names) == 0 {
		warnings = append(warnings, "no headers captured; re-record with [payload] include_headers = true for a runnable request (Cookie/Authorization are otherwise missing)")
	}

	return strings.Join(parts, " \\\n  "), warnings
}

// skipHeader drops headers that must not be re-sent verbatim: HTTP/2
// pseudo-headers (":authority", ":method", …) are not valid curl -H inputs, and
// content-length is recomputed by curl from the body.
func skipHeader(name string) bool {
	if strings.HasPrefix(name, ":") {
		return true
	}
	return strings.EqualFold(name, "content-length")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
