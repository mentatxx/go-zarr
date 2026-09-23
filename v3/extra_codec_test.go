package v3_test

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
	v3 "github.com/mentatxx/go-zarr/v3"
	"github.com/stretchr/testify/require"
)

func TestReshapeCodec(t *testing.T) {
	ctx := context.Background()
	h := store.NewMemory().Resolve("r")
	meta, err := v3.NewMetadataBuilder().
		WithShape(8, 8).
		WithDataType(ndarray.Int32).
		WithChunkShape(8, 8).
		WithFillValue(0).
		WithCodecs(func(b *v3.CodecBuilder) *v3.CodecBuilder {
			return b.WithReshape([]any{float64(64)})
		}).
		Build()
	require.NoError(t, err)
	arr, err := v3.Create(ctx, h, meta, false)
	require.NoError(t, err)
	data := ndarray.New(ndarray.Int32, []int{8, 8}, binary.LittleEndian)
	for i := 0; i < data.Len(); i++ {
		data.SetInt64(i, int64(i))
	}
	require.NoError(t, arr.Write(ctx, nil, data))
	got, err := arr.Read(ctx, nil, nil)
	require.NoError(t, err)
	require.Equal(t, int64(63), got.GetInt64(63))
}

func TestCastValueCodec(t *testing.T) {
	ctx := context.Background()
	h := store.NewMemory().Resolve("c")
	meta, err := v3.NewMetadataBuilder().
		WithShape(4, 4).
		WithDataType(ndarray.Float64).
		WithChunkShape(4, 4).
		WithFillValue(0).
		WithCodecs(func(b *v3.CodecBuilder) *v3.CodecBuilder {
			return b.WithCastValue(ndarray.Int32)
		}).
		Build()
	require.NoError(t, err)
	arr, err := v3.Create(ctx, h, meta, false)
	require.NoError(t, err)
	data := ndarray.New(ndarray.Float64, []int{4, 4}, binary.LittleEndian)
	for i := 0; i < data.Len(); i++ {
		data.SetFloat64(i, float64(i)+0.2)
	}
	require.NoError(t, arr.Write(ctx, nil, data))
	got, err := arr.Read(ctx, nil, nil)
	require.NoError(t, err)
	require.Equal(t, ndarray.Float64, got.DType)
}
