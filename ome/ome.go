package ome

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mentatxx/go-zarr/core"
	"github.com/mentatxx/go-zarr/store"
	v2 "github.com/mentatxx/go-zarr/v2"
	v3 "github.com/mentatxx/go-zarr/v3"
)

// Axis is an OME-NGFF axis.
type Axis struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
	Unit string `json:"unit,omitempty"`
}

// CoordinateSystem is an OME-Zarr v0.6 named axis set.
type CoordinateSystem struct {
	Name string `json:"name"`
	Axes []Axis `json:"axes"`
}

// Transformation is a coordinate transformation (scale/translation/affine/...).
type Transformation struct {
	Type        string      `json:"type"`
	Scale       []float64   `json:"scale,omitempty"`
	Translation []float64   `json:"translation,omitempty"`
	Affine      [][]float64 `json:"affine,omitempty"`
	Path        string      `json:"path,omitempty"`
	Input       any         `json:"input,omitempty"`
	Output      any         `json:"output,omitempty"`
	Name        string      `json:"name,omitempty"`
}

func (t Transformation) InputName() string  { return refName(t.Input) }
func (t Transformation) OutputName() string { return refName(t.Output) }

func refName(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case map[string]any:
		path, _ := x["path"].(string)
		name, _ := x["name"].(string)
		if path != "" && name != "" {
			return path + "#" + name
		}
		if name != "" {
			return name
		}
		return path
	}
	return fmt.Sprint(v)
}

// Dataset is one scale level.
type Dataset struct {
	Path                      string           `json:"path"`
	CoordinateTransformations []Transformation `json:"coordinateTransformations"`
}

// MultiscalesEntry is one multiscale description.
type MultiscalesEntry struct {
	Version                   string             `json:"version,omitempty"`
	Name                      string             `json:"name,omitempty"`
	Axes                      []Axis             `json:"axes"`
	Datasets                  []Dataset          `json:"datasets"`
	CoordinateTransformations []Transformation   `json:"coordinateTransformations,omitempty"`
	CoordinateSystems         []CoordinateSystem `json:"coordinateSystems,omitempty"`
	Type                      string             `json:"type,omitempty"`
}

// OmeroChannel is a display channel.
type OmeroChannel struct {
	Label  string         `json:"label"`
	Color  string         `json:"color"`
	Active *bool          `json:"active,omitempty"`
	Window map[string]any `json:"window,omitempty"`
}

// Omero is display metadata.
type Omero struct {
	ID       any            `json:"id,omitempty"`
	Version  string         `json:"version,omitempty"`
	Name     string         `json:"name,omitempty"`
	Channels []OmeroChannel `json:"channels,omitempty"`
	Rdefs    map[string]any `json:"rdefs,omitempty"`
}

// Image is a unified OME-Zarr multiscale image.
type Image struct {
	Handle  store.Handle
	Version string
	Entries []MultiscalesEntry
	Omero   *Omero
	BF2Raw  any
	v3      bool
	rawOME  map[string]any
}

func (im *Image) AxisNames() []string {
	if len(im.Entries) == 0 {
		return nil
	}
	e := im.Entries[0]
	if len(e.Axes) > 0 {
		out := make([]string, len(e.Axes))
		for i, a := range e.Axes {
			out[i] = a.Name
		}
		return out
	}
	if len(e.CoordinateSystems) > 0 {
		axes := e.CoordinateSystems[0].Axes
		out := make([]string, len(axes))
		for i, a := range axes {
			out[i] = a.Name
		}
		return out
	}
	return nil
}

func (im *Image) ScaleLevelCount() int {
	if len(im.Entries) == 0 {
		return 0
	}
	return len(im.Entries[0].Datasets)
}

func (im *Image) GetMultiscaleNode(i int) (MultiscalesEntry, error) {
	if i < 0 || i >= len(im.Entries) {
		return MultiscalesEntry{}, core.NewError("multiscale index out of range")
	}
	return im.Entries[i], nil
}

func (im *Image) OpenScaleLevel(ctx context.Context, i int) (any, error) {
	if len(im.Entries) == 0 || i >= len(im.Entries[0].Datasets) {
		return nil, core.NewError("scale level out of range")
	}
	path := im.Entries[0].Datasets[i].Path
	child := im.Handle.Resolve(splitPath(path)...)
	if im.v3 {
		return v3.Open(ctx, child)
	}
	return v2.Open(ctx, child)
}

func splitPath(p string) []string {
	var out []string
	for _, part := range strings.Split(p, "/") {
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return []string{p}
	}
	return out
}

type omeRoot struct {
	Version     string             `json:"version"`
	Multiscales []MultiscalesEntry `json:"multiscales"`
	Omero       *Omero             `json:"omero"`
	BF          any                `json:"bioformats2raw.layout"`
	Scene       json.RawMessage    `json:"scene"`
	Plate       json.RawMessage    `json:"plate"`
}

