package ndarray

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Array is a C-order (row-major) N-dimensional typed array backed by []byte.
type Array struct {
	DType DType
	Shape []int
	Data  []byte
	Order binary.ByteOrder
}

// New allocates a zeroed array.
func New(dtype DType, shape []int, order binary.ByteOrder) *Array {
	if order == nil {
		order = binary.LittleEndian
	}
	n := 1
	for _, s := range shape {
		n *= s
	}
	return &Array{
		DType: dtype,
		Shape: append([]int(nil), shape...),
		Data:  make([]byte, n*dtype.Size()),
		Order: order,
	}
}

// FromBytes wraps existing bytes. Data is not copied.
func FromBytes(dtype DType, shape []int, order binary.ByteOrder, data []byte) (*Array, error) {
	if order == nil {
		order = binary.LittleEndian
	}
	n := 1
	for _, s := range shape {
		n *= s
	}
	need := n * dtype.Size()
	if len(data) != need {
		return nil, fmt.Errorf("ndarray: data length %d != expected %d", len(data), need)
	}
	return &Array{
		DType: dtype,
		Shape: append([]int(nil), shape...),
		Data:  data,
		Order: order,
	}, nil
}

// NDim returns the number of dimensions.
func (a *Array) NDim() int { return len(a.Shape) }

// Len returns the number of elements.
func (a *Array) Len() int {
	n := 1
	for _, s := range a.Shape {
		n *= s
	}
	return n
}

// Clone returns a deep copy.
func (a *Array) Clone() *Array {
	out := New(a.DType, a.Shape, a.Order)
	copy(out.Data, a.Data)
	return out
}

func (a *Array) strides() []int {
	n := len(a.Shape)
	st := make([]int, n)
	if n == 0 {
		return st
	}
	st[n-1] = a.DType.Size()
	for i := n - 2; i >= 0; i-- {
		st[i] = st[i+1] * a.Shape[i+1]
	}
	return st
}

func (a *Array) byteOffset(indices []int) int {
	st := a.strides()
	off := 0
	for i, idx := range indices {
		off += idx * st[i]
	}
	return off
}

func (a *Array) linearOffset(i int) int {
	return i * a.DType.Size()
}

// GetBool returns the boolean at linear index i.
func (a *Array) GetBool(i int) bool { return a.Data[a.linearOffset(i)] != 0 }

// SetBool sets the boolean at linear index i.
func (a *Array) SetBool(i int, v bool) {
	if v {
		a.Data[a.linearOffset(i)] = 1
	} else {
		a.Data[a.linearOffset(i)] = 0
	}
}

func (a *Array) getRaw(i int) []byte {
	off := a.linearOffset(i)
	return a.Data[off : off+a.DType.Size()]
}

// GetInt64 reads element i converted to int64.
func (a *Array) GetInt64(i int) int64 {
	b := a.getRaw(i)
	switch a.DType {
	case Bool:
		if b[0] != 0 {
			return 1
		}
		return 0
	case Int8:
		return int64(int8(b[0]))
	case Uint8:
		return int64(b[0])
	case Int16:
		return int64(int16(a.Order.Uint16(b)))
	case Uint16:
		return int64(a.Order.Uint16(b))
	case Int32:
		return int64(int32(a.Order.Uint32(b)))
	case Uint32:
		return int64(a.Order.Uint32(b))
	case Int64:
		return int64(a.Order.Uint64(b))
	case Uint64:
		return int64(a.Order.Uint64(b))
	case Float32:
		return int64(math.Float32frombits(a.Order.Uint32(b)))
	case Float64:
		return int64(math.Float64frombits(a.Order.Uint64(b)))
	}
	return 0
}

// GetUint64 reads element i converted to uint64.
func (a *Array) GetUint64(i int) uint64 {
	b := a.getRaw(i)
	switch a.DType {
	case Bool:
		if b[0] != 0 {
			return 1
		}
		return 0
	case Int8:
		return uint64(int8(b[0]))
	case Uint8:
		return uint64(b[0])
	case Int16:
		return uint64(int16(a.Order.Uint16(b)))
	case Uint16:
		return uint64(a.Order.Uint16(b))
	case Int32:
		return uint64(int32(a.Order.Uint32(b)))
	case Uint32:
		return uint64(a.Order.Uint32(b))
	case Int64:
		return a.Order.Uint64(b)
	case Uint64:
		return a.Order.Uint64(b)
	case Float32:
		return uint64(math.Float32frombits(a.Order.Uint32(b)))
	case Float64:
		return uint64(math.Float64frombits(a.Order.Uint64(b)))
	}
	return 0
}

