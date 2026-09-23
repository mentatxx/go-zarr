package store

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"iter"
	"path"
	"strings"
	"sync"
)

func zipEntryName(keys []string) string {
	return strings.Join(JoinKeys(keys), "/")
}

func normalizeZipName(name string) string {
	name = strings.TrimPrefix(name, "/")
	return strings.TrimSuffix(name, "/")
}

// ReadOnlyZip provides read-only access to a zip archive stored in an underlying handle.
type ReadOnlyZip struct {
	Underlying Handle
	mu         sync.Mutex
	files      map[string]*zip.File
	data       []byte
}

// NewReadOnlyZip creates a read-only zip store.
func NewReadOnlyZip(underlying Handle) *ReadOnlyZip {
	return &ReadOnlyZip{Underlying: underlying}
}

func (s *ReadOnlyZip) load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.files != nil {
		return nil
	}
	data, err := s.Underlying.Read(ctx)
	if err != nil {
		return err
	}
	if data == nil {
		return fmt.Errorf("store: zip archive not found")
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	s.data = data
	s.files = make(map[string]*zip.File, len(r.File))
	for _, f := range r.File {
		s.files[normalizeZipName(f.Name)] = f
	}
	return nil
}

func (s *ReadOnlyZip) Exists(ctx context.Context, keys []string) (bool, error) {
	if err := s.load(ctx); err != nil {
		return false, err
	}
	_, ok := s.files[zipEntryName(keys)]
	return ok, nil
}

func (s *ReadOnlyZip) Get(ctx context.Context, keys []string, start, end int64) ([]byte, error) {
	if err := s.load(ctx); err != nil {
		return nil, err
	}
	f, ok := s.files[zipEntryName(keys)]
	if !ok {
		return nil, nil
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	all, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	if end < 0 {
		end = int64(len(all))
	}
	if start < 0 {
		start = int64(len(all)) + start
	}
	if start < 0 {
		start = 0
	}
	if end > int64(len(all)) {
		end = int64(len(all))
	}
	if start > end {
		return []byte{}, nil
	}
	out := make([]byte, end-start)
	copy(out, all[start:end])
	return out, nil
}

func (s *ReadOnlyZip) Set(ctx context.Context, keys []string, data []byte) error {
	return fmt.Errorf("store: zip store is read-only")
}

func (s *ReadOnlyZip) Delete(ctx context.Context, keys []string) error {
	return fmt.Errorf("store: zip store is read-only")
}

func (s *ReadOnlyZip) Size(ctx context.Context, keys []string) (int64, error) {
	if err := s.load(ctx); err != nil {
		return 0, err
	}
	f, ok := s.files[zipEntryName(keys)]
	if !ok {
		return -1, nil
	}
	return int64(f.UncompressedSize64), nil
}

func (s *ReadOnlyZip) List(ctx context.Context, prefix []string) iter.Seq2[[]string, error] {
	return func(yield func([]string, error) bool) {
		if err := s.load(ctx); err != nil {
			yield(nil, err)
			return
		}
		pref := zipEntryName(prefix)
		for name := range s.files {
			if pref != "" && name != pref && !strings.HasPrefix(name, pref+"/") {
				continue
			}
			rel := name
			if pref != "" {
				if name == pref {
					rel = ""
				} else {
					rel = strings.TrimPrefix(name, pref+"/")
				}
			}
			if rel == "" {
				continue
			}
			if !yield(strings.Split(rel, "/"), nil) {
				return
			}
		}
	}
}

func (s *ReadOnlyZip) ListChildren(ctx context.Context, prefix []string) iter.Seq2[[]string, error] {
	return func(yield func([]string, error) bool) {
		if err := s.load(ctx); err != nil {
			yield(nil, err)
			return
		}
		pref := zipEntryName(prefix)
		seen := map[string]struct{}{}
		for name := range s.files {
			rel := name
			if pref != "" {
				if name == pref || !strings.HasPrefix(name, pref+"/") {
					continue
				}
				rel = strings.TrimPrefix(name, pref+"/")
			}
			child, _, _ := strings.Cut(rel, "/")
			if _, ok := seen[child]; ok {
				continue
			}
			seen[child] = struct{}{}
			if !yield([]string{child}, nil) {
				return
			}
		}
	}
}

func (s *ReadOnlyZip) String() string {
	return fmt.Sprintf("zip://%s", s.Underlying)
}

// Resolve returns a handle at keys.
func (s *ReadOnlyZip) Resolve(keys ...string) Handle {
	return NewHandle(s, keys...)
}

// BufferedZip buffers a zip archive in memory and can flush it back.
type BufferedZip struct {
	Underlying Handle
	buf        *Memory
	comment    string
}

// NewBufferedZip loads an existing zip (if present) into a memory buffer.
func NewBufferedZip(ctx context.Context, underlying Handle) (*BufferedZip, error) {
	z := &BufferedZip{Underlying: underlying, buf: NewMemory()}
	data, err := underlying.Read(ctx)
	if err != nil {
		return nil, err
	}
	if data == nil || len(data) == 0 {
		return z, nil
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	z.comment = r.Comment
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "/") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		keys := strings.Split(normalizeZipName(f.Name), "/")
		if err := z.buf.Set(ctx, keys, b); err != nil {
			return nil, err
		}
	}
	return z, nil
}

