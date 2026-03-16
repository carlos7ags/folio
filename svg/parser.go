// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package svg

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// SVG is a parsed SVG document ready to be rendered.
type SVG struct {
	root    *Node
	width   float64 // from width attribute (0 if not set)
	height  float64 // from height attribute (0 if not set)
	viewBox ViewBox
	defs    map[string]*Node // reusable elements indexed by id
}

// ViewBox defines the SVG coordinate system.
type ViewBox struct {
	MinX, MinY, Width, Height float64
	Valid                     bool // true if viewBox was specified
}

// Parse parses SVG from a string.
func Parse(svgXML string) (*SVG, error) {
	return ParseReader(strings.NewReader(svgXML))
}

// ParseBytes parses SVG from bytes.
func ParseBytes(data []byte) (*SVG, error) {
	return ParseReader(strings.NewReader(string(data)))
}

// ParseReader parses SVG from a reader.
func ParseReader(r io.Reader) (*SVG, error) {
	dec := xml.NewDecoder(r)
	// Be lenient with character sets.
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	dec.Entity = xml.HTMLEntity

	root, err := parseElement(dec)
	if err != nil {
		return nil, fmt.Errorf("svg: parse error: %w", err)
	}
	if root == nil {
		return nil, fmt.Errorf("svg: empty document")
	}

	// Walk up to find the <svg> root if the first element is not <svg>.
	svgRoot := findSVGRoot(root)
	if svgRoot == nil {
		return nil, fmt.Errorf("svg: no <svg> element found")
	}

	s := &SVG{root: svgRoot}
	s.extractDimensions()
	s.indexDefs()
	return s, nil
}

// Defs returns the map of reusable elements indexed by id.
func (s *SVG) Defs() map[string]*Node {
	return s.defs
}

// Width returns the SVG width. If not specified, returns viewBox width.
func (s *SVG) Width() float64 {
	if s.width > 0 {
		return s.width
	}
	if s.viewBox.Valid {
		return s.viewBox.Width
	}
	return 0
}

// Height returns the SVG height. If not specified, returns viewBox height.
func (s *SVG) Height() float64 {
	if s.height > 0 {
		return s.height
	}
	if s.viewBox.Valid {
		return s.viewBox.Height
	}
	return 0
}

// ViewBox returns the parsed viewBox.
func (s *SVG) ViewBox() ViewBox {
	return s.viewBox
}

// AspectRatio returns width/height. Returns 0 if height is zero or dimensions are unknown.
func (s *SVG) AspectRatio() float64 {
	w := s.Width()
	h := s.Height()
	if h == 0 {
		return 0
	}
	return w / h
}

// Root returns the root SVG node.
func (s *SVG) Root() *Node {
	return s.root
}

// extractDimensions reads width, height, and viewBox from the root <svg> element.
func (s *SVG) extractDimensions() {
	if s.root == nil {
		return
	}

	if w, ok := s.root.Attrs["width"]; ok {
		s.width = parseDimension(w)
	}
	if h, ok := s.root.Attrs["height"]; ok {
		s.height = parseDimension(h)
	}
	if vb, ok := s.root.Attrs["viewBox"]; ok {
		s.viewBox = parseViewBox(vb)
	}
}

// parseElement recursively parses XML tokens into a Node tree.
// It returns the next complete element or nil if the stream is exhausted.
func parseElement(dec *xml.Decoder) (*Node, error) {
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			node := &Node{
				Tag:       localName(t.Name),
				Attrs:     make(map[string]string),
				Transform: Identity(),
			}
			for _, attr := range t.Attr {
				key := localName(attr.Name)
				node.Attrs[key] = attr.Value
			}
			// Parse transform attribute into the Transform matrix.
			if tf, ok := node.Attrs["transform"]; ok {
				node.Transform = ParseTransform(tf)
			}

			// Recursively parse children.
			if err := parseChildren(dec, node); err != nil {
				return nil, err
			}
			return node, nil

		case xml.CharData:
			// Stray text outside elements — skip.
			continue

		case xml.Comment, xml.ProcInst, xml.Directive:
			continue

		case xml.EndElement:
			// Unexpected end element — return nil.
			return nil, nil
		}
	}
}