// GetFloat64 reads element i converted to float64.
func (a *Array) GetFloat64(i int) float64 {
	b := a.getRaw(i)
	switch a.DType {
	case Float32:
		return float64(math.Float32frombits(a.Order.Uint32(b)))
	case Float64:
		return math.Float64frombits(a.Order.Uint64(b))
	case Bool:
		if b[0] != 0 {
			return 1
		}
		return 0
	default:
		if a.DType.IsUnsigned() {
			return float64(a.GetUint64(i))
		}
		return float64(a.GetInt64(i))
	}
}

// SetInt64 stores v at linear index i, converting to the array dtype.
func (a *Array) SetInt64(i int, v int64) {
	b := a.getRaw(i)
	switch a.DType {
	case Bool:
		if v != 0 {
			b[0] = 1
		} else {
			b[0] = 0
		}
	case Int8, Uint8:
		b[0] = byte(v)
	case Int16, Uint16:
		a.Order.PutUint16(b, uint16(v))
	case Int32, Uint32:
		a.Order.PutUint32(b, uint32(v))
	case Int64, Uint64:
		a.Order.PutUint64(b, uint64(v))
	case Float32:
		a.Order.PutUint32(b, math.Float32bits(float32(v)))
	case Float64:
		a.Order.PutUint64(b, math.Float64bits(float64(v)))
	}
}

// SetUint64 stores v at linear index i.
func (a *Array) SetUint64(i int, v uint64) {
	b := a.getRaw(i)
	switch a.DType {
	case Bool:
		if v != 0 {
			b[0] = 1
		} else {
			b[0] = 0
		}
	case Int8, Uint8:
		b[0] = byte(v)
	case Int16, Uint16:
		a.Order.PutUint16(b, uint16(v))
	case Int32, Uint32:
		a.Order.PutUint32(b, uint32(v))
	case Int64, Uint64:
		a.Order.PutUint64(b, v)
	case Float32:
		a.Order.PutUint32(b, math.Float32bits(float32(v)))
	case Float64:
		a.Order.PutUint64(b, math.Float64bits(float64(v)))
	}
}

// SetFloat64 stores v at linear index i.
func (a *Array) SetFloat64(i int, v float64) {
	b := a.getRaw(i)
	switch a.DType {
	case Float32:
		a.Order.PutUint32(b, math.Float32bits(float32(v)))
	case Float64:
		a.Order.PutUint64(b, math.Float64bits(v))
	case Bool:
		if v != 0 {
			b[0] = 1
		} else {
			b[0] = 0
		}
	default:
		a.SetInt64(i, int64(v))
	}
}

// SetValue stores a Go value of the matching dtype at linear index i.
func (a *Array) SetValue(i int, v any) {
	switch a.DType {
	case Bool:
		a.SetBool(i, asBool(v))
	case Int8, Int16, Int32, Int64:
		a.SetInt64(i, asInt64(v))
	case Uint8, Uint16, Uint32, Uint64:
		a.SetUint64(i, asUint64(v))
	case Float32, Float64:
		a.SetFloat64(i, asFloat64(v))
	}
}

// GetValue returns a typed Go value for linear index i.
func (a *Array) GetValue(i int) any {
	switch a.DType {
	case Bool:
		return a.GetBool(i)
	case Int8:
		return int8(a.GetInt64(i))
	case Int16:
		return int16(a.GetInt64(i))
	case Int32:
		return int32(a.GetInt64(i))
	case Int64:
		return a.GetInt64(i)
	case Uint8:
		return uint8(a.GetUint64(i))
	case Uint16:
		return uint16(a.GetUint64(i))
	case Uint32:
		return uint32(a.GetUint64(i))
	case Uint64:
		return a.GetUint64(i)
	case Float32:
		return float32(a.GetFloat64(i))
	case Float64:
		return a.GetFloat64(i)
	}
	return nil
}

func asBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case int:
		return x != 0
	case int8:
		return x != 0
	case int16:
		return x != 0
	case int32:
		return x != 0
	case int64:
		return x != 0
	case uint8:
		return x != 0
	case uint16:
		return x != 0
	case uint32:
		return x != 0
	case uint64:
		return x != 0
	case float32:
		return x != 0
	case float64:
		return x != 0
	}
	return false
}

