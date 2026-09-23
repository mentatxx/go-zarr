package v3_test

import (
	"context"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/mentatxx/go-zarr/codec"
	"github.com/mentatxx/go-zarr/core"
	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
	v3 "github.com/mentatxx/go-zarr/v3"
	"github.com/stretchr/testify/require"
)

func TestInvalidCodecConfigurations(t *testing.T) {
	ctx := context.Background()
	builders := []func(*v3.CodecBuilder) *v3.CodecBuilder{
		func(b *v3.CodecBuilder) *v3.CodecBuilder { return b.WithBytes("little").WithBytes("little") },
		func(b *v3.CodecBuilder) *v3.CodecBuilder { return b.WithBlosc().WithBytes("little") },
		func(b *v3.CodecBuilder) *v3.CodecBuilder { return b.WithBytes("little").WithTranspose([]int{1, 0}) },
		func(b *v3.CodecBuilder) *v3.CodecBuilder {
			return b.WithTranspose([]int{1, 0}).WithBytes("little").WithTranspose([]int{1, 0})
		},
	}
	for i, fn := range builders {
		_, err := v3.NewMetadataBuilder().
			WithShape(4, 4).
			WithDataType(ndarray.Uint32).
			WithChunkShape(2, 2).
			WithCodecs(fn).
			Build()
		require.Error(t, err, i)
		_ = ctx
	}
}

func TestInvalidChunkDimensions(t *testing.T) {
	for _, cs := range [][]int{{1}, {1, 1, 1}} {
		_, err := v3.NewMetadataBuilder().
			WithShape(4, 4).
			WithDataType(ndarray.Uint32).
			WithChunkShape(cs...).
			Build()
		require.Error(t, err, cs)
	}
}

func TestInvalidShardingBounds(t *testing.T) {
	inner := []int{2, 2}
	for _, shard := range [][]int{{4}, {4, 4, 4}, {1, 1}, {5, 5}, {2, 1}, {2, 5}} {
		_, err := v3.NewMetadataBuilder().
			WithShape(10, 10).
			WithDataType(ndarray.Uint32).
			WithChunkShape(shard...).
			WithCodecs(func(b *v3.CodecBuilder) *v3.CodecBuilder {
				return b.WithSharding(inner, func(n *v3.CodecBuilder) *v3.CodecBuilder { return n.WithBytes("LITTLE") })
			}).
			Build()
		require.Error(t, err, shard)
	}
}

func TestZstdCodecReadWrite(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		level    int
		checksum bool
	}{{0, true}, {0, false}, {5, true}, {5, false}} {
		h := store.NewMemory().Resolve("zstd", string(rune('a'+tc.level)))
		meta, err := v3.NewMetadataBuilder().
			WithShape(16, 16, 16).
			WithDataType(ndarray.Uint32).
			WithChunkShape(2, 4, 8).
			WithFillValue(0).
			WithCodecs(func(b *v3.CodecBuilder) *v3.CodecBuilder { return b.WithZstd(tc.level, tc.checksum) }).
			Build()
		require.NoError(t, err)
		arr, err := v3.Create(ctx, h, meta, true)
		require.NoError(t, err)
		data := testdata(ndarray.Uint32)
		require.NoError(t, arr.Write(ctx, nil, data))
		got, err := arr.Read(ctx, nil, nil)
		require.NoError(t, err)
		require.Equal(t, data.Len(), got.Len())
		require.Equal(t, int64(15), got.GetInt64(15))
	}
}

func TestShardingWithZstd(t *testing.T) {
	ctx := context.Background()
	h := store.NewMemory().Resolve("sz")
	meta, err := v3.NewMetadataBuilder().
		WithShape(16, 16, 16).
		WithDataType(ndarray.Uint32).
		WithChunkShape(8, 8, 8).
		WithFillValue(0).
		WithCodecs(func(b *v3.CodecBuilder) *v3.CodecBuilder {
			return b.WithSharding([]int{2, 4, 8}, func(n *v3.CodecBuilder) *v3.CodecBuilder { return n.WithZstd() })
		}).
		Build()
	require.NoError(t, err)
	arr, err := v3.Create(ctx, h, meta, false)
	require.NoError(t, err)
	require.NoError(t, arr.Write(ctx, nil, testdata(ndarray.Uint32)))
	got, err := arr.Read(ctx, nil, nil)
	require.NoError(t, err)
	assertTestdata(t, got, ndarray.Uint32)
}

