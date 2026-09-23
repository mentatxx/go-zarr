package core

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mentatxx/go-zarr/ndarray"
)

// ParseFillValue converts a JSON fill_value into a typed Go value.
func ParseFillValue(fill any, dt ndarray.DType) (any, error) {
	if fill == nil {
		return nil, nil
	}
	if b, ok := fill.(bool); ok {
		if dt == ndarray.Bool {
			return b, nil
		}
	}
	switch dt {
	case ndarray.Bool:
		switch x := fill.(type) {
		case bool:
			return x, nil
		case float64:
			return x != 0, nil
		case jsonNumber:
			return x != 0, nil
		}
	}
	if s, ok := fill.(string); ok {
		return parseFillString(s, dt)
	}
	n, err := toFloat64(fill)
	if err != nil {
		return nil, NewError(fmt.Sprintf("invalid fill value '%v'", fill))
	}
	return coerceFill(n, dt), nil
}

type jsonNumber interface{}

func parseFillString(s string, dt ndarray.DType) (any, error) {
	switch s {
	case "NaN":
		if dt == ndarray.Float32 {
			return float32(math.NaN()), nil
		}
		if dt == ndarray.Float64 {
			return math.NaN(), nil
		}
		return nil, NewError("invalid fill value 'NaN' for " + string(dt))
	case "+Infinity":
		if dt == ndarray.Float32 {
			return float32(math.Inf(1)), nil
		}
		if dt == ndarray.Float64 {
			return math.Inf(1), nil
		}
		return nil, NewError("invalid fill value '+Infinity' for " + string(dt))
	case "-Infinity":
		if dt == ndarray.Float32 {
			return float32(math.Inf(-1)), nil
		}
		if dt == ndarray.Float64 {
			return math.Inf(-1), nil
		}
		return nil, NewError("invalid fill value '-Infinity' for " + string(dt))
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0b") {
		buf, err := parseFillBits(s, dt.Size())
		if err != nil {
			return nil, err
		}
		return valueFromBytes(buf, dt, binary.LittleEndian), nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return coerceFill(f, dt), nil
	}
	return nil, NewError("invalid fill value '" + s + "'")
}

func parseFillBits(s string, n int) ([]byte, error) {
	if strings.HasPrefix(s, "0x") {
		hexs := s[2:]
		if len(hexs) != n*2 {
			return nil, NewError("invalid hex fill value length")
		}
		return hex.DecodeString(hexs)
	}
	bits := s[2:]
	if len(bits) != n*8 {
		return nil, NewError("invalid binary fill value length")
	}
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		v, err := strconv.ParseUint(bits[i*8:(i+1)*8], 2, 8)
		if err != nil {
			return nil, err
		}
		out[i] = byte(v)
	}
	return out, nil
}

func coerceFill(n float64, dt ndarray.DType) any {
	switch dt {
	case ndarray.Bool:
		return n != 0
	case ndarray.Int8:
		return int8(int64(n))
	case ndarray.Int16:
		return int16(int64(n))
	case ndarray.Int32:
		return int32(int64(n))
	case ndarray.Int64:
		return int64(n)
	case ndarray.Uint8:
		return uint8(uint64(n))
	case ndarray.Uint16:
		return uint16(uint64(n))
	case ndarray.Uint32:
		return uint32(uint64(n))
	case ndarray.Uint64:
		return uint64(n)
	case ndarray.Float32:
		return float32(n)
	case ndarray.Float64:
		return n
	}
	return n
}

func valueFromBytes(b []byte, dt ndarray.DType, order binary.ByteOrder) any {
	a, _ := ndarray.FromBytes(dt, []int{1}, order, b)
	return a.GetValue(0)
}

// DefaultChunkShape matches zarr-java / JZarr (~512 elements per dim).
func DefaultChunkShape(shape []int64) []int {
	chunks := make([]int, len(shape))
	for i, shapeDim := range shape {
		numChunks := int(shapeDim / 512)
		if numChunks > 0 {
			chunkDim := int(shapeDim / int64(numChunks+1))
			if shapeDim%int64(chunkDim) == 0 {
				chunks[i] = chunkDim
			} else {
				chunks[i] = chunkDim + 1
			}
		} else {
			chunks[i] = int(shapeDim)
		}
	}
	return chunks
}
