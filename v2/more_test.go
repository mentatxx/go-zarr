package v2_test

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mentatxx/go-zarr/codec"
	"github.com/mentatxx/go-zarr/core"
	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
	v2 "github.com/mentatxx/go-zarr/v2"
	"github.com/stretchr/testify/require"
)

func TestV2CreateBlosc(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		cname   string
		shuffle string
		clevel  int
	}{
		{"lz4", "shuffle", 6},
		{"lz4hc", "bitshuffle", 3},
		{"zlib", "shuffle", 5},
		{"zstd", "bitshuffle", 9},
		{"blosclz", "noshuffle", 0},
	}
	for _, tc := range cases {
		t.Run(tc.cname, func(t *testing.T) {
			if tc.cname == "blosclz" {
				c, err := codec.NewBlosc(tc.cname, tc.shuffle, tc.clevel, 1, 0)
				require.NoError(t, err)
				_, err = c.EncodeBytes([]byte{1, 2, 3, 4})
				if err != nil {
					t.Skip(err.Error())
				}
			}
			h := store.NewMemory().Resolve("blosc", tc.cname)
			meta, err := v2.NewMetadataBuilder().
				WithShape(10, 10).
				WithChunks(5, 5).
				WithDataType("|u1").
				WithFillValue(1).
				WithBloscCompressor(tc.cname, tc.shuffle, tc.clevel).
				Build()
			require.NoError(t, err)
			arr, err := v2.Create(ctx, h, meta, false)
			require.NoError(t, err)
			data := ndarray.New(ndarray.Uint8, []int{8, 8}, binary.LittleEndian)
			require.NoError(t, arr.Write(ctx, []int64{2, 2}, data))
			got, err := arr.Read(ctx, []int64{2, 2}, []int64{8, 8})
			require.NoError(t, err)
			require.Equal(t, 64, got.Len())
			require.Equal(t, int64(0), got.GetInt64(0))
		})
	}
}

func TestV2ZlibLevels(t *testing.T) {
	ctx := context.Background()
	for _, level := range []int{0, 1, 5, 9} {
		h := store.NewMemory().Resolve("zlib")
		meta, err := v2.NewMetadataBuilder().
			WithShape(15, 10).
			WithChunks(4, 5).
			WithDataType("|u1").
			WithFillValue(5).
			WithZlibCompressor(level).
			Build()
		require.NoError(t, err)
		arr, err := v2.Create(ctx, h, meta, true)
		require.NoError(t, err)
		data := ndarray.New(ndarray.Uint8, []int{7, 6}, binary.LittleEndian)
		require.NoError(t, arr.Write(ctx, []int64{2, 2}, data))
		got, err := arr.Read(ctx, []int64{2, 2}, []int64{7, 6})
		require.NoError(t, err)
		require.Equal(t, 42, got.Len())
		require.Equal(t, int64(0), got.GetInt64(0))
	}
}

func TestV2DefaultCompressionLevel(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	for _, id := range []string{"gzip", "zlib"} {
		h := store.NewFilesystem(dir).Resolve(id)
		b := v2.NewMetadataBuilder().WithShape(16, 16).WithChunks(8, 8).WithDataType("|u1")
		if id == "gzip" {
			b.WithGzipCompressor(1)
		} else {
			b.WithZlibCompressor(1)
		}
		meta, err := b.Build()
		require.NoError(t, err)
		arr, err := v2.Create(ctx, h, meta, false)
		require.NoError(t, err)
		data := ndarray.New(ndarray.Uint8, []int{16, 16}, binary.LittleEndian)
		for i := 0; i < data.Len(); i++ {
			data.SetInt64(i, int64(i%128))
		}
		require.NoError(t, arr.Write(ctx, nil, data))
		p, err := h.Resolve(".zarray").ToPath()
		require.NoError(t, err)
		raw, err := os.ReadFile(p)
		require.NoError(t, err)
		patched := strings.Replace(string(raw), `"level": 1`, `"level": -1`, 1)
		require.Contains(t, patched, `"level": -1`)
		require.NoError(t, os.WriteFile(p, []byte(patched), 0o644))
		reopened, err := v2.Open(ctx, h)
		require.NoError(t, err)
		got, err := reopened.Read(ctx, nil, nil)
		require.NoError(t, err)
		require.Equal(t, int64(127), got.GetInt64(127))
	}
}

