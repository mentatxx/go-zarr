package v3

import (
	"encoding/json"
	"fmt"

	"github.com/mentatxx/go-zarr/codec"
	"github.com/mentatxx/go-zarr/core"
	"github.com/mentatxx/go-zarr/ndarray"
)

type codecEnvelope struct {
	Name          string          `json:"name"`
	Configuration json.RawMessage `json:"configuration"`
}

// UnmarshalCodecs parses a v3 codecs array.
func UnmarshalCodecs(raw []json.RawMessage) ([]codec.Codec, error) {
	out := make([]codec.Codec, 0, len(raw))
	for _, r := range raw {
		c, err := unmarshalCodec(r)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func unmarshalCodec(r json.RawMessage) (codec.Codec, error) {
	var env codecEnvelope
	if err := json.Unmarshal(r, &env); err != nil {
		return nil, err
	}
	switch env.Name {
	case "bytes":
		var cfg struct {
			Endian string `json:"endian"`
		}
		_ = json.Unmarshal(env.Configuration, &cfg)
		order, err := codec.ParseEndian(cfg.Endian)
		if err != nil {
			return nil, err
		}
		return codec.NewBytes(order), nil
	case "gzip":
		var cfg struct {
			Level int `json:"level"`
		}
		cfg.Level = 5
		_ = json.Unmarshal(env.Configuration, &cfg)
		return codec.NewGzip(cfg.Level)
	case "zstd":
		var cfg struct {
			Level    int   `json:"level"`
			Checksum *bool `json:"checksum"`
		}
		cfg.Level = 5
		_ = json.Unmarshal(env.Configuration, &cfg)
		cs := true
		if cfg.Checksum != nil {
			cs = *cfg.Checksum
		}
		return codec.NewZstd(cfg.Level, cs)
	case "blosc":
		var cfg struct {
			CName     string `json:"cname"`
			CLevel    int    `json:"clevel"`
			Shuffle   string `json:"shuffle"`
			Typesize  int    `json:"typesize"`
			Blocksize int    `json:"blocksize"`
		}
		cfg.CName = "zstd"
		cfg.CLevel = 5
		cfg.Shuffle = "noshuffle"
		_ = json.Unmarshal(env.Configuration, &cfg)
		return codec.NewBlosc(cfg.CName, cfg.Shuffle, cfg.CLevel, cfg.Typesize, cfg.Blocksize)
	case "crc32c":
		return codec.NewCRC32C(), nil
	case "transpose":
		var cfg struct {
			Order []int `json:"order"`
		}
		if err := json.Unmarshal(env.Configuration, &cfg); err != nil {
			return nil, err
		}
		return NewTranspose(cfg.Order), nil
	case "reshape":
		var cfg struct {
			Shape []any `json:"shape"`
		}
		if err := json.Unmarshal(env.Configuration, &cfg); err != nil {
			return nil, err
		}
		return NewReshape(cfg.Shape), nil
	case "cast_value":
		var cfg CastConfig
		if err := json.Unmarshal(env.Configuration, &cfg); err != nil {
			return nil, err
		}
		return NewCast(cfg), nil
	case "sharding_indexed":
		var cfg struct {
			ChunkShape    []int             `json:"chunk_shape"`
			Codecs        []json.RawMessage `json:"codecs"`
			IndexCodecs   []json.RawMessage `json:"index_codecs"`
			IndexLocation string            `json:"index_location"`
		}
		if err := json.Unmarshal(env.Configuration, &cfg); err != nil {
			return nil, err
		}
		inner, err := UnmarshalCodecs(cfg.Codecs)
		if err != nil {
			return nil, err
		}
		index, err := UnmarshalCodecs(cfg.IndexCodecs)
		if err != nil {
			return nil, err
		}
		return NewSharding(cfg.ChunkShape, inner, index, cfg.IndexLocation)
	default:
		return nil, core.NewError("unknown codec " + env.Name)
	}
}

// CodecBuilder constructs a v3 codec pipeline.
type CodecBuilder struct {
	dtype  ndarray.DType
	codecs []codec.Codec
}

func NewCodecBuilder(dt ndarray.DType) *CodecBuilder {
	return &CodecBuilder{dtype: dt}
}

func (b *CodecBuilder) WithBlosc(args ...any) *CodecBuilder {
	cname, shuffle, clevel := "zstd", "noshuffle", 5
	if len(args) >= 1 {
		cname = fmt.Sprint(args[0])
	}
	if len(args) >= 2 {
		if s, ok := args[1].(string); ok && (s == "noshuffle" || s == "shuffle" || s == "bitshuffle") {
			shuffle = s
			if len(args) >= 3 {
				clevel = asInt(args[2])
			}
		} else {
			clevel = asInt(args[1])
		}
	}
	c, err := codec.NewBlosc(cname, shuffle, clevel, b.dtype.Size(), 0)
	if err != nil {
		panic(err)
	}
	b.codecs = append(b.codecs, c)
	return b
}

func (b *CodecBuilder) WithGzip(level ...int) *CodecBuilder {
	lv := 5
	if len(level) > 0 {
		lv = level[0]
	}
	c, err := codec.NewGzip(lv)
	if err != nil {
		panic(err)
	}
	b.codecs = append(b.codecs, c)
	return b
}

func (b *CodecBuilder) WithZstd(args ...any) *CodecBuilder {
	level, checksum := 5, true
	if len(args) >= 1 {
		level = asInt(args[0])
	}
	if len(args) >= 2 {
		checksum, _ = args[1].(bool)
	}
	c, err := codec.NewZstd(level, checksum)
	if err != nil {
		panic(err)
	}
	b.codecs = append(b.codecs, c)
	return b
}

func (b *CodecBuilder) WithBytes(endian ...string) *CodecBuilder {
	e := "little"
	if len(endian) > 0 {
		e = endian[0]
	}
	order, err := codec.ParseEndian(e)
	if err != nil {
		panic(err)
	}
	b.codecs = append(b.codecs, codec.NewBytes(order))
	return b
}

func (b *CodecBuilder) WithTranspose(order []int) *CodecBuilder {
	b.codecs = append(b.codecs, NewTranspose(order))
	return b
}

func (b *CodecBuilder) WithCrc32c() *CodecBuilder {
	b.codecs = append(b.codecs, codec.NewCRC32C())
	return b
}

func (b *CodecBuilder) WithSharding(chunkShape []int, nested func(*CodecBuilder) *CodecBuilder, indexLocation ...string) *CodecBuilder {
	loc := "end"
	if len(indexLocation) > 0 {
		loc = indexLocation[0]
	}
	inner := NewCodecBuilder(b.dtype)
	if nested != nil {
		inner = nested(inner)
	}
	innerCodecs := inner.Build()
	index := []codec.Codec{codec.NewBytes(nil), codec.NewCRC32C()}
	c, err := NewSharding(chunkShape, innerCodecs, index, loc)
	if err != nil {
		panic(err)
	}
	b.codecs = append(b.codecs, c)
	return b
}

func (b *CodecBuilder) WithReshape(shape []any) *CodecBuilder {
	b.codecs = append(b.codecs, NewReshape(shape))
	return b
}

func (b *CodecBuilder) WithCastValue(dt ndarray.DType) *CodecBuilder {
	b.codecs = append(b.codecs, NewCast(CastConfig{DataType: dt}))
	return b
}

func (b *CodecBuilder) Build() []codec.Codec {
	hasAB := false
	for _, c := range b.codecs {
		if c.Kind() == codec.KindArrayBytes {
			hasAB = true
			break
		}
	}
	if !hasAB {
		var aa, bb []codec.Codec
		for _, c := range b.codecs {
			if c.Kind() == codec.KindArrayArray {
				aa = append(aa, c)
			} else {
				bb = append(bb, c)
			}
		}
		b.codecs = append(aa, codec.NewBytes(nil))
		b.codecs = append(b.codecs, bb...)
	}
	return b.codecs
}

func asInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	}
	return 0
}
