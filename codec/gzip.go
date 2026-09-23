package codec

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"

	"github.com/mentatxx/go-zarr/core"
)

// Gzip is a bytes->bytes gzip codec.
type Gzip struct {
	base
	Level int
}

func NewGzip(level int) (*Gzip, error) {
	// -1 is gzip.DefaultCompression; other implementations write it to metadata.
	if level < -1 || level > 9 {
		return nil, core.NewError("'level' needs to be between -1 and 9")
	}
	return &Gzip{Level: level}, nil
}

func (c *Gzip) Name() string { return "gzip" }
func (c *Gzip) Kind() Kind   { return KindBytesBytes }

func (c *Gzip) ComputeEncodedSize(n int64) (int64, error) {
	return 0, core.NewError("Not implemented for Gzip codec.")
}

func (c *Gzip) EncodeBytes(b []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, c.Level)
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

func (c *Gzip) DecodeBytes(b []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, core.WrapError("error in decoding gzip", err)
	}
	defer r.Close()
	return io.ReadAll(r)
}

func (c *Gzip) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name          string `json:"name"`
		Configuration struct {
			Level int `json:"level"`
		} `json:"configuration"`
	}{Name: "gzip", Configuration: struct {
		Level int `json:"level"`
	}{Level: c.Level}})
}
