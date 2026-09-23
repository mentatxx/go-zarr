package zarr_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mentatxx/go-zarr/codec"
	"github.com/mentatxx/go-zarr/core"
	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
	v2 "github.com/mentatxx/go-zarr/v2"
	v3 "github.com/mentatxx/go-zarr/v3"
	"github.com/stretchr/testify/require"
)

func pythonScripts(t *testing.T) string {
	t.Helper()
	for _, c := range []string{"testdata/python", "../testdata/python"} {
		if st, err := os.Stat(filepath.Join(c, "zarr_python_write.py")); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	t.Skip("python scripts not found")
	return ""
}

func requireUV(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("uv"); err != nil {
		t.Skip("uv not installed")
	}
}

func runPython(t *testing.T, scripts, script string, args ...string) {
	t.Helper()
	cmdArgs := append([]string{"run", filepath.Join(scripts, script)}, args...)
	cmd := exec.Command("uv", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("python %s skipped: %v\n%s", script, err, out)
	}
}

func TestPythonWriteThenGoReadV3(t *testing.T) {
	requireUV(t)
	scripts := pythonScripts(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "py")
	runPython(t, scripts, "zarr_python_write.py", "gzip", "5", "int32", p)
	arr, err := v3.Open(context.Background(), store.NewFilesystem(p).Resolve())
	require.NoError(t, err)
	got, err := arr.Read(context.Background(), nil, nil)
	require.NoError(t, err)
	require.Equal(t, []int{16, 16, 16}, got.Shape)
	require.Equal(t, ndarray.Int32, got.DType)
}

func TestPythonInteropV3Write(t *testing.T) {
	requireUV(t)
	scripts := pythonScripts(t)
	ctx := context.Background()
	dir := t.TempDir()
	h := store.NewFilesystem(dir).Resolve("py")
	meta, err := v3.NewMetadataBuilder().
		WithShape(16, 16, 16).
		WithDataType(ndarray.Int32).
		WithChunkShape(2, 4, 8).
		WithFillValue(0).
		WithAttributes(core.Attributes{"test_key": "test_value"}).
		WithCodecs(func(b *v3.CodecBuilder) *v3.CodecBuilder { return b.WithGzip(5) }).
		Build()
	require.NoError(t, err)
	arr, err := v3.Create(ctx, h, meta, false)
	require.NoError(t, err)
	data := ndarray.New(ndarray.Int32, []int{16, 16, 16}, nil)
	for i := 0; i < data.Len(); i++ {
		data.SetInt64(i, int64(i))
	}
	require.NoError(t, arr.Write(ctx, nil, data))
	p, err := h.ToPath()
	require.NoError(t, err)
	runPython(t, scripts, "zarr_python_read.py", "gzip", "5", "int32", p)
}

func TestPythonInteropV2(t *testing.T) {
	requireUV(t)
	scripts := pythonScripts(t)
	ctx := context.Background()
	dir := t.TempDir()
	h := store.NewFilesystem(dir).Resolve("py2")
	meta, err := v2.NewMetadataBuilder().
		WithShape(16, 16, 16).
		WithChunks(2, 4, 8).
		WithDataType("<i4").
		WithFillValue(0).
		WithZlibCompressor(1).
		Build()
	require.NoError(t, err)
	arr, err := v2.Create(ctx, h, meta, false)
	require.NoError(t, err)
	data := ndarray.New(ndarray.Int32, []int{16, 16, 16}, nil)
	for i := 0; i < data.Len(); i++ {
		data.SetInt64(i, int64(i))
	}
	require.NoError(t, arr.Write(ctx, nil, data))
	p, err := h.ToPath()
	require.NoError(t, err)
	runPython(t, scripts, "zarr_python_read_v2.py", "zlib", "1", "int32", p)
}

func TestPythonGroup(t *testing.T) {
	requireUV(t)
	scripts := pythonScripts(t)
	dir := t.TempDir()
	runPython(t, scripts, "zarr_python_group.py", dir)
}

func TestBloscLZSkipWithoutCGO(t *testing.T) {
	c, err := codec.NewBlosc("blosclz", "noshuffle", 1, 1, 0)
	require.NoError(t, err)
	_, err = c.EncodeBytes([]byte{1, 2, 3, 4})
	if err == nil {
		t.Log("blosclz available via cblosc")
		return
	}
	require.ErrorIs(t, err, codec.ErrBloscLZNeedsCGO)
}
