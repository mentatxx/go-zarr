package zarr_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mentatxx/go-zarr"
	"github.com/mentatxx/go-zarr/core"
	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
	v2 "github.com/mentatxx/go-zarr/v2"
	v3 "github.com/mentatxx/go-zarr/v3"
	"github.com/stretchr/testify/require"
)

func TestOpenArrayAndGroup(t *testing.T) {
	ctx := context.Background()
	h := store.NewMemory().Resolve("n")
	meta, err := v3.NewMetadataBuilder().WithShape(4, 4).WithDataType(ndarray.Int8).WithChunkShape(2, 2).Build()
	require.NoError(t, err)
	_, err = v3.Create(ctx, h, meta, false)
	require.NoError(t, err)
	n, err := zarr.OpenArray(ctx, h)
	require.NoError(t, err)
	require.IsType(t, &v3.Array{}, n)

	gh := store.NewMemory().Resolve("g")
	_, err = v3.CreateGroup(ctx, gh, v3.DefaultGroupMetadata())
	require.NoError(t, err)
	gn, err := zarr.OpenGroup(ctx, gh)
	require.NoError(t, err)
	require.IsType(t, &v3.Group{}, gn)
}

func TestOpenPathFilesystem(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	h := store.NewFilesystem(dir).Resolve()
	meta, err := v3.NewMetadataBuilder().WithShape(2, 2).WithDataType(ndarray.Uint8).WithChunkShape(2, 2).WithFillValue(0).Build()
	require.NoError(t, err)
	arr, err := v3.Create(ctx, h, meta, false)
	require.NoError(t, err)
	data := ndarray.New(ndarray.Uint8, []int{2, 2}, nil)
	data.SetInt64(0, 7)
	require.NoError(t, arr.Write(ctx, nil, data))
	n, err := zarr.OpenPath(ctx, dir)
	require.NoError(t, err)
	require.IsType(t, &v3.Array{}, n)
}

func TestOpenV2Node(t *testing.T) {
	ctx := context.Background()
	h := store.NewMemory().Resolve("v2g")
	_, err := v2.CreateGroup(ctx, h, core.Attributes{})
	require.NoError(t, err)
	n, err := zarr.Open(ctx, h)
	require.NoError(t, err)
	require.IsType(t, &v2.Group{}, n)
}

func TestCLIPrintsArray(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	h := store.NewFilesystem(dir).Resolve()
	meta, err := v3.NewMetadataBuilder().WithShape(2).WithDataType(ndarray.Int32).WithChunkShape(2).WithFillValue(0).Build()
	require.NoError(t, err)
	arr, err := v3.Create(ctx, h, meta, false)
	require.NoError(t, err)
	data := ndarray.New(ndarray.Int32, []int{2}, nil)
	data.SetInt64(0, 1)
	data.SetInt64(1, 2)
	require.NoError(t, arr.Write(ctx, nil, data))

	name := "zarr"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/zarr")
	cmd.Dir = mustRepoRoot(t)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	run := exec.Command(bin, "--array_path", dir)
	out, err = run.CombinedOutput()
	require.NoError(t, err, string(out))
	require.Contains(t, string(out), "1")
}

func mustRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	if _, err := os.Stat(filepath.Join(wd, "cmd", "zarr", "main.go")); err == nil {
		return wd
	}
	if _, err := os.Stat(filepath.Join(wd, "..", "cmd", "zarr", "main.go")); err == nil {
		return filepath.Join(wd, "..")
	}
	t.Fatal("cannot find repo root")
	return ""
}
