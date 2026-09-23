package v3

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mentatxx/go-zarr/core"
	"github.com/mentatxx/go-zarr/store"
)

// Node is an array or group.
type Node interface {
	Handle() store.Handle
}

// Group is a Zarr v3 group.
type Group struct {
	handle store.Handle
	meta   GroupMetadata
}

func (g *Group) Handle() store.Handle    { return g.handle }
func (g *Group) Metadata() GroupMetadata { return g.meta }

// OpenGroup opens an existing v3 group.
func OpenGroup(ctx context.Context, h store.Handle) (*Group, error) {
	b, err := h.Resolve(ZarrJSON).Read(ctx)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, core.ErrNotFound
	}
	var meta GroupMetadata
	if err := json.Unmarshal(b, &meta); err != nil {
		return nil, err
	}
	return &Group{handle: h, meta: meta}, nil
}

// CreateGroup writes group metadata.
func CreateGroup(ctx context.Context, h store.Handle, meta GroupMetadata) (*Group, error) {
	if meta.ZarrFormat == 0 {
		meta.ZarrFormat = Format
	}
	if meta.NodeType == "" {
		meta.NodeType = "group"
	}
	raw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := h.Resolve(ZarrJSON).Set(ctx, raw); err != nil {
		return nil, err
	}
	return &Group{handle: h, meta: meta}, nil
}

// Get opens a child node (array or group).
func (g *Group) Get(ctx context.Context, name string) (Node, error) {
	child := g.handle.Resolve(name)
	return OpenNode(ctx, child)
}

// CreateGroup creates a subgroup.
func (g *Group) CreateGroup(ctx context.Context, name string, attrs core.Attributes) (*Group, error) {
	if attrs == nil {
		attrs = core.Attributes{}
	}
	return CreateGroup(ctx, g.handle.Resolve(name), GroupMetadata{
		ZarrFormat: Format,
		NodeType:   "group",
		Attributes: attrs,
	})
}

// CreateArray creates a child array.
func (g *Group) CreateArray(ctx context.Context, name string, build func(*MetadataBuilder) *MetadataBuilder) (*Array, error) {
	meta, err := build(NewMetadataBuilder()).Build()
	if err != nil {
		return nil, err
	}
	return Create(ctx, g.handle.Resolve(name), meta, false)
}

func (g *Group) SetAttributes(ctx context.Context, attrs core.Attributes) (*Group, error) {
	meta := g.meta
	meta.Attributes = attrs
	return CreateGroup(ctx, g.handle, meta)
}

func (g *Group) String() string {
	return fmt.Sprintf("<v3.Group {%s}>", g.handle)
}

// OpenNode opens a v3 array or group by inspecting node_type.
func OpenNode(ctx context.Context, h store.Handle) (Node, error) {
	b, err := h.Resolve(ZarrJSON).Read(ctx)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, core.ErrNotFound
	}
	var head struct {
		NodeType string `json:"node_type"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	switch head.NodeType {
	case "array":
		return Open(ctx, h)
	case "group":
		return OpenGroup(ctx, h)
	default:
		return nil, core.NewError("unsupported node_type '" + head.NodeType + "'")
	}
}
