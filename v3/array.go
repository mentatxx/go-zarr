package v3

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mentatxx/go-zarr/codec"
	"github.com/mentatxx/go-zarr/core"
	"github.com/mentatxx/go-zarr/internal/engine"
	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
)

// Array is a Zarr v3 array.
type Array struct {
	handle store.Handle
	meta   Metadata
	eng    *engine.Array
}

func (a *Array) Handle() store.Handle { return a.handle }
func (a *Array) Metadata() Metadata   { return a.meta }

func newArray(h store.Handle, meta Metadata) (*Array, error) {
	pipe, err := codec.NewPipeline(meta.Codecs, meta.ArrayMeta())
	if err != nil {
		return nil, err
	}
	return &Array{
		handle: h,
		meta:   meta,
		eng: &engine.Array{
			Handle:    h,
			Meta:      meta.ArrayMeta(),
			Pipeline:  pipe,
			EncodeKey: meta.EncodeChunkKey,
			Parallel:  true,
		},
	}, nil
}

// Open opens an existing v3 array.
func Open(ctx context.Context, h store.Handle) (*Array, error) {
	b, err := h.Resolve(ZarrJSON).Read(ctx)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, core.ErrNotFound
	}
	var meta Metadata
	if err := json.Unmarshal(b, &meta); err != nil {
		return nil, core.WrapError("invalid array metadata", err)
	}
	return newArray(h, meta)
}

// OpenPath opens from a filesystem path.
func OpenPath(ctx context.Context, path string) (*Array, error) {
	return Open(ctx, store.NewFilesystem(path).Resolve())
}

