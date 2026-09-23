package v3_test

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/mentatxx/go-zarr/core"
	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
	v3 "github.com/mentatxx/go-zarr/v3"
	"github.com/stretchr/testify/require"
)

func testdata(dt ndarray.DType) *ndarray.Array {
	a := ndarray.New(dt, []int{16, 16, 16}, binary.LittleEndian)
	for i := 0; i < a.Len(); i++ {
		switch dt {
		case ndarray.Bool:
			a.SetBool(i, i%2 == 0)
		case ndarray.Float32, ndarray.Float64:
			a.SetFloat64(i, float64(i))
		case ndarray.Uint8, ndarray.Uint16, ndarray.Uint32, ndarray.Uint64:
			a.SetUint64(i, uint64(i))
		default:
			a.SetInt64(i, int64(i))
		}
	}
	return a
}

func assertTestdata(t *testing.T, a *ndarray.Array, dt ndarray.DType) {
	t.Helper()
	for i := 0; i < a.Len(); i++ {
		switch dt {
		case ndarray.Bool:
			require.Equal(t, i%2 == 0, a.GetBool(i))
		case ndarray.Float32:
			require.InDelta(t, float64(i), a.GetFloat64(i), 1e-5)
		case ndarray.Float64:
			require.InDelta(t, float64(i), a.GetFloat64(i), 1e-12)
		default:
			require.Equal(t, int64(i)%(1<<32), a.GetInt64(i)%(1<<32))
		}
	}
}

func TestCreateReadWrite(t *testing.T) {
	ctx := context.Background()
	h := store.NewMemory().Resolve("arr")
	meta, err := v3.NewMetadataBuilder().
		WithShape(16, 16, 16).
		WithDataType(ndarray.Int32).
		WithChunkShape(2, 4, 8).
		WithFillValue(0).
		WithAttributes(core.Attributes{"test_key": "test_value"}).
		WithCodecs(func(b *v3.CodecBuilder) *v3.CodecBuilder { return b.WithBlosc() }).
		Build()
	require.NoError(t, err)
	arr, err := v3.Create(ctx, h, meta, false)
	require.NoError(t, err)
	require.NoError(t, arr.Write(ctx, nil, testdata(ndarray.Int32)))
	got, err := arr.Read(ctx, nil, nil)
	require.NoError(t, err)
	require.Equal(t, []int{16, 16, 16}, got.Shape)
	assertTestdata(t, got, ndarray.Int32)
	opened, err := v3.Open(ctx, h)
	require.NoError(t, err)
	require.Equal(t, "test_value", opened.Metadata().Attributes["test_key"])
}

func TestLargerChunkThanArray(t *testing.T) {
	ctx := context.Background()
	h := store.NewMemory().Resolve("bigchunk")
	meta, err := v3.NewMetadataBuilder().
		WithShape(16, 16, 16).
		WithDataType(ndarray.Uint32).
		WithChunkShape(32, 32, 32).
		WithFillValue(0).
		Build()
	require.NoError(t, err)
	arr, err := v3.Create(ctx, h, meta, false)
	require.NoError(t, err)
	data := testdata(ndarray.Uint32)
	require.NoError(t, arr.Write(ctx, nil, data))
	got, err := arr.Read(ctx, nil, nil)
	require.NoError(t, err)
	require.Equal(t, data.Len(), got.Len())
}

func TestInvalidCodecConfig(t *testing.T) {
	ctx := context.Background()
	h := store.NewMemory().Resolve("bad")
	meta, err := v3.NewMetadataBuilder().
		WithShape(4, 4).
		WithDataType(ndarray.Uint32).
		WithChunkShape(2, 2).
		WithCodecs(func(b *v3.CodecBuilder) *v3.CodecBuilder {
			return b.WithBytes("little").WithBytes("little")
		}).
		Build()
	if err != nil {
		return
	}
	_, err = v3.Create(ctx, h, meta, false)
	require.Error(t, err)
}