func asInt64(v any) int64 {
	switch x := v.(type) {
	case int:
		return int64(x)
	case int8:
		return int64(x)
	case int16:
		return int64(x)
	case int32:
		return int64(x)
	case int64:
		return x
	case uint8:
		return int64(x)
	case uint16:
		return int64(x)
	case uint32:
		return int64(x)
	case uint64:
		return int64(x)
	case float32:
		return int64(x)
	case float64:
		return int64(x)
	case bool:
		if x {
			return 1
		}
		return 0
	}
	return 0
}

func asUint64(v any) uint64 {
	switch x := v.(type) {
	case uint64:
		return x
	case uint:
		return uint64(x)
	case uint8:
		return uint64(x)
	case uint16:
		return uint64(x)
	case uint32:
		return uint64(x)
	case int:
		return uint64(x)
	case int8:
		return uint64(x)
	case int16:
		return uint64(x)
	case int32:
		return uint64(x)
	case int64:
		return uint64(x)
	case float32:
		return uint64(x)
	case float64:
		return uint64(x)
	case bool:
		if x {
			return 1
		}
		return 0
	}
	return 0
}

func asFloat64(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int8:
		return float64(x)
	case int16:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case uint8:
		return float64(x)
	case uint16:
		return float64(x)
	case uint32:
		return float64(x)
	case uint64:
		return float64(x)
	case bool:
		if x {
			return 1
		}
		return 0
	}
	return 0
}

func valuesEqual(dt DType, a, b any) bool {
	switch dt {
	case Bool:
		return asBool(a) == asBool(b)
	case Float32:
		fa, fb := float32(asFloat64(a)), float32(asFloat64(b))
		if math.IsNaN(float64(fa)) && math.IsNaN(float64(fb)) {
			return true
		}
		return fa == fb
	case Float64:
		fa, fb := asFloat64(a), asFloat64(b)
		if math.IsNaN(fa) && math.IsNaN(fb) {
			return true
		}
		return fa == fb
	case Uint8, Uint16, Uint32, Uint64:
		return asUint64(a) == asUint64(b)
	default:
		return asInt64(a) == asInt64(b)
	}
}

// Fill fills every element with v.
func (a *Array) Fill(v any) {
	n := a.Len()
	for i := 0; i < n; i++ {
		a.SetValue(i, v)
	}
}

// AllValuesEqual reports whether every element equals v.
func (a *Array) AllValuesEqual(v any) bool {
	n := a.Len()
	for i := 0; i < n; i++ {
		if !valuesEqual(a.DType, a.GetValue(i), v) {
			return false
		}
	}
	return true
}

// Equal reports whether a and b have the same shape, dtype, and values.
func (a *Array) Equal(b *Array) bool {
	if a.DType != b.DType || len(a.Shape) != len(b.Shape) {
		return false
	}
	for i := range a.Shape {
		if a.Shape[i] != b.Shape[i] {
			return false
		}
	}
	n := a.Len()
	for i := 0; i < n; i++ {
		if !valuesEqual(a.DType, a.GetValue(i), b.GetValue(i)) {
			return false
		}
	}
	return true
}

// CopyRegion copies a rectangular region from a into dst (same dtypes).
func (a *Array) CopyRegion(srcOff []int, dst *Array, dstOff []int, shape []int) error {
	if a.DType != dst.DType {
		return fmt.Errorf("ndarray: dtype mismatch %s vs %s", a.DType, dst.DType)
	}
	ndim := len(shape)
	if len(srcOff) != ndim || len(dstOff) != ndim || a.NDim() != ndim || dst.NDim() != ndim {
		return fmt.Errorf("ndarray: rank mismatch")
	}
	for i := 0; i < ndim; i++ {
		if srcOff[i]+shape[i] > a.Shape[i] {
			return fmt.Errorf("ndarray: source region out of bounds")
		}
		if dstOff[i]+shape[i] > dst.Shape[i] {
			return fmt.Errorf("ndarray: dest region out of bounds")
		}
	}
	item := a.DType.Size()
	srcSt := a.strides()
	dstSt := dst.strides()
	idx := make([]int, ndim)
	for {
		srcPos := 0
		dstPos := 0
		for i := 0; i < ndim; i++ {
			srcPos += (srcOff[i] + idx[i]) * srcSt[i]
			dstPos += (dstOff[i] + idx[i]) * dstSt[i]
		}
		copy(dst.Data[dstPos:dstPos+item], a.Data[srcPos:srcPos+item])
		d := ndim - 1
		for d >= 0 {
			idx[d]++
			if idx[d] < shape[d] {
				break
			}
			idx[d] = 0
			d--
		}
		if d < 0 {
			break
		}
	}
	return nil
}

