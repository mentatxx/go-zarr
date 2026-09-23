package indexing_test

import (
	"testing"

	"github.com/mentatxx/go-zarr/internal/indexing"
	"github.com/stretchr/testify/require"
)

func TestComputeChunkCoordsFull(t *testing.T) {
	coords, err := indexing.ComputeChunkCoords([]int64{10, 10}, []int{4, 4}, nil, nil)
	require.NoError(t, err)
	require.Len(t, coords, 9)
	require.Equal(t, []int64{0, 0}, coords[0])
	require.Equal(t, []int64{2, 2}, coords[8])
}

func TestComputeProjectionPartial(t *testing.T) {
	p, err := indexing.ComputeProjection([]int64{1, 1}, []int64{10, 10}, []int{4, 4}, []int64{5, 5}, []int64{3, 3})
	require.NoError(t, err)
	require.Equal(t, []int{1, 1}, p.ChunkOffset)
	require.Equal(t, []int{0, 0}, p.OutOffset)
	require.Equal(t, []int{3, 3}, p.Shape)
}

func TestIsSingleFullChunk(t *testing.T) {
	require.True(t, indexing.IsSingleFullChunk([]int64{4, 0}, []int64{4, 4}, []int{4, 4}))
	require.False(t, indexing.IsSingleFullChunk([]int64{1, 0}, []int64{4, 4}, []int{4, 4}))
}
