package codec

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/mentatxx/go-zarr/core"
	bloscgo "github.com/mrjoshuak/go-blosc"
	"github.com/pierrec/lz4/v4"
)

// ErrBloscLZNeedsCGO is returned when blosclz is requested without -tags cblosc.
var ErrBloscLZNeedsCGO = core.NewError("blosc: cname blosclz requires building with -tags cblosc")

// Blosc is the bytes->bytes Blosc codec.
type Blosc struct {
	base
	CName     string
	CLevel    int
	Shuffle   string
	TypeSize  int
	BlockSize int
}

func NewBlosc(cname, shuffle string, clevel, typesize, blocksize int) (*Blosc, error) {
	if clevel < 0 || clevel > 9 {
		return nil, core.NewError("'clevel' needs to be between 0 and 9")
	}
	if shuffle == "" {
		shuffle = "noshuffle"
	}
	if cname == "" {
		cname = "zstd"
	}
	if typesize < 1 && shuffle != "noshuffle" {
		return nil, core.NewError("'typesize' needs to be larger than 0")
	}
	if typesize < 1 {
		typesize = 1
	}
	return &Blosc{CName: cname, CLevel: clevel, Shuffle: shuffle, TypeSize: typesize, BlockSize: blocksize}, nil
}

func (c *Blosc) Name() string { return "blosc" }
func (c *Blosc) Kind() Kind   { return KindBytesBytes }

func (c *Blosc) resolvedShuffle() string {
	if c.Shuffle == "autoshuffle" || c.Shuffle == "-1" {
		if c.TypeSize <= 1 {
			return "bitshuffle"
		}
		return "shuffle"
	}
	return c.Shuffle
}

func (c *Blosc) EncodeBytes(b []byte) ([]byte, error) {
	if impl := bloscCGOCompress; impl != nil {
		return impl(b, c)
	}
	if c.CName == "blosclz" {
		return nil, ErrBloscLZNeedsCGO
	}
	codec, err := bloscCodec(c.CName)
	if err != nil {
		return nil, err
	}
	sh := bloscShuffle(c.resolvedShuffle())
	ts := c.TypeSize
	if ts < 1 {
		ts = 1
	}
	level := c.CLevel
	if level < 1 {
		level = 1
	}
	return bloscgo.Compress(b, codec, level, sh, ts)
}

func (c *Blosc) DecodeBytes(b []byte) ([]byte, error) {
	if impl := bloscCGODecompress; impl != nil {
		return impl(b)
	}
	out, err := bloscgo.Decompress(b)
	if err != nil {
		return nil, core.WrapError("error in decoding blosc", err)
	}
	return out, nil
}

func bloscCodec(name string) (bloscgo.Codec, error) {
	switch name {
	case "lz4":
		return bloscgo.LZ4, nil
	case "lz4hc":
		return bloscgo.LZ4HC, nil
	case "zstd":
		return bloscgo.ZSTD, nil
	case "zlib":
		return bloscgo.ZLIB, nil
	case "snappy":
		return bloscgo.Snappy, nil
	case "blosclz":
		return 0, ErrBloscLZNeedsCGO
	default:
		return 0, core.NewError("unknown blosc cname " + name)
	}
}

func bloscShuffle(s string) bloscgo.Shuffle {
	switch s {
	case "shuffle", "byte_shuffle", "1":
		return bloscgo.Shuffle1
	case "bitshuffle", "2":
		return bloscgo.BitShuffle
	default:
		return bloscgo.NoShuffle
	}
}

func (c *Blosc) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name          string `json:"name"`
		Configuration struct {
			CName     string `json:"cname"`
			CLevel    int    `json:"clevel"`
			Shuffle   string `json:"shuffle"`
			Typesize  int    `json:"typesize"`
			Blocksize int    `json:"blocksize"`
		} `json:"configuration"`
	}{Name: "blosc", Configuration: struct {
		CName     string `json:"cname"`
		CLevel    int    `json:"clevel"`
		Shuffle   string `json:"shuffle"`
		Typesize  int    `json:"typesize"`
		Blocksize int    `json:"blocksize"`
	}{CName: c.CName, CLevel: c.CLevel, Shuffle: c.Shuffle, Typesize: c.TypeSize, Blocksize: c.BlockSize}})
}

// Optional CGO hooks, set from blosc_cgo.go.
var (
	bloscCGOCompress   func([]byte, *Blosc) ([]byte, error)
	bloscCGODecompress func([]byte) ([]byte, error)
)

// Zlib is raw zlib (v2 compressor).
type Zlib struct {
	base
	Level int
}

func NewZlib(level int) (*Zlib, error) {
	if level < -1 || level > 9 {
		return nil, core.NewError("'level' needs to be between -1 and 9")
	}
	return &Zlib{Level: level}, nil
}

func (c *Zlib) Name() string { return "zlib" }
func (c *Zlib) Kind() Kind   { return KindBytesBytes }

func (c *Zlib) EncodeBytes(b []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := zlib.NewWriterLevel(&buf, c.Level)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(b); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *Zlib) DecodeBytes(b []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func (c *Zlib) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID    string `json:"id"`
		Level int    `json:"level"`
	}{ID: "zlib", Level: c.Level})
}

// Keep lz4/zstd imports referenced for future full-format encoder.
var (
	_ = lz4.ErrInvalidSourceShortBuffer
	_ = zstd.SpeedDefault
	_ = binary.LittleEndian
	_ = fmt.Sprintf
)