// Section copies a rectangular subset into a new contiguous array.
func (a *Array) Section(offset, shape []int) (*Array, error) {
	out := New(a.DType, shape, a.Order)
	zeros := make([]int, len(shape))
	if err := a.CopyRegion(offset, out, zeros, shape); err != nil {
		return nil, err
	}
	return out, nil
}

// Reshape returns a view-like copy with a new shape (same number of elements).
func (a *Array) Reshape(shape []int) (*Array, error) {
	n := 1
	neg := -1
	for i, s := range shape {
		if s == -1 {
			if neg >= 0 {
				return nil, fmt.Errorf("ndarray: at most one -1 in reshape")
			}
			neg = i
			continue
		}
		if s < 0 {
			return nil, fmt.Errorf("ndarray: invalid reshape dim %d", s)
		}
		n *= s
	}
	outShape := append([]int(nil), shape...)
	if neg >= 0 {
		if a.Len()%n != 0 {
			return nil, fmt.Errorf("ndarray: cannot infer reshape dimension")
		}
		outShape[neg] = a.Len() / n
		n = a.Len()
	}
	if n != a.Len() {
		return nil, fmt.Errorf("ndarray: reshape size mismatch %d vs %d", n, a.Len())
	}
	out := New(a.DType, outShape, a.Order)
	copy(out.Data, a.Data)
	return out, nil
}

// Transpose permutes axes according to order (a permutation of 0..ndim-1).
func (a *Array) Transpose(order []int) (*Array, error) {
	ndim := a.NDim()
	if len(order) != ndim {
		return nil, fmt.Errorf("ndarray: transpose order rank mismatch")
	}
	seen := make([]bool, ndim)
	for _, o := range order {
		if o < 0 || o >= ndim || seen[o] {
			return nil, fmt.Errorf("ndarray: invalid transpose order %v", order)
		}
		seen[o] = true
	}
	newShape := make([]int, ndim)
	for i, o := range order {
		newShape[i] = a.Shape[o]
	}
	out := New(a.DType, newShape, a.Order)
	srcIdx := make([]int, ndim)
	dstIdx := make([]int, ndim)
	n := a.Len()
	for lin := 0; lin < n; lin++ {
		rest := lin
		for i := ndim - 1; i >= 0; i-- {
			srcIdx[i] = rest % a.Shape[i]
			rest /= a.Shape[i]
		}
		for i, o := range order {
			dstIdx[i] = srcIdx[o]
		}
		srcOff := a.byteOffset(srcIdx)
		dstOff := out.byteOffset(dstIdx)
		item := a.DType.Size()
		copy(out.Data[dstOff:dstOff+item], a.Data[srcOff:srcOff+item])
	}
	return out, nil
}

// EncodeBytes serializes the array as C-order raw bytes with the given endianness.
func (a *Array) EncodeBytes(order binary.ByteOrder) []byte {
	if order == nil {
		order = a.Order
	}
	if order == a.Order || a.DType.Size() == 1 {
		out := make([]byte, len(a.Data))
		copy(out, a.Data)
		return out
	}
	out := make([]byte, len(a.Data))
	sz := a.DType.Size()
	n := a.Len()
	for i := 0; i < n; i++ {
		src := a.Data[i*sz : (i+1)*sz]
		dst := out[i*sz : (i+1)*sz]
		switch sz {
		case 2:
			order.PutUint16(dst, a.Order.Uint16(src))
		case 4:
			order.PutUint32(dst, a.Order.Uint32(src))
		case 8:
			order.PutUint64(dst, a.Order.Uint64(src))
		}
	}
	return out
}

// DecodeBytes constructs an array from C-order raw bytes.
func DecodeBytes(dtype DType, shape []int, order binary.ByteOrder, data []byte) (*Array, error) {
	a, err := FromBytes(dtype, shape, order, append([]byte(nil), data...))
	if err != nil {
		return nil, err
	}
	return a, nil
}

// String returns a compact debug representation similar to Java Array.toString.
func (a *Array) String() string {
	n := a.Len()
	parts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		parts = append(parts, fmt.Sprint(a.GetValue(i)))
	}
	return fmt.Sprintf("%v", parts)
}
