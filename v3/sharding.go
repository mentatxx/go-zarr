package v3

import (
	"context"
	"encoding/binary"
	"encoding/json"

	"github.com/mentatxx/go-zarr/codec"
	"github.com/mentatxx/go-zarr/core"
	"github.com/mentatxx/go-zarr/internal/indexing"
	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
)

// Sharding is the sharding_indexed array->bytes codec.
type Sharding struct {
	meta          codec.ArrayMeta
	InnerShape    []int
	Inner         []codec.Codec
	Index         []codec.Codec
	IndexLocation string
	innerPipe     *codec.Pipeline
	indexPipe     *codec.Pipeline
}

func NewSharding(chunkShape []int, inner, index []codec.Codec, loc string) (*Sharding, error) {
	if loc == "" {
		loc = "end"
	}
	if loc != "start" && loc != "end" {
		return nil, core.NewError("only index_location start or end are supported")
	}
	return &Sharding{InnerShape: chunkShape, Inner: inner, Index: index, IndexLocation: loc}, nil
}

func (c *Sharding) Name() string     { return "sharding_indexed" }
func (c *Sharding) Kind() codec.Kind { return codec.KindArrayBytes }

func (c *Sharding) SetMeta(meta codec.ArrayMeta) error {
	if len(c.InnerShape) != len(meta.ChunkShape) {
		return core.NewError("sharding: inner chunk shape rank mismatch")
	}
	for i, inner := range c.InnerShape {
		if inner <= 0 || meta.ChunkShape[i]%inner != 0 {
			return core.NewError("sharding: outer chunk shape must be a multiple of inner chunk shape")
		}
	}
	c.meta = meta
	innerMeta := codec.ArrayMeta{
		Shape:      indexing.IntsToInt64(meta.ChunkShape),
		ChunkShape: append([]int(nil), c.InnerShape...),
		DType:      meta.DType,
		Order:      meta.Order,
		Fill:       meta.Fill,
	}
	var err error
	c.innerPipe, err = codec.NewPipeline(c.Inner, innerMeta)
	if err != nil {
		return err
	}
	cps := c.chunksPerShard()
	indexShape := append(append([]int{}, cps...), 2)
	indexMeta := codec.ArrayMeta{
		Shape:      indexing.IntsToInt64(indexShape),
		ChunkShape: indexShape,
		DType:      ndarray.Uint64,
		Order:      binary.LittleEndian,
		Fill:       uint64(^uint64(0)), // -1 as uint64
	}
	c.indexPipe, err = codec.NewPipeline(c.Index, indexMeta)
	return err
}

func (c *Sharding) ResolveMeta() codec.ArrayMeta { return c.meta }

func (c *Sharding) chunksPerShard() []int {
	out := make([]int, len(c.meta.ChunkShape))
	for i := range out {
		out[i] = c.meta.ChunkShape[i] / c.InnerShape[i]
	}
	return out
}

func (c *Sharding) indexSize() (int64, error) {
	n := 1
	for _, v := range c.chunksPerShard() {
		n *= v
	}
	return c.indexPipe.ComputeEncodedSize(16 * int64(n))
}

func (c *Sharding) ComputeEncodedSize(n int64) (int64, error) {
	idx, err := c.indexSize()
	if err != nil {
		return 0, err
	}
	return n + idx, nil
}

