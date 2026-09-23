package codec

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
)

// ArrayMeta is the array view seen by a codec in the pipeline.
type ArrayMeta struct {
	Shape      []int64
	ChunkShape []int
	DType      ndarray.DType
	Order      binary.ByteOrder
	Fill       any
}

func (m ArrayMeta) NDim() int { return len(m.Shape) }

func (m ArrayMeta) ChunkSize() int {
	n := 1
	for _, s := range m.ChunkShape {
		n *= s
	}
	return n
}

func (m ArrayMeta) ChunkByteLength() int {
	return m.ChunkSize() * m.DType.Size()
}

func (m ArrayMeta) Clone() ArrayMeta {
	out := m
	out.Shape = append([]int64(nil), m.Shape...)
	out.ChunkShape = append([]int(nil), m.ChunkShape...)
	return out
}

func (m ArrayMeta) AllocateFill() *ndarray.Array {
	a := ndarray.New(m.DType, m.ChunkShape, m.Order)
	if m.Fill != nil {
		a.Fill(m.Fill)
	}
	return a
}

// Kind classifies a codec in the pipeline.
type Kind int

const (
	KindArrayArray Kind = iota
	KindArrayBytes
	KindBytesBytes
)

// Codec is a named codec in a pipeline.
type Codec interface {
	Name() string
	Kind() Kind
	SetMeta(meta ArrayMeta) error
	ResolveMeta() ArrayMeta
	ComputeEncodedSize(n int64) (int64, error)
}

// ArrayArray transforms arrays.
type ArrayArray interface {
	Codec
	EncodeArray(a *ndarray.Array) (*ndarray.Array, error)
	DecodeArray(a *ndarray.Array) (*ndarray.Array, error)
}

// ArrayBytes converts arrays to bytes.
type ArrayBytes interface {
	Codec
	EncodeArrayBytes(a *ndarray.Array) ([]byte, error)
	DecodeArrayBytes(b []byte) (*ndarray.Array, error)
}

// PartialDecoder can decode a region without reading the full chunk.
type PartialDecoder interface {
	ArrayBytes
	DecodePartial(ctx context.Context, h store.Handle, offset []int64, shape []int) (*ndarray.Array, error)
}

// BytesBytes transforms byte buffers.
type BytesBytes interface {
	Codec
	EncodeBytes(b []byte) ([]byte, error)
	DecodeBytes(b []byte) ([]byte, error)
}

type base struct {
	meta ArrayMeta
}

func (b *base) SetMeta(meta ArrayMeta) error {
	b.meta = meta
	return nil
}
func (b *base) ResolveMeta() ArrayMeta { return b.meta }
func (b *base) ComputeEncodedSize(n int64) (int64, error) {
	return 0, fmt.Errorf("codec: encoded size not implemented")
}