func TestTransposeCodecRoundtrip(t *testing.T) {
	src := ndarray.New(ndarray.Uint32, []int{2, 3, 3}, binary.LittleEndian)
	for i := 0; i < src.Len(); i++ {
		src.SetInt64(i, int64(i))
	}
	c := v3.NewTranspose([]int{1, 2, 0})
	require.NoError(t, c.SetMeta(codecMeta(src)))
	enc, err := c.EncodeArray(src)
	require.NoError(t, err)
	require.Equal(t, []int{3, 3, 2}, enc.Shape)
	dec, err := c.DecodeArray(enc)
	require.NoError(t, err)
	require.True(t, src.Equal(dec))
}

func TestInvalidTransposeOrder(t *testing.T) {
	src := ndarray.New(ndarray.Uint32, []int{2, 3, 3}, binary.LittleEndian)
	for _, order := range [][]int{{1, 0, 0}, {1, 2, 3}, {1, 2, 3, 0}, {1, 2}} {
		c := v3.NewTranspose(order)
		err := c.SetMeta(codecMeta(src))
		require.Error(t, err, order)
	}
}

func TestFillValueParsing(t *testing.T) {
	v, err := core.ParseFillValue(0, ndarray.Uint32)
	require.NoError(t, err)
	require.Equal(t, uint32(0), v)
	v, err = core.ParseFillValue("0x00010203", ndarray.Uint32)
	require.NoError(t, err)
	require.Equal(t, uint32(0x03020100), v)
	v, err = core.ParseFillValue("0b00000010", ndarray.Uint8)
	require.NoError(t, err)
	require.Equal(t, uint8(2), v)
	v, err = core.ParseFillValue("NaN", ndarray.Float64)
	require.NoError(t, err)
	require.True(t, math.IsNaN(v.(float64)))
	v, err = core.ParseFillValue("-Infinity", ndarray.Float64)
	require.NoError(t, err)
	require.True(t, math.IsInf(v.(float64), -1))
}

func TestChunkKeyEncodings(t *testing.T) {
	ctx := context.Background()
	for _, fn := range []func(*v3.MetadataBuilder) *v3.MetadataBuilder{
		func(b *v3.MetadataBuilder) *v3.MetadataBuilder { return b.WithDefaultChunkKeyEncoding() },
		func(b *v3.MetadataBuilder) *v3.MetadataBuilder { return b.WithV2ChunkKeyEncoding() },
	} {
		h := store.NewMemory().Resolve("cke")
		meta, err := fn(v3.NewMetadataBuilder().WithShape(8, 8).WithDataType(ndarray.Int16).WithChunkShape(4, 4).WithFillValue(0)).Build()
		require.NoError(t, err)
		arr, err := v3.Create(ctx, h, meta, true)
		require.NoError(t, err)
		data := ndarray.New(ndarray.Int16, []int{8, 8}, binary.LittleEndian)
		for i := 0; i < data.Len(); i++ {
			data.SetInt64(i, int64(i))
		}
		require.NoError(t, arr.Write(ctx, nil, data))
		got, err := arr.Read(ctx, nil, nil)
		require.NoError(t, err)
		require.Equal(t, int64(7), got.GetInt64(7))
	}
}

func TestEndianness(t *testing.T) {
	ctx := context.Background()
	for _, endian := range []string{"LITTLE", "BIG"} {
		h := store.NewMemory().Resolve("e", endian)
		meta, err := v3.NewMetadataBuilder().
			WithShape(8, 8).
			WithDataType(ndarray.Float32).
			WithChunkShape(4, 4).
			WithFillValue(0).
			WithCodecs(func(b *v3.CodecBuilder) *v3.CodecBuilder { return b.WithBytes(endian) }).
			Build()
		require.NoError(t, err)
		arr, err := v3.Create(ctx, h, meta, false)
		require.NoError(t, err)
		data := ndarray.New(ndarray.Float32, []int{8, 8}, binary.LittleEndian)
		for i := 0; i < data.Len(); i++ {
			data.SetFloat64(i, float64(i))
		}
		require.NoError(t, arr.Write(ctx, nil, data))
		got, err := arr.Read(ctx, nil, nil)
		require.NoError(t, err)
		require.InDelta(t, 7.0, got.GetFloat64(7), 1e-6)
	}
}

func TestStorageTransformerRejected(t *testing.T) {
	ctx := context.Background()
	root := testdataRoot(t)
	if root == "" {
		t.Skip("testdata missing")
	}
	h := store.NewFilesystem(filepath.Join(root, "storage_transformer", "exists")).Resolve()
	_, err := v3.Open(ctx, h)
	require.Error(t, err)
	require.Contains(t, err.Error(), "storage transformers")
}