// Create writes metadata and returns a new array. existsOk overwrites metadata.
func Create(ctx context.Context, h store.Handle, meta Metadata, existsOk bool) (*Array, error) {
	meta.ZarrFormat = Format
	meta.NodeType = "array"
	if meta.ChunkGrid.Name == "" {
		meta.ChunkGrid.Name = "regular"
	}
	if meta.ChunkKeyEncoding.Name == "" {
		meta.ChunkKeyEncoding.Name = "default"
		meta.ChunkKeyEncoding.Configuration.Separator = "/"
	}
	if len(meta.Codecs) == 0 {
		meta.Codecs = []codec.Codec{codec.NewBytes(nil)}
	}
	if err := meta.validate(); err != nil {
		return nil, err
	}
	mh := h.Resolve(ZarrJSON)
	if !existsOk {
		ok, err := mh.Exists(ctx)
		if err != nil {
			return nil, err
		}
		if ok {
			return nil, fmt.Errorf("%w: metadata already exists at %s", core.ErrExists, h)
		}
	}
	raw, err := json.MarshalIndent(&meta, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := mh.Set(ctx, raw); err != nil {
		return nil, err
	}
	return newArray(h, meta)
}

func (a *Array) Read(ctx context.Context, offset, shape []int64) (*ndarray.Array, error) {
	return a.eng.Read(ctx, offset, shape)
}

func (a *Array) Write(ctx context.Context, offset []int64, data *ndarray.Array) error {
	return a.eng.Write(ctx, offset, data)
}

func (a *Array) ReadChunk(ctx context.Context, coords []int64) (*ndarray.Array, error) {
	return a.eng.ReadChunk(ctx, coords)
}

func (a *Array) WriteChunk(ctx context.Context, coords []int64, data *ndarray.Array) error {
	return a.eng.WriteChunk(ctx, coords, data)
}

func (a *Array) SetParallel(p bool) { a.eng.Parallel = p }

func (a *Array) Resize(ctx context.Context, newShape []int64, metadataOnly bool) (*Array, error) {
	if len(newShape) != a.meta.NDim() {
		return nil, fmt.Errorf("zarr: newShape rank mismatch")
	}
	if !metadataOnly {
		if err := a.eng.CleanupChunksForResize(ctx, newShape); err != nil {
			return nil, err
		}
	}
	meta := a.meta
	meta.Shape = append([]int64(nil), newShape...)
	return Create(ctx, a.handle, meta, true)
}

func (a *Array) SetAttributes(ctx context.Context, attrs core.Attributes) (*Array, error) {
	meta := a.meta
	meta.Attributes = attrs
	return Create(ctx, a.handle, meta, true)
}

func (a *Array) String() string {
	return fmt.Sprintf("<v3.Array {%s} %v %s>", a.handle, a.meta.Shape, a.meta.DataType)
}

// MetadataBuilder constructs v3 array metadata.
type MetadataBuilder struct {
	shape       []int64
	dtype       ndarray.DType
	chunkShape  []int
	fill        any
	codecs      []codec.Codec
	codecFn     func(*CodecBuilder) *CodecBuilder
	attrs       core.Attributes
	dims        []string
	chunkKey    ChunkKeyEnc
	hasChunkKey bool
}

func NewMetadataBuilder() *MetadataBuilder {
	return &MetadataBuilder{fill: 0, attrs: core.Attributes{}, chunkKey: ChunkKeyEnc{Name: "default", Configuration: struct {
		Separator string `json:"separator"`
	}{Separator: "/"}}}
}

func (b *MetadataBuilder) WithShape(shape ...int64) *MetadataBuilder {
	b.shape = shape
	return b
}
func (b *MetadataBuilder) WithDataType(dt ndarray.DType) *MetadataBuilder {
	b.dtype = dt
	return b
}
func (b *MetadataBuilder) WithChunkShape(cs ...int) *MetadataBuilder {
	b.chunkShape = cs
	return b
}
func (b *MetadataBuilder) WithFillValue(v any) *MetadataBuilder {
	b.fill = v
	return b
}
func (b *MetadataBuilder) WithAttributes(a core.Attributes) *MetadataBuilder {
	b.attrs = a
	return b
}
func (b *MetadataBuilder) WithCodecs(fn func(*CodecBuilder) *CodecBuilder) *MetadataBuilder {
	b.codecFn = fn
	return b
}
func (b *MetadataBuilder) WithDefaultChunkKeyEncoding() *MetadataBuilder {
	b.chunkKey = ChunkKeyEnc{Name: "default"}
	b.chunkKey.Configuration.Separator = "/"
	b.hasChunkKey = true
	return b
}
func (b *MetadataBuilder) WithV2ChunkKeyEncoding() *MetadataBuilder {
	b.chunkKey = ChunkKeyEnc{Name: "v2"}
	b.chunkKey.Configuration.Separator = "."
	b.hasChunkKey = true
	return b
}
func (b *MetadataBuilder) WithDimensionNames(names ...string) *MetadataBuilder {
	b.dims = names
	return b
}

func (b *MetadataBuilder) Build() (Metadata, error) {
	if b.shape == nil {
		return Metadata{}, core.NewError("shape needs to be provided")
	}
	if b.dtype == "" {
		return Metadata{}, core.NewError("data type needs to be provided")
	}
	cs := b.chunkShape
	if cs == nil {
		cs = core.DefaultChunkShape(b.shape)
	}
	var codecs []codec.Codec
	if b.codecFn != nil {
		codecs = b.codecFn(NewCodecBuilder(b.dtype)).Build()
	} else if b.codecs != nil {
		codecs = b.codecs
	} else {
		codecs = []codec.Codec{codec.NewBytes(nil)}
	}
	m := Metadata{
		ZarrFormat:       Format,
		NodeType:         "array",
		Shape:            b.shape,
		DataType:         b.dtype,
		ChunkGrid:        ChunkGrid{Name: "regular"},
		ChunkKeyEncoding: b.chunkKey,
		FillValue:        b.fill,
		Codecs:           codecs,
		Attributes:       b.attrs,
		DimensionNames:   b.dims,
	}
	m.ChunkGrid.Configuration.ChunkShape = cs
	if err := m.validate(); err != nil {
		return Metadata{}, err
	}
	if _, err := codec.NewPipeline(codecs, m.ArrayMeta()); err != nil {
		return Metadata{}, err
	}
	return m, nil
}
