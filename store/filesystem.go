package store

import (
	"context"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"strings"
)

// Filesystem is a directory-backed listable store.
type Filesystem struct {
	Root string
}

// NewFilesystem creates a filesystem store rooted at path.
func NewFilesystem(root string) *Filesystem {
	return &Filesystem{Root: root}
}

func (s *Filesystem) resolveKeys(keys []string) (string, error) {
	p := s.Root
	for _, k := range JoinKeys(keys) {
		p = filepath.Join(p, k)
	}
	absRoot, err := filepath.Abs(s.Root)
	if err != nil {
		return "", err
	}
	absRoot = filepath.Clean(absRoot)
	absTarget, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	absTarget = filepath.Clean(absTarget)
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("store: key resolves outside of store root: %s", absTarget)
	}
	return absTarget, nil
}

func (s *Filesystem) Exists(ctx context.Context, keys []string) (bool, error) {
	p, err := s.resolveKeys(keys)
	if err != nil {
		return false, err
	}
	st, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return st.Mode().IsRegular(), nil
}

func (s *Filesystem) Get(ctx context.Context, keys []string, start, end int64) ([]byte, error) {
	p, err := s.resolveKeys(keys)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := st.Size()
	if start < 0 {
		start = size + start
	}
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = size
	}
	if start > size {
		return []byte{}, nil
	}
	if end > size {
		end = size
	}
	if start > end {
		return []byte{}, nil
	}
	buf := make([]byte, end-start)
	_, err = f.ReadAt(buf, start)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return buf, nil
}

func (s *Filesystem) Set(ctx context.Context, keys []string, data []byte) error {
	p, err := s.resolveKeys(keys)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

func (s *Filesystem) Delete(ctx context.Context, keys []string) error {
	p, err := s.resolveKeys(keys)
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *Filesystem) Size(ctx context.Context, keys []string) (int64, error) {
	p, err := s.resolveKeys(keys)
	if err != nil {
		return 0, err
	}
	st, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return -1, nil
		}
		return 0, err
	}
	return st.Size(), nil
}

func (s *Filesystem) List(ctx context.Context, prefix []string) iter.Seq2[[]string, error] {
	return func(yield func([]string, error) bool) {
		root, err := s.resolveKeys(prefix)
		if err != nil {
			yield(nil, err)
			return
		}
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				yield(nil, err)
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			parts := strings.Split(filepath.ToSlash(rel), "/")
			if !yield(parts, nil) {
				return fmt.Errorf("stop")
			}
			return nil
		})
	}
}

func (s *Filesystem) ListChildren(ctx context.Context, prefix []string) iter.Seq2[[]string, error] {
	return func(yield func([]string, error) bool) {
		root, err := s.resolveKeys(prefix)
		if err != nil {
			yield(nil, err)
			return
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				return
			}
			yield(nil, err)
			return
		}
		for _, e := range entries {
			if !yield([]string{e.Name()}, nil) {
				return
			}
		}
	}
}

func (s *Filesystem) String() string {
	p, _ := filepath.Abs(s.Root)
	return strings.TrimRight("file://"+filepath.ToSlash(p), "/")
}

// Resolve returns a handle at keys.
func (s *Filesystem) Resolve(keys ...string) Handle {
	return NewHandle(s, keys...)
}
