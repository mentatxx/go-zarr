package v3

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
	ZarrJSON = "zarr.json"
	Format   = 3
)

// Metadata is Zarr v3 array metadata.
type Metadata struct {
	ZarrFormat          int               `json:"zarr_format"`
	NodeType            string            `json:"node_type"`
	Shape               []int64           `json:"shape"`
	DataType            ndarray.DType     `json:"data_type"`
	ChunkGrid           ChunkGrid         `json:"chunk_grid"`
	ChunkKeyEncoding    ChunkKeyEnc       `json:"chunk_key_encoding"`
	FillValue           any               `json:"fill_value"`
	Codecs              []codec.Codec     `json:"-"`
	CodecSpecs          []json.RawMessage `json:"codecs"`
	Attributes          core.Attributes   `json:"attributes,omitempty"`
	DimensionNames      []string          `json:"dimension_names,omitempty"`
	StorageTransformers []any             `json:"storage_transformers,omitempty"`
	ParsedFill          any               `json:"-"`
}

type ChunkGrid struct {
	Name          string `json:"name"`
	Configuration struct {
		ChunkShape []int `json:"chunk_shape"`
	} `json:"configuration"`
}

type ChunkKeyEnc struct {
	Name          string `json:"name"`
	Configuration struct {
		Separator string `json:"separator"`
	} `json:"configuration"`
}

func (m *Metadata) NDim() int { return len(m.Shape) }
func (m *Metadata) ChunkShape() []int {
	return m.ChunkGrid.Configuration.ChunkShape
}

func (m *Metadata) EncodeChunkKey(coords []int64) []string {
	sep := m.ChunkKeyEncoding.Configuration.Separator
	if sep == "" {
		if m.ChunkKeyEncoding.Name == "v2" {
			sep = "."
		} else {
			sep = "/"
		}
	}
	parts := make([]string, len(coords))
	for i, c := range coords {
		parts[i] = strconv.FormatInt(c, 10)
	}
	switch m.ChunkKeyEncoding.Name {
	case "v2":
		if sep == "/" {
			return parts
		}
		return []string{strings.Join(parts, sep)}
	default:
		if sep == "/" {
			return append([]string{"c"}, parts...)
		}
		return []string{"c" + sep + strings.Join(parts, sep)}
	}
}

func (m *Metadata) ArrayMeta() codec.ArrayMeta {
	var order binary.ByteOrder = binary.LittleEndian
	for _, c := range m.Codecs {
		if b, ok := c.(*codec.Bytes); ok && b.Endian != nil {
			order = b.Endian
		}
	}
	return codec.ArrayMeta{
		Shape:      m.Shape,
		ChunkShape: append([]int(nil), m.ChunkShape()...),
		DType:      m.DataType,
		Order:      order,
		Fill:       m.ParsedFill,
	}
}

func (m *Metadata) validate() error {
	if m.ZarrFormat != Format {
		return core.NewError(fmt.Sprintf("expected zarr format %d, got %d", Format, m.ZarrFormat))
	}
	if m.NodeType != "array" {
		return core.NewError("expected node type 'array', got '" + m.NodeType + "'")
	}
	if len(m.StorageTransformers) > 0 {
		return core.NewError("storage transformers are not supported")
	}
	cs := m.ChunkShape()
	if len(cs) != len(m.Shape) {
		return core.NewError("shape and chunk shape ndim mismatch")
	}
	var err error
	m.ParsedFill, err = core.ParseFillValue(m.FillValue, m.DataType)
	return err
}

func (m *Metadata) UnmarshalJSON(b []byte) error {
	type raw Metadata
	var r raw
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*m = Metadata(r)
	codecs, err := UnmarshalCodecs(m.CodecSpecs)
	if err != nil {
		return err
	}
	m.Codecs = codecs
	return m.validate()
}

func (m *Metadata) MarshalJSON() ([]byte, error) {
	type alias struct {
		ZarrFormat          int               `json:"zarr_format"`
		NodeType            string            `json:"node_type"`
		Shape               []int64           `json:"shape"`
		DataType            ndarray.DType     `json:"data_type"`
		ChunkGrid           ChunkGrid         `json:"chunk_grid"`
		ChunkKeyEncoding    ChunkKeyEnc       `json:"chunk_key_encoding"`
		FillValue           any               `json:"fill_value"`
		Codecs              []json.RawMessage `json:"codecs"`
		Attributes          core.Attributes   `json:"attributes,omitempty"`
		DimensionNames      []string          `json:"dimension_names,omitempty"`
		StorageTransformers []any             `json:"storage_transformers,omitempty"`
	}
	specs := m.CodecSpecs
	if len(specs) == 0 {
		for _, c := range m.Codecs {
			if mj, ok := c.(json.Marshaler); ok {
				b, err := mj.MarshalJSON()
				if err != nil {
					return nil, err
				}
				specs = append(specs, b)
			}
		}
	}
	return json.Marshal(alias{
		ZarrFormat:       Format,
		NodeType:         "array",
		Shape:            m.Shape,
		DataType:         m.DataType,
		ChunkGrid:        m.ChunkGrid,
		ChunkKeyEncoding: m.ChunkKeyEncoding,
		FillValue:        m.FillValue,
		Codecs:           specs,
		Attributes:       m.Attributes,
		DimensionNames:   m.DimensionNames,
	})
}

// GroupMetadata is Zarr v3 group metadata.
type GroupMetadata struct {
	ZarrFormat           int             `json:"zarr_format"`
	NodeType             string          `json:"node_type"`
	Attributes           core.Attributes `json:"attributes,omitempty"`
	ConsolidatedMetadata any             `json:"consolidated_metadata,omitempty"`
}

func (g *GroupMetadata) UnmarshalJSON(b []byte) error {
	type raw GroupMetadata
	var r raw
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	if r.ZarrFormat != Format {
		return core.NewError(fmt.Sprintf("expected zarr format %d, got %d", Format, r.ZarrFormat))
	}
	if r.NodeType != "group" {
		return core.NewError("expected node type 'group'")
	}
	*g = GroupMetadata(r)
	if g.Attributes == nil {
		g.Attributes = core.Attributes{}
	}
	return nil
}

func DefaultGroupMetadata() GroupMetadata {
	return GroupMetadata{ZarrFormat: Format, NodeType: "group", Attributes: core.Attributes{}}
}
