package engine

import (
	"context"
	"fmt"
	"sync"

	"github.com/mentatxx/go-zarr/codec"
	"github.com/mentatxx/go-zarr/core"
	"github.com/mentatxx/go-zarr/internal/indexing"
	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
	"golang.org/x/sync/errgroup"
)

// Array is the shared read/write engine.
type Array struct {
	Handle    store.Handle
	Meta      codec.ArrayMeta
	Pipeline  *codec.Pipeline
	EncodeKey func([]int64) []string
	Parallel  bool
}

func (a *Array) workers() int {
	if a.Parallel {
		return 8
	}
	return 1
}

func (a *Array) chunkInArray(coords []int64) bool {
	for i, c := range coords {
		if c < 0 || c*int64(a.Meta.ChunkShape[i]) >= a.Meta.Shape[i] {
			return false
		}
	}
	return true
}

// ReadChunk reads one chunk, filling if missing.
func (a *Array) ReadChunk(ctx context.Context, coords []int64) (*ndarray.Array, error) {
	if !a.chunkInArray(coords) {
		return nil, core.ErrOutOfBounds
	}
	h := a.Handle.Resolve(a.EncodeKey(coords)...)
	b, err := h.Read(ctx)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return a.Meta.AllocateFill(), nil
	}
	return a.Pipeline.Decode(b)
}

// WriteChunk writes one chunk, deleting it if it is all fill values.
func (a *Array) WriteChunk(ctx context.Context, coords []int64, chunk *ndarray.Array) error {
	h := a.Handle.Resolve(a.EncodeKey(coords)...)
	if a.Meta.Fill != nil && chunk.AllValuesEqual(a.Meta.Fill) {
		return h.Delete(ctx)
	}
	b, err := a.Pipeline.Encode(chunk)
	if err != nil {
		return err
	}
	return h.Set(ctx, b)
}