func TestGzipZstdBytes(t *testing.T) {
	ctx := context.Background()
	cases := []func(*v3.CodecBuilder) *v3.CodecBuilder{
		func(b *v3.CodecBuilder) *v3.CodecBuilder { return b.WithGzip(5) },
		func(b *v3.CodecBuilder) *v3.CodecBuilder { return b.WithZstd(5, true) },
		func(b *v3.CodecBuilder) *v3.CodecBuilder { return b.WithBytes("LITTLE") },
		func(b *v3.CodecBuilder) *v3.CodecBuilder { return b.WithCrc32c() },
		func(b *v3.CodecBuilder) *v3.CodecBuilder { return b.WithTranspose([]int{1, 0, 2}) },
		func(b *v3.CodecBuilder) *v3.CodecBuilder {
			return b.WithSharding([]int{2, 2, 4}, func(n *v3.CodecBuilder) *v3.CodecBuilder { return n.WithBytes("LITTLE") }, "end")
		},
	}
	for i, fn := range cases {
		h := store.NewMemory().Resolve("c", string(rune('a'+i)))
		meta, err := v3.NewMetadataBuilder().
			WithShape(16, 16, 16).
			WithDataType(ndarray.Int32).
			WithChunkShape(2, 4, 8).
			WithFillValue(0).
			WithCodecs(fn).
			Build()
		require.NoError(t, err, i)
		arr, err := v3.Create(ctx, h, meta, false)
		require.NoError(t, err, i)
		require.NoError(t, arr.Write(ctx, nil, testdata(ndarray.Int32)), i)
		got, err := arr.Read(ctx, nil, nil)
		require.NoError(t, err, i)
		assertTestdata(t, got, ndarray.Int32)
	}
}

func TestGroup(t *testing.T) {
	ctx := context.Background()
	root := store.NewMemory().Resolve("g")
	g, err := v3.CreateGroup(ctx, root, v3.DefaultGroupMetadata())
	require.NoError(t, err)
	g, err = g.SetAttributes(ctx, core.Attributes{"attr": "value"})
	require.NoError(t, err)
	sub, err := g.CreateGroup(ctx, "group", nil)
	require.NoError(t, err)
	_, err = sub.CreateArray(ctx, "array", func(b *v3.MetadataBuilder) *v3.MetadataBuilder {
		return b.WithShape(16, 16, 16).WithDataType(ndarray.Int32).WithChunkShape(2, 4, 8)
	})
	require.NoError(t, err)
	opened, err := v3.OpenGroup(ctx, root)
	require.NoError(t, err)
	n, err := opened.Get(ctx, "group")
	require.NoError(t, err)
	sg := n.(*v3.Group)
	n, err = sg.Get(ctx, "array")
	require.NoError(t, err)
	require.IsType(t, &v3.Array{}, n)
}

func TestResize(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	h := store.NewFilesystem(dir).Resolve("a")
	meta, err := v3.NewMetadataBuilder().WithShape(16, 16).WithDataType(ndarray.Int32).WithChunkShape(8, 8).WithFillValue(0).Build()
	require.NoError(t, err)
	arr, err := v3.Create(ctx, h, meta, false)
	require.NoError(t, err)
	data := ndarray.New(ndarray.Int32, []int{16, 16}, binary.LittleEndian)
	for i := 0; i < data.Len(); i++ {
		data.SetInt64(i, int64(i))
	}
	require.NoError(t, arr.Write(ctx, nil, data))
	resized, err := arr.Resize(ctx, []int64{8, 8}, false)
	require.NoError(t, err)
	require.Equal(t, []int64{8, 8}, resized.Metadata().Shape)
	got, err := resized.Read(ctx, nil, nil)
	require.NoError(t, err)
	require.Equal(t, []int{8, 8}, got.Shape)
}

func TestShardingIndexLocationTestdata(t *testing.T) {
	ctx := context.Background()
	root := testdataRoot(t)
	if root == "" {
		t.Skip("testdata/sharding_index_location not present")
	}
	for _, loc := range []string{"end", "start"} {
		h := store.NewFilesystem(filepath.Join(root, "sharding_index_location", loc)).Resolve()
		arr, err := v3.Open(ctx, h)
		require.NoError(t, err, loc)
		got, err := arr.Read(ctx, nil, nil)
		require.NoError(t, err, loc)
		require.NotNil(t, got)
	}
}

func testdataRoot(t *testing.T) string {
	t.Helper()
	candidates := []string{"testdata", "../testdata"}
	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(c, "sharding_index_location")); err == nil && st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}
