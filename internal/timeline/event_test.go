package timeline

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEncodeJSONLWritesOneEventPerLine(t *testing.T) {
	var out strings.Builder
	event := Event{
		Seq:         42,
		Time:        time.Date(2026, 6, 27, 14, 2, 40, 118000000, time.UTC),
		Kind:        KindCall,
		Origin:      "https://account.int.example",
		Method:      "POST",
		Path:        "/api/v1/profile",
		Status:      201,
		RequestRef:  "payloads/0042-req.txt",
		ResponseRef: "payloads/0042-res.txt",
	}

	if err := WriteJSONL(&out, event); err != nil {
		t.Fatalf("WriteJSONL returned error: %v", err)
	}
	if !strings.HasSuffix(out.String(), "\n") {
		t.Fatalf("JSONL output does not end with newline: %q", out.String())
	}

	var decoded Event
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &decoded); err != nil {
		t.Fatalf("JSONL line is not valid Event JSON: %v", err)
	}
	if decoded.Seq != event.Seq || decoded.Kind != event.Kind || decoded.Path != event.Path {
		t.Fatalf("decoded event = %+v, want matching %+v", decoded, event)
	}
	if !strings.Contains(out.String(), `"request_ref":"payloads/0042-req.txt"`) {
		t.Fatalf("JSONL output = %q, want request_ref", out.String())
	}
	if !strings.Contains(out.String(), `"response_ref":"payloads/0042-res.txt"`) {
		t.Fatalf("JSONL output = %q, want response_ref", out.String())
	}
}

func TestReadJSONLReadsEvents(t *testing.T) {
	input := strings.NewReader(`{"seq":1,"kind":"call","origin":"https://example.test","method":"GET","path":"/api","status":200}` + "\n")

	events, err := ReadJSONL(input)

	if err != nil {
		t.Fatalf("ReadJSONL returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Kind != KindCall || events[0].Path != "/api" {
		t.Fatalf("event = %+v, want call /api", events[0])
	}
}