func (c *Sharding) EncodeArrayBytes(shard *ndarray.Array) ([]byte, error) {
	cps := c.chunksPerShard()
	indexShape := append(append([]int{}, cps...), 2)
	index := ndarray.New(ndarray.Uint64, indexShape, binary.LittleEndian)
	nInner := 1
	for _, v := range cps {
		nInner *= v
	}
	coords, err := indexing.ComputeChunkCoords(c.innerPipe.Meta.Shape, c.innerPipe.Meta.ChunkShape, nil, nil)
	if err != nil {
		return nil, err
	}
	var chunks [][]byte
	idxSize, err := c.indexSize()
	if err != nil {
		return nil, err
	}
	for _, cc := range coords {
		proj, err := indexing.ComputeProjection(cc, c.innerPipe.Meta.Shape, c.innerPipe.Meta.ChunkShape, nil, nil)
		if err != nil {
			return nil, err
		}
		inner, err := shard.Section(proj.OutOffset, proj.Shape)
		if err != nil {
			return nil, err
		}
		lin := shardIndex(cc, cps)
		if c.meta.Fill != nil && inner.AllValuesEqual(c.meta.Fill) {
			index.SetUint64(lin*2, ^uint64(0))
			index.SetUint64(lin*2+1, ^uint64(0))
			continue
		}
		b, err := c.innerPipe.Encode(inner)
		if err != nil {
			return nil, err
		}
		off := 0
		for _, ch := range chunks {
			off += len(ch)
		}
		if c.IndexLocation == "start" {
			off += int(idxSize)
		}
		index.SetUint64(lin*2, uint64(off))
		index.SetUint64(lin*2+1, uint64(len(b)))
		chunks = append(chunks, b)
	}
	idxBytes, err := c.indexPipe.Encode(index)
	if err != nil {
		return nil, err
	}
	total := len(idxBytes)
	for _, ch := range chunks {
		total += len(ch)
	}
	out := make([]byte, 0, total)
	if c.IndexLocation == "start" {
		out = append(out, idxBytes...)
	}
	for _, ch := range chunks {
		out = append(out, ch...)
	}
	if c.IndexLocation != "start" {
		out = append(out, idxBytes...)
	}
	return out, nil
}

func (c *Sharding) DecodeArrayBytes(b []byte) (*ndarray.Array, error) {
	return c.decodeBuf(b, make([]int64, c.meta.NDim()), c.meta.ChunkShape)
}

func (c *Sharding) DecodePartial(ctx context.Context, h store.Handle, offset []int64, shape []int) (*ndarray.Array, error) {
	data, err := h.Read(ctx)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return c.meta.AllocateFill(), nil
	}
	return c.decodeBuf(data, offset, shape)
}

func (c *Sharding) decodeBuf(b []byte, offset []int64, shape []int) (*ndarray.Array, error) {
	out := ndarray.New(c.meta.DType, shape, c.meta.Order)
	if c.meta.Fill != nil {
		out.Fill(c.meta.Fill)
	}
	idxSize, err := c.indexSize()
	if err != nil {
		return nil, err
	}
	if int64(len(b)) < idxSize {
		return out, nil
	}
	var idxBytes []byte
	if c.IndexLocation == "start" {
		idxBytes = b[:idxSize]
	} else {
		idxBytes = b[len(b)-int(idxSize):]
	}
	index, err := c.indexPipe.Decode(idxBytes)
	if err != nil {
		return nil, err
	}
	cps := c.chunksPerShard()
	coords, err := indexing.ComputeChunkCoords(c.innerPipe.Meta.Shape, c.innerPipe.Meta.ChunkShape, offset, indexing.IntsToInt64(shape))
	if err != nil {
		return nil, err
	}
	for _, cc := range coords {
		lin := shardIndex(cc, cps)
		byteOff := index.GetUint64(lin * 2)
		byteLen := index.GetUint64(lin*2 + 1)
		if byteOff == ^uint64(0) || byteLen == ^uint64(0) {
			continue
		}
		chunkBytes := b[byteOff : byteOff+byteLen]
		inner, err := c.innerPipe.Decode(chunkBytes)
		if err != nil {
			return nil, err
		}
		proj, err := indexing.ComputeProjection(cc, c.innerPipe.Meta.Shape, c.innerPipe.Meta.ChunkShape, offset, indexing.IntsToInt64(shape))
		if err != nil {
			return nil, err
		}
		if err := inner.CopyRegion(proj.ChunkOffset, out, proj.OutOffset, proj.Shape); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func shardIndex(coords []int64, cps []int) int {
	lin := 0
	for i, c := range coords {
		lin = lin*cps[i] + int(c)
	}
	return lin
}

func (c *Sharding) MarshalJSON() ([]byte, error) {
	inner := marshalCodecList(c.Inner)
	index := marshalCodecList(c.Index)
	return json.Marshal(map[string]any{
		"name": "sharding_indexed",
		"configuration": map[string]any{
			"chunk_shape":    c.InnerShape,
			"codecs":         jsonRaw(inner),
			"index_codecs":   jsonRaw(index),
			"index_location": c.IndexLocation,
		},
	})
}

func marshalCodecList(cs []codec.Codec) []json.RawMessage {
	var out []json.RawMessage
	for _, c := range cs {
		if m, ok := c.(json.Marshaler); ok {
			b, _ := m.MarshalJSON()
			out = append(out, b)
		}
	}
	return out
}

func jsonRaw(in []json.RawMessage) []json.RawMessage { return in }
