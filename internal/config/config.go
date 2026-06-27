package config

type Mode string

const (
	ModeLaunch Mode = "launch"
	ModeAttach Mode = "attach"
)

type Config struct {
	Target       Target
	Filter       Filter
	Redact       Redact
	Correlation  Correlation
	Extract      []ExtractRule
	Supplemental []SupplementalSource
}

type Target struct {
	Mode       Mode
	Browser    string
	AttachURL  string
	ProfileDir string
	StartURL   string
}

type Filter struct {
	ResourceTypes []string
	AllowHosts    []string
	DenyPaths     []string
	DropPreflight bool
}

type Redact struct {
	MaskFields      []string
	MaxPayloadBytes int
}

type Correlation struct {
	Header string
}

type ExtractRule struct {
	WhenPath string
	From     string
	JSONPath string
	As       string
}

type SupplementalSource struct {
	Name       string
	URL        string
	ExtractOTP string
}

func Default() Config {
	return Config{
		Target: Target{
			Mode:    ModeLaunch,
			Browser: "chrome",
		},
		Filter: Filter{
			ResourceTypes: []string{"Document", "XHR", "Fetch", "WebSocket"},
			DropPreflight: true,
		},
		Redact: Redact{
			MaskFields:      []string{"password", "otp", "token"},
			MaxPayloadBytes: 8192,
		},
		Correlation: Correlation{
			Header: "traceparent",
		},
	}
}
