package codec_test

import (
	"encoding/binary"
	"testing"

	"github.com/mentatxx/go-zarr/codec"
	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/stretchr/testify/require"
)

func testArray() *ndarray.Array {
	a := ndarray.New(ndarray.Int32, []int{4, 4}, binary.LittleEndian)
	for i := 0; i < a.Len(); i++ {
		a.SetInt64(i, int64(i))
	}
	return a
}

func TestBytesRoundtrip(t *testing.T) {
	meta := codec.ArrayMeta{Shape: []int64{4, 4}, ChunkShape: []int{4, 4}, DType: ndarray.Int32, Order: binary.LittleEndian}
	c := codec.NewBytes(binary.LittleEndian)
	require.NoError(t, c.SetMeta(meta))
	src := testArray()
	b, err := c.EncodeArrayBytes(src)
	require.NoError(t, err)
	got, err := c.DecodeArrayBytes(b)
	require.NoError(t, err)
	require.True(t, src.Equal(got))
}

func TestGzipZstdCRCRoundtrip(t *testing.T) {
	src := make([]byte, 256)
	for i := range src {
		src[i] = byte(i)
	}
	g, err := codec.NewGzip(5)
	require.NoError(t, err)
	gb, err := g.EncodeBytes(src)
	require.NoError(t, err)
	out, err := g.DecodeBytes(gb)
	require.NoError(t, err)
	require.Equal(t, src, out)

	z, err := codec.NewZstd(5, true)
	require.NoError(t, err)
	zb, err := z.EncodeBytes(src)
	require.NoError(t, err)
	out, err = z.DecodeBytes(zb)
	require.NoError(t, err)
	require.Equal(t, src, out)

	c := codec.NewCRC32C()
	cb, err := c.EncodeBytes(src)
	require.NoError(t, err)
	out, err = c.DecodeBytes(cb)
	require.NoError(t, err)
	require.Equal(t, src, out)
}

func TestBloscRoundtrip(t *testing.T) {
	src := make([]byte, 1024)
	for i := range src {
		src[i] = byte(i)
	}
	c, err := codec.NewBlosc("zstd", "shuffle", 5, 4, 0)
	require.NoError(t, err)
	b, err := c.EncodeBytes(src)
	require.NoError(t, err)
	out, err := c.DecodeBytes(b)
	require.NoError(t, err)
	require.Equal(t, src, out)
}

func TestPipeline(t *testing.T) {
	meta := codec.ArrayMeta{Shape: []int64{4, 4}, ChunkShape: []int{4, 4}, DType: ndarray.Int32, Order: binary.LittleEndian, Fill: int32(0)}
	gz, err := codec.NewGzip(1)
	require.NoError(t, err)
	p, err := codec.NewPipeline([]codec.Codec{codec.NewBytes(binary.LittleEndian), gz}, meta)
	require.NoError(t, err)
	src := testArray()
	b, err := p.Encode(src)
	require.NoError(t, err)
	got, err := p.Decode(b)
	require.NoError(t, err)
	require.True(t, src.Equal(got))
}

func TestInvalidPipeline(t *testing.T) {
	meta := codec.ArrayMeta{ChunkShape: []int{2}, DType: ndarray.Int32, Order: binary.LittleEndian}
	_, err := codec.NewPipeline([]codec.Codec{codec.NewBytes(nil), codec.NewBytes(nil)}, meta)
	require.Error(t, err)
}
