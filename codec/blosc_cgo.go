//go:build cblosc

package codec

/*
#cgo pkg-config: blosc
#include <blosc.h>
#include <stdlib.h>
*/
import "C"
import (
	"unsafe"

	"github.com/mentatxx/go-zarr/core"
)

func init() {
	C.blosc_init()
	bloscCGOCompress = cgoCompress
	bloscCGODecompress = cgoDecompress
}

func cgoCompress(b []byte, cdc *Blosc) ([]byte, error) {
	cname := C.CString(cdc.CName)
	defer C.free(unsafe.Pointer(cname))
	out := make([]byte, len(b)+C.BLOSC_MAX_OVERHEAD)
	var src unsafe.Pointer
	if len(b) > 0 {
		src = unsafe.Pointer(&b[0])
	}
	n := int(C.blosc_compress_ctx(
		C.int(cdc.CLevel),
		C.int(shuffleCode(cdc.resolvedShuffle())),
		C.size_t(cdc.TypeSize),
		C.size_t(len(b)),
		src,
		unsafe.Pointer(&out[0]),
		C.size_t(len(out)),
		cname,
		C.size_t(cdc.BlockSize),
		C.int(1),
	))
	if n < 0 {
		return nil, core.NewError("blosc: compression failed")
	}
	return out[:n], nil
}

func cgoDecompress(b []byte) ([]byte, error) {
	if len(b) < 16 {
		return nil, core.NewError("blosc: buffer too short")
	}
	var nbytes, cbytes, blocksize C.size_t
	C.blosc_cbuffer_sizes(unsafe.Pointer(&b[0]), &nbytes, &cbytes, &blocksize)
	out := make([]byte, int(nbytes))
	var dst unsafe.Pointer
	if len(out) > 0 {
		dst = unsafe.Pointer(&out[0])
	}
	n := int(C.blosc_decompress_ctx(unsafe.Pointer(&b[0]), dst, C.size_t(len(out)), C.int(1)))
	if n < 0 {
		return nil, core.NewError("blosc: decompression failed")
	}
	return out[:n], nil
}

func shuffleCode(s string) int {
	switch s {
	case "shuffle", "1":
		return 1
	case "bitshuffle", "2":
		return 2
	default:
		return 0
	}
}
