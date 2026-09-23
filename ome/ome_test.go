package ome_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mentatxx/go-zarr/ome"
	"github.com/mentatxx/go-zarr/store"
	v2 "github.com/mentatxx/go-zarr/v2"
	v3 "github.com/mentatxx/go-zarr/v3"
	"github.com/stretchr/testify/require"
)

func testdataOME(t *testing.T, rel string) string {
	t.Helper()
	for _, c := range []string{filepath.Join("testdata", rel), filepath.Join("..", "testdata", rel)} {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	t.Skipf("missing testdata %s", rel)
	return ""
}

func TestOMEv04(t *testing.T) {
	ctx := context.Background()
	h := store.NewFilesystem(testdataOME(t, "ome/v0.4")).Resolve()
	im, err := ome.Open(ctx, h)
	require.NoError(t, err)
	require.Equal(t, "0.4", im.Version)
	require.Equal(t, []string{"t", "c", "z", "y", "x"}, im.AxisNames())
	require.Equal(t, 2, im.ScaleLevelCount())
	arr, err := im.OpenScaleLevel(ctx, 0)
	require.NoError(t, err)
	a := arr.(*v2.Array)
	require.Equal(t, []int64{1, 2, 8, 16, 16}, a.Metadata().Shape)
	labels, err := im.Labels(ctx)
	require.NoError(t, err)
	require.Contains(t, labels, "nuclei")
	entry, err := im.GetMultiscaleNode(0)
	require.NoError(t, err)
	require.Equal(t, "0.4", entry.Version)
	require.Equal(t, []float64{1, 1, 0.5, 0.5, 0.5}, entry.Datasets[0].CoordinateTransformations[0].Scale)
}

func TestOMEv05(t *testing.T) {
	ctx := context.Background()
	h := store.NewFilesystem(testdataOME(t, "ome/v0.5")).Resolve()
	im, err := ome.Open(ctx, h)
	require.NoError(t, err)
	require.True(t, im.Version == "0.5" || im.Version == "")
	require.Equal(t, []string{"t", "c", "z", "y", "x"}, im.AxisNames())
	require.Equal(t, 2, im.ScaleLevelCount())
	arr, err := im.OpenScaleLevel(ctx, 0)
	require.NoError(t, err)
	a := arr.(*v3.Array)
	require.Equal(t, []int64{1, 2, 8, 16, 16}, a.Metadata().Shape)
}

func TestOMEv04HCS(t *testing.T) {
	ctx := context.Background()
	h := store.NewFilesystem(testdataOME(t, "ome/v0.4_hcs")).Resolve()
	p, err := ome.OpenPlate(ctx, h)
	require.NoError(t, err)
	require.NotEmpty(t, p.Meta.Wells)
	well, err := p.OpenWell(ctx, p.Meta.Wells[0].Path)
	require.NoError(t, err)
	require.NotEmpty(t, well.Meta.Images)
	im, err := well.OpenImage(ctx, well.Meta.Images[0].Path)
	require.NoError(t, err)
	require.Equal(t, "0.4", im.Version)
}

func TestOMEv05HCS(t *testing.T) {
	ctx := context.Background()
	h := store.NewFilesystem(testdataOME(t, "ome/v0.5_hcs")).Resolve()
	p, err := ome.OpenPlate(ctx, h)
	require.NoError(t, err)
	require.NotEmpty(t, p.Meta.Wells)
}

func TestOMESceneExample1(t *testing.T) {
	ctx := context.Background()
	h := store.NewFilesystem(testdataOME(t, "ome/v0.6_scene/example1_instrument_registration.zarr")).Resolve()
	_, err := ome.Open(ctx, h)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Scene")

	scene, err := ome.OpenScene(ctx, h)
	require.NoError(t, err)
	require.Len(t, scene.Meta.CoordinateTransformations, 1)
	ct := scene.Meta.CoordinateTransformations[0]
	require.Equal(t, "affine", ct.Type)
	require.Equal(t, "sampleA_instrument2#physical_instrument2", ct.InputName())
	require.Equal(t, "sampleA_instrument1#physical_instrument1", ct.OutputName())
	require.Equal(t, "coordinateTransformations/sampleA_instrument2-to-instrument1", ct.Path)

	names, err := scene.ListImageNodes(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"sampleA_instrument1", "sampleA_instrument2"}, names)

	i1, err := scene.OpenImageNode(ctx, "sampleA_instrument1")
	require.NoError(t, err)
	require.Equal(t, []string{"z", "y", "x"}, i1.AxisNames())
	i2, err := scene.OpenImageNode(ctx, "sampleA_instrument2")
	require.NoError(t, err)
	require.Equal(t, []string{"z", "y", "x"}, i2.AxisNames())

	g := scene.CoordinateTransformationGraph()
	require.Len(t, g.Nodes, 2)
	require.Len(t, g.Edges, 1)
	require.Empty(t, g.Warnings)
	require.Equal(t, "coordinateTransformations/lens", ome.NormalizeCoordinateTransformPath("./coordinateTransformations/lens"))
}

func TestOMESceneExample2(t *testing.T) {
	ctx := context.Background()
	h := store.NewFilesystem(testdataOME(t, "ome/v0.6_scene/example2_multi_instrument_chain.zarr")).Resolve()
	scene, err := ome.OpenScene(ctx, h)
	require.NoError(t, err)
	names, err := scene.ListImageNodes(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"instrument1", "instrument2", "instrument3"}, names)
	require.Len(t, scene.Meta.CoordinateTransformations, 2)
	g := scene.CoordinateTransformationGraph()
	require.Len(t, g.Nodes, 3)
	require.Len(t, g.Edges, 2)
}

func TestOMEv06ExamplesSkipIfMissing(t *testing.T) {
	ctx := context.Background()
	root := testdataOME(t, "ome/v0.6/examples")
	p := filepath.Join(root, "2d", "basic", "scale_multiscale.zarr")
	if _, err := os.Stat(p); err != nil {
		t.Skip("ome v0.6 examples submodule not present")
	}
	im, err := ome.Open(ctx, store.NewFilesystem(p).Resolve())
	require.NoError(t, err)
	require.Equal(t, 3, im.ScaleLevelCount())
	require.Equal(t, []string{"y", "x"}, im.AxisNames())
}
