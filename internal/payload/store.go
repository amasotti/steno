package payload

import (
	"fmt"
	"os"
	"path/filepath"
)

type Kind string

const (
	KindRequest  Kind = "req"
	KindResponse Kind = "res"
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

	rel := filepath.Join("payloads", fmt.Sprintf("%04d-%s.txt", seq, kind))
	path := filepath.Join(s.root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", fmt.Errorf("create payload directory: %w", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return "", fmt.Errorf("write payload: %w", err)
	}
	return filepath.ToSlash(rel), nil
}
