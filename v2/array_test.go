package v2_test

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/mentatxx/go-zarr/core"
	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
	v2 "github.com/mentatxx/go-zarr/v2"
	"github.com/stretchr/testify/require"
)

func testdata(dt ndarray.DType) *ndarray.Array {
	a := ndarray.New(dt, []int{16, 16, 16}, binary.LittleEndian)
	for i := 0; i < a.Len(); i++ {
		a.SetInt64(i, int64(i))
	}
	return a
}

func TestV2CreateReadWrite(t *testing.T) {
	ctx := context.Background()
	h := store.NewMemory().Resolve("arr")
	meta, err := v2.NewMetadataBuilder().
		WithShape(16, 16, 16).
		WithChunks(2, 4, 8).
		WithDataType("<i4").
		WithFillValue(0).
		WithAttributes(core.Attributes{"test_key": "test_value"}).
		WithZlibCompressor(1).
		Build()
	require.NoError(t, err)
	arr, err := v2.Create(ctx, h, meta, false)
	require.NoError(t, err)
	require.NoError(t, arr.Write(ctx, nil, testdata(ndarray.Int32)))
	got, err := arr.Read(ctx, nil, nil)
	require.NoError(t, err)
	require.Equal(t, []int{16, 16, 16}, got.Shape)
	opened, err := v2.Open(ctx, h)
	require.NoError(t, err)
	require.Equal(t, "test_value", opened.Metadata().Attributes["test_key"])
}

func TestV2Endianness(t *testing.T) {
	ctx := context.Background()
	for _, big := range []bool{false, true} {
		h := store.NewMemory().Resolve("e")
		b := v2.NewMetadataBuilder().WithShape(8, 8).WithChunks(4, 4).WithFillValue(0)
		b.WithDataTypeSpec(ndarray.Int32, big)
		meta, err := b.Build()
		require.NoError(t, err)
		arr, err := v2.Create(ctx, h, meta, true)
		require.NoError(t, err)
		data := ndarray.New(ndarray.Int32, []int{8, 8}, meta.ParsedDType.Order)
		for i := 0; i < data.Len(); i++ {
			data.SetInt64(i, int64(i))
		}
		require.NoError(t, arr.Write(ctx, nil, data))
		got, err := arr.Read(ctx, nil, nil)
		require.NoError(t, err)
		require.Equal(t, int64(7), got.GetInt64(7))
	}
}

func TestV2SampleTestdata(t *testing.T) {
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
	g, err := v2.OpenGroup(ctx, store.NewFilesystem(root).Resolve())
	require.NoError(t, err)
	n, err := g.Get(ctx, "bool")
	require.NoError(t, err)
	arr := n.(*v2.Array)
	got, err := arr.Read(ctx, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, got)
}

func TestV2Group(t *testing.T) {
	ctx := context.Background()
	h := store.NewMemory().Resolve("g")
	g, err := v2.CreateGroup(ctx, h, core.Attributes{"attr": "value"})
	require.NoError(t, err)
	sub, err := g.CreateGroup(ctx, "group")
	require.NoError(t, err)
	_, err = sub.CreateArray(ctx, "array", func(b *v2.MetadataBuilder) *v2.MetadataBuilder {
		return b.WithShape(4, 4).WithChunks(2, 2).WithDataType("<i4")
	})
	require.NoError(t, err)
	n, err := g.Get(ctx, "group")
	require.NoError(t, err)
	require.IsType(t, &v2.Group{}, n)
}