// Read reads a selection.
func (a *Array) Read(ctx context.Context, offset, shape []int64) (*ndarray.Array, error) {
	if offset == nil {
		offset = make([]int64, a.Meta.NDim())
	}
	if shape == nil {
		shape = append([]int64(nil), a.Meta.Shape...)
	}
	if len(offset) != a.Meta.NDim() || len(shape) != a.Meta.NDim() {
		return nil, fmt.Errorf("zarr: offset/shape rank mismatch")
	}
	for i := range offset {
		if offset[i] < 0 || offset[i]+shape[i] > a.Meta.Shape[i] {
			return nil, core.ErrOutOfBounds
		}
	}
	outShape := indexing.Int64ToInt(shape)
	if indexing.IsSingleFullChunk(offset, shape, a.Meta.ChunkShape) {
		return a.ReadChunk(ctx, indexing.ComputeSingleChunkCoords(offset, a.Meta.ChunkShape))
	}
	out := ndarray.New(a.Meta.DType, outShape, a.Meta.Order)
	if a.Meta.Fill != nil {
		out.Fill(a.Meta.Fill)
	}
	coords, err := indexing.ComputeChunkCoords(a.Meta.Shape, a.Meta.ChunkShape, offset, shape)
	if err != nil {
		return nil, err
	}
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(a.workers())
	var mu sync.Mutex
	for _, cc := range coords {
		cc := cc
		g.Go(func() error {
			proj, err := indexing.ComputeProjection(cc, a.Meta.Shape, a.Meta.ChunkShape, offset, shape)
			if err != nil {
				return err
			}
			h := a.Handle.Resolve(a.EncodeKey(cc)...)
			if a.Pipeline.SupportsPartialDecode() {
				ok, err := h.Exists(ctx)
				if err != nil || !ok {
					return err
				}
				chunk, err := a.Pipeline.DecodePartial(ctx, h, indexing.IntsToInt64(proj.ChunkOffset), proj.Shape)
				if err != nil {
					return err
				}
				mu.Lock()
				defer mu.Unlock()
				return chunk.CopyRegion(make([]int, a.Meta.NDim()), out, proj.OutOffset, proj.Shape)
			}
			b, err := h.Read(ctx)
			if err != nil {
				return err
			}
			if b == nil {
				return nil
			}
			chunk, err := a.Pipeline.Decode(b)
			if err != nil {
				return err
			}
			mu.Lock()
			defer mu.Unlock()
			return chunk.CopyRegion(proj.ChunkOffset, out, proj.OutOffset, proj.Shape)
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return out, nil
}

// Write writes data at offset.
func (a *Array) Write(ctx context.Context, offset []int64, data *ndarray.Array) error {
	if offset == nil {
		offset = make([]int64, a.Meta.NDim())
	}
	if len(offset) != a.Meta.NDim() || data.NDim() != a.Meta.NDim() {
		return fmt.Errorf("zarr: write rank mismatch")
	}
	shape := indexing.IntsToInt64(data.Shape)
	coords, err := indexing.ComputeChunkCoords(a.Meta.Shape, a.Meta.ChunkShape, offset, shape)
	if err != nil {
		return err
	}
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(a.workers())
	for _, cc := range coords {
		cc := cc
		g.Go(func() error {
			proj, err := indexing.ComputeProjection(cc, a.Meta.Shape, a.Meta.ChunkShape, offset, shape)
			if err != nil {
				return err
			}
			var chunk *ndarray.Array
			if indexing.IsFullChunk(proj.ChunkOffset, proj.Shape, a.Meta.ChunkShape) {
				chunk, err = data.Section(proj.OutOffset, proj.Shape)
				if err != nil {
					return err
				}
			} else {
				chunk, err = a.ReadChunk(ctx, cc)
				if err != nil {
					return err
				}
				if err := data.CopyRegion(proj.OutOffset, chunk, proj.ChunkOffset, proj.Shape); err != nil {
					return err
				}
			}
			return a.WriteChunk(ctx, cc, chunk)
		})
	}
	return g.Wait()
}

// CleanupChunksForResize deletes or trims chunks after shrinking.
func (a *Array) CleanupChunksForResize(ctx context.Context, newShape []int64) error {
	ndim := a.Meta.NDim()
	newMax := make([]int64, ndim)
	for i := 0; i < ndim; i++ {
		newMax[i] = (newShape[i] + int64(a.Meta.ChunkShape[i]) - 1) / int64(a.Meta.ChunkShape[i])
	}
	old, err := indexing.ComputeChunkCoords(a.Meta.Shape, a.Meta.ChunkShape, nil, nil)
	if err != nil {
		return err
	}
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(a.workers())
	for _, cc := range old {
		cc := cc
		g.Go(func() error {
			outside := false
			onBoundary := false
			for i := 0; i < ndim; i++ {
				if cc[i] >= newMax[i] {
					outside = true
					break
				}
				if (cc[i]+1)*int64(a.Meta.ChunkShape[i]) > newShape[i] {
					onBoundary = true
				}
			}
			h := a.Handle.Resolve(a.EncodeKey(cc)...)
			if outside {
				return h.Delete(ctx)
			}
			if onBoundary {
				return a.trimBoundary(ctx, cc, newShape)
			}
			return nil
		})
	}
	return g.Wait()
}

func (a *Array) trimBoundary(ctx context.Context, coords, newShape []int64) error {
	valid := make([]int, a.Meta.NDim())
	need := false
	for i := range valid {
		start := coords[i] * int64(a.Meta.ChunkShape[i])
		end := start + int64(a.Meta.ChunkShape[i])
		if end > newShape[i] {
			valid[i] = int(newShape[i] - start)
			need = true
		} else {
			valid[i] = a.Meta.ChunkShape[i]
		}
	}
	if !need {
		return nil
	}
	old, err := a.ReadChunk(ctx, coords)
	if err != nil {
		return err
	}
	neu := a.Meta.AllocateFill()
	if err := old.CopyRegion(make([]int, a.Meta.NDim()), neu, make([]int, a.Meta.NDim()), valid); err != nil {
		return err
	}
	return a.WriteChunk(ctx, coords, neu)
}
