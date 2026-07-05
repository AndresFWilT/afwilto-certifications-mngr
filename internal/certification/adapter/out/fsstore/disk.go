// Package fsstore stores attachment bytes on the local filesystem (a named
// Docker volume in the platform), implementing the FileStorage out-port.
package fsstore

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type DiskFileStorage struct {
	dir string
}

func NewDiskFileStorage(dir string) *DiskFileStorage {
	return &DiskFileStorage{dir: dir}
}

func (s *DiskFileStorage) path(key string) (string, error) {
	clean := filepath.Clean(key)
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", fmt.Errorf("invalid storage key %q", key)
	}
	return filepath.Join(s.dir, clean), nil
}

// Put writes atomically: temp file + rename, so a replaced attachment is never
// observed half-written.
func (s *DiskFileStorage) Put(_ context.Context, key string, content io.Reader) error {
	target, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".upload-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := io.Copy(tmp, content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), target)
}

func (s *DiskFileStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	target, err := s.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(target)
}

func (s *DiskFileStorage) Delete(_ context.Context, key string) error {
	target, err := s.path(key)
	if err != nil {
		return err
	}
	err = os.Remove(target)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