func parseOME(raw json.RawMessage) (omeRoot, error) {
	var ome omeRoot
	if err := json.Unmarshal(raw, &ome); err != nil {
		return ome, err
	}
	return ome, nil
}

// Open auto-detects OME-Zarr v0.4 / v0.5 / v0.6.
func Open(ctx context.Context, h store.Handle) (*Image, error) {
	if b, err := h.Resolve("zarr.json").Read(ctx); err == nil && b != nil {
		var root struct {
			Attributes struct {
				OME json.RawMessage `json:"ome"`
			} `json:"attributes"`
		}
		if err := json.Unmarshal(b, &root); err == nil && len(root.Attributes.OME) > 0 {
			ome, err := parseOME(root.Attributes.OME)
			if err != nil {
				return nil, err
			}
			if len(ome.Scene) > 0 && len(ome.Multiscales) == 0 {
				return nil, core.NewError("this node is an OME-Zarr Scene; use Scene.Open")
			}
			ver := ome.Version
			if ver == "" {
				ver = "0.5"
			}
			var raw map[string]any
			_ = json.Unmarshal(root.Attributes.OME, &raw)
			return &Image{
				Handle:  h,
				Version: ver,
				Entries: ome.Multiscales,
				Omero:   ome.Omero,
				BF2Raw:  ome.BF,
				v3:      true,
				rawOME:  raw,
			}, nil
		}
	}
	if b, err := h.Resolve(".zattrs").Read(ctx); err == nil && b != nil {
		var root struct {
			Multiscales []MultiscalesEntry `json:"multiscales"`
			Omero       *Omero             `json:"omero"`
			BF          any                `json:"bioformats2raw.layout"`
			Version     string             `json:"version"`
		}
		if err := json.Unmarshal(b, &root); err != nil {
			return nil, err
		}
		if len(root.Multiscales) > 0 {
			ver := root.Multiscales[0].Version
			if ver == "" {
				ver = "0.4"
			}
			return &Image{
				Handle:  h,
				Version: ver,
				Entries: root.Multiscales,
				Omero:   root.Omero,
				BF2Raw:  root.BF,
				v3:      false,
			}, nil
		}
	}
	return nil, core.NewError(fmt.Sprintf("no OME-Zarr multiscale metadata found at %s", h))
}

// Labels returns label names if present.
func (im *Image) Labels(ctx context.Context) ([]string, error) {
	lh := im.Handle.Resolve("labels")
	if b, err := lh.Resolve("zarr.json").Read(ctx); err == nil && b != nil {
		var root struct {
			Attributes struct {
				Labels []string `json:"labels"`
				OME    struct {
					Labels []string `json:"labels"`
				} `json:"ome"`
			} `json:"attributes"`
		}
		if err := json.Unmarshal(b, &root); err == nil {
			if len(root.Attributes.Labels) > 0 {
				return root.Attributes.Labels, nil
			}
			if len(root.Attributes.OME.Labels) > 0 {
				return root.Attributes.OME.Labels, nil
			}
		}
	}
	if b, err := lh.Resolve(".zattrs").Read(ctx); err == nil && b != nil {
		var root struct {
			Labels []string `json:"labels"`
		}
		if err := json.Unmarshal(b, &root); err == nil {
			return root.Labels, nil
		}
	}
	return nil, nil
}

// Plate is a high-content screening plate.
type Plate struct {
	Handle store.Handle
	Meta   PlateMetadata
	raw    map[string]any
	v3     bool
}

type NamedEntry struct {
	Name string `json:"name"`
}

type WellRef struct {
	Path        string `json:"path"`
	RowIndex    int    `json:"rowIndex"`
	ColumnIndex int    `json:"columnIndex"`
}

type PlateMetadata struct {
	Columns    []NamedEntry `json:"columns"`
	Rows       []NamedEntry `json:"rows"`
	Wells      []WellRef    `json:"wells"`
	FieldCount any          `json:"field_count,omitempty"`
	Name       string       `json:"name,omitempty"`
	Version    string       `json:"version,omitempty"`
}

func OpenPlate(ctx context.Context, h store.Handle) (*Plate, error) {
	p := &Plate{Handle: h}
	if b, err := h.Resolve("zarr.json").Read(ctx); err == nil && b != nil {
		var root struct {
			Attributes struct {
				OME struct {
					Version string        `json:"version"`
					Plate   PlateMetadata `json:"plate"`
				} `json:"ome"`
				Plate PlateMetadata `json:"plate"`
			} `json:"attributes"`
		}
		if err := json.Unmarshal(b, &root); err != nil {
			return nil, err
		}
		p.Meta = root.Attributes.OME.Plate
		if len(p.Meta.Wells) == 0 {
			p.Meta = root.Attributes.Plate
		}
		p.v3 = true
		var raw map[string]any
		_ = json.Unmarshal(b, &raw)
		p.raw = raw
		return p, nil
	}
	if b, err := h.Resolve(".zattrs").Read(ctx); err == nil && b != nil {
		var root struct {
			Plate PlateMetadata `json:"plate"`
		}
		if err := json.Unmarshal(b, &root); err != nil {
			return nil, err
		}
		p.Meta = root.Plate
		var raw map[string]any
		_ = json.Unmarshal(b, &raw)
		p.raw = raw
		return p, nil
	}
	return nil, core.NewError("no plate metadata")
}