func TestReadmeWriteSubset(t *testing.T) {
	ctx := context.Background()
	h := store.NewMemory().Resolve("color", "1")
	meta, err := v3.NewMetadataBuilder().
		WithShape(1, 64, 64, 16).
		WithDataType(ndarray.Uint32).
		WithChunkShape(1, 16, 16, 16).
		WithFillValue(0).
		WithCodecs(func(b *v3.CodecBuilder) *v3.CodecBuilder {
			return b.WithSharding([]int{1, 8, 8, 8}, func(n *v3.CodecBuilder) *v3.CodecBuilder { return n.WithBlosc() })
		}).
		Build()
	require.NoError(t, err)
	arr, err := v3.Create(ctx, h, meta, false)
	require.NoError(t, err)
	data := ndarray.New(ndarray.Uint32, []int{1, 1, 2, 2}, binary.LittleEndian)
	data.SetInt64(0, 1)
	data.SetInt64(1, 2)
	data.SetInt64(2, 3)
	data.SetInt64(3, 4)
	require.NoError(t, arr.Write(ctx, []int64{0, 0, 0, 0}, data))
	got, err := arr.Read(ctx, []int64{0, 0, 0, 0}, []int64{1, 1, 2, 2})
	require.NoError(t, err)
	require.True(t, data.Equal(got))
}

func TestL4SampleSkipIfMissing(t *testing.T) {
	ctx := context.Background()
	root := testdataRoot(t)
	p := filepath.Join(root, "l4_sample", "color", "1")
	if _, err := os.Stat(p); err != nil {
		t.Skip("testdata/l4_sample not present; run `make testdata`")
	}
	arr, err := v3.Open(ctx, store.NewFilesystem(p).Resolve())
	require.NoError(t, err)
	got, err := arr.Read(ctx, []int64{0, 3073, 3073, 513}, []int64{1, 64, 64, 64})
	require.NoError(t, err)
	require.Equal(t, 64*64*64, got.Len())
	require.Equal(t, int64(-98), int8(got.GetInt64(0)))
}

func TestNestedSharding(t *testing.T) {
	ctx := context.Background()
	h := store.NewMemory().Resolve("nested")
	meta, err := v3.NewMetadataBuilder().
		WithShape(16, 16).
		WithDataType(ndarray.Int32).
		WithChunkShape(8, 8).
		WithFillValue(0).
		WithCodecs(func(b *v3.CodecBuilder) *v3.CodecBuilder {
			return b.WithSharding([]int{4, 4}, func(n *v3.CodecBuilder) *v3.CodecBuilder {
				return n.WithSharding([]int{2, 2}, func(nn *v3.CodecBuilder) *v3.CodecBuilder { return nn.WithBytes("LITTLE") })
			})
		}).
		Build()
	require.NoError(t, err)
	arr, err := v3.Create(ctx, h, meta, false)
	require.NoError(t, err)
	data := ndarray.New(ndarray.Int32, []int{16, 16}, binary.LittleEndian)
	for i := 0; i < data.Len(); i++ {
		data.SetInt64(i, int64(i))
	}
	require.NoError(t, arr.Write(ctx, nil, data))
	got, err := arr.Read(ctx, nil, nil)
	require.NoError(t, err)
	require.Equal(t, int64(255), got.GetInt64(255))
}

func TestUnalignedWrite(t *testing.T) {
	ctx := context.Background()
	h := store.NewMemory().Resolve("u")
	meta, err := v3.NewMetadataBuilder().
		WithShape(52).
		WithDataType(ndarray.Int32).
		WithChunkShape(17).
		WithFillValue(0).
		Build()
	require.NoError(t, err)
	arr, err := v3.Create(ctx, h, meta, false)
	require.NoError(t, err)
	data := ndarray.New(ndarray.Int32, []int{32}, binary.LittleEndian)
	for i := 0; i < data.Len(); i++ {
		data.SetInt64(i, int64(i+1))
	}
	require.NoError(t, arr.Write(ctx, []int64{3}, data))
	got, err := arr.Read(ctx, []int64{3}, []int64{32})
	require.NoError(t, err)
	require.Equal(t, int64(1), got.GetInt64(0))
	require.Equal(t, int64(32), got.GetInt64(31))
}

func codecMeta(a *ndarray.Array) codec.ArrayMeta {
	sh := make([]int64, len(a.Shape))
	cs := make([]int, len(a.Shape))
	for i, s := range a.Shape {
		sh[i] = int64(s)
		cs[i] = s
	}
	return codec.ArrayMeta{Shape: sh, ChunkShape: cs, DType: a.DType, Order: a.Order}
}
