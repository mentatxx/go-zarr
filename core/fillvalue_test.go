package core_test

import (
	"math"
	"testing"

	"github.com/mentatxx/go-zarr/core"
	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/stretchr/testify/require"
)

func TestParseFillValue(t *testing.T) {
	v, err := core.ParseFillValue(0, ndarray.Uint32)
	require.NoError(t, err)
	require.Equal(t, uint32(0), v)

	v, err = core.ParseFillValue("0x00010203", ndarray.Uint32)
	require.NoError(t, err)
	require.Equal(t, uint32(50462976), v)

	v, err = core.ParseFillValue("0b00000010", ndarray.Uint8)
	require.NoError(t, err)
	require.Equal(t, uint8(2), v)

	v, err = core.ParseFillValue("NaN", ndarray.Float64)
	require.NoError(t, err)
	require.True(t, math.IsNaN(v.(float64)))

	v, err = core.ParseFillValue("+Infinity", ndarray.Float32)
	require.NoError(t, err)
	require.True(t, math.IsInf(float64(v.(float32)), 1))

	v, err = core.ParseFillValue("-Infinity", ndarray.Float64)
	require.NoError(t, err)
	require.True(t, math.IsInf(v.(float64), -1))
}

func TestDefaultChunkShape(t *testing.T) {
	require.Equal(t, []int{4, 4}, core.DefaultChunkShape([]int64{4, 4}))
	cs := core.DefaultChunkShape([]int64{1024, 10})
	require.Len(t, cs, 2)
	require.Equal(t, 10, cs[1])
}
