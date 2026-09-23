package codec

import (
	"encoding/json"

	"github.com/klauspost/compress/zstd"
	"github.com/mentatxx/go-zarr/core"
)

// Zstd is a bytes->bytes zstd codec.
type Zstd struct {
	base
	Level    int
	Checksum bool
}

func NewZstd(level int, checksum bool) (*Zstd, error) {
	if level < -131072 || level > 22 {
		return nil, core.NewError("'level' needs to be between -131072 and 22")
	}
	return &Zstd{Level: level, Checksum: checksum}, nil
}

func (c *Zstd) Name() string { return "zstd" }
func (c *Zstd) Kind() Kind   { return KindBytesBytes }

func (c *Zstd) EncodeBytes(b []byte) ([]byte, error) {
	opts := []zstd.EOption{zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(c.Level))}
	if c.Checksum {
		opts = append(opts, zstd.WithEncoderCRC(true))
	} else {
		opts = append(opts, zstd.WithEncoderCRC(false))
	}
	enc, err := zstd.NewWriter(nil, opts...)
	if err != nil {
		return nil, err
	}
	defer enc.Close()
	return enc.EncodeAll(b, nil), nil
}

func (c *Zstd) DecodeBytes(b []byte) ([]byte, error) {
	dec, err := zstd.NewReader(nil)
	if err != nil {
		return nil, err
	}
	defer dec.Close()
	out, err := dec.DecodeAll(b, nil)
	if err != nil {
		return nil, core.WrapError("error in decoding zstd", err)
	}
	return out, nil
}

func (c *Zstd) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name          string `json:"name"`
		Configuration struct {
			Level    int  `json:"level"`
			Checksum bool `json:"checksum"`
		} `json:"configuration"`
	}{Name: "zstd", Configuration: struct {
		Level    int  `json:"level"`
		Checksum bool `json:"checksum"`
	}{Level: c.Level, Checksum: c.Checksum}})
}