func (s *BufferedZip) Exists(ctx context.Context, keys []string) (bool, error) {
	return s.buf.Exists(ctx, keys)
}
func (s *BufferedZip) Get(ctx context.Context, keys []string, start, end int64) ([]byte, error) {
	return s.buf.Get(ctx, keys, start, end)
}
func (s *BufferedZip) Set(ctx context.Context, keys []string, data []byte) error {
	return s.buf.Set(ctx, keys, data)
}
func (s *BufferedZip) Delete(ctx context.Context, keys []string) error {
	return s.buf.Delete(ctx, keys)
}
func (s *BufferedZip) Size(ctx context.Context, keys []string) (int64, error) {
	return s.buf.Size(ctx, keys)
}
func (s *BufferedZip) List(ctx context.Context, prefix []string) iter.Seq2[[]string, error] {
	return s.buf.List(ctx, prefix)
}
func (s *BufferedZip) ListChildren(ctx context.Context, prefix []string) iter.Seq2[[]string, error] {
	return s.buf.ListChildren(ctx, prefix)
}

// Flush writes the buffer as a zip archive to the underlying store.
func (s *BufferedZip) Flush(ctx context.Context) error {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	if s.comment != "" {
		w.SetComment(s.comment)
	}
	var keys [][]string
	for k, err := range s.buf.List(ctx, nil) {
		if err != nil {
			return err
		}
		keys = append(keys, k)
	}
	sortZipKeys(keys)
	for _, k := range keys {
		data, err := s.buf.Get(ctx, k, 0, -1)
		if err != nil {
			return err
		}
		fw, err := w.Create(path.Join(k...))
		if err != nil {
			return err
		}
		if _, err := fw.Write(data); err != nil {
			return err
		}
	}
	if err := w.Close(); err != nil {
		return err
	}
	return s.Underlying.Set(ctx, buf.Bytes())
}

func (s *BufferedZip) Close() error {
	return s.Flush(context.Background())
}

func (s *BufferedZip) String() string {
	return fmt.Sprintf("buffered-zip://%s", s.Underlying)
}

func (s *BufferedZip) Resolve(keys ...string) Handle {
	return NewHandle(s, keys...)
}

func sortZipKeys(keys [][]string) {
	// zarr.json first (BFS by depth), then lexicographic
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if zipKeyLess(keys[j], keys[i]) {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
}

func zipKeyLess(a, b []string) bool {
	aZ := len(a) > 0 && a[len(a)-1] == "zarr.json"
	bZ := len(b) > 0 && b[len(b)-1] == "zarr.json"
	if aZ && !bZ {
		return true
	}
	if !aZ && bZ {
		return false
	}
	if aZ && bZ {
		if len(a) != len(b) {
			return len(a) < len(b)
		}
	}
	return strings.Join(a, "/") < strings.Join(b, "/")
}
