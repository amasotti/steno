package payload

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreWritesRequestAndResponsePayloads(t *testing.T) {
	store := NewStore(t.TempDir())

	reqRef, err := store.Save(42, KindRequest, []byte(`{"username":"admin","password":"admin"}`))
	if err != nil {
		t.Fatalf("Save request returned error: %v", err)
	}
	resRef, err := store.Save(42, KindResponse, []byte(`{"ok":true}`))
	if err != nil {
		t.Fatalf("Save response returned error: %v", err)
	}

	if reqRef != "payloads/0042-req.txt" {
		t.Fatalf("request ref = %q, want payloads/0042-req.txt", reqRef)
	}
	if resRef != "payloads/0042-res.txt" {
		t.Fatalf("response ref = %q, want payloads/0042-res.txt", resRef)
	}

	req, err := os.ReadFile(filepath.Join(store.Root(), reqRef))
	if err != nil {
		t.Fatalf("read request payload: %v", err)
	}
	if string(req) != `{"username":"admin","password":"admin"}` {
		t.Fatalf("request payload = %q", req)
	}

	res, err := os.ReadFile(filepath.Join(store.Root(), resRef))
	if err != nil {
		t.Fatalf("read response payload: %v", err)
	}
	if string(res) != `{"ok":true}` {
		t.Fatalf("response payload = %q", res)
	}
}

func TestStoreSkipsEmptyPayloads(t *testing.T) {
	store := NewStore(t.TempDir())

	ref, err := store.Save(1, KindRequest, nil)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if ref != "" {
		t.Fatalf("ref = %q, want empty ref", ref)
	}
}

func TestStoreWritesHeadersAsJSON(t *testing.T) {
	store := NewStore(t.TempDir())

	ref, err := store.SaveHeaders(7, KindRequestHeaders, map[string]string{
		"content-type": "application/json",
		"cookie":       "session=abc; csrf=xyz",
	})
	if err != nil {
		t.Fatalf("SaveHeaders returned error: %v", err)
	}
	if ref != "payloads/0007-req-headers.json" {
		t.Fatalf("ref = %q, want payloads/0007-req-headers.json", ref)
	}

	raw, err := os.ReadFile(filepath.Join(store.Root(), ref))
	if err != nil {
		t.Fatalf("read headers file: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("headers file is not valid JSON: %v", err)
	}
	if got["cookie"] != "session=abc; csrf=xyz" {
		t.Fatalf("cookie header = %q", got["cookie"])
	}
}

func TestStoreSkipsEmptyHeaders(t *testing.T) {
	store := NewStore(t.TempDir())

	ref, err := store.SaveHeaders(1, KindResponseHeaders, nil)
	if err != nil {
		t.Fatalf("SaveHeaders returned error: %v", err)
	}
	if ref != "" {
		t.Fatalf("ref = %q, want empty ref", ref)
	}
}
