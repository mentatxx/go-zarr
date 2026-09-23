package zarr

import (
	"context"

	"github.com/mentatxx/go-zarr/core"
	"github.com/mentatxx/go-zarr/store"
	v2 "github.com/mentatxx/go-zarr/v2"
	v3 "github.com/mentatxx/go-zarr/v3"
)

// Re-exported types and helpers.
type (
	Attributes = core.Attributes
	Error      = core.Error
)

var (
	ErrNotFound        = core.ErrNotFound
	ErrExists          = core.ErrExists
	ErrInvalidMetadata = core.ErrInvalidMetadata
	ErrOutOfBounds     = core.ErrOutOfBounds
	NewError           = core.NewError
	WrapError          = core.WrapError
	ParseFillValue     = core.ParseFillValue
	DefaultChunkShape  = core.DefaultChunkShape
)

const (
	ZarrJSON = "zarr.json"
	ZArray   = ".zarray"
	ZGroup   = ".zgroup"
	ZAttrs   = ".zattrs"
)

// Node is a v2 or v3 array or group.
type Node any

// OpenArray auto-detects Zarr version and opens an array.
func OpenArray(ctx context.Context, h store.Handle) (Node, error) {
	ok3, _ := h.Resolve(ZarrJSON).Exists(ctx)
	ok2, _ := h.Resolve(ZArray).Exists(ctx)
	if ok3 && ok2 {
		return nil, NewError("both Zarr v2 and v3 arrays found at the specified location")
	}
	if ok3 {
		return v3.Open(ctx, h)
	}
	if ok2 {
		return v2.Open(ctx, h)
	}
	return nil, NewError("no Zarr array found at the specified location")
}

// OpenGroup auto-detects Zarr version and opens a group.
func OpenGroup(ctx context.Context, h store.Handle) (Node, error) {
	ok3, _ := h.Resolve(ZarrJSON).Exists(ctx)
	ok2, _ := h.Resolve(ZGroup).Exists(ctx)
	if ok3 && ok2 {
		return nil, NewError("both Zarr v2 and v3 nodes found")
	}
	if ok3 {
		return v3.OpenGroup(ctx, h)
	}
	if ok2 {
		return v2.OpenGroup(ctx, h)
	}
	return nil, NewError("no Zarr group found at the specified location")
}

// Open auto-detects a node (array or group).
func Open(ctx context.Context, h store.Handle) (Node, error) {
	ok3, _ := h.Resolve(ZarrJSON).Exists(ctx)
	ok2a, _ := h.Resolve(ZArray).Exists(ctx)
	ok2g, _ := h.Resolve(ZGroup).Exists(ctx)
	if ok3 && (ok2a || ok2g) {
		return nil, NewError("both Zarr v2 and v3 nodes found")
	}
	if ok3 {
		return v3.OpenNode(ctx, h)
	}
	if ok2a || ok2g {
		return v2.OpenNode(ctx, h)
	}
	return nil, NewError("no Zarr node found at the specified location")
}

// OpenPath opens a filesystem path.
func OpenPath(ctx context.Context, path string) (Node, error) {
	return Open(ctx, store.NewFilesystem(path).Resolve())
}
