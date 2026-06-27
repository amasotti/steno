package timeline

import (
	"encoding/json"
	"io"
	"time"
)

type Kind string

const (
	KindNavigation   Kind = "navigation"
	KindCall         Kind = "call"
	KindConsole      Kind = "console"
	KindFact         Kind = "fact"
	KindSupplemental Kind = "supplemental"
)

type Event struct {
	Seq                int             `json:"seq"`
	Time               time.Time       `json:"ts"`
	Kind               Kind            `json:"kind"`
	Origin             string          `json:"origin,omitempty"`
	Method             string          `json:"method,omitempty"`
	Path               string          `json:"path,omitempty"`
	Status             int             `json:"status,omitempty"`
	DurationMS         int             `json:"duration_ms,omitempty"`
	SizeBytes          int             `json:"size_bytes,omitempty"`
	Correlation        string          `json:"correlation,omitempty"`
	RequestPayload     json.RawMessage `json:"request_payload,omitempty"`
	ResponsePayload    json.RawMessage `json:"response_payload,omitempty"`
	RequestRef         string          `json:"request_ref,omitempty"`
	ResponseRef        string          `json:"response_ref,omitempty"`
	RequestHeadersRef  string          `json:"request_headers_ref,omitempty"`
	ResponseHeadersRef string          `json:"response_headers_ref,omitempty"`
}

func WriteJSONL(w io.Writer, event Event) error {
	encoded, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err = w.Write(encoded); err != nil {
		return err
	}
	_, err = w.Write([]byte("\n"))
	return err
}