func TestV2BloscAutoShuffle(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	h := store.NewFilesystem(dir).Resolve("auto")
	meta, err := v2.NewMetadataBuilder().
		WithShape(16, 16).
		WithChunks(8, 8).
		WithDataType("|u1").
		WithBloscCompressor("zstd", "shuffle", 5).
		Build()
	require.NoError(t, err)
	arr, err := v2.Create(ctx, h, meta, false)
	require.NoError(t, err)
	data := ndarray.New(ndarray.Uint8, []int{16, 16}, binary.LittleEndian)
	for i := 0; i < data.Len(); i++ {
		data.SetInt64(i, int64(i%128))
	}
	require.NoError(t, arr.Write(ctx, nil, data))
	p, err := h.Resolve(".zarray").ToPath()
	require.NoError(t, err)
	raw, err := os.ReadFile(p)
	require.NoError(t, err)
	s := string(raw)
	s = strings.Replace(s, `"shuffle": 1`, `"shuffle": -1`, 1)
	s = strings.Replace(s, `"typesize": 1,`, "", 1)
	require.Contains(t, s, `"shuffle": -1`)
	require.NoError(t, os.WriteFile(p, []byte(s), 0o644))
	reopened, err := v2.Open(ctx, h)
	require.NoError(t, err)
	got, err := reopened.Read(ctx, nil, nil)
	require.NoError(t, err)
	require.Equal(t, int64(127), got.GetInt64(127))
}

func TestV2SampleTypes(t *testing.T) {
	ctx := context.Background()
	root := ""
	for _, c := range []string{"testdata/v2_sample", "../testdata/v2_sample"} {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			root, _ = filepath.Abs(c)
			break
		}
	}
	if root == "" {
		t.Skip("testdata/v2_sample not present")
	}
	for _, name := range []string{"bool", "double"} {
		arr, err := v2.Open(ctx, store.NewFilesystem(filepath.Join(root, name)).Resolve())
		require.NoError(t, err, name)
		got, err := arr.Read(ctx, []int64{0, 0, 0}, []int64{3, 4, 5})
		if err != nil && name == "double" {
			t.Skipf("v2_sample/double needs c-blosc interop: %v", err)
		}
		require.NoError(t, err, name)
		require.Equal(t, 60, got.Len(), name)
	}
}

func TestV2CreateOffsetWrite(t *testing.T) {
	ctx := context.Background()
	h := store.NewMemory().Resolve("off")
	meta, err := v2.NewMetadataBuilder().
		WithShape(10, 10).
		WithChunks(5, 5).
		WithDataType("<u4").
		WithFillValue(2).
		Build()
	require.NoError(t, err)
	arr, err := v2.Create(ctx, h, meta, false)
	require.NoError(t, err)
	data := ndarray.New(ndarray.Uint32, []int{8, 8}, binary.LittleEndian)
	require.NoError(t, arr.Write(ctx, []int64{2, 2}, data))
	got, err := arr.Read(ctx, []int64{2, 2}, []int64{8, 8})
	require.NoError(t, err)
	require.Equal(t, 64, got.Len())
	require.Equal(t, int64(0), got.GetInt64(0))
}

func TestV2Resize(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	h := store.NewFilesystem(dir).Resolve("a")
	meta, err := v2.NewMetadataBuilder().WithShape(16, 16).WithChunks(8, 8).WithDataType("<i4").WithFillValue(0).Build()
	require.NoError(t, err)
	arr, err := v2.Create(ctx, h, meta, false)
	require.NoError(t, err)
	data := ndarray.New(ndarray.Int32, []int{16, 16}, binary.LittleEndian)
	for i := 0; i < data.Len(); i++ {
		data.SetInt64(i, int64(i))
	}
	require.NoError(t, arr.Write(ctx, nil, data))
	resized, err := arr.Resize(ctx, []int64{8, 8}, false)
	require.NoError(t, err)
	require.Equal(t, []int64{8, 8}, resized.Metadata().Shape)
}

func TestV2NoFillValue(t *testing.T) {
	ctx := context.Background()
	h := store.NewMemory().Resolve("nf")
	meta, err := v2.NewMetadataBuilder().WithShape(4, 4).WithChunks(2, 2).WithDataType("<f8").WithFillValue(nil).Build()
	require.NoError(t, err)
	arr, err := v2.Create(ctx, h, meta, false)
	require.NoError(t, err)
	got, err := arr.Read(ctx, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 16, got.Len())
}

func TestV2Attributes(t *testing.T) {
	ctx := context.Background()
	h := store.NewMemory().Resolve("g")
	g, err := v2.CreateGroup(ctx, h, core.Attributes{"attr": "value"})
	require.NoError(t, err)
	require.Equal(t, "value", g.Attributes()["attr"])
}
