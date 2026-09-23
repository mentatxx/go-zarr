package codec

import (
	"encoding/binary"
	"encoding/json"

	"github.com/mentatxx/go-zarr/core"
	"github.com/mentatxx/go-zarr/ndarray"
)

// Bytes is the array->bytes endian codec (`bytes`).
type Bytes struct {
	base
	Endian binary.ByteOrder
}

func NewBytes(order binary.ByteOrder) *Bytes {
	if order == nil {
		order = binary.LittleEndian
	}
	return &Bytes{Endian: order}
}

func (c *Bytes) Name() string { return "bytes" }
func (c *Bytes) Kind() Kind   { return KindArrayBytes }

func (c *Bytes) ComputeEncodedSize(n int64) (int64, error) { return n, nil }

func (c *Bytes) EncodeArrayBytes(a *ndarray.Array) ([]byte, error) {
	return a.EncodeBytes(c.order()), nil
}

func (c *Bytes) DecodeArrayBytes(b []byte) (*ndarray.Array, error) {
	return ndarray.DecodeBytes(c.meta.DType, c.meta.ChunkShape, c.order(), b)
}

func (c *Bytes) order() binary.ByteOrder {
	if c.meta.DType.Size() <= 1 {
		return binary.BigEndian
	}
	if c.Endian == nil {
		return binary.LittleEndian
	}
	return c.Endian
}

func (c *Bytes) MarshalJSON() ([]byte, error) {
	type conf struct {
		Endian string `json:"endian,omitempty"`
	}
	cfg := conf{}
	if c.order() == binary.BigEndian {
		cfg.Endian = "big"
	} else {
		cfg.Endian = "little"
	}
	return json.Marshal(struct {
		Name          string `json:"name"`
		Configuration conf   `json:"configuration"`
	}{Name: "bytes", Configuration: cfg})
}

func ParseEndian(s string) (binary.ByteOrder, error) {
	switch s {
	case "", "little", "LITTLE":
		return binary.LittleEndian, nil
	case "big", "BIG":
		return binary.BigEndian, nil
	default:
		return nil, core.NewError("unknown endian " + s)
	}
}
