package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigHasUsefulCaptureDefaults(t *testing.T) {
	cfg := Default()

	if cfg.Target.Mode != ModeLaunch {
		t.Fatalf("Target.Mode = %q, want %q", cfg.Target.Mode, ModeLaunch)
	}
	if cfg.Filter.DropPreflight != true {
		t.Fatalf("Filter.DropPreflight = false, want true")
	}
	if got, want := cfg.Redact.MaxPayloadBytes, 8192; got != want {
		t.Fatalf("Redact.MaxPayloadBytes = %d, want %d", got, want)
	}
	if cfg.Payload.IncludeDocuments {
		t.Fatal("Payload.IncludeDocuments = true, want false by default")
	}
}

func TestLoadOverlaysOntoDefaults(t *testing.T) {
	toml := `
[target]
start_url = "https://app.example.test"

[filter]
allow_hosts = ["auth.example.test", "*.int.example.test"]
deny_paths = ["/health"]
`
	path := filepath.Join(t.TempDir(), "flow.toml")
	if err := os.WriteFile(path, []byte(toml), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	// Provided values are applied.
	if cfg.Target.StartURL != "https://app.example.test" {
		t.Fatalf("StartURL = %q, want https://app.example.test", cfg.Target.StartURL)
	}
	if len(cfg.Filter.AllowHosts) != 2 || cfg.Filter.AllowHosts[1] != "*.int.example.test" {
		t.Fatalf("AllowHosts = %v, want the two configured hosts", cfg.Filter.AllowHosts)
	}

	// Omitted values keep their defaults.
	if cfg.Target.Mode != ModeLaunch {
		t.Fatalf("Mode = %q, want default %q", cfg.Target.Mode, ModeLaunch)
	}
	if !cfg.Filter.DropPreflight {
		t.Fatal("DropPreflight should keep default true when omitted")
	}
	want := []string{"Document", "XHR", "Fetch", "WebSocket"}
	if len(cfg.Filter.ResourceTypes) != len(want) {
		t.Fatalf("ResourceTypes = %v, want default %v", cfg.Filter.ResourceTypes, want)
	}
}

func TestLoadMissingFileErrors(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.toml")); err == nil {
		t.Fatal("Load should error on a missing file")
	}
}
