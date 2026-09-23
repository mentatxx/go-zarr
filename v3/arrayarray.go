package v3

import (
	"encoding/json"

	"github.com/mentatxx/go-zarr/codec"
	"github.com/mentatxx/go-zarr/core"
	"github.com/mentatxx/go-zarr/ndarray"
)

// Transpose is the array->array transpose codec.
type Transpose struct {
	meta  codec.ArrayMeta
	Order []int
}

func NewTranspose(order []int) *Transpose { return &Transpose{Order: order} }

func (c *Transpose) Name() string { return "transpose" }
func (c *Transpose) Kind() codec.Kind {
	return codec.KindArrayArray
}
func (c *Transpose) SetMeta(meta codec.ArrayMeta) error {
	c.meta = meta
	n := len(meta.ChunkShape)
	if len(c.Order) != n {
		return core.NewError("transpose: order length must match rank")
	}
	seen := make([]bool, n)
	for _, o := range c.Order {
		if o < 0 || o >= n || seen[o] {
			return core.NewError("transpose: invalid order")
		}
		seen[o] = true
	}
	return nil
}
func (c *Transpose) ResolveMeta() codec.ArrayMeta {
	out := c.meta.Clone()
	cs := make([]int, len(c.Order))
	for i, o := range c.Order {
		cs[i] = c.meta.ChunkShape[o]
	}
	out.ChunkShape = cs
	return out
}
func (c *Transpose) ComputeEncodedSize(n int64) (int64, error) { return n, nil }

func (c *Transpose) EncodeArray(a *ndarray.Array) (*ndarray.Array, error) {
	return a.Transpose(c.Order)
}
func (c *Transpose) DecodeArray(a *ndarray.Array) (*ndarray.Array, error) {
	inv := make([]int, len(c.Order))
	for i, o := range c.Order {
		inv[o] = i
	}
	return a.Transpose(inv)
}

func (c *Transpose) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"name":          "transpose",
		"configuration": map[string]any{"order": c.Order},
	})
}

// Reshape is the array->array reshape codec.
type Reshape struct {
	meta     codec.ArrayMeta
	Spec     []any
	outShape []int
}

func NewReshape(spec []any) *Reshape { return &Reshape{Spec: spec} }

func (c *Reshape) Name() string                              { return "reshape" }
func (c *Reshape) Kind() codec.Kind                          { return codec.KindArrayArray }
func (c *Reshape) ComputeEncodedSize(n int64) (int64, error) { return n, nil }

func (c *Reshape) SetMeta(meta codec.ArrayMeta) error {
	c.meta = meta
	sh, err := resolveReshape(meta.ChunkShape, c.Spec)
	if err != nil {
		return err
	}
	c.outShape = sh
	return nil
}
func (c *Reshape) ResolveMeta() codec.ArrayMeta {
	out := c.meta.Clone()
	out.ChunkShape = append([]int(nil), c.outShape...)
	return out
}
func (c *Reshape) EncodeArray(a *ndarray.Array) (*ndarray.Array, error) {
	return a.Reshape(c.outShape)
}
func (c *Reshape) DecodeArray(a *ndarray.Array) (*ndarray.Array, error) {
	return a.Reshape(c.meta.ChunkShape)
}
func (c *Reshape) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"name":          "reshape",
		"configuration": map[string]any{"shape": c.Spec},
	})
}

func resolveReshape(in []int, spec []any) ([]int, error) {
	prodIn := 1
	for _, s := range in {
		prodIn *= s
	}
	out := make([]int, len(spec))
	neg := -1
	known := 1
	for i, s := range spec {
		switch v := s.(type) {
		case float64:
			n := int(v)
			if n == -1 {
				if neg >= 0 {
					return nil, core.NewError("reshape: at most one -1")
				}
				neg = i
				continue
			}
			out[i] = n
			known *= n
		case json.Number:
			n64, _ := v.Int64()
			n := int(n64)
			if n == -1 {
				neg = i
				continue
			}
			out[i] = n
			known *= n
		case []any:
			p := 1
			for _, d := range v {
				idx := int(d.(float64))
				p *= in[idx]
			}
			out[i] = p
			known *= p
		default:
			return nil, core.NewError("reshape: invalid shape entry")
		}
	}
	if neg >= 0 {
		if known == 0 || prodIn%known != 0 {
			return nil, core.NewError("reshape: cannot infer dimension")
		}
		out[neg] = prodIn / known
	}
	return out, nil
}

// CastConfig configures cast_value.
type CastConfig struct {
	DataType   ndarray.DType `json:"data_type"`
	Rounding   string        `json:"rounding,omitempty"`
	OutOfRange string        `json:"out_of_range,omitempty"`
}

// Cast is a simplified cast_value codec.
type Cast struct {
	meta   codec.ArrayMeta
	Config CastConfig
}

func NewCast(cfg CastConfig) *Cast { return &Cast{Config: cfg} }

func (c *Cast) Name() string     { return "cast_value" }
func (c *Cast) Kind() codec.Kind { return codec.KindArrayArray }
func (c *Cast) SetMeta(meta codec.ArrayMeta) error {
	c.meta = meta
	return nil
}
func (c *Cast) ResolveMeta() codec.ArrayMeta {
	out := c.meta.Clone()
	out.DType = c.Config.DataType
	if out.Fill != nil {
		f, _ := core.ParseFillValue(out.Fill, out.DType)
		out.Fill = f
	}
	return out
}
func (c *Cast) ComputeEncodedSize(n int64) (int64, error) {
	elems := n / int64(c.meta.DType.Size())
	return elems * int64(c.Config.DataType.Size()), nil
}
func (c *Cast) EncodeArray(a *ndarray.Array) (*ndarray.Array, error) {
	return castArray(a, c.Config.DataType)
}
func (c *Cast) DecodeArray(a *ndarray.Array) (*ndarray.Array, error) {
	return castArray(a, c.meta.DType)
}
func (c *Cast) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"name":          "cast_value",
		"configuration": c.Config,
	})
}

func castArray(a *ndarray.Array, dt ndarray.DType) (*ndarray.Array, error) {
	out := ndarray.New(dt, a.Shape, a.Order)
	n := a.Len()
	for i := 0; i < n; i++ {
		switch {
		case dt.IsFloat() || a.DType.IsFloat():
			out.SetFloat64(i, a.GetFloat64(i))
		case dt.IsUnsigned():
			out.SetUint64(i, a.GetUint64(i))
		default:
			out.SetInt64(i, a.GetInt64(i))
		}
	}
	return out, nil
}
