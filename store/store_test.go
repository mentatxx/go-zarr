package store_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mentatxx/go-zarr/store"
	"github.com/stretchr/testify/require"
)

func testData() []byte {
	b := make([]byte, 1024)
	for i := range b {
		b[i] = byte(i % 256)
	}
	return b
}

func testStoreBasics(t *testing.T, s store.Store, writable bool) {
	t.Helper()
	ctx := context.Background()
	data := testData()
	h := store.NewHandle(s, "chunk.bin")
	if writable {
		require.NoError(t, h.Set(ctx, data))
	}
	ok, err := h.Exists(ctx)
	require.NoError(t, err)
	require.True(t, ok)
	got, err := h.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, data, got)
	sz, err := h.Size(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(len(data)), sz)
	part, err := h.ReadRange(ctx, 5, 15)
	require.NoError(t, err)
	require.Equal(t, data[5:15], part)
	fromEnd, err := h.ReadRange(ctx, -10, -1)
	require.NoError(t, err)
	require.Equal(t, data[len(data)-10:], fromEnd)
	rc, err := h.OpenReader(ctx, 10, 20)
	require.NoError(t, err)
	require.NotNil(t, rc)
	buf, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, data[10:20], buf)
	_ = rc.Close()
	missing := store.NewHandle(s, "no-such-key")
	ok, err = missing.Exists(ctx)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestMemoryStore(t *testing.T) {
	s := store.NewMemory()
	testStoreBasics(t, s, true)
	ctx := context.Background()
	require.NoError(t, s.Set(ctx, []string{"a", "b"}, []byte("x")))
	require.NoError(t, s.Set(ctx, []string{"a", "c"}, []byte("y")))
	var children []string
	for k, err := range s.ListChildren(ctx, []string{"a"}) {
		require.NoError(t, err)
		children = append(children, k[0])
	}
	require.ElementsMatch(t, []string{"b", "c"}, children)
}

func TestFilesystemStore(t *testing.T) {
	dir := t.TempDir()
	s := store.NewFilesystem(dir)
	testStoreBasics(t, s, true)
	ctx := context.Background()
	require.NoError(t, s.Set(ctx, []string{"g", "zarr.json"}, []byte("{}")))
	p, err := store.NewHandle(s, "g", "zarr.json").ToPath()
	require.NoError(t, err)
	require.FileExists(t, p)
}

func TestFilesystemTraversal(t *testing.T) {
	dir := t.TempDir()
	s := store.NewFilesystem(dir)
	ctx := context.Background()
	err := s.Set(ctx, []string{"..", "evil"}, []byte("no"))
	require.Error(t, err)
}

func TestHTTPStore(t *testing.T) {
	data := testData()
	mux := http.NewServeMux()
	mux.HandleFunc("/chunk.bin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", "1024")
			return
		}
		rng := r.Header.Get("Range")
		if rng == "" {
			w.Write(data)
			return
		}
		if n, _ := parseBytesRange(rng, len(data)); n != nil {
			w.Write(data[n[0]:n[1]])
			return
		}
		w.Write(data)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	s := store.NewHTTP(srv.URL)
	ctx := context.Background()
	got, err := s.Get(ctx, []string{"chunk.bin"}, 0, -1)
	require.NoError(t, err)
	require.Equal(t, data, got)
	ok, err := s.Exists(ctx, []string{"chunk.bin"})
	require.NoError(t, err)
	require.True(t, ok)
}

func parseBytesRange(h string, n int) ([]int, error) {
	if h == "bytes=5-14" {
		return []int{5, 15}, nil
	}
	return nil, nil
}

func TestZipStores(t *testing.T) {
	ctx := context.Background()
	mem := store.NewMemory()
	under := mem.Resolve("archive.zip")
	bz, err := store.NewBufferedZip(ctx, under)
	require.NoError(t, err)
	require.NoError(t, bz.Set(ctx, []string{"a", "b.txt"}, []byte("hello")))
	require.NoError(t, bz.Flush(ctx))
	ro := store.NewReadOnlyZip(under)
	got, err := ro.Get(ctx, []string{"a", "b.txt"}, 0, -1)
	require.NoError(t, err)
	require.Equal(t, []byte("hello"), got)
}

func TestFilesystemList(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x"), []byte("1"), 0o644))
	s := store.NewFilesystem(dir)
	ctx := context.Background()
	var keys [][]string
	for k, err := range s.List(ctx, nil) {
		require.NoError(t, err)
		keys = append(keys, k)
	}
	require.Equal(t, [][]string{{"x"}}, keys)
}

func TestHandleOpenEndedReader(t *testing.T) {
	s := store.NewMemory()
	ctx := context.Background()
	h := s.Resolve("k")
	require.NoError(t, h.Set(ctx, []byte("abcdef")))
	rc, err := h.OpenReader(ctx, 0, -1)
	require.NoError(t, err)
	b, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.True(t, bytes.Equal(b, []byte("abcdef")))
}
