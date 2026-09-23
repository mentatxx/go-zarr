# Руководство пользователя go-zarr

Порт [zarr-java](https://github.com/zarr-developers/zarr-java) на Go: Zarr v2 и v3, сторы, кодеки (включая sharding), OME-Zarr, CLI.

Английская версия: [USERGUIDE.md](USERGUIDE.md).

## Установка

```bash
go get github.com/mentatxx/go-zarr
```

Нужен Go 1.22+.

## Открытие

`zarr.Open` / `OpenArray` / `OpenGroup` / `OpenPath` сами определяют версию по `zarr.json`, `.zarray` и `.zgroup`.

## Запись v3

```go
h := store.NewFilesystem("/data").Resolve("arr")
meta, _ := v3.NewMetadataBuilder().
    WithShape(1000, 1000).
    WithDataType(ndarray.Float32).
    WithChunkShape(100, 100).
    WithFillValue(0).
    WithCodecs(func(b *v3.CodecBuilder) *v3.CodecBuilder { return b.WithGzip(5) }).
    Build()
arr, _ := v3.Create(ctx, h, meta, false)
```

## Сторы

Файловая система, память, HTTP (только чтение), S3 (`aws-sdk-go-v2`), ZIP и BufferedZip.

## Кодеки

Цепочка как в спецификации v3: ArrayArray → ровно один ArrayBytes → BytesBytes.

Blosc по умолчанию — чистый Go. Для `blosclz` и полного c-blosc собирайте с `-tags cblosc`.

## OME-Zarr

`ome.Open` — изображение v0.4/v0.5/v0.6, `ome.OpenPlate` — HCS-плашка, `ome.OpenScene` — сцена v0.6.

## CLI

```bash
go run ./cmd/zarr --array_path /path/to/array
```
