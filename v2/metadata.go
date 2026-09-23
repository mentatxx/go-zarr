package v2

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mentatxx/go-zarr/codec"
	"github.com/mentatxx/go-zarr/core"
	"github.com/mentatxx/go-zarr/ndarray"
)

const (
	ZArray = ".zarray"
	ZGroup = ".zgroup"
	ZAttrs = ".zattrs"
	Format = 2
)

// Metadata is Zarr v2 array metadata (.zarray).
type Metadata struct {
	ZarrFormat         int               `json:"zarr_format"`
	Shape              []int64           `json:"shape"`
	Chunks             []int             `json:"chunks"`
	DType              string            `json:"dtype"`
	FillValue          any               `json:"fill_value"`
	Order              string            `json:"order"`
	Filters            []json.RawMessage `json:"filters"`
	Compressor         json.RawMessage   `json:"compressor"`
	DimensionSeparator string            `json:"dimension_separator,omitempty"`
	Attributes         core.Attributes   `json:"-"`
	ParsedDType        ndarray.V2Spec    `json:"-"`
	ParsedFill         any               `json:"-"`
	Codecs             []codec.Codec     `json:"-"`
}

func (m *Metadata) NDim() int         { return len(m.Shape) }
func (m *Metadata) ChunkShape() []int { return m.Chunks }

func (m *Metadata) EncodeChunkKey(coords []int64) []string {
	sep := m.DimensionSeparator
	if sep == "" {
		sep = "."
	}
	parts := make([]string, len(coords))
	for i, c := range coords {
		parts[i] = strconv.FormatInt(c, 10)
	}
	if sep == "/" {
		return parts
	}
	return []string{strings.Join(parts, sep)}
}

func (m *Metadata) ArrayMeta() codec.ArrayMeta {
	order := m.ParsedDType.Order
	if m.Order == "F" {
		// Fortran order is not fully implemented; bytes still use dtype endian.
	}
	return codec.ArrayMeta{
		Shape:      m.Shape,
		ChunkShape: append([]int(nil), m.Chunks...),
		DType:      m.ParsedDType.DType,
		Order:      order,
		Fill:       m.ParsedFill,
	}
}

func (m *Metadata) UnmarshalJSON(b []byte) error {
	type raw Metadata
	var r raw
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*m = Metadata(r)
	if m.ZarrFormat != Format {
		return core.NewError(fmt.Sprintf("expected zarr format 2, got %d", m.ZarrFormat))
	}
	spec, err := ndarray.ParseV2(m.DType)
	if err != nil {
		return err
	}
	m.ParsedDType = spec
	m.ParsedFill, err = core.ParseFillValue(m.FillValue, spec.DType)
	if err != nil {
		return err
	}
	m.Codecs, err = v2Codecs(m, spec)
	return err
}

func v2Codecs(m *Metadata, spec ndarray.V2Spec) ([]codec.Codec, error) {
	var out []codec.Codec
	out = append(out, codec.NewBytes(spec.Order))
	if len(m.Compressor) > 0 && string(m.Compressor) != "null" {
		c, err := unmarshalV2Compressor(m.Compressor, spec)
		if err != nil {
			return nil, err
		}
		if c != nil {
			out = append(out, c)
		}
	}
	return out, nil
}

func unmarshalV2Compressor(raw json.RawMessage, spec ndarray.V2Spec) (codec.Codec, error) {
	var head struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, err
	}
	switch head.ID {
	case "gzip":
		var cfg struct {
			Level int `json:"level"`
		}
		cfg.Level = 1
		_ = json.Unmarshal(raw, &cfg)
		return codec.NewGzip(cfg.Level)
	case "zlib":
		var cfg struct {
			Level int `json:"level"`
		}
		cfg.Level = 1
		_ = json.Unmarshal(raw, &cfg)
		return codec.NewZlib(cfg.Level)
	case "zstd":
		var cfg struct {
			Level    int  `json:"level"`
			Checksum bool `json:"checksum"`
		}
		_ = json.Unmarshal(raw, &cfg)
		return codec.NewZstd(cfg.Level, cfg.Checksum)
	case "blosc":
		var cfg struct {
			CName     string `json:"cname"`
			CLevel    int    `json:"clevel"`
			Shuffle   any    `json:"shuffle"`
			BlockSize int    `json:"blocksize"`
		}
		cfg.CName = "lz4"
		cfg.CLevel = 5
		_ = json.Unmarshal(raw, &cfg)
		shuffle := "noshuffle"
		switch s := cfg.Shuffle.(type) {
		case float64:
			switch int(s) {
			case -1:
				shuffle = "autoshuffle"
			case 1:
				shuffle = "shuffle"
			case 2:
				shuffle = "bitshuffle"
			}
		case json.Number:
			n, _ := s.Int64()
			switch int(n) {
			case -1:
				shuffle = "autoshuffle"
			case 1:
				shuffle = "shuffle"
			case 2:
				shuffle = "bitshuffle"
			}
		case string:
			shuffle = s
		}
		ts := specSize(raw)
		if ts < 1 {
			ts = spec.DType.Size()
		}
		return codec.NewBlosc(cfg.CName, shuffle, cfg.CLevel, ts, cfg.BlockSize)
	default:
		return nil, core.NewError("unknown v2 compressor " + head.ID)
	}
}

func specSize(raw json.RawMessage) int {
	var cfg struct {
		Typesize int `json:"typesize"`
	}
	_ = json.Unmarshal(raw, &cfg)
	if cfg.Typesize > 0 {
		return cfg.Typesize
	}
	return 1
}

func (m *Metadata) MarshalJSON() ([]byte, error) {
	type alias Metadata
	return json.Marshal((*alias)(m))
}

// GroupMetadata is .zgroup content.
type GroupMetadata struct {
	ZarrFormat int `json:"zarr_format"`
}

// Keep endian import used.
var _ = binary.LittleEndian
