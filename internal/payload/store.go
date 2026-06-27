package payload

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Kind string

const (
	KindRequest         Kind = "req"
	KindResponse        Kind = "res"
	KindRequestHeaders  Kind = "req-headers"
	KindResponseHeaders Kind = "res-headers"
)

type Store struct {
	root string
}

func NewStore(root string) *Store {
	return &Store{root: root}
}

func (s *Store) Root() string {
	return s.root
}

func (s *Store) Save(seq int, kind Kind, body []byte) (string, error) {
	if len(body) == 0 {
		return "", nil
	}
	return s.write(seq, kind, "txt", body)
}

// SaveHeaders writes a raw header map as a JSON file. Cookie and Set-Cookie
// travel inside the header map, so no separate cookie store is needed.
func (s *Store) SaveHeaders(seq int, kind Kind, headers map[string]string) (string, error) {
	if len(headers) == 0 {
		return "", nil
	}
	body, err := json.MarshalIndent(headers, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal headers: %w", err)
	}
	return s.write(seq, kind, "json", body)
}

func (s *Store) write(seq int, kind Kind, ext string, body []byte) (string, error) {
	rel := filepath.Join("payloads", fmt.Sprintf("%04d-%s.%s", seq, kind, ext))
	path := filepath.Join(s.root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", fmt.Errorf("create payload directory: %w", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return "", fmt.Errorf("write payload: %w", err)
	}
	return filepath.ToSlash(rel), nil
}
