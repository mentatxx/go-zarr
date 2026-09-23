package ndarray_test

import (
	"encoding/binary"
	"testing"

	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/stretchr/testify/require"
)

func TestCopyRegionAndFill(t *testing.T) {
	src := ndarray.New(ndarray.Int32, []int{4, 4}, binary.LittleEndian)
	for i := 0; i < src.Len(); i++ {
		src.SetInt64(i, int64(i+1))
	}
	dst := ndarray.New(ndarray.Int32, []int{4, 4}, binary.LittleEndian)
	dst.Fill(int32(0))
	err := src.CopyRegion([]int{1, 1}, dst, []int{0, 0}, []int{2, 2})
	require.NoError(t, err)
	require.Equal(t, int64(6), dst.GetInt64(0))
	require.Equal(t, int64(7), dst.GetInt64(1))
	require.Equal(t, int64(10), dst.GetInt64(4))
	require.Equal(t, int64(11), dst.GetInt64(5))
	require.True(t, dst.AllValuesEqual(int32(0)) == false)
}

func TestAllValuesEqual(t *testing.T) {
	a := ndarray.New(ndarray.Float32, []int{2, 2}, binary.LittleEndian)
	a.Fill(float32(3.5))
	require.True(t, a.AllValuesEqual(float32(3.5)))
	a.SetFloat64(0, 1)
	require.False(t, a.AllValuesEqual(float32(3.5)))
}

func TestSectionAndTranspose(t *testing.T) {
	a := ndarray.New(ndarray.Int16, []int{2, 3}, binary.BigEndian)
	for i := 0; i < 6; i++ {
		a.SetInt64(i, int64(i))
	}
	sec, err := a.Section([]int{0, 1}, []int{2, 2})
	require.NoError(t, err)
	require.Equal(t, []int{2, 2}, sec.Shape)
	require.Equal(t, int64(1), sec.GetInt64(0))
	require.Equal(t, int64(2), sec.GetInt64(1))
	tr, err := a.Transpose([]int{1, 0})
	require.NoError(t, err)
	require.Equal(t, []int{3, 2}, tr.Shape)
	require.Equal(t, int64(0), tr.GetInt64(0))
	require.Equal(t, int64(3), tr.GetInt64(1))
	require.Equal(t, int64(1), tr.GetInt64(2))
}

func TestReshape(t *testing.T) {
	a := ndarray.New(ndarray.Uint8, []int{2, 3}, binary.LittleEndian)
	for i := 0; i < 6; i++ {
		a.SetUint64(i, uint64(i))
	}
	b, err := a.Reshape([]int{3, 2})
	require.NoError(t, err)
	require.Equal(t, []int{3, 2}, b.Shape)
	require.Equal(t, uint64(5), b.GetUint64(5))
	c, err := a.Reshape([]int{-1, 2})
	require.NoError(t, err)
	require.Equal(t, []int{3, 2}, c.Shape)
}

func TestParseV2(t *testing.T) {
	spec, err := ndarray.ParseV2(">i4")
	require.NoError(t, err)
	require.Equal(t, ndarray.Int32, spec.DType)
	require.Equal(t, binary.BigEndian, spec.Order)
	require.Equal(t, ">i4", ndarray.FormatV2(spec.DType, spec.Order))
}

func TestEncodeEndianSwap(t *testing.T) {
	a := ndarray.New(ndarray.Int32, []int{2}, binary.LittleEndian)
	a.SetInt64(0, 0x01020304)
	enc := a.EncodeBytes(binary.BigEndian)
	require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, enc[:4])
}
