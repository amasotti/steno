package config

import "testing"

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
}
