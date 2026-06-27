package cli

import "github.com/fatih/color"

var (
	// styleHeader marks top-level section titles ("Session summary").
	styleHeader = color.New(color.FgCyan, color.Bold).SprintFunc()
	// styleLabel marks field labels ("Calls:", "Timeline:").
	styleLabel = color.New(color.Bold).SprintFunc()
	// stylePath dims structural detail like origins and file paths.
	stylePath = color.New(color.FgHiBlack).SprintFunc()

	style2xx = color.New(color.FgGreen).SprintFunc()
	style3xx = color.New(color.FgCyan).SprintFunc()
	style4xx = color.New(color.FgYellow).SprintFunc()
	style5xx = color.New(color.FgRed).SprintFunc()
)

// styleStatus colors an HTTP status code by class. The argument is the
// already-formatted text so callers control width/padding.
func styleStatus(code int, text string) string {
	switch {
	case code >= 200 && code < 300:
		return style2xx(text)
	case code >= 300 && code < 400:
		return style3xx(text)
	case code >= 400 && code < 500:
		return style4xx(text)
	case code >= 500:
		return style5xx(text)
	default:
		return text
	}
}
