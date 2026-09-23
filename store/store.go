package store

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"iter"
	"path"
	"strings"
)

// Store is a key/value backend for Zarr arrays and groups.
// Get returns (nil, nil) when the key does not exist.
type Store interface {
	Exists(ctx context.Context, keys []string) (bool, error)
	Get(ctx context.Context, keys []string, start, end int64) ([]byte, error)
	Set(ctx context.Context, keys []string, data []byte) error
	Delete(ctx context.Context, keys []string) error
	Size(ctx context.Context, keys []string) (int64, error)
}

// Listable is a Store that can enumerate keys.
type Listable interface {
	Store
	List(ctx context.Context, prefix []string) iter.Seq2[[]string, error]
	ListChildren(ctx context.Context, prefix []string) iter.Seq2[[]string, error]
}

// Handle is a location inside a Store.
type Handle struct {
	Store Store
	Keys  []string
}

// NewHandle constructs a handle.
func NewHandle(s Store, keys ...string) Handle {
	return Handle{Store: s, Keys: append([]string(nil), keys...)}
}

// Resolve appends sub-keys.
func (h Handle) Resolve(sub ...string) Handle {
	keys := make([]string, 0, len(h.Keys)+len(sub))
	keys = append(keys, h.Keys...)
	keys = append(keys, sub...)
	return Handle{Store: h.Store, Keys: keys}
}

// Exists reports whether the key is present.
func (h Handle) Exists(ctx context.Context) (bool, error) {
	return h.Store.Exists(ctx, h.Keys)
}

// Read reads the full object. Missing keys return (nil, nil).
func (h Handle) Read(ctx context.Context) ([]byte, error) {
	return h.Store.Get(ctx, h.Keys, 0, -1)
}

// ReadRange reads [start, end). end < 0 means to the end of the object.
func (h Handle) ReadRange(ctx context.Context, start, end int64) ([]byte, error) {
	return h.Store.Get(ctx, h.Keys, start, end)
}

// Set writes the object.
func (h Handle) Set(ctx context.Context, data []byte) error {
	return h.Store.Set(ctx, h.Keys, data)
}

// Delete removes the object.
func (h Handle) Delete(ctx context.Context) error {
	return h.Store.Delete(ctx, h.Keys)
}

// Size returns the object size, or -1 if missing.
func (h Handle) Size(ctx context.Context) (int64, error) {
	return h.Store.Size(ctx, h.Keys)
}

// OpenReader returns a reader for [start, end).
func (h Handle) OpenReader(ctx context.Context, start, end int64) (io.ReadCloser, error) {
	data, err := h.Store.Get(ctx, h.Keys, start, end)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

// ToPath returns the filesystem path when the store is a *Filesystem.
func (h Handle) ToPath() (string, error) {
	fs, ok := h.Store.(*Filesystem)
	if !ok {
		return "", fmt.Errorf("store: underlying store is not a filesystem")
	}
	return fs.resolveKeys(h.Keys)
}

func (h Handle) String() string {
	return path.Join(append([]string{fmt.Sprint(h.Store)}, h.Keys...)...)
}

// JoinKeys flattens nested path segments.
func JoinKeys(keys []string) []string {
	var out []string
	for _, k := range keys {
		if k == "" {
			continue
		}
		k = strings.TrimPrefix(k, "/")
		if k == "" {
			continue
		}
		out = append(out, strings.Split(k, "/")...)
	}
	return out
}

// KeyPath joins keys with "/".
func KeyPath(keys []string) string {
	return strings.Join(JoinKeys(keys), "/")
}
