# Руководство пользователя go-zarr

Реализация спецификации [Zarr](https://zarr.dev/) (v2 и v3) на Go, порт [zarr-java](https://github.com/zarr-developers/zarr-java).

English version: [USERGUIDE.md](USERGUIDE.md).

Каждый пример на Go ниже — законченная программа. Примеры с меткой `// doc:run` выполняет `go test -run TestUserGuide`. Примеры с меткой `// doc:build` только компилируются: им нужен удалённый сервер или уже существующее дерево OME-Zarr.

## Установка

Нужен Go 1.27 или новее, как в `go.mod`.

```bash
go get github.com/mentatxx/go-zarr
```

Корневой пакет нужен, когда версию надо определить автоматически. Пакеты `v2` и `v3` — когда формат уже известен.

```text
github.com/mentatxx/go-zarr          определение версии, ошибки, атрибуты
github.com/mentatxx/go-zarr/v3       массивы и группы Zarr v3
github.com/mentatxx/go-zarr/v2       массивы и группы Zarr v2
github.com/mentatxx/go-zarr/store    файловая система, память, HTTP, S3, ZIP
github.com/mentatxx/go-zarr/ndarray  массивы в памяти для Read и Write
github.com/mentatxx/go-zarr/ome      чтение OME-Zarr
```

## Модель данных

**Store** — ключ-значение (`store.Store`). **Handle** (`store.Handle`) — одно место внутри store: сам store и префикс ключа. `Resolve` дописывает сегменты пути и возвращает новый handle. Массив или группа лежат на handle, а не в корне store, если вы не вызвали `Resolve` без дополнительных ключей.

Zarr v3 хранит метаданные массива и группы в `zarr.json` (`node_type` равен `"array"` или `"group"`). Zarr v2 использует `.zarray`, `.zgroup` и `.zattrs`.

Массив режется на **чанки**. Чанк, целиком равный fill value, удаляется; последующее чтение этой области возвращает fill value. `Read(ctx, nil, nil)` и `Write(ctx, nil, data)` используют нулевое смещение, а чтение без shape — всю форму массива.

`ndarray.Array` — буфер в памяти. Его форма имеет тип `[]int`. Смещения и формы в `Read`, `Write`, `Resize` и координаты чанка имеют тип `[]int64`. Порядок элементов — C (по строкам). `nil` в качестве порядка байт означает little-endian.

## Массивы в памяти

В `Write` передаётся `ndarray`, и `Read` возвращает `ndarray`. Линейный индекс `0` — первый элемент в порядке C. `Section` копирует прямоугольник в новый массив. `FromBytes` оборачивает уже существующий буфер; длина должна быть `product(shape) * dtype.Size()`.

Поддерживаются типы `bool`, `int8`, `int16`, `int32`, `int64`, `uint8`, `uint16`, `uint32`, `uint64`, `float32` и `float64`.

```go
// doc:run
// example: ndarray
package main

import (
	"github.com/mentatxx/go-zarr/ndarray"
)

func main() {
	data := ndarray.New(ndarray.Float32, []int{2, 3}, nil)
	data.Fill(1)
	data.SetFloat64(0, 4)
	if data.GetFloat64(0) != 4 || data.Len() != 6 {
		panic(data)
	}
	sec, err := data.Section([]int{0, 1}, []int{2, 2})
	if err != nil {
		panic(err)
	}
	if sec.Shape[0] != 2 || sec.Shape[1] != 2 || sec.GetFloat64(0) != 1 {
		panic(sec)
	}
	raw := []byte{0, 0, 128, 63}
	wrapped, err := ndarray.FromBytes(ndarray.Float32, []int{1}, nil, raw)
	if err != nil {
		panic(err)
	}
	if wrapped.GetFloat64(0) != 1 {
		panic(wrapped.GetFloat64(0))
	}
}
```

`GetInt64` / `SetInt64` и `GetUint64` / `SetUint64` приводят значение к типу массива и обратно. `SetValue` принимает значение Go подходящего вида (`bool`, целое, float).

## Создание массива v3

`v3.NewMetadataBuilder` требует форму и тип данных. Если форму чанка не задать, её выбирает `zarr.DefaultChunkShape` (около 512 элементов по каждой оси или вся ось, если она короче). Если кодеки не задать, цепочка состоит из одного little-endian кодека `bytes`.

`v3.Create` записывает `zarr.json`. При `existsOk == false` и уже существующем файле возвращается `zarr.ErrExists`. При `existsOk == true` документ метаданных заменяется, чанки при этом не удаляются.

`WithV2ChunkKeyEncoding` кладёт чанк в ключ `0.1` вместо `c/0/1` по умолчанию. `WithAttributes` и `WithDimensionNames` сохраняются в метаданных массива.

```go
// doc:run
// example: create-v3
package main

import (
	"context"

	"github.com/mentatxx/go-zarr"
	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
	v3 "github.com/mentatxx/go-zarr/v3"
)

func main() {
	ctx := context.Background()
	h := store.NewMemory().Resolve("temperature")
	meta, err := v3.NewMetadataBuilder().
		WithShape(8, 8).
		WithDataType(ndarray.Float32).
		WithChunkShape(4, 4).
		WithFillValue(0).
		WithAttributes(zarr.Attributes{"units": "K"}).
		WithDimensionNames("y", "x").
		WithV2ChunkKeyEncoding().
		WithCodecs(func(b *v3.CodecBuilder) *v3.CodecBuilder {
			return b.WithBlosc()
		}).
		Build()
	if err != nil {
		panic(err)
	}
	arr, err := v3.Create(ctx, h, meta, false)
	if err != nil {
		panic(err)
	}
	tile := ndarray.New(ndarray.Float32, []int{4, 4}, nil)
	tile.Fill(21)
	if err := arr.Write(ctx, []int64{0, 0}, tile); err != nil {
		panic(err)
	}
	got, err := arr.Read(ctx, []int64{1, 1}, []int64{2, 2})
	if err != nil {
		panic(err)
	}
	if got.Shape[0] != 2 || got.GetFloat64(0) != 21 {
		panic(got)
	}
	units, err := arr.Metadata().Attributes.GetString("units")
	if err != nil || units != "K" {
		panic(units)
	}
	names := arr.Metadata().DimensionNames
	if len(names) != 2 || names[0] != "y" || names[1] != "x" {
		panic(names)
	}
	key := arr.Metadata().EncodeChunkKey([]int64{0, 1})
	if len(key) != 1 || key[0] != "0.1" {
		panic(key)
	}
	whole, err := arr.Read(ctx, nil, nil)
	if err != nil || whole.Shape[0] != 8 || whole.GetFloat64(0) != 21 {
		panic(whole)
	}
	outside := whole.GetFloat64(4 * 8)
	if outside != 0 {
		panic(outside)
	}
}
```

`WithBlosc()` без аргументов — это Blosc `zstd`, shuffle `noshuffle`, уровень `5`. Сборщик сам вставляет кодек `bytes` перед Blosc: в цепочке обязан быть ровно один кодек array-to-bytes. Ячейка с линейным индексом `4*8` — начало второй строки, её не записывали, поэтому чтение возвращает fill value `0`.

`SetAttributes` переписывает метаданные и возвращает новый `*v3.Array`. Тот же метод есть у групп v3 и у массивов v2. Нужно использовать возвращённое значение: получатель на месте не обновляется.

## Открытие существующего узла

`zarr.Open`, `OpenArray`, `OpenGroup` и `OpenPath` смотрят на `zarr.json`, `.zarray` и `.zgroup` и возвращают подходящее значение v2 или v3. Тип результата — `zarr.Node` (`any`). Место, где лежат метаданные и v2, и v3 сразу, считается ошибкой.

`v3.Open` / `v2.Open` открывают массив только этой версии. То же разделение есть для групп (`OpenGroup`) и для «массив или группа» (`OpenNode`). `OpenPath` — сокращение для файловой системы: в корневом пакете это `Open(ctx, store.NewFilesystem(path).Resolve())`, в `v2` и `v3` — тот же приём для своей версии.

```go
// doc:run
// example: open
package main

import (
	"context"

	"github.com/mentatxx/go-zarr"
	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
	v2 "github.com/mentatxx/go-zarr/v2"
	v3 "github.com/mentatxx/go-zarr/v3"
)

func main() {
	ctx := context.Background()
	mem := store.NewMemory()

	meta, err := v3.NewMetadataBuilder().
		WithShape(4, 4).
		WithDataType(ndarray.Int32).
		WithChunkShape(2, 2).
		WithFillValue(0).
		Build()
	if err != nil {
		panic(err)
	}
	created, err := v3.Create(ctx, mem.Resolve("cube"), meta, false)
	if err != nil {
		panic(err)
	}
	buf := ndarray.New(ndarray.Int32, []int{4, 4}, nil)
	buf.SetInt64(0, 9)
	if err := created.Write(ctx, nil, buf); err != nil {
		panic(err)
	}

	n, err := zarr.Open(ctx, mem.Resolve("cube"))
	if err != nil {
		panic(err)
	}
	arr, ok := n.(*v3.Array)
	if !ok {
		panic(n)
	}
	got, err := arr.Read(ctx, nil, nil)
	if err != nil || got.GetInt64(0) != 9 {
		panic(got)
	}

	asArray, err := zarr.OpenArray(ctx, mem.Resolve("cube"))
	if err != nil {
		panic(err)
	}
	if _, ok := asArray.(*v3.Array); !ok {
		panic(asArray)
	}

	g, err := v2.CreateGroup(ctx, mem.Resolve("legacy"), zarr.Attributes{"gen": "v2"})
	if err != nil {
		panic(err)
	}
	if _, err := g.CreateArray(ctx, "a", func(b *v2.MetadataBuilder) *v2.MetadataBuilder {
		return b.WithShape(2).WithChunks(2).WithDataType("<i4").WithFillValue(0)
	}); err != nil {
		panic(err)
	}
	node, err := zarr.Open(ctx, mem.Resolve("legacy"))
	if err != nil {
		panic(err)
	}
	if _, ok := node.(*v2.Group); !ok {
		panic(node)
	}
}
```

## Чтение, чанки и fill value

`Read(ctx, offset, shape)` возвращает новый `ndarray` для этого прямоугольника. `ReadChunk` / `WriteChunk` адресуют один чанк координатами сетки (`0, 0` — чанк в начале). Эти два метода есть у `*v3.Array`. В v2 чтение и запись идут только через `Read` и `Write`.

Отсутствующий чанк — не ошибка. `ReadChunk` возвращает массив, заполненный fill value. `Write` удаляет чанк, если каждое значение равно fill value: следующий `Exists` по ключу чанка даёт false, а чтение по-прежнему возвращает fill value.

Сборщики принимают fill value как число Go и как строки `"NaN"`, `"Infinity"`, `"-Infinity"`, шестнадцатеричную строку вроде `"0x2a"` или двоичную вроде `"0b1010"` для целых типов. `zarr.ParseFillValue` применяет те же правила.

```go
// doc:run
// example: chunks-fill
package main

import (
	"context"
	"math"

	"github.com/mentatxx/go-zarr"
	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
	v3 "github.com/mentatxx/go-zarr/v3"
)

func main() {
	ctx := context.Background()
	nan, err := zarr.ParseFillValue("NaN", ndarray.Float64)
	if err != nil || !math.IsNaN(nan.(float64)) {
		panic(nan)
	}
	meta, err := v3.NewMetadataBuilder().
		WithShape(4, 4).
		WithDataType(ndarray.Int32).
		WithChunkShape(4, 4).
		WithFillValue(-1).
		WithDefaultChunkKeyEncoding().
		Build()
	if err != nil {
		panic(err)
	}
	arr, err := v3.Create(ctx, store.NewMemory().Resolve("a"), meta, false)
	if err != nil {
		panic(err)
	}
	missing, err := arr.ReadChunk(ctx, []int64{0, 0})
	if err != nil || missing.GetInt64(0) != -1 {
		panic(missing)
	}
	data := ndarray.New(ndarray.Int32, []int{4, 4}, nil)
	data.SetInt64(2, 5)
	if err := arr.Write(ctx, nil, data); err != nil {
		panic(err)
	}
	key := arr.Metadata().EncodeChunkKey([]int64{0, 0})
	ok, err := arr.Handle().Resolve(key...).Exists(ctx)
	if err != nil || !ok {
		panic(ok)
	}
	data.Fill(-1)
	if err := arr.Write(ctx, nil, data); err != nil {
		panic(err)
	}
	ok, err = arr.Handle().Resolve(key...).Exists(ctx)
	if err != nil || ok {
		panic("fill chunk should be deleted")
	}
	again, err := arr.Read(ctx, []int64{0, 0}, []int64{1, 1})
	if err != nil || again.GetInt64(0) != -1 {
		panic(again)
	}
}
```

Ключи чанков по умолчанию имеют вид `c/<z>/<y>/<x>`, если разделитель — `/`. Разделитель задают `WithDefaultChunkKeyEncoding` (`/`) и `WithV2ChunkKeyEncoding` (`.`).

## Кодеки

Цепочка v3 идёт в таком порядке:

1. Ноль или больше кодеков array-to-array: `transpose`, `reshape`, `cast_value`.
2. Ровно один кодек array-to-bytes: `bytes` или `sharding_indexed`.
3. Ноль или больше кодеков bytes-to-bytes: `gzip`, `zstd`, `blosc`, `crc32c`.

`CodecBuilder.Build` вставляет little-endian `bytes`, если кодек array-to-bytes вы не добавили сами. `WithBytes("little")`, `WithBytes("LITTLE")`, `WithBytes("big")` и `WithBytes("BIG")` выбирают порядок байт. Неверные аргументы сборщика вызывают panic. Несогласованная цепочка заставляет `MetadataBuilder.Build` вернуть ошибку.

| Метод | Смысл |
| --- | --- |
| `WithGzip(level...)` | gzip, уровень по умолчанию 5 |
| `WithZstd(level, checksum)` | zstd, по умолчанию уровень 5 и контрольная сумма включена. Вызов: `WithZstd()` или `WithZstd(3, false)` |
| `WithBlosc(cname, shuffleOrLevel, level)` | Blosc. По умолчанию: `zstd`, `noshuffle`, уровень 5. `cname`: `lz4`, `lz4hc`, `zstd`, `zlib`, `snappy` или `blosclz` |
| `WithCrc32c()` | контрольная сумма CRC32C в конце |
| `WithTranspose(order)` | перестановка осей чанка, например `[]int{1, 0}` |
| `WithReshape(shape)` | меняет форму **чанка**. Элементы — `float64` (числа JSON). Один `float64(-1)` выводится. Произведение формы должно совпасть с чанком |
| `WithCastValue(dtype)` | хранит чанк в другом числовом типе и приводит обратно при чтении. Усечение идёт через числовое значение |
| `WithSharding(inner, nested, indexLocation...)` | шард, внутренняя форма чанка делит внешнюю по каждой оси. Положение индекса — `"end"` (по умолчанию) или `"start"`. Кодек индекса — `bytes` + `crc32c` |

Чистый Go-Blosc не реализует `blosclz`. Этот компрессор требует `go build -tags cblosc` и системный c-blosc (`pkg-config: blosc`). Shuffle: `noshuffle`, `shuffle` или `bitshuffle`. Уровень сжатия — от 0 до 9. Второй аргумент `WithBlosc` — имя shuffle, если это одна из этих трёх строк; иначе это уровень: `WithBlosc("lz4", 1)`.

`reshape` и `transpose` меняют представление чанка. Форма, которую вы передаёте в `Read`, остаётся формой из `WithShape`. `cast_value` не меняет `Metadata().DataType`: значения из `Read` приводятся обратно.

```go
// doc:run
// example: codecs
package main

import (
	"context"

	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
	v3 "github.com/mentatxx/go-zarr/v3"
)

func main() {
	ctx := context.Background()
	type spec struct {
		name   string
		shape  []int64
		chunks []int
		dt     ndarray.DType
		pipe   func(*v3.CodecBuilder) *v3.CodecBuilder
	}
	cases := []spec{
		{"gzip", []int64{8, 8}, []int{4, 4}, ndarray.Int32, func(b *v3.CodecBuilder) *v3.CodecBuilder {
			return b.WithGzip(5)
		}},
		{"zstd", []int64{8, 8}, []int{4, 4}, ndarray.Int32, func(b *v3.CodecBuilder) *v3.CodecBuilder {
			return b.WithZstd(3, false)
		}},
		{"blosc", []int64{8, 8}, []int{4, 4}, ndarray.Int32, func(b *v3.CodecBuilder) *v3.CodecBuilder {
			return b.WithBlosc("lz4", "shuffle", 1)
		}},
		{"crc", []int64{8, 8}, []int{4, 4}, ndarray.Int32, func(b *v3.CodecBuilder) *v3.CodecBuilder {
			return b.WithCrc32c()
		}},
		{"bytes", []int64{8, 8}, []int{4, 4}, ndarray.Int32, func(b *v3.CodecBuilder) *v3.CodecBuilder {
			return b.WithBytes("big")
		}},
		{"transpose", []int64{4, 6}, []int{4, 6}, ndarray.Int32, func(b *v3.CodecBuilder) *v3.CodecBuilder {
			return b.WithTranspose([]int{1, 0})
		}},
		{"reshape", []int64{8, 8}, []int{8, 8}, ndarray.Int32, func(b *v3.CodecBuilder) *v3.CodecBuilder {
			return b.WithReshape([]any{float64(64)})
		}},
		{"cast", []int64{4, 4}, []int{4, 4}, ndarray.Float64, func(b *v3.CodecBuilder) *v3.CodecBuilder {
			return b.WithCastValue(ndarray.Int32)
		}},
		{"shard", []int64{8, 8}, []int{4, 8}, ndarray.Uint16, func(b *v3.CodecBuilder) *v3.CodecBuilder {
			return b.WithSharding([]int{2, 4}, func(n *v3.CodecBuilder) *v3.CodecBuilder {
				return n.WithGzip()
			}, "end")
		}},
	}
	for _, c := range cases {
		meta, err := v3.NewMetadataBuilder().
			WithShape(c.shape...).
			WithDataType(c.dt).
			WithChunkShape(c.chunks...).
			WithFillValue(0).
			WithCodecs(c.pipe).
			Build()
		if err != nil {
			panic(c.name + ": " + err.Error())
		}
		arr, err := v3.Create(ctx, store.NewMemory().Resolve(c.name), meta, false)
		if err != nil {
			panic(err)
		}
		sh := make([]int, len(c.shape))
		for i, n := range c.shape {
			sh[i] = int(n)
		}
		data := ndarray.New(c.dt, sh, nil)
		for i := 0; i < data.Len(); i++ {
			data.SetInt64(i, int64(i+1))
		}
		if err := arr.Write(ctx, nil, data); err != nil {
			panic(c.name + ": " + err.Error())
		}
		got, err := arr.Read(ctx, nil, nil)
		if err != nil {
			panic(err)
		}
		if got.DType != c.dt || got.GetInt64(3) != 4 {
			panic(c.name)
		}
	}
}
```

## Группы и атрибуты

Группа v3 — это `zarr.json` с `node_type: "group"`. Её пишет `v3.CreateGroup`. `DefaultGroupMetadata()` — пустая группа. `CreateGroup(name, attrs)` и `CreateArray(name, build)` создают потомков. `Get` возвращает `v3.Node`: это `*v3.Array` или `*v3.Group`. `CreateArray` всегда отказывается заменить существующего потомка (внутри метода `existsOk` равен false).

Атрибуты — JSON-объект (`zarr.Attributes`, псевдоним `map[string]any`). Типизированные методы: `GetString`, `GetBool`, `GetInt`, `GetFloat64`, `GetFloat32`, `GetList` и `GetAttributes`.

```go
// doc:run
// example: groups
package main

import (
	"context"

	"github.com/mentatxx/go-zarr"
	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
	v3 "github.com/mentatxx/go-zarr/v3"
)

func main() {
	ctx := context.Background()
	root := store.NewMemory().Resolve("root")
	g, err := v3.CreateGroup(ctx, root, v3.DefaultGroupMetadata())
	if err != nil {
		panic(err)
	}
	g, err = g.SetAttributes(ctx, zarr.Attributes{"title": "demo"})
	if err != nil {
		panic(err)
	}
	sub, err := g.CreateGroup(ctx, "color", zarr.Attributes{"channel": "red"})
	if err != nil {
		panic(err)
	}
	arr, err := sub.CreateArray(ctx, "0", func(b *v3.MetadataBuilder) *v3.MetadataBuilder {
		return b.WithShape(4, 4).WithDataType(ndarray.Uint16).WithChunkShape(2, 2).WithFillValue(0)
	})
	if err != nil {
		panic(err)
	}
	px := ndarray.New(ndarray.Uint16, []int{2, 2}, nil)
	px.Fill(uint16(7))
	if err := arr.Write(ctx, []int64{0, 0}, px); err != nil {
		panic(err)
	}

	opened, err := zarr.OpenGroup(ctx, root)
	if err != nil {
		panic(err)
	}
	vg := opened.(*v3.Group)
	title, err := vg.Metadata().Attributes.GetString("title")
	if err != nil || title != "demo" {
		panic(title)
	}
	child, err := vg.Get(ctx, "color")
	if err != nil {
		panic(err)
	}
	channel, err := child.(*v3.Group).Metadata().Attributes.GetString("channel")
	if err != nil || channel != "red" {
		panic(channel)
	}
	node, err := child.(*v3.Group).Get(ctx, "0")
	if err != nil {
		panic(err)
	}
	got, err := node.(*v3.Array).Read(ctx, []int64{0, 0}, []int64{1, 1})
	if err != nil || got.GetUint64(0) != 7 {
		panic(got)
	}
}
```

Отдельного читателя consolidated metadata нет. Поле `consolidated_metadata` сохраняется в `v3.GroupMetadata`, если оно есть в `zarr.json`. Список ключей даёт сам store, когда вы вызываете `List`.

## Zarr v2

Метаданные v2 собирает `v2.NewMetadataBuilder`. `WithChunks` — форма чанка. `WithDataType` принимает строку в стиле NumPy (`"<f4"`, `">f8"`, `"|b1"`, `"i1"`). `WithDataTypeSpec(ndarray.Float32, bigEndian)` форматирует такую строку за вас. Однобайтовые типы пишутся без префикса порядка байт (`|b1`, `|i1`, `|u1`).

Компрессор по умолчанию отсутствует (`null` в `.zarray`). Выберите `WithZlibCompressor`, `WithGzipCompressor`, `WithZstdCompressor(level, checksum)` или `WithBloscCompressor(cname, shuffle, clevel)`. Атрибуты массива лежат в `.zattrs`, а не внутри `.zarray`. `SetAttributes` переписывает оба файла.

`v2.CreateGroup(ctx, handle, attrs)` пишет `.zgroup` и `.zattrs`. Метод `(*v2.Group).CreateGroup(ctx, name)` создаёт потомка с пустыми атрибутами; атрибуты передаются только в `CreateGroup` уровня пакета.

```go
// doc:run
// example: v2
package main

import (
	"context"

	"github.com/mentatxx/go-zarr"
	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
	v2 "github.com/mentatxx/go-zarr/v2"
)

func main() {
	ctx := context.Background()
	mem := store.NewMemory()
	type spec struct {
		name string
		dt   ndarray.DType
		raw  string
		pipe func(*v2.MetadataBuilder) *v2.MetadataBuilder
	}
	cases := []spec{
		{"zlib", ndarray.Int32, "", func(b *v2.MetadataBuilder) *v2.MetadataBuilder {
			return b.WithDataTypeSpec(ndarray.Int32, false).WithZlibCompressor(1)
		}},
		{"gzip", ndarray.Int16, "<i2", func(b *v2.MetadataBuilder) *v2.MetadataBuilder {
			return b.WithDataType("<i2").WithGzipCompressor(1)
		}},
		{"zstd", ndarray.Float64, "", func(b *v2.MetadataBuilder) *v2.MetadataBuilder {
			return b.WithDataTypeSpec(ndarray.Float64, true).WithZstdCompressor(1, false)
		}},
		{"blosc", ndarray.Uint8, "", func(b *v2.MetadataBuilder) *v2.MetadataBuilder {
			return b.WithDataType("|u1").WithBloscCompressor("zstd", "noshuffle", 5)
		}},
	}
	for _, c := range cases {
		b := v2.NewMetadataBuilder().WithShape(4, 4).WithChunks(2, 2).WithFillValue(0).WithAttributes(zarr.Attributes{"note": c.name})
		meta, err := c.pipe(b).Build()
		if err != nil {
			panic(err)
		}
		arr, err := v2.Create(ctx, mem.Resolve(c.name), meta, false)
		if err != nil {
			panic(err)
		}
		data := ndarray.New(c.dt, []int{4, 4}, nil)
		data.SetInt64(5, 3)
		if err := arr.Write(ctx, nil, data); err != nil {
			panic(c.name + ": " + err.Error())
		}
		got, err := arr.Read(ctx, nil, nil)
		if err != nil || got.GetInt64(5) != 3 {
			panic(c.name)
		}
		note, err := arr.Metadata().Attributes.GetString("note")
		if err != nil || note != c.name {
			panic(note)
		}
	}

	g, err := v2.CreateGroup(ctx, mem.Resolve("grp"), zarr.Attributes{"name": "root"})
	if err != nil {
		panic(err)
	}
	sub, err := g.CreateGroup(ctx, "child")
	if err != nil {
		panic(err)
	}
	if _, err := sub.CreateArray(ctx, "a", func(b *v2.MetadataBuilder) *v2.MetadataBuilder {
		return b.WithShape(4).WithChunks(2).WithDataType("<i4").WithFillValue(0)
	}); err != nil {
		panic(err)
	}
	opened, err := zarr.OpenGroup(ctx, mem.Resolve("grp"))
	if err != nil {
		panic(err)
	}
	name, err := opened.(*v2.Group).Attributes().GetString("name")
	if err != nil || name != "root" {
		panic(name)
	}
	child, err := opened.(*v2.Group).Get(ctx, "child")
	if err != nil {
		panic(err)
	}
	if _, err := child.(*v2.Group).Get(ctx, "a"); err != nil {
		panic(err)
	}
}
```

Массивы v2 хранятся в порядке C. Флаг Fortran `order` в метаданных не реализован как отдельная раскладка в памяти.

## Хранилища

| Store | Конструктор | Чтение | Запись | Список |
| --- | --- | --- | --- | --- |
| Файловая система | `store.NewFilesystem(root)` | да | да | да |
| Память | `store.NewMemory()` | да | да | да |
| HTTP | `store.NewHTTP(baseURL)` | да | нет | нет |
| S3 | `store.NewS3(client, bucket, prefix)` | да | да | да |
| ZIP | `store.NewReadOnlyZip(handle)` | да | нет | да |
| Буферизованный ZIP | `store.NewBufferedZip(ctx, handle)` | да | да, до `Flush` | да |

У `HTTP` таймаут клиента — 60 секунд. `Set` и `Delete` возвращают ошибку. Клиент — экспортируемое поле `HTTP.Client`, его можно заменить.

`NewS3` принимает `*s3.Client` из `github.com/aws/aws-sdk-go-v2/service/s3`. У префикса срезаются крайние слэши, и он добавляется к каждому ключу.

`NewBufferedZip` загружает существующий архив в память, если объект внизу непустой. Запись остаётся в этом буфере, пока `Flush` (его же вызывает `Close`) не сохранит zip по нижележащему handle. `NewReadOnlyZip` читает этот объект.

Методы handle для прикладного кода: `Resolve`, `Exists`, `Read`, `ReadRange` (`end < 0` значит «до конца»), `Set`, `Delete`, `Size` (для отсутствующего объекта возвращается `-1`), `OpenReader` и `ToPath` (только handle файловой системы). `store.KeyPath` склеивает сегменты ключа через `/`.

```go
// doc:run
// example: filesystem
package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/mentatxx/go-zarr"
	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
	v3 "github.com/mentatxx/go-zarr/v3"
)

func main() {
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "go-zarr-guide")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	root := store.NewFilesystem(dir)
	h := root.Resolve("array")
	meta, err := v3.NewMetadataBuilder().
		WithShape(4, 4).
		WithDataType(ndarray.Int16).
		WithChunkShape(2, 2).
		WithFillValue(0).
		Build()
	if err != nil {
		panic(err)
	}
	arr, err := v3.Create(ctx, h, meta, false)
	if err != nil {
		panic(err)
	}
	data := ndarray.New(ndarray.Int16, []int{4, 4}, nil)
	data.SetInt64(0, 8)
	if err := arr.Write(ctx, nil, data); err != nil {
		panic(err)
	}
	path, err := h.ToPath()
	if err != nil || path != filepath.Join(dir, "array") {
		panic(path)
	}
	opened, err := zarr.OpenPath(ctx, path)
	if err != nil {
		panic(err)
	}
	got, err := opened.(*v3.Array).Read(ctx, nil, nil)
	if err != nil || got.GetInt64(0) != 8 {
		panic(got)
	}
	found := false
	for keys, err := range root.List(ctx, nil) {
		if err != nil {
			panic(err)
		}
		if strings.HasSuffix(store.KeyPath(keys), "zarr.json") {
			found = true
		}
	}
	if !found {
		panic("zarr.json not listed")
	}
}
```

```go
// doc:run
// example: zip
package main

import (
	"context"

	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
	v3 "github.com/mentatxx/go-zarr/v3"
)

func main() {
	ctx := context.Background()
	mem := store.NewMemory()
	archive := mem.Resolve("dataset.zip")
	buf, err := store.NewBufferedZip(ctx, archive)
	if err != nil {
		panic(err)
	}
	meta, err := v3.NewMetadataBuilder().
		WithShape(4).
		WithDataType(ndarray.Int32).
		WithChunkShape(2).
		WithFillValue(0).
		Build()
	if err != nil {
		panic(err)
	}
	arr, err := v3.Create(ctx, buf.Resolve("arr"), meta, false)
	if err != nil {
		panic(err)
	}
	data := ndarray.New(ndarray.Int32, []int{4}, nil)
	data.SetInt64(1, 6)
	if err := arr.Write(ctx, nil, data); err != nil {
		panic(err)
	}
	if err := buf.Flush(ctx); err != nil {
		panic(err)
	}
	opened, err := v3.Open(ctx, store.NewReadOnlyZip(archive).Resolve("arr"))
	if err != nil {
		panic(err)
	}
	got, err := opened.Read(ctx, nil, nil)
	if err != nil || got.GetInt64(1) != 6 {
		panic(got)
	}
}
```

HTTP и S3 тест документации только компилирует и здесь не запускает.

```go
// doc:build
// example: http
package main

import (
	"context"
	"fmt"

	"github.com/mentatxx/go-zarr"
	"github.com/mentatxx/go-zarr/store"
	v3 "github.com/mentatxx/go-zarr/v3"
)

func main() {
	ctx := context.Background()
	h := store.NewHTTP("https://example.com/dataset.zarr").Resolve()
	if err := h.Set(ctx, []byte("nope")); err == nil {
		panic("HTTP store is read-only")
	}
	n, err := zarr.Open(ctx, h)
	if err != nil {
		fmt.Println(err)
		return
	}
	if arr, ok := n.(*v3.Array); ok {
		_, _ = arr.Read(ctx, nil, nil)
	}
}
```

```go
// doc:build
// example: s3
package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/mentatxx/go-zarr/store"
)

func main() {
	client := s3.New(s3.Options{Region: "us-east-1"})
	h := store.NewS3(client, "my-bucket", "prefix/dataset").Resolve("array")
	fmt.Println(h)
}
```

В `s3.Options` задаются endpoint, регион и учётные данные. `store.NewS3` хранит только клиент, бакет и префикс ключа.

## Изменение формы и параллельный ввод-вывод

`Resize(ctx, newShape, metadataOnly)` сохраняет ту же размерность. Дальше нужно пользоваться возвращённым массивом. При `metadataOnly == false` удаляются чанки за пределами новой формы. При `metadataOnly == true` обновляется `zarr.json`, а объекты чанков остаются на месте. Ячейки, которые не записывали, в том числе добавленные ростом формы, читаются как fill value.

Чтение и запись используют пул из 8 воркеров, когда параллельный ввод-вывод включён. Так и есть по умолчанию. `SetParallel(false)` выполняет операции с чанками этого массива по одной. У v2 тот же переключатель и та же сигнатура `Resize`, но нет `ReadChunk` / `WriteChunk`.

```go
// doc:run
// example: resize
package main

import (
	"context"

	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
	v3 "github.com/mentatxx/go-zarr/v3"
)

func main() {
	ctx := context.Background()
	meta, err := v3.NewMetadataBuilder().
		WithShape(8, 8).
		WithDataType(ndarray.Int32).
		WithChunkShape(4, 4).
		WithFillValue(0).
		Build()
	if err != nil {
		panic(err)
	}
	arr, err := v3.Create(ctx, store.NewMemory().Resolve("r"), meta, false)
	if err != nil {
		panic(err)
	}
	arr.SetParallel(false)
	data := ndarray.New(ndarray.Int32, []int{8, 8}, nil)
	for i := 0; i < data.Len(); i++ {
		data.SetInt64(i, int64(i+1))
	}
	if err := arr.Write(ctx, nil, data); err != nil {
		panic(err)
	}
	arr.SetParallel(true)
	shrunk, err := arr.Resize(ctx, []int64{4, 4}, false)
	if err != nil {
		panic(err)
	}
	if shrunk.Metadata().Shape[0] != 4 || shrunk.Metadata().Shape[1] != 4 {
		panic(shrunk.Metadata().Shape)
	}
	got, err := shrunk.Read(ctx, nil, nil)
	if err != nil || got.Shape[0] != 4 || got.GetInt64(0) != 1 {
		panic(got)
	}
	grown, err := shrunk.Resize(ctx, []int64{6, 6}, true)
	if err != nil {
		panic(err)
	}
	edge, err := grown.Read(ctx, []int64{5, 5}, []int64{1, 1})
	if err != nil || edge.GetInt64(0) != 0 {
		panic(edge)
	}
}
```

## Ошибки

| Сигнал | Когда совпадает `errors.Is` |
| --- | --- |
| `zarr.ErrNotFound` | `v2.Open`, `v3.Open`, `v2.OpenGroup`, `v3.OpenGroup`, `v2.OpenNode` и `v3.OpenNode`, если объекта метаданных нет. То же делает `ome.OpenScene` |
| `zarr.ErrExists` | `v2.Create` и `v3.Create`, если `existsOk` равен false и метаданные уже есть. v3 оборачивает сигнал путём handle |
| `zarr.ErrOutOfBounds` | `Read` / `ReadChunk` за пределами массива |
| `zarr.ErrInvalidMetadata` | экспортируемый сигнал неверных метаданных. Сборщики сейчас возвращают `*zarr.Error` из `zarr.NewError`, поэтому `errors.Is(err, zarr.ErrInvalidMetadata)` не совпадает с неудачным `Build` |

`zarr.Open`, `OpenArray`, `OpenGroup` и `OpenPath` возвращают `*zarr.Error` (см. `zarr.NewError` / `zarr.WrapError`), если по handle ничего не лежит. Эта ошибка — не `ErrNotFound`. `Get` store для отсутствующего ключа — это `(nil, nil)`, а не `ErrNotFound`.

```go
// doc:run
// example: errors
package main

import (
	"context"
	"errors"

	"github.com/mentatxx/go-zarr"
	"github.com/mentatxx/go-zarr/ndarray"
	"github.com/mentatxx/go-zarr/store"
	v3 "github.com/mentatxx/go-zarr/v3"
)

func main() {
	ctx := context.Background()
	h := store.NewMemory().Resolve("a")
	meta, err := v3.NewMetadataBuilder().
		WithShape(4, 4).
		WithDataType(ndarray.Int8).
		WithChunkShape(2, 2).
		WithFillValue(0).
		Build()
	if err != nil {
		panic(err)
	}
	arr, err := v3.Create(ctx, h, meta, false)
	if err != nil {
		panic(err)
	}
	_, err = v3.Create(ctx, h, meta, false)
	if !errors.Is(err, zarr.ErrExists) {
		panic(err)
	}
	if _, err = v3.Create(ctx, h, meta, true); err != nil {
		panic(err)
	}
	_, err = arr.Read(ctx, []int64{0, 0}, []int64{100, 100})
	if !errors.Is(err, zarr.ErrOutOfBounds) {
		panic(err)
	}
	_, err = arr.ReadChunk(ctx, []int64{9, 9})
	if !errors.Is(err, zarr.ErrOutOfBounds) {
		panic(err)
	}
	_, err = v3.Open(ctx, store.NewMemory().Resolve("missing"))
	if !errors.Is(err, zarr.ErrNotFound) {
		panic(err)
	}
	_, err = zarr.Open(ctx, store.NewMemory().Resolve("missing"))
	if err == nil || errors.Is(err, zarr.ErrNotFound) {
		panic(err)
	}
	if _, err = v3.NewMetadataBuilder().WithDataType(ndarray.Int8).Build(); err == nil {
		panic("shape is required")
	}
}
```

## OME-Zarr

`ome` читает изображения OME-Zarr (v0.4, v0.5 и v0.6), HCS-планшеты и сцены v0.6. Метаданные OME он не пишет. `ome.Open` возвращает изображение. Узел, который является только сценой, возвращает ошибку с указанием вызвать `ome.OpenScene`.

`OpenScaleLevel` возвращает `*v3.Array` или `*v2.Array` (`any`) в зависимости от формата файла этого уровня. `AxisNames` берёт оси из первой multiscale-записи или из её первой системы координат. `ScaleLevelCount` — число datasets в этой записи.

```go
// doc:build
// example: ome
package main

import (
	"context"
	"fmt"

	"github.com/mentatxx/go-zarr/ome"
	"github.com/mentatxx/go-zarr/store"
	v2 "github.com/mentatxx/go-zarr/v2"
	v3 "github.com/mentatxx/go-zarr/v3"
)

func main() {
	ctx := context.Background()
	h := store.NewFilesystem("/path/to/ome-zarr").Resolve()
	im, err := ome.Open(ctx, h)
	if err != nil {
		panic(err)
	}
	fmt.Println(im.Version, im.AxisNames(), im.ScaleLevelCount())
	level, err := im.OpenScaleLevel(ctx, 0)
	if err != nil {
		panic(err)
	}
	switch a := level.(type) {
	case *v3.Array:
		_, _ = a.Read(ctx, nil, nil)
	case *v2.Array:
		_, _ = a.Read(ctx, nil, nil)
	}
	labels, err := im.Labels(ctx)
	if err != nil {
		panic(err)
	}
	fmt.Println(labels)

	plate, err := ome.OpenPlate(ctx, h)
	if err == nil && len(plate.Meta.Wells) > 0 {
		well, err := plate.OpenWell(ctx, plate.Meta.Wells[0].Path)
		if err == nil && len(well.Meta.Images) > 0 {
			_, _ = well.OpenImage(ctx, well.Meta.Images[0].Path)
		}
	}
	scene, err := ome.OpenScene(ctx, h)
	if err == nil {
		names, err := scene.ListImageNodes(ctx)
		if err != nil {
			panic(err)
		}
		for _, name := range names {
			if _, err := scene.OpenImageNode(ctx, name); err != nil {
				panic(err)
			}
		}
		_ = scene.CoordinateTransformationGraph()
	}
}
```

`Labels` читает список имён меток из `labels/zarr.json` (`attributes.labels` или `attributes.ome.labels`) или из `labels/.zattrs`. Если этих метаданных нет, возвращается `(nil, nil)`. `OpenPlate` / `OpenWell` читают HCS-метаданные из v3 `attributes.ome` или из v2 `.zattrs`. `CoordinateTransformationGraph` кратко описывает преобразования сцены. `ome.NormalizeCoordinateTransformationPath` приводит путь преобразования к нормальному виду.

## Командная строка

`cmd/zarr` печатает один массив. Это точка входа для [zarr-conformance-tests](https://github.com/zarr-developers/zarr-specs). Путь должен указывать на массив. Группа завершает процесс с ошибкой.

```bash
go run ./cmd/zarr --array_path /path/to/array
```

Процесс вызывает `zarr.OpenPath` и `Read(ctx, nil, nil)`, затем печатает `ndarray.Array.String()`.

## Сборка и тесты

```bash
go test ./...
go test -tags python ./...    # нужны uv и пакет zarr для Python
go test -tags cblosc ./...    # нужен системный c-blosc, включая blosclz
RUN_S3_TESTS=1 go test ./store -run TestS3Store
make testdata                 # скачать фикстуру l4_sample
```

`go test -run TestUserGuide` компилирует каждый пример на Go из этого руководства и запускает те, что помечены `// doc:run`.

## Ограничения

- Storage transformers отклоняются.
- Сетки чанков, кроме `regular`, не реализованы.
- Типы, кроме bool и перечисленных выше целых и float, не реализованы. Нет строк, complex и типов переменной длины.
- Порядок Fortran в v2 не является отдельной раскладкой в памяти.
- Blosc на чистом Go не сжимает и не распаковывает `blosclz`.
- Поддержка OME-Zarr читает уже существующие метаданные. Она не создаёт планшеты, лунки, сцены и multiscale-раскладки.
- `zarr.ErrInvalidMetadata` — не то, что сегодня возвращает `MetadataBuilder.Build`.
