# zarr (Go)

Go implementation of the [Zarr](https://zarr.dev/) specification (v2 and v3), ported from [zarr-java](https://github.com/zarr-developers/zarr-java).

## Features

- Zarr v2 and v3 read/write
- Stores: filesystem, memory, HTTP, S3, ZIP
- Codecs: bytes, gzip, zlib, zstd, crc32c, blosc, transpose, reshape, cast_value, sharding
- OME-Zarr v0.4 / v0.5 / v0.6 (experimental)
- CLI for [zarr conformance tests](https://github.com/zarr-developers/zarr-specs)

Blosc uses a pure-Go codec by default. Build with `-tags cblosc` (and system `c-blosc`) for full c-blosc / `blosclz` interoperability.

## Install

```bash
go get github.com/mentatxx/go-zarr
```

## Quick start

```go
package main

import (
    "context"
    "github.com/mentatxx/go-zarr/ndarray"
    "github.com/mentatxx/go-zarr/store"
    "github.com/mentatxx/go-zarr/v3"
)

func main() {
    ctx := context.Background()
    h := store.NewFilesystem("/path/to/zarr").Resolve("array")
    meta, _ := v3.NewMetadataBuilder().
        WithShape(100, 100).
        WithDataType(ndarray.Float32).
        WithChunkShape(10, 10).
        WithFillValue(0).
        WithCodecs(func(b *v3.CodecBuilder) *v3.CodecBuilder { return b.WithBlosc() }).
        Build()
    arr, _ := v3.Create(ctx, h, meta, false)
    data := ndarray.New(ndarray.Float32, []int{10, 10}, nil)
    _ = arr.Write(ctx, []int64{0, 0}, data)
}
```

## Tests

```bash
go test ./...
# optional Python interop (uv + zarr)
go test -tags python ./...
# optional c-blosc
go test -tags cblosc ./...
```

Fixture `testdata/l4_sample` (used by some integration tests) can be downloaded with `make testdata`.

## License

MIT. Algorithms and tests follow zarr-java (MIT).
