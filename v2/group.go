package v2

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mentatxx/go-zarr/core"
	"github.com/mentatxx/go-zarr/store"
)

type Node interface {
	Handle() store.Handle
}

type Group struct {
	handle store.Handle
	meta   GroupMetadata
	attrs  core.Attributes
}

func (g *Group) Handle() store.Handle    { return g.handle }
func (g *Group) Metadata() GroupMetadata { return g.meta }
func (g *Group) Attributes() core.Attributes {
	if g.attrs == nil {
		return core.Attributes{}
	}
	return g.attrs
}

func OpenGroup(ctx context.Context, h store.Handle) (*Group, error) {
	b, err := h.Resolve(ZGroup).Read(ctx)
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
	g := &Group{handle: h, meta: meta, attrs: core.Attributes{}}
	ab, _ := h.Resolve(ZAttrs).Read(ctx)
	if ab != nil {
		_ = json.Unmarshal(ab, &g.attrs)
	}
	return g, nil
}

func CreateGroup(ctx context.Context, h store.Handle, attrs core.Attributes) (*Group, error) {
	meta := GroupMetadata{ZarrFormat: Format}
	raw, _ := json.MarshalIndent(meta, "", "  ")
	if err := h.Resolve(ZGroup).Set(ctx, raw); err != nil {
		return nil, err
	}
	if attrs == nil {
		attrs = core.Attributes{}
	}
	ab, _ := json.MarshalIndent(attrs, "", "  ")
	_ = h.Resolve(ZAttrs).Set(ctx, ab)
	return &Group{handle: h, meta: meta, attrs: attrs}, nil
}

func (g *Group) Get(ctx context.Context, name string) (Node, error) {
	return OpenNode(ctx, g.handle.Resolve(name))
}

func (g *Group) CreateGroup(ctx context.Context, name string) (*Group, error) {
	return CreateGroup(ctx, g.handle.Resolve(name), core.Attributes{})
}

func (g *Group) CreateArray(ctx context.Context, name string, build func(*MetadataBuilder) *MetadataBuilder) (*Array, error) {
	meta, err := build(NewMetadataBuilder()).Build()
	if err != nil {
		return nil, err
	}
	return Create(ctx, g.handle.Resolve(name), meta, false)
}

func OpenNode(ctx context.Context, h store.Handle) (Node, error) {
	ok, _ := h.Resolve(ZArray).Exists(ctx)
	if ok {
		return Open(ctx, h)
	}
	ok, _ = h.Resolve(ZGroup).Exists(ctx)
	if ok {
		return OpenGroup(ctx, h)
	}
	return nil, core.ErrNotFound
}

func (g *Group) String() string { return fmt.Sprintf("<v2.Group {%s}>", g.handle) }
