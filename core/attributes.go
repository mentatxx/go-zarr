package core

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
)

// Attributes is a JSON object stored on arrays and groups.
type Attributes map[string]any

// GetString returns a string attribute.
func (a Attributes) GetString(key string) (string, error) {
	v, ok := a[key]
	if !ok {
		return "", fmt.Errorf("zarr: missing attribute %q", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("zarr: attribute %q is not a string", key)
	}
	return s, nil
}

// GetBool returns a boolean attribute.
func (a Attributes) GetBool(key string) (bool, error) {
	v, ok := a[key]
	if !ok {
		return false, fmt.Errorf("zarr: missing attribute %q", key)
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("zarr: attribute %q is not a boolean", key)
	}
	return b, nil
}

// GetInt returns an integer attribute.
func (a Attributes) GetInt(key string) (int, error) {
	v, ok := a[key]
	if !ok {
		return 0, fmt.Errorf("zarr: missing attribute %q", key)
	}
	return toInt(v)
}

// GetFloat64 returns a float attribute.
func (a Attributes) GetFloat64(key string) (float64, error) {
	v, ok := a[key]
	if !ok {
		return 0, fmt.Errorf("zarr: missing attribute %q", key)
	}
	return toFloat64(v)
}

// GetFloat32 returns a float32 attribute.
func (a Attributes) GetFloat32(key string) (float32, error) {
	f, err := a.GetFloat64(key)
	return float32(f), err
}

// GetList returns a list attribute.
func (a Attributes) GetList(key string) ([]any, error) {
	v, ok := a[key]
	if !ok {
		return nil, fmt.Errorf("zarr: missing attribute %q", key)
	}
	switch x := v.(type) {
	case []any:
		return x, nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice {
		out := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out[i] = rv.Index(i).Interface()
		}
		return out, nil
	}
	return nil, fmt.Errorf("zarr: attribute %q is not a list", key)
}

// GetAttributes returns a nested object.
func (a Attributes) GetAttributes(key string) (Attributes, error) {
	v, ok := a[key]
	if !ok {
		return nil, fmt.Errorf("zarr: missing attribute %q", key)
	}
	switch x := v.(type) {
	case Attributes:
		return x, nil
	case map[string]any:
		return Attributes(x), nil
	}
	return nil, fmt.Errorf("zarr: attribute %q is not an object", key)
}

func toInt(v any) (int, error) {
	switch x := v.(type) {
	case int:
		return x, nil
	case int8:
		return int(x), nil
	case int16:
		return int(x), nil
	case int32:
		return int(x), nil
	case int64:
		return int(x), nil
	case uint:
		return int(x), nil
	case uint8:
		return int(x), nil
	case uint16:
		return int(x), nil
	case uint32:
		return int(x), nil
	case uint64:
		return int(x), nil
	case float32:
		return int(x), nil
	case float64:
		return int(x), nil
	case json.Number:
		i, err := x.Int64()
		return int(i), err
	case string:
		return strconv.Atoi(x)
	}
	return 0, fmt.Errorf("zarr: not an integer: %T", v)
}

func toFloat64(v any) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case float32:
		return float64(x), nil
	case int:
		return float64(x), nil
	case int64:
		return float64(x), nil
	case json.Number:
		return x.Float64()
	}
	i, err := toInt(v)
	if err != nil {
		return 0, err
	}
	return float64(i), nil
}

// Clone returns a shallow copy.
func (a Attributes) Clone() Attributes {
	if a == nil {
		return Attributes{}
	}
	out := make(Attributes, len(a))
	for k, v := range a {
		out[k] = v
	}
	return out
}
