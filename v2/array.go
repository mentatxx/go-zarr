package v2

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/mentatxx/go-zarr/codec"
	"github.com/mentatxx/go-zarr/core"
	"github.com/mentatxx/go-zarr/internal/engine"
	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
)

// Array is a Zarr v2 array.
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
	return &Array{handle: h, meta: meta, eng: &engine.Array{
		Handle:    h,
		Meta:      meta.ArrayMeta(),
		Pipeline:  pipe,
		EncodeKey: meta.EncodeChunkKey,
		Parallel:  true,
	}}, nil
}

func Open(ctx context.Context, h store.Handle) (*Array, error) {
	b, err := h.Resolve(ZArray).Read(ctx)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, core.ErrNotFound
	}
	var meta Metadata
	if err := json.Unmarshal(b, &meta); err != nil {
		return nil, err
	}
	attrs, _ := h.Resolve(ZAttrs).Read(ctx)
	if attrs != nil {
		_ = json.Unmarshal(attrs, &meta.Attributes)
	}
	return newArray(h, meta)
}

func OpenPath(ctx context.Context, path string) (*Array, error) {
	return Open(ctx, store.NewFilesystem(path).Resolve())
}

func Create(ctx context.Context, h store.Handle, meta Metadata, existsOk bool) (*Array, error) {
	meta.ZarrFormat = Format
	if meta.Order == "" {
		meta.Order = "C"
	}
	if meta.DType == "" && meta.ParsedDType.DType != "" {
		meta.DType = ndarray.FormatV2(meta.ParsedDType.DType, meta.ParsedDType.Order)
	}
	spec, err := ndarray.ParseV2(meta.DType)
	if err != nil {
		return nil, err
	}
	meta.ParsedDType = spec
	meta.ParsedFill, err = core.ParseFillValue(meta.FillValue, spec.DType)
	if err != nil {
		return nil, err
	}
	if len(meta.Codecs) == 0 {
		meta.Codecs, err = v2Codecs(&meta, spec)
		if err != nil {
			return nil, err
		}
	}
	mh := h.Resolve(ZArray)
	if !existsOk {
		ok, err := mh.Exists(ctx)
		if err != nil {
			return nil, err
		}
		if ok {
			return nil, core.ErrExists
		}
	}
	raw, err := json.MarshalIndent(&meta, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := mh.Set(ctx, raw); err != nil {
		return nil, err
	}
	if meta.Attributes != nil {
		ab, _ := json.MarshalIndent(meta.Attributes, "", "  ")
		_ = h.Resolve(ZAttrs).Set(ctx, ab)
	}
	return newArray(h, meta)
}

func (a *Array) Read(ctx context.Context, offset, shape []int64) (*ndarray.Array, error) {
	return a.eng.Read(ctx, offset, shape)
}
func (a *Array) Write(ctx context.Context, offset []int64, data *ndarray.Array) error {
	return a.eng.Write(ctx, offset, data)
}
func (a *Array) SetParallel(p bool) { a.eng.Parallel = p }

func (a *Array) Resize(ctx context.Context, newShape []int64, metadataOnly bool) (*Array, error) {
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
	return fmt.Sprintf("<v2.Array {%s} %v %s>", a.handle, a.meta.Shape, a.meta.DType)
}

type MetadataBuilder struct {
	shape      []int64
	chunks     []int
	dtype      string
	fill       any
	attrs      core.Attributes
	compressor codec.Codec
	sep        string
	order      string
}

func NewMetadataBuilder() *MetadataBuilder {
	return &MetadataBuilder{fill: 0, attrs: core.Attributes{}, order: "C", sep: "."}
}

func (b *MetadataBuilder) WithShape(s ...int64) *MetadataBuilder { b.shape = s; return b }
func (b *MetadataBuilder) WithChunks(c ...int) *MetadataBuilder  { b.chunks = c; return b }
func (b *MetadataBuilder) WithDataType(dt string) *MetadataBuilder {
	b.dtype = dt
	return b
}
func (b *MetadataBuilder) WithDataTypeSpec(dt ndarray.DType, bigEndian bool) *MetadataBuilder {
	var order binary.ByteOrder = binary.LittleEndian
	if bigEndian {
		order = binary.BigEndian
	}
	b.dtype = ndarray.FormatV2(dt, order)
	return b
}
func (b *MetadataBuilder) WithFillValue(v any) *MetadataBuilder { b.fill = v; return b }
func (b *MetadataBuilder) WithAttributes(a core.Attributes) *MetadataBuilder {
	b.attrs = a
	return b
}
func (b *MetadataBuilder) WithBloscCompressor(cname, shuffle string, clevel int) *MetadataBuilder {
	c, err := codec.NewBlosc(cname, shuffle, clevel, 1, 0)
	if err != nil {
		panic(err)
	}
	b.compressor = c
	return b
}
func (b *MetadataBuilder) WithGzipCompressor(level int) *MetadataBuilder {
	c, err := codec.NewGzip(level)
	if err != nil {
		panic(err)
	}
	b.compressor = c
	return b
}
func (b *MetadataBuilder) WithZlibCompressor(level int) *MetadataBuilder {
	c, err := codec.NewZlib(level)
	if err != nil {
		panic(err)
	}
	b.compressor = c
	return b
}
func (b *MetadataBuilder) WithZstdCompressor(level int, checksum bool) *MetadataBuilder {
	c, err := codec.NewZstd(level, checksum)
	if err != nil {
		panic(err)
	}
	b.compressor = c
	return b
}

func (b *MetadataBuilder) Build() (Metadata, error) {
	if b.chunks == nil {
		b.chunks = core.DefaultChunkShape(b.shape)
	}
	compRaw, _ := json.Marshal(b.compressor)
	if b.compressor == nil {
		compRaw = []byte("null")
	} else {
		compRaw = v2CompressorJSON(b.compressor)
	}
	m := Metadata{
		ZarrFormat:         Format,
		Shape:              b.shape,
		Chunks:             b.chunks,
		DType:              b.dtype,
		FillValue:          b.fill,
		Order:              b.order,
		Compressor:         compRaw,
		DimensionSeparator: b.sep,
		Attributes:         b.attrs,
	}
	spec, err := ndarray.ParseV2(b.dtype)
	if err != nil {
		return Metadata{}, err
	}
	if bl, ok := b.compressor.(*codec.Blosc); ok {
		bl.TypeSize = spec.DType.Size()
		if bl.TypeSize < 1 {
			bl.TypeSize = 1
		}
		m.Compressor = v2CompressorJSON(bl)
	}
	m.ParsedDType = spec
	m.ParsedFill, err = core.ParseFillValue(b.fill, spec.DType)
	if err != nil {
		return Metadata{}, err
	}
	m.Codecs, err = v2Codecs(&m, spec)
	return m, err
}

func v2CompressorJSON(c codec.Codec) json.RawMessage {
	switch x := c.(type) {
	case *codec.Gzip:
		b, _ := json.Marshal(map[string]any{"id": "gzip", "level": x.Level})
		return b
	case *codec.Zlib:
		b, _ := json.Marshal(map[string]any{"id": "zlib", "level": x.Level})
		return b
	case *codec.Zstd:
		b, _ := json.Marshal(map[string]any{"id": "zstd", "level": x.Level, "checksum": x.Checksum})
		return b
	case *codec.Blosc:
		sh := 0
		switch x.Shuffle {
		case "shuffle":
			sh = 1
		case "bitshuffle":
			sh = 2
		}
		b, _ := json.Marshal(map[string]any{"id": "blosc", "cname": x.CName, "clevel": x.CLevel, "shuffle": sh, "blocksize": x.BlockSize, "typesize": x.TypeSize})
		return b
	}
	b, _ := json.Marshal(c)
	return b
}
