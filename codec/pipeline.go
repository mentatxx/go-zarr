package codec

import (
	"context"
	"fmt"

	"github.com/mentatxx/go-zarr/core"
	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
)

// Pipeline encodes/decodes chunks through a sequence of codecs.
type Pipeline struct {
	Meta       ArrayMeta
	Codecs     []Codec
	arrayArray []ArrayArray
	arrayBytes ArrayBytes
	bytesBytes []BytesBytes
}

// NewPipeline validates and initializes a codec pipeline.
func NewPipeline(codecs []Codec, meta ArrayMeta) (*Pipeline, error) {
	p := &Pipeline{Meta: meta, Codecs: codecs}
	nAB := 0
	var prev Codec
	cur := meta
	for _, c := range codecs {
		if _, ok := c.(ArrayBytes); ok {
			nAB++
		}
		if prev != nil {
			_, prevAB := prev.(ArrayBytes)
			_, prevBB := prev.(BytesBytes)
			_, curAB := c.(ArrayBytes)
			_, curAA := c.(ArrayArray)
			_, curBB := c.(BytesBytes)
			_ = curBB
			if curAB && prevAB {
				return nil, core.NewError("exactly 1 ArrayBytesCodec is required")
			}
			if curAB && prevBB {
				return nil, core.NewError("ArrayBytesCodec cannot follow BytesBytesCodec")
			}
			if curAA && prevAB {
				return nil, core.NewError("ArrayArrayCodec cannot follow ArrayBytesCodec")
			}
			if curAA && prevBB {
				return nil, core.NewError("ArrayArrayCodec cannot follow BytesBytesCodec")
			}
		}
		if err := c.SetMeta(cur); err != nil {
			return nil, err
		}
		cur = c.ResolveMeta()
		prev = c
	}
	if nAB != 1 {
		return nil, core.NewError(fmt.Sprintf("exactly 1 ArrayBytesCodec is required, found %d", nAB))
	}
	for _, c := range codecs {
		switch x := c.(type) {
		case ArrayArray:
			p.arrayArray = append(p.arrayArray, x)
		case ArrayBytes:
			p.arrayBytes = x
		case BytesBytes:
			p.bytesBytes = append(p.bytesBytes, x)
		default:
			return nil, core.NewError(fmt.Sprintf("unknown codec kind %T", c))
		}
	}
	return p, nil
}

// SupportsPartialDecode reports whether a single array-bytes codec supports partial decode.
func (p *Pipeline) SupportsPartialDecode() bool {
	_, ok := p.arrayBytes.(PartialDecoder)
	return ok && len(p.Codecs) == 1
}

// DecodePartial decodes a region of a chunk.
func (p *Pipeline) DecodePartial(ctx context.Context, h store.Handle, offset []int64, shape []int) (*ndarray.Array, error) {
	if !p.SupportsPartialDecode() {
		return nil, core.NewError("partial decode is not supported for these codecs")
	}
	return p.arrayBytes.(PartialDecoder).DecodePartial(ctx, h, offset, shape)
}

// Decode converts encoded bytes into an array.
func (p *Pipeline) Decode(b []byte) (*ndarray.Array, error) {
	if b == nil {
		return nil, core.NewError("chunkBytes is null")
	}
	cur := b
	for i := len(p.bytesBytes) - 1; i >= 0; i-- {
		var err error
		cur, err = p.bytesBytes[i].DecodeBytes(cur)
		if err != nil {
			return nil, err
		}
	}
	arr, err := p.arrayBytes.DecodeArrayBytes(cur)
	if err != nil {
		return nil, err
	}
	for i := len(p.arrayArray) - 1; i >= 0; i-- {
		arr, err = p.arrayArray[i].DecodeArray(arr)
		if err != nil {
			return nil, err
		}
	}
	return arr, nil
}

// Encode converts an array into encoded bytes.
func (p *Pipeline) Encode(a *ndarray.Array) ([]byte, error) {
	arr := a
	var err error
	for _, c := range p.arrayArray {
		arr, err = c.EncodeArray(arr)
		if err != nil {
			return nil, err
		}
	}
	b, err := p.arrayBytes.EncodeArrayBytes(arr)
	if err != nil {
		return nil, err
	}
	for _, c := range p.bytesBytes {
		b, err = c.EncodeBytes(b)
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

// ComputeEncodedSize walks codecs to estimate encoded size when possible.
func (p *Pipeline) ComputeEncodedSize(n int64) (int64, error) {
	var err error
	for _, c := range p.Codecs {
		n, err = c.ComputeEncodedSize(n)
		if err != nil {
			return 0, err
		}
	}
	return n, nil
}