// parseChildren reads child elements and text content until the matching end element.
func parseChildren(dec *xml.Decoder, parent *Node) error {
	var textBuf strings.Builder

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			child := &Node{
				Tag:       localName(t.Name),
				Attrs:     make(map[string]string),
				Transform: Identity(),
			}
			for _, attr := range t.Attr {
				key := localName(attr.Name)
				child.Attrs[key] = attr.Value
			}
			if tf, ok := child.Attrs["transform"]; ok {
				child.Transform = ParseTransform(tf)
			}
			if err := parseChildren(dec, child); err != nil {
				return err
			}
			parent.Children = append(parent.Children, child)

		case xml.CharData:
			textBuf.Write(t)

		case xml.EndElement:
			text := strings.TrimSpace(textBuf.String())
			if text != "" {
				parent.Text = text
			}
			return nil

		case xml.Comment, xml.ProcInst, xml.Directive:
			continue
		}
	}
}

// localName strips the namespace prefix and returns just the local element/attribute name.
func localName(name xml.Name) string {
	return name.Local
}

// findSVGRoot returns the node if its tag is "svg", or searches its children.
func findSVGRoot(node *Node) *Node {
	if node == nil {
		return nil
	}
	if node.Tag == "svg" {
		return node
	}
	for _, child := range node.Children {
		if found := findSVGRoot(child); found != nil {
			return found
		}
	}
	return nil
}

// parseDimension parses a dimension value like "100", "100px", "100.5px" into a float64.
// Only "px" and unitless values are supported; other units are stripped and parsed as-is.
func parseDimension(s string) float64 {
	s = strings.TrimSpace(s)
	// Strip known suffixes.
	for _, suffix := range []string{"px", "pt", "em", "rem", "cm", "mm", "in", "%"} {
		s = strings.TrimSuffix(s, suffix)
	}
	s = strings.TrimSpace(s)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// parseViewBox parses a viewBox attribute "minX minY width height" into a ViewBox.
func parseViewBox(s string) ViewBox {
	s = strings.TrimSpace(s)
	if s == "" {
		return ViewBox{}
	}
	// viewBox values can be separated by commas and/or whitespace.
	s = strings.ReplaceAll(s, ",", " ")
	parts := strings.Fields(s)
	if len(parts) < 4 {
		return ViewBox{}
	}
	minX, err1 := strconv.ParseFloat(parts[0], 64)
	minY, err2 := strconv.ParseFloat(parts[1], 64)
	width, err3 := strconv.ParseFloat(parts[2], 64)
	height, err4 := strconv.ParseFloat(parts[3], 64)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return ViewBox{}
	}
	// ViewBox with non-positive dimensions is invalid per SVG spec.
	if width <= 0 || height <= 0 {
		return ViewBox{
			MinX:   minX,
			MinY:   minY,
			Width:  width,
			Height: height,
			Valid:  false,
		}
	}
	return ViewBox{
		MinX:   minX,
		MinY:   minY,
		Width:  width,
		Height: height,
		Valid:  true,
	}
}

// indexDefs walks the SVG tree and indexes all elements inside <defs> blocks
// by their id attribute. Also indexes any element with an id outside <defs>
// so that <use> can reference it.
func (s *SVG) indexDefs() {
	s.defs = make(map[string]*Node)
	if s.root == nil {
		return
	}
	for _, child := range s.root.Children {
		if child.Tag == "defs" {
			for _, defChild := range child.Children {
				if id, ok := defChild.Attrs["id"]; ok && id != "" {
					s.defs[id] = defChild
				}
			}
		}
		// Also index top-level elements with id (can be referenced by <use>).
		if id, ok := child.Attrs["id"]; ok && id != "" && child.Tag != "defs" {
			s.defs[id] = child
		}
	}
}
