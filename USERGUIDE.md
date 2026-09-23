# go-zarr User Guide

Go implementation of the [Zarr](https://zarr.dev/) specification (v2 and v3), ported from [zarr-java](https://github.com/zarr-developers/zarr-java).

Русская версия: [USERGUIDE.ru.md](USERGUIDE.ru.md).

## Install

```bash
go get github.com/mentatxx/go-zarr
```

Go 1.22+ is required.

## Open an array or group

```go
n, err := zarr.OpenPath(ctx, "/path/to/zarr")
switch a := n.(type) {
case *v3.Array:
    data, _ := a.Read(ctx, nil, nil)
case *v3.Group:
    child, _ := a.Get(ctx, "color")
case *v2.Array, *v2.Group:
    // Zarr v2
}
```

`zarr.Open` / `OpenArray` / `OpenGroup` auto-detect v2 vs v3 from `zarr.json`, `.zarray`, and `.zgroup`.

## Create a v3 array

```go
h := store.NewFilesystem("/data").Resolve("myarray")
meta, err := v3.NewMetadataBuilder().
    WithShape(1000, 1000).
    WithDataType(ndarray.Float32).
    WithChunkShape(100, 100).
    WithFillValue(0).
    WithCodecs(func(b *v3.CodecBuilder) *v3.CodecBuilder {
        return b.WithBlosc()
    }).
    Build()
arr, err := v3.Create(ctx, h, meta, false)
_ = arr.Write(ctx, []int64{0, 0}, ndarray.New(ndarray.Float32, []int{100, 100}, nil))
subset, err := arr.Read(ctx, []int64{10, 10}, []int64{20, 20})
```

## Create a v2 array

```go
meta, err := v2.NewMetadataBuilder().
    WithShape(100, 100).
    WithChunks(10, 10).
    WithDataType("<f4").
    WithFillValue(0).
    WithZlibCompressor(1).
    Build()
arr, err := v2.Create(ctx, h, meta, false)
```

## Stores

| Store | Constructor | Notes |
| --- | --- | --- |
| Filesystem | `store.NewFilesystem(root)` | default |
| Memory | `store.NewMemory()` | tests |
| HTTP | `store.NewHTTP(baseURL)` | read-only |
| S3 | `store.NewS3(client, bucket, prefix)` | aws-sdk-go-v2 |
| ZIP | `store.NewReadOnlyZip(handle)` | read |
| Buffered ZIP | `store.NewBufferedZip(ctx, handle)` | write + `Flush` |

`store.Handle` is a location (`Store` + key path). Use `h.Resolve("child")`.

## Codecs (v3 pipeline)

Array→array (`transpose`, `reshape`, `cast_value`) → exactly one array→bytes (`bytes` or `sharding_indexed`) → bytes→bytes (`gzip`, `zstd`, `blosc`, `crc32c`).

Blosc is pure Go by default (`lz4`, `lz4hc`, `zstd`, `zlib`, `snappy`). `blosclz` needs `-tags cblosc` and system c-blosc.

## Groups

```go
g, _ := v3.CreateGroup(ctx, h, v3.DefaultGroupMetadata())
sub, _ := g.CreateGroup(ctx, "color", nil)
_, _ = sub.CreateArray(ctx, "0", func(b *v3.MetadataBuilder) *v3.MetadataBuilder {
    return b.WithShape(64, 64).WithDataType(ndarray.Uint16).WithChunkShape(32, 32)
})
```

## OME-Zarr

```go
im, _ := ome.Open(ctx, h)          // v0.4 / v0.5 / v0.6 image
_ = im.AxisNames()
level0, _ := im.OpenScaleLevel(ctx, 0)
plate, _ := ome.OpenPlate(ctx, h)
scene, _ := ome.OpenScene(ctx, h)  // v0.6 scene
```

## Parallel I/O

Reads/writes use a worker pool by default. Disable with `arr.SetParallel(false)`.

## CLI (conformance)

```bash
go run ./cmd/zarr --array_path /path/to/array
```

Prints the full array (used by zarr-conformance-tests).

## Tests

```bash
go test ./...
go test -tags python ./...   # uv + zarr Python
RUN_S3_TESTS=1 go test ./store -run TestS3Store
make testdata                # download l4_sample fixture
```