func (p *Plate) OpenWell(ctx context.Context, path string) (*Well, error) {
	return OpenWell(ctx, p.Handle.Resolve(splitPath(path)...))
}

// Well is an HCS well.
type Well struct {
	Handle store.Handle
	Meta   WellMetadata
}

type WellImage struct {
	Path        string `json:"path"`
	Acquisition any    `json:"acquisition,omitempty"`
}

type WellMetadata struct {
	Images []WellImage `json:"images"`
}

func OpenWell(ctx context.Context, h store.Handle) (*Well, error) {
	w := &Well{Handle: h}
	if b, err := h.Resolve("zarr.json").Read(ctx); err == nil && b != nil {
		var root struct {
			Attributes struct {
				OME struct {
					Well WellMetadata `json:"well"`
				} `json:"ome"`
				Well WellMetadata `json:"well"`
			} `json:"attributes"`
		}
		_ = json.Unmarshal(b, &root)
		w.Meta = root.Attributes.OME.Well
		if len(w.Meta.Images) == 0 {
			w.Meta = root.Attributes.Well
		}
		return w, nil
	}
	if b, err := h.Resolve(".zattrs").Read(ctx); err == nil && b != nil {
		var root struct {
			Well WellMetadata `json:"well"`
		}
		_ = json.Unmarshal(b, &root)
		w.Meta = root.Well
		return w, nil
	}
	return nil, core.NewError("no well metadata")
}

func (w *Well) OpenImage(ctx context.Context, path string) (*Image, error) {
	return Open(ctx, w.Handle.Resolve(splitPath(path)...))
}

// Scene is an OME-Zarr v0.6 scene (collection of images + transforms).
type Scene struct {
	Handle store.Handle
	Meta   SceneMetadata
}

type SceneMetadata struct {
	CoordinateTransformations []Transformation   `json:"coordinateTransformations"`
	CoordinateSystems         []CoordinateSystem `json:"coordinateSystems,omitempty"`
}

func OpenScene(ctx context.Context, h store.Handle) (*Scene, error) {
	b, err := h.Resolve("zarr.json").Read(ctx)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, core.ErrNotFound
	}
	var root struct {
		Attributes struct {
			OME struct {
				Version string        `json:"version"`
				Scene   SceneMetadata `json:"scene"`
			} `json:"ome"`
		} `json:"attributes"`
	}
	if err := json.Unmarshal(b, &root); err != nil {
		return nil, err
	}
	if len(root.Attributes.OME.Scene.CoordinateTransformations) == 0 && len(root.Attributes.OME.Scene.CoordinateSystems) == 0 {
		return nil, core.NewError("no scene metadata")
	}
	return &Scene{Handle: h, Meta: root.Attributes.OME.Scene}, nil
}

func (s *Scene) ListImageNodes(ctx context.Context) ([]string, error) {
	g, err := v3.OpenGroup(ctx, s.Handle)
	if err != nil {
		return nil, err
	}
	var names []string
	if ls, ok := g.Handle().Store.(store.Listable); ok {
		for k, err := range ls.ListChildren(ctx, g.Handle().Keys) {
			if err != nil {
				return nil, err
			}
			if len(k) == 0 {
				continue
			}
			name := k[0]
			if name == "coordinateTransformations" {
				continue
			}
			child := s.Handle.Resolve(name)
			if _, err := Open(ctx, child); err == nil {
				names = append(names, name)
			}
		}
	}
	return names, nil
}

func (s *Scene) OpenImageNode(ctx context.Context, name string) (*Image, error) {
	return Open(ctx, s.Handle.Resolve(name))
}

// Graph is a simple transformation graph of a scene.
type Graph struct {
	Nodes    []string
	Edges    [][2]string
	Warnings []string
}

func (s *Scene) CoordinateTransformationGraph() Graph {
	g := Graph{}
	seen := map[string]bool{}
	for _, t := range s.Meta.CoordinateTransformations {
		in, out := t.InputName(), t.OutputName()
		if in != "" && !seen[in] {
			seen[in] = true
			g.Nodes = append(g.Nodes, in)
		}
		if out != "" && !seen[out] {
			seen[out] = true
			g.Nodes = append(g.Nodes, out)
		}
		if in != "" && out != "" {
			g.Edges = append(g.Edges, [2]string{in, out})
		}
	}
	return g
}

func NormalizeCoordinateTransformPath(p string) string {
	return strings.TrimPrefix(p, "./")
}
