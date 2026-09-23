package ndarray

import (
	"encoding/binary"
	"fmt"
)

// DType is a Zarr numeric/boolean element type (endianness is stored separately).
type DType string

const (
	Bool    DType = "bool"
	Int8    DType = "int8"
	Int16   DType = "int16"
	Int32   DType = "int32"
	Int64   DType = "int64"
	Uint8   DType = "uint8"
	Uint16  DType = "uint16"
	Uint32  DType = "uint32"
	Uint64  DType = "uint64"
	Float32 DType = "float32"
	Float64 DType = "float64"
)

// Size returns the element size in bytes.
func (d DType) Size() int {
	switch d {
	case Bool, Int8, Uint8:
		return 1
	case Int16, Uint16:
		return 2
	case Int32, Uint32, Float32:
		return 4
	case Int64, Uint64, Float64:
		return 8
	default:
		return 0
	}
}

func (d DType) String() string { return string(d) }

// IsInteger reports whether d is an integer type (not bool/float).
func (d DType) IsInteger() bool {
	switch d {
	case Int8, Int16, Int32, Int64, Uint8, Uint16, Uint32, Uint64:
		return true
	}
	return false
}

// IsUnsigned reports whether d is an unsigned integer type.
func (d DType) IsUnsigned() bool {
	switch d {
	case Uint8, Uint16, Uint32, Uint64:
		return true
	}
	return false
}

// IsFloat reports whether d is a floating-point type.
func (d DType) IsFloat() bool {
	return d == Float32 || d == Float64
}

// ParseV3 parses a Zarr v3 data_type string.
func ParseV3(s string) (DType, error) {
	d := DType(s)
	if d.Size() == 0 {
		return "", fmt.Errorf("ndarray: unknown v3 data type %q", s)
	}
	return d, nil
}

// V2Spec describes a Zarr v2 dtype (NumPy-style).
type V2Spec struct {
	DType DType
	Order binary.ByteOrder
	Raw   string
}

// ParseV2 parses a Zarr v2 dtype such as "<i4", ">f8", "|b1", "i1".
func ParseV2(s string) (V2Spec, error) {
	if s == "" {
		return V2Spec{}, fmt.Errorf("ndarray: empty v2 dtype")
	}
	order := binary.ByteOrder(binary.LittleEndian)
	rest := s
	switch s[0] {
	case '<':
		order = binary.LittleEndian
		rest = s[1:]
	case '>':
		order = binary.BigEndian
		rest = s[1:]
	case '|':
		order = binary.LittleEndian
		rest = s[1:]
	}
	var dt DType
	switch rest {
	case "b1", "b":
		dt = Bool
	case "i1":
		dt = Int8
	case "i2":
		dt = Int16
	case "i4":
		dt = Int32
	case "i8":
		dt = Int64
	case "u1":
		dt = Uint8
	case "u2":
		dt = Uint16
	case "u4":
		dt = Uint32
	case "u8":
		dt = Uint64
	case "f4":
		dt = Float32
	case "f8":
		dt = Float64
	default:
		return V2Spec{}, fmt.Errorf("ndarray: unknown v2 dtype %q", s)
	}
	return V2Spec{DType: dt, Order: order, Raw: s}, nil
}

// FormatV2 formats a dtype as a Zarr v2 dtype string.
func FormatV2(dt DType, order binary.ByteOrder) string {
	code := ""
	switch dt {
	case Bool:
		return "|b1"
	case Int8:
		return "|i1"
	case Uint8:
		return "|u1"
	case Int16:
		code = "i2"
	case Int32:
		code = "i4"
	case Int64:
		code = "i8"
	case Uint16:
		code = "u2"
	case Uint32:
		code = "u4"
	case Uint64:
		code = "u8"
	case Float32:
		code = "f4"
	case Float64:
		code = "f8"
	default:
		return string(dt)
	}
	prefix := "<"
	if order == binary.BigEndian {
		prefix = ">"
	}
	return prefix + code
}

// AllV3 returns every supported v3 data type.
func AllV3() []DType {
	return []DType{Bool, Int8, Uint8, Int16, Uint16, Int32, Uint32, Int64, Uint64, Float32, Float64}
}
