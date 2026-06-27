package config

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type Mode string

const (
	ModeLaunch Mode = "launch"
	ModeAttach Mode = "attach"
)

type Config struct {
	Target       Target               `toml:"target"`
	Filter       Filter               `toml:"filter"`
	Redact       Redact               `toml:"redact"`
	Correlation  Correlation          `toml:"correlation"`
	Extract      []ExtractRule        `toml:"extract"`
	Supplemental []SupplementalSource `toml:"supplemental"`
}

type Target struct {
	Mode       Mode   `toml:"mode"`
	Browser    string `toml:"browser"`
	AttachURL  string `toml:"attach_url"`
	ProfileDir string `toml:"profile_dir"`
	StartURL   string `toml:"start_url"`
}

type Filter struct {
	ResourceTypes []string `toml:"resource_types"`
	AllowHosts    []string `toml:"allow_hosts"`
	DenyPaths     []string `toml:"deny_paths"`
	DropPreflight bool     `toml:"drop_preflight"`
}

type Redact struct {
	MaskFields      []string `toml:"mask_fields"`
	MaxPayloadBytes int      `toml:"max_payload_bytes"`
}

type Correlation struct {
	Header string `toml:"header"`
}

type ExtractRule struct {
	WhenPath string `toml:"when_path"`
	From     string `toml:"from"`
	JSONPath string `toml:"json_path"`
	As       string `toml:"as"`
}

type SupplementalSource struct {
	Name       string `toml:"name"`
	URL        string `toml:"url"`
	ExtractOTP string `toml:"extract_otp"`
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

// Load reads a TOML config file, overlaying it on top of Default. Fields the
// file omits keep their default; an explicit empty array clears a default.
// Unknown keys are ignored (lenient) so partial configs stay forward-compatible.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	cfg := Default()
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	return cfg, nil
}
