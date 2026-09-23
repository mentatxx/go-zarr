package codec

import (
	"encoding/binary"
	"encoding/json"
	"hash/crc32"

	"github.com/mentatxx/go-zarr/core"
)

var crc32cTable = crc32.MakeTable(crc32.Castagnoli)

// CRC32C appends/checks a CRC32C checksum (bytes->bytes).
type CRC32C struct {
	base
}

func NewCRC32C() *CRC32C { return &CRC32C{} }

func (c *CRC32C) Name() string { return "crc32c" }
func (c *CRC32C) Kind() Kind   { return KindBytesBytes }

func (c *CRC32C) EncodeBytes(b []byte) ([]byte, error) {
	sum := crc32.Checksum(b, crc32cTable)
	out := make([]byte, len(b)+4)
	copy(out, b)
	binary.LittleEndian.PutUint32(out[len(b):], sum)
	return out, nil
}

func (c *CRC32C) DecodeBytes(b []byte) ([]byte, error) {
	if len(b) < 4 {
		return nil, core.NewError("crc32c: buffer too short")
	}
	payload := b[:len(b)-4]
	want := binary.LittleEndian.Uint32(b[len(b)-4:])
	got := crc32.Checksum(payload, crc32cTable)
	if want != got {
		return nil, core.NewError("crc32c: checksum mismatch")
	}
	return payload, nil
}

func (c *CRC32C) ComputeEncodedSize(n int64) (int64, error) { return n + 4, nil }

func (c *CRC32C) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name string `json:"name"`
	}{Name: "crc32c"})
}
