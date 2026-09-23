package engine_test

import (
	"context"
	"encoding/binary"
	"sync"
	"testing"

	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
	v3 "github.com/mentatxx/go-zarr/v3"
	"github.com/stretchr/testify/require"
)

func TestParallelWrite(t *testing.T) {
	ctx := context.Background()
	h := store.NewMemory().Resolve("p")
	meta, err := v3.NewMetadataBuilder().
		WithShape(32, 32).
		WithDataType(ndarray.Int32).
		WithChunkShape(8, 8).
		WithFillValue(0).
		Build()
	require.NoError(t, err)
	arr, err := v3.Create(ctx, h, meta, false)
	require.NoError(t, err)
	var wg sync.WaitGroup
	errCh := make(chan error, 16)
	for y := 0; y < 32; y += 8 {
		for x := 0; x < 32; x += 8 {
			wg.Add(1)
			go func(y, x int) {
				defer wg.Done()
				chunk := ndarray.New(ndarray.Int32, []int{8, 8}, binary.LittleEndian)
				for i := 0; i < chunk.Len(); i++ {
					chunk.SetInt64(i, int64(y*32+x+i))
				}
				if werr := arr.Write(ctx, []int64{int64(y), int64(x)}, chunk); werr != nil {
					errCh <- werr
				}
			}(y, x)
		}
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}
	got, err := arr.Read(ctx, nil, nil)
	require.NoError(t, err)
	require.Equal(t, []int{32, 32}, got.Shape)
}
