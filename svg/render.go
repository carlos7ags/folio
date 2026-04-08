// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package svg

import (
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/content"
)

// RenderOptions configures SVG rendering into a PDF content stream.
type RenderOptions struct {
	// RegisterOpacity is called when the renderer needs an ExtGState for opacity.
	// It returns the resource name (e.g. "GS1"). If nil, opacity is applied as
	// fill/stroke alpha instead (not correct for overlapping elements but works
	// for simple cases).
	RegisterOpacity func(opacity float64) string

	// RegisterFont is called when rendering <text> elements. It returns the
	// resource name for the given font. If nil, text elements are skipped.
	RegisterFont func(family, weight, style string, size float64) string

	// MeasureText returns the width of text in PDF points for the given font
	// and size. Used for text-anchor alignment. If nil, text-anchor is ignored.
	MeasureText func(family, weight, style string, size float64, text string) float64

	// RegisterImage is called when rendering an <image> element. The callback
	// receives the raw href attribute value (commonly a data: URI) and must
	// return the PDF XObject resource name (e.g. "Im3") and the image's
	// intrinsic pixel dimensions. An empty name causes the element to be
	// skipped. If nil, <image> elements are skipped. Keeping decoding out of
	// the svg package avoids coupling it to the folio/image package.
	RegisterImage func(href string) (name string, intrinsicW, intrinsicH float64)

	// RegisterGradient is called when a shape's fill or stroke references a
	// linearGradient or radialGradient. The callback receives the gradient
	// node (use node.LinearGradient() or node.RadialGradient() to access its
	// parsed form) and the bounding box of the shape being painted in the
	// current SVG local coordinate space. It should rasterize the gradient,
	// register it as a PDF image XObject, and return the resource name.
	// Returning an empty string causes the renderer to fall back to the
	// gradient's first stop color (legacy behavior). If nil, gradient
	// references are always collapsed to their first stop color.
	RegisterGradient func(gradient *Node, bbox BBox) string

	// Defs holds reusable elements indexed by id (from <defs> blocks).
	// This is set internally during rendering and should not be set by callers.
	defs map[string]*Node
}

// Draw renders the SVG into a PDF content stream at position (x, y) bottom-left
// with dimensions (w, h) in PDF points.
func (s *SVG) Draw(stream *content.Stream, x, y, w, h float64) {
	s.DrawWithOptions(stream, x, y, w, h, RenderOptions{})
}

// DrawWithOptions renders the SVG with explicit options for resource registration.
func (s *SVG) DrawWithOptions(stream *content.Stream, x, y, w, h float64, opts RenderOptions) {
	if s.root == nil {
		return
	}

	// Skip rendering if target dimensions are zero — nothing to draw.
	if w == 0 || h == 0 {
		return
	}

	stream.SaveState()

	// Translate to the target position (bottom-left corner in PDF space).
	stream.ConcatMatrix(1, 0, 0, 1, x, y)

	// Compute viewBox dimensions, falling back to the SVG width/height.
	vb := s.ViewBox()
	vbW := vb.Width
	vbH := vb.Height
	if !vb.Valid {
		vbW = s.Width()
		vbH = s.Height()
	}

	// Skip rendering if viewBox dimensions are zero — would cause divide-by-zero.
	if vbW == 0 || vbH == 0 {
		stream.RestoreState()
		return
	}

	// Scale from viewBox units to target (w, h) in PDF points.
	sx := w / vbW
	sy := h / vbH
	stream.ConcatMatrix(sx, 0, 0, sy, 0, 0)

	// Flip Y axis: SVG is top-down, PDF is bottom-up.
	// After this transform, SVG (0,0) is at the top-left of the target rect.
	stream.ConcatMatrix(1, 0, 0, -1, 0, vbH)

	// Apply viewBox offset if present.
	if vb.Valid && (vb.MinX != 0 || vb.MinY != 0) {
		stream.ConcatMatrix(1, 0, 0, 1, -vb.MinX, -vb.MinY)
	}

	// Pass defs into opts for <use> resolution.
	opts.defs = s.defs

	// Walk the tree with the default parent style.
	parentStyle := defaultStyle()
	for _, child := range s.root.Children {
		renderNode(stream, child, parentStyle, opts)
	}

	// Also render the root itself if it carries shape content (unlikely but valid).
	if s.root.Tag != "svg" {
		renderNode(stream, s.root, parentStyle, opts)
	}

	stream.RestoreState()
}

// renderNode dispatches rendering for a single SVG node.
func renderNode(stream *content.Stream, node *Node, parentStyle Style, opts RenderOptions) {
	if node == nil {
		return
	}

	style := resolveStyle(node, parentStyle)

	// Skip hidden elements.
	if style.Display == "none" {
		return
	}
	if style.Visibility == "hidden" {
		// visibility:hidden still occupies space but is not painted.
		// Children may override with visibility:visible, but for simplicity
		// we skip the entire subtree.
		return
	}

	// Determine if we need a graphics state wrapper for a transform or opacity.
	hasTransform := !isIdentity(node.Transform)
	groupOpacity := style.Opacity
	needsState := hasTransform || groupOpacity < 1.0

	if needsState {
		stream.SaveState()
	}

	// Apply group/element opacity via ExtGState.
	if groupOpacity < 1.0 && opts.RegisterOpacity != nil {
		gsName := opts.RegisterOpacity(groupOpacity)
		if gsName != "" {
			stream.SetExtGState(gsName)
		}
	}

	// Apply the element's transform attribute.
	if hasTransform {
		m := node.Transform
		stream.ConcatMatrix(m.A, m.B, m.C, m.D, m.E, m.F)
	}

	// Resolve gradient references to solid colors when no gradient handler
	// is available. When RegisterGradient is set, the FillRef is left in
	// place so the shape renderer can dispatch to the gradient path.
	// Stroke gradients are always collapsed for v1 — only fill gradients
	// are rasterized, since SVG stroke gradients require additional work
	// (stroking with a pattern) that is not yet implemented.
	if opts.RegisterGradient == nil {
		if style.FillRef != "" && style.Fill == nil && opts.defs != nil {
			if c := resolveGradientColor(opts.defs, style.FillRef); c != nil {
				style.Fill = c
			}
		}
	}
	if style.StrokeRef != "" && style.Stroke == nil && opts.defs != nil {
		if c := resolveGradientColor(opts.defs, style.StrokeRef); c != nil {
			style.Stroke = c
		}
	}

	switch node.Tag {
	case "g", "svg":
		// Group: just recurse into children.
		for _, child := range node.Children {
			renderNode(stream, child, style, opts)
		}
	case "defs":
		// <defs> children are not rendered directly — they are
		// referenced via <use>. Skip the entire subtree.
	case "use":
		renderUse(stream, node, style, opts)
	case "rect":
		renderRect(stream, node, style, opts)
	case "circle":
		renderCircle(stream, node, style, opts)
	case "ellipse":
		renderEllipse(stream, node, style, opts)
	case "line":
		renderLine(stream, node, style)
	case "polyline":
		renderPolyline(stream, node, style, false, opts)
	case "polygon":
		renderPolyline(stream, node, style, true, opts)
	case "path":
		renderPath(stream, node, style, opts)
	case "image":
		renderImage(stream, node, opts)
	case "text":
		renderText(stream, node, style, opts)
	default:
		// Unknown element — recurse into children in case there are
		// renderable descendants (e.g. <a>, etc.).
		for _, child := range node.Children {
			renderNode(stream, child, style, opts)
		}
	}

	if needsState {
		stream.RestoreState()
	}
}

// ---------------------------------------------------------------------------
// Shape renderers
// ---------------------------------------------------------------------------

// renderRect renders an SVG <rect> element.
func renderRect(stream *content.Stream, node *Node, style Style, opts RenderOptions) {
	x := attrFloat(node, "x", 0)
	y := attrFloat(node, "y", 0)
	w := attrFloat(node, "width", 0)
	h := attrFloat(node, "height", 0)
	if w <= 0 || h <= 0 {
		return
	}

	rx := attrFloat(node, "rx", 0)
	ry := attrFloat(node, "ry", 0)

	// SVG spec: if only one of rx/ry is specified, use it for both.
	if rx > 0 && ry == 0 {
		ry = rx
	} else if ry > 0 && rx == 0 {
		rx = ry
	}

	// Clamp radii per SVG spec.
	if rx > w/2 {
		rx = w / 2
	}
	if ry > h/2 {
		ry = h / 2
	}

	applyStrokeStyle(stream, style)

	buildPath := func() {
		if rx == 0 && ry == 0 {
			stream.Rectangle(x, y, w, h)
		} else {
			buildRoundedRect(stream, x, y, w, h, rx, ry)
		}
	}
	buildPath()
	paintPathOrGradient(stream, style, opts, BBox{X: x, Y: y, W: w, H: h}, buildPath)
}

// buildRoundedRect appends a rounded rectangle subpath in SVG coordinate space.
// Note: SVG rect (x,y) is the top-left corner, and y increases downward.
func buildRoundedRect(stream *content.Stream, x, y, w, h, rx, ry float64) {
	const k = 0.5522847498 // Bezier circle approximation constant
	kx := rx * k
	ky := ry * k

	// Start at top edge, past the top-left corner.
	stream.MoveTo(x+rx, y)

	// Top edge -> top-right corner.
	stream.LineTo(x+w-rx, y)
	stream.CurveTo(x+w-rx+kx, y, x+w, y+ry-ky, x+w, y+ry)

	// Right edge -> bottom-right corner.
	stream.LineTo(x+w, y+h-ry)
	stream.CurveTo(x+w, y+h-ry+ky, x+w-rx+kx, y+h, x+w-rx, y+h)

	// Bottom edge -> bottom-left corner.
	stream.LineTo(x+rx, y+h)
	stream.CurveTo(x+rx-kx, y+h, x, y+h-ry+ky, x, y+h-ry)

	// Left edge -> top-left corner.
	stream.LineTo(x, y+ry)
	stream.CurveTo(x, y+ry-ky, x+rx-kx, y, x+rx, y)

	stream.ClosePath()
}

// renderCircle renders an SVG <circle> element.
func renderCircle(stream *content.Stream, node *Node, style Style, opts RenderOptions) {
	cx := attrFloat(node, "cx", 0)
	cy := attrFloat(node, "cy", 0)
	r := attrFloat(node, "r", 0)
	if r <= 0 {
		return
	}

	applyStrokeStyle(stream, style)
	buildPath := func() { stream.Circle(cx, cy, r) }
	buildPath()
	paintPathOrGradient(stream, style, opts, BBox{X: cx - r, Y: cy - r, W: 2 * r, H: 2 * r}, buildPath)
}

// renderEllipse renders an SVG <ellipse> element.
func renderEllipse(stream *content.Stream, node *Node, style Style, opts RenderOptions) {
	cx := attrFloat(node, "cx", 0)
	cy := attrFloat(node, "cy", 0)
	rx := attrFloat(node, "rx", 0)
	ry := attrFloat(node, "ry", 0)
	if rx <= 0 || ry <= 0 {
		return
	}

	applyStrokeStyle(stream, style)
	buildPath := func() { stream.Ellipse(cx, cy, rx, ry) }
	buildPath()
	paintPathOrGradient(stream, style, opts, BBox{X: cx - rx, Y: cy - ry, W: 2 * rx, H: 2 * ry}, buildPath)
}

// renderLine renders an SVG <line> element.
func renderLine(stream *content.Stream, node *Node, style Style) {
	x1 := attrFloat(node, "x1", 0)
	y1 := attrFloat(node, "y1", 0)
	x2 := attrFloat(node, "x2", 0)
	y2 := attrFloat(node, "y2", 0)

	applyStrokeStyle(stream, style)
	stream.MoveTo(x1, y1)
	stream.LineTo(x2, y2)

	// Lines can only be stroked (fill does not apply to open paths).
	if style.Stroke != nil {
		stream.SetStrokeColorRGB(style.Stroke.R, style.Stroke.G, style.Stroke.B)
		stream.Stroke()
	} else {
		stream.EndPath()
	}
}

// renderPolyline renders an SVG <polyline> or <polygon> element.
// If closed is true, the path is closed (polygon behavior).
func renderPolyline(stream *content.Stream, node *Node, style Style, closed bool, opts RenderOptions) {
	points := parsePoints(node.Attrs["points"])
	if len(points) < 4 { // Need at least 2 points (4 values).
		return
	}

	applyStrokeStyle(stream, style)

	buildPath := func() {
		stream.MoveTo(points[0], points[1])
		for i := 2; i+1 < len(points); i += 2 {
			stream.LineTo(points[i], points[i+1])
		}
		if closed {
			stream.ClosePath()
		}
	}
	buildPath()

	if closed {
		paintPathOrGradient(stream, style, opts, pointsBBox(points), buildPath)
	} else {
		// Polyline: open path — stroke only.
		if style.Stroke != nil {
			stream.SetStrokeColorRGB(style.Stroke.R, style.Stroke.G, style.Stroke.B)
			stream.Stroke()
		} else {
			stream.EndPath()
		}
	}
}

// renderPath renders an SVG <path> element.
func renderPath(stream *content.Stream, node *Node, style Style, opts RenderOptions) {
	d := node.Attrs["d"]
	if d == "" {
		return
	}

	cmds, err := parsePathData(d)
	if err != nil || len(cmds) == 0 {
		return
	}

	applyStrokeStyle(stream, style)
	buildPath := func() { emitPathCommands(stream, cmds) }
	buildPath()
	paintPathOrGradient(stream, style, opts, pathBBox(cmds), buildPath)
}

// emitPathCommands converts parsed SVG path commands into PDF content stream
// operators. All coordinates must already be absolute (ParsePathData is
// expected to normalize relative commands).
func emitPathCommands(stream *content.Stream, cmds []PathCommand) {
	var curX, curY float64     // current point
	var startX, startY float64 // start of current subpath (for Z)

	for _, cmd := range cmds {
		switch cmd.Type {
		case 'M':
			if len(cmd.Args) >= 2 {
				curX, curY = cmd.Args[0], cmd.Args[1]
				startX, startY = curX, curY
				stream.MoveTo(curX, curY)
			}
		case 'L':
			if len(cmd.Args) >= 2 {
				curX, curY = cmd.Args[0], cmd.Args[1]
				stream.LineTo(curX, curY)
			}
		case 'H':
			if len(cmd.Args) >= 1 {
				curX = cmd.Args[0]
				stream.LineTo(curX, curY)
			}
		case 'V':
			if len(cmd.Args) >= 1 {
				curY = cmd.Args[0]
				stream.LineTo(curX, curY)
			}
		case 'C':
			if len(cmd.Args) >= 6 {
				x1, y1 := cmd.Args[0], cmd.Args[1]
				x2, y2 := cmd.Args[2], cmd.Args[3]
				curX, curY = cmd.Args[4], cmd.Args[5]
				stream.CurveTo(x1, y1, x2, y2, curX, curY)
			}
		case 'S':
			// Smooth cubic: reflected control point. The caller (ParsePathData)
			// should ideally normalize S into C. If not, we treat it as a cubic
			// where cp1 = current point (degenerate but safe).
			if len(cmd.Args) >= 4 {
				x2, y2 := cmd.Args[0], cmd.Args[1]
				curX, curY = cmd.Args[2], cmd.Args[3]
				stream.CurveTo(curX, curY, x2, y2, curX, curY)
			}
		case 'Q':
			// Quadratic Bezier: convert to cubic.
			if len(cmd.Args) >= 4 {
				qx, qy := cmd.Args[0], cmd.Args[1]
				endX, endY := cmd.Args[2], cmd.Args[3]
				// cp1 = start + 2/3 * (ctrl - start)
				cp1x := curX + 2.0/3.0*(qx-curX)
				cp1y := curY + 2.0/3.0*(qy-curY)
				// cp2 = end + 2/3 * (ctrl - end)
				cp2x := endX + 2.0/3.0*(qx-endX)
				cp2y := endY + 2.0/3.0*(qy-endY)
				curX, curY = endX, endY
				stream.CurveTo(cp1x, cp1y, cp2x, cp2y, curX, curY)
			}
		case 'T':
			// Smooth quadratic: reflected control point. Without tracking the
			// previous Q control point, we degenerate to a line.
			if len(cmd.Args) >= 2 {
				curX, curY = cmd.Args[0], cmd.Args[1]
				stream.LineTo(curX, curY)
			}
		case 'A':
			// Arc: convert to cubic Bezier curves via ArcToCubics.
			if len(cmd.Args) >= 7 {
				rx, ry := cmd.Args[0], cmd.Args[1]
				xRot := cmd.Args[2]
				largeArc := cmd.Args[3] != 0
				sweep := cmd.Args[4] != 0
				endX, endY := cmd.Args[5], cmd.Args[6]
				cubics := arcToCubics(curX, curY, rx, ry, xRot, largeArc, sweep, endX, endY)
				for _, c := range cubics {
					if c.Type == 'C' && len(c.Args) >= 6 {
						stream.CurveTo(c.Args[0], c.Args[1], c.Args[2], c.Args[3], c.Args[4], c.Args[5])
					}
				}
				curX, curY = endX, endY
			}
		case 'Z':
			stream.ClosePath()
			curX, curY = startX, startY
		}
	}
}

// renderText renders an SVG <text> element with tspan, text-anchor,
// and dominant-baseline support.
func renderText(stream *content.Stream, node *Node, style Style, opts RenderOptions) {
	if opts.RegisterFont == nil {
		return
	}

	x := attrFloat(node, "x", 0)
	y := attrFloat(node, "y", 0)

	// Check if we have <tspan> children for multi-run text.
	hasTspan := false
	for _, child := range node.Children {
		if child.Tag == "tspan" {
			hasTspan = true
			break
		}
	}

	stream.SaveState()

	if hasTspan {
		renderTextWithTspan(stream, node, style, opts, x, y)
	} else {
		// Simple single-run text (original path).
		text := node.Text
		if text == "" {
			var sb strings.Builder
			collectText(node, &sb)
			text = sb.String()
		}
		text = strings.TrimSpace(text)
		if text == "" {
			stream.RestoreState()
			return
		}
		renderTextRun(stream, text, style, opts, x, y)
	}

	stream.RestoreState()
}

// renderTextRun renders a single text run at the given position,
// applying text-anchor and dominant-baseline adjustments.
func renderTextRun(stream *content.Stream, text string, style Style, opts RenderOptions, x, y float64) {
	fontSize := style.FontSize
	if fontSize <= 0 {
		fontSize = 16
	}

	fontName := opts.RegisterFont(style.FontFamily, style.FontWeight, style.FontStyle, fontSize)
	if fontName == "" {
		return
	}

	// Apply text-anchor adjustment.
	if style.TextAnchor != "start" && opts.MeasureText != nil {
		tw := opts.MeasureText(style.FontFamily, style.FontWeight, style.FontStyle, fontSize, text)
		switch style.TextAnchor {
		case "middle":
			x -= tw / 2
		case "end":
			x -= tw
		}
	}

	// Apply dominant-baseline adjustment.
	switch style.DominantBaseline {
	case "middle", "central":
		y += fontSize * 0.35
	case "hanging", "text-before-edge":
		y += fontSize * 0.8
	}

	if style.Fill != nil {
		stream.SetFillColorRGB(style.Fill.R, style.Fill.G, style.Fill.B)
	}

	// The outer SVG renderer applies a Y-flip (ConcatMatrix 1,0,0,-1,0,vbH)
	// to convert SVG top-down coords to PDF bottom-up. This flips text glyphs
	// too, making them mirrored. Counter-flip at the text position so glyphs
	// render right-side up.
	stream.ConcatMatrix(1, 0, 0, -1, 0, 2*y)

	stream.BeginText()
	stream.SetFont(fontName, fontSize)
	stream.MoveText(x, y)
	stream.ShowText(text)
	stream.EndText()
}

// renderTextWithTspan renders a <text> element containing <tspan> children.
func renderTextWithTspan(stream *content.Stream, node *Node, parentStyle Style, opts RenderOptions, baseX, baseY float64) {
	curX := baseX
	curY := baseY

	for _, child := range node.Children {
		if child.Tag != "tspan" {
			// Bare text content between tspans.
			text := strings.TrimSpace(child.Text)
			if text == "" && child.Tag == "" && node.Text != "" {
				continue // skip non-element nodes
			}
			if text != "" {
				renderTextRun(stream, text, parentStyle, opts, curX, curY)
				if opts.MeasureText != nil {
					fontSize := parentStyle.FontSize
					if fontSize <= 0 {
						fontSize = 16
					}
					curX += opts.MeasureText(parentStyle.FontFamily, parentStyle.FontWeight, parentStyle.FontStyle, fontSize, text)
				}
			}
			continue
		}

		childStyle := resolveStyle(child, parentStyle)

		// Absolute repositioning.
		if _, ok := child.Attrs["x"]; ok {
			curX = attrFloat(child, "x", curX)
		}
		if _, ok := child.Attrs["y"]; ok {
			curY = attrFloat(child, "y", curY)
		}

		// Relative offset.
		curX += attrFloat(child, "dx", 0)
		curY += attrFloat(child, "dy", 0)

		text := child.Text
		if text == "" {
			var sb strings.Builder
			collectText(child, &sb)
			text = sb.String()
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		renderTextRun(stream, text, childStyle, opts, curX, curY)

		// Advance cursor past this run.
		if opts.MeasureText != nil {
			fontSize := childStyle.FontSize
			if fontSize <= 0 {
				fontSize = 16
			}
			curX += opts.MeasureText(childStyle.FontFamily, childStyle.FontWeight, childStyle.FontStyle, fontSize, text)
		}
	}

	// Handle bare text on the <text> node itself (before any tspan children).
	if node.Text != "" {
		text := strings.TrimSpace(node.Text)
		if text != "" {
			renderTextRun(stream, text, parentStyle, opts, curX, curY)
		}
	}
}

// collectText recursively gathers text content from a node and its children.
func collectText(node *Node, sb *strings.Builder) {
	if node.Text != "" {
		sb.WriteString(node.Text)
	}
	for _, child := range node.Children {
		collectText(child, sb)
	}
}

// ---------------------------------------------------------------------------
// <use> element support
// ---------------------------------------------------------------------------

// renderUse renders an SVG <use> element by looking up the referenced node
// from <defs> and rendering it at the use site with optional translation.
func renderUse(stream *content.Stream, node *Node, style Style, opts RenderOptions) {
	if opts.defs == nil {
		return
	}

	// Resolve href: try href first, then xlink:href.
	href := node.Attrs["href"]
	if href == "" {
		href = node.Attrs["xlink:href"]
	}
	if href == "" {
		return
	}

	// Strip leading '#'.
	id := strings.TrimPrefix(href, "#")
	ref, ok := opts.defs[id]
	if !ok || ref == nil {
		return
	}

	// Apply translation from x/y attributes.
	tx := attrFloat(node, "x", 0)
	ty := attrFloat(node, "y", 0)

	if tx != 0 || ty != 0 {
		stream.SaveState()
		stream.ConcatMatrix(1, 0, 0, 1, tx, ty)
		defer stream.RestoreState()
	}

	// Render the referenced node with the use-site's style as parent.
	renderNode(stream, ref, style, opts)
}

// ---------------------------------------------------------------------------
// <image> element support
// ---------------------------------------------------------------------------

// renderImage renders an SVG <image> element by delegating the decoding and
// page-registration work to opts.RegisterImage. The svg package itself stays
// free of image-format dependencies: the caller provides the bytes→XObject
// mapping and receives the raw href string.
//
// The outer SVG renderer flips the y-axis so we are drawing in a top-down
// coordinate system. PDF image XObjects are drawn with their unit square
// origin at the bottom-left in the current CTM. To place the image with its
// visual top-left at SVG (x, y) and its bottom-right at (x+w, y+h), while
// counter-flipping so the raster is not mirrored vertically, we concat the
// matrix [w 0 0 -h x y+h]. Verification:
//
//	(0,0)   -> (x,   y+h)    lower-left of target rect in flipped space
//	(1,0)   -> (x+w, y+h)    lower-right
//	(0,1)   -> (x,   y)      upper-left
//	(1,1)   -> (x+w, y)      upper-right
//
// The negative y-scale inverts the image content so that, combined with the
// outer SVG y-flip, the raster appears upright in the page's visual space.
func renderImage(stream *content.Stream, node *Node, opts RenderOptions) {
	if opts.RegisterImage == nil {
		return
	}

	// Resolve href. The parser strips namespaces (localName), so both
	// href and xlink:href arrive as "href" in the attrs map. We still
	// check xlink:href as a safety net for parsers that might change.
	href := node.Attrs["href"]
	if href == "" {
		href = node.Attrs["xlink:href"]
	}
	if href == "" {
		return
	}

	name, intrinsicW, intrinsicH := opts.RegisterImage(href)
	if name == "" {
		return
	}

	x := attrFloat(node, "x", 0)
	y := attrFloat(node, "y", 0)
	w := attrFloat(node, "width", 0)
	h := attrFloat(node, "height", 0)

	// If width/height are missing, fall back to the intrinsic pixel
	// dimensions reported by the callback. SVG spec says <image>
	// without width/height has zero size (no intrinsic from raster), but
	// we treat the raster dimensions as a sensible default.
	if w <= 0 {
		w = intrinsicW
	}
	if h <= 0 {
		h = intrinsicH
	}
	if w <= 0 || h <= 0 {
		return
	}

	stream.SaveState()
	stream.ConcatMatrix(w, 0, 0, -h, x, y+h)
	stream.Do(name)
	stream.RestoreState()
}

// ---------------------------------------------------------------------------
// Gradient support
// ---------------------------------------------------------------------------

// resolveGradientColor resolves a gradient reference to its first stop color.
// This is a pragmatic approximation — true gradient rendering would require
// PDF shading patterns (Type 2 axial). Returns nil if the reference is not
// a recognized gradient.
func resolveGradientColor(defs map[string]*Node, id string) *Color {
	node, ok := defs[id]
	if !ok || node == nil {
		return nil
	}
	if node.Tag != "linearGradient" && node.Tag != "radialGradient" {
		return nil
	}

	// Find the first <stop> child with a valid color.
	for _, child := range node.Children {
		if child.Tag != "stop" {
			continue
		}
		colorStr := child.Attrs["stop-color"]
		if colorStr == "" {
			// Check inline style.
			if styleAttr, ok := child.Attrs["style"]; ok {
				props := parseInlineStyle(styleAttr)
				colorStr = props["stop-color"]
			}
		}
		if colorStr == "" {
			continue
		}
		if c, ok := parseColor(colorStr); ok {
			// Apply stop-opacity if present.
			if opStr, has := child.Attrs["stop-opacity"]; has {
				if v, err := strconv.ParseFloat(opStr, 64); err == nil {
					c.A = clamp01(v)
				}
			}
			return &c
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Style application and paint decision
// ---------------------------------------------------------------------------

// applyStrokeStyle sets the stroke-related graphics state from the style.
func applyStrokeStyle(stream *content.Stream, style Style) {
	if style.StrokeWidth > 0 {
		stream.SetLineWidth(style.StrokeWidth)
	}

	switch style.StrokeLineCap {
	case "round":
		stream.SetLineCap(1)
	case "square":
		stream.SetLineCap(2)
	default:
		// "butt" is the PDF default (0), no need to set explicitly
		// unless we are in a nested state that changed it.
	}

	switch style.StrokeLineJoin {
	case "round":
		stream.SetLineJoin(1)
	case "bevel":
		stream.SetLineJoin(2)
	default:
		// "miter" is the PDF default (0).
	}

	if style.StrokeMiterLimit > 0 && style.StrokeMiterLimit != 4 {
		stream.SetMiterLimit(style.StrokeMiterLimit)
	}

	if len(style.StrokeDashArray) > 0 {
		stream.SetDashPattern(style.StrokeDashArray, style.StrokeDashOffset)
	}
}

// paintPathOrGradient paints the current path, dispatching to a gradient
// image XObject when the style carries a FillRef that resolves to a
// linearGradient or radialGradient in the document's <defs>. Falls back to
// paintPath (solid-color fill/stroke) for everything else.
//
// When a gradient is used, the current path is consumed as a clipping
// region and the pathBuilder closure is invoked a second time to rebuild
// the path for stroking. This is cheaper than stashing the path in a
// form XObject and keeps the renderer's state management simple.
func paintPathOrGradient(stream *content.Stream, style Style, opts RenderOptions, bbox BBox, buildPath func()) {
	// Only the fill side is rasterized for v1. Stroke gradients still
	// collapse to their first stop (handled at style-resolution time).
	if style.FillRef == "" || style.Fill != nil || opts.RegisterGradient == nil || opts.defs == nil {
		paintPath(stream, style)
		return
	}
	gradientNode, ok := opts.defs[style.FillRef]
	if !ok || gradientNode == nil ||
		(gradientNode.Tag != "linearGradient" && gradientNode.Tag != "radialGradient") {
		paintPath(stream, style)
		return
	}
	name := opts.RegisterGradient(gradientNode, bbox)
	if name == "" {
		// Callback declined — fall back to the first-stop color so the
		// shape is still visible.
		if c := resolveGradientColor(opts.defs, style.FillRef); c != nil {
			style.Fill = c
		}
		paintPath(stream, style)
		return
	}

	// Gradient fill: clip to path, draw gradient image over bbox, then
	// rebuild the path and stroke if needed.
	stream.SaveState()
	if style.FillRule == "evenodd" {
		stream.ClipEvenOdd()
	} else {
		stream.ClipNonZero()
	}
	stream.EndPath() // "n": end path without painting; clipping is now active.

	// Map the image unit square onto bbox in the current (y-flipped) SVG
	// local space. Same transform as renderImage.
	stream.ConcatMatrix(bbox.W, 0, 0, -bbox.H, bbox.X, bbox.Y+bbox.H)
	stream.Do(name)
	stream.RestoreState()

	// Re-emit the path for the stroke pass. Without this the path was
	// consumed by the clip operation above.
	if style.Stroke != nil && style.StrokeWidth > 0 {
		buildPath()
		stream.SetStrokeColorRGB(style.Stroke.R, style.Stroke.G, style.Stroke.B)
		stream.Stroke()
	}
}

// pointsBBox returns the axis-aligned bounding box of a flat point list
// [x0, y0, x1, y1, ...]. Used for polygon/polyline gradient fills.
func pointsBBox(points []float64) BBox {
	if len(points) < 2 {
		return BBox{}
	}
	minX, maxX := points[0], points[0]
	minY, maxY := points[1], points[1]
	for i := 2; i+1 < len(points); i += 2 {
		x, y := points[i], points[i+1]
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}
	return BBox{X: minX, Y: minY, W: maxX - minX, H: maxY - minY}
}

// pathBBox returns a conservative axis-aligned bounding box of an SVG path
// by taking the min/max of each command's endpoint and control-point
// arguments. Bezier handles may extend slightly beyond the curve, so the
// result is a loose upper bound — good enough for sizing gradient fills.
func pathBBox(cmds []PathCommand) BBox {
	minX, maxX := 0.0, 0.0
	minY, maxY := 0.0, 0.0
	seen := false
	consume := func(x, y float64) {
		if !seen {
			minX, maxX = x, x
			minY, maxY = y, y
			seen = true
			return
		}
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}
	for _, cmd := range cmds {
		// Each command's args are a flat list of coordinates; pairs are
		// (x, y). H and V are single-value commands whose arg is an
		// absolute x or y respectively (they do not include the "other"
		// axis, so we cannot widen the bbox correctly from the command
		// alone — this is a known limitation of the loose bbox).
		if cmd.Type == 'Z' {
			continue
		}
		if cmd.Type == 'H' {
			if len(cmd.Args) > 0 {
				// Pair the x with the current y tracker (maxY which we
				// don't track precisely here). Best effort: widen X only.
				if !seen {
					minX, maxX = cmd.Args[0], cmd.Args[0]
					minY, maxY = 0, 0
					seen = true
				} else {
					if cmd.Args[0] < minX {
						minX = cmd.Args[0]
					}
					if cmd.Args[0] > maxX {
						maxX = cmd.Args[0]
					}
				}
			}
			continue
		}
		if cmd.Type == 'V' {
			if len(cmd.Args) > 0 {
				if !seen {
					minX, maxX = 0, 0
					minY, maxY = cmd.Args[0], cmd.Args[0]
					seen = true
				} else {
					if cmd.Args[0] < minY {
						minY = cmd.Args[0]
					}
					if cmd.Args[0] > maxY {
						maxY = cmd.Args[0]
					}
				}
			}
			continue
		}
		for i := 0; i+1 < len(cmd.Args); i += 2 {
			consume(cmd.Args[i], cmd.Args[i+1])
		}
	}
	if !seen {
		return BBox{}
	}
	return BBox{X: minX, Y: minY, W: maxX - minX, H: maxY - minY}
}

// paintPath decides how to paint the current path based on the resolved style.
// Gradient references should be resolved to solid colors before calling this.
func paintPath(stream *content.Stream, style Style) {
	hasFill := style.Fill != nil
	hasStroke := style.Stroke != nil && style.StrokeWidth > 0
	evenOdd := style.FillRule == "evenodd"

	if hasFill {
		stream.SetFillColorRGB(style.Fill.R, style.Fill.G, style.Fill.B)
	}
	if hasStroke {
		stream.SetStrokeColorRGB(style.Stroke.R, style.Stroke.G, style.Stroke.B)
	}

	switch {
	case hasFill && hasStroke:
		if evenOdd {
			// PDF has B* for fill-even-odd-and-stroke, but the content stream
			// builder doesn't expose it. Use separate operations instead.
			stream.SaveState()
			stream.FillEvenOdd()
			stream.RestoreState()
			stream.Stroke()
		} else {
			stream.FillAndStroke()
		}
	case hasFill:
		if evenOdd {
			stream.FillEvenOdd()
		} else {
			stream.Fill()
		}
	case hasStroke:
		stream.Stroke()
	default:
		stream.EndPath()
	}
}

// ---------------------------------------------------------------------------
// Attribute helpers
// ---------------------------------------------------------------------------

// attrFloat reads a float64 attribute from a node, returning def if missing
// or unparseable.
func attrFloat(node *Node, attr string, def float64) float64 {
	s, ok := node.Attrs[attr]
	if !ok || s == "" {
		return def
	}
	// Strip trailing "px" or other simple unit suffixes.
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "px")
	s = strings.TrimSuffix(s, "pt")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return v
}

// parsePoints parses an SVG points attribute (used by polyline and polygon)
// into a flat slice of float64 values [x1, y1, x2, y2, ...].
func parsePoints(s string) []float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Replace commas with spaces and split.
	s = strings.ReplaceAll(s, ",", " ")
	parts := strings.Fields(s)
	result := make([]float64, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			continue
		}
		result = append(result, v)
	}
	return result
}

// isIdentity returns true if the matrix is the identity matrix.
func isIdentity(m Matrix) bool {
	return m.A == 1 && m.B == 0 && m.C == 0 && m.D == 1 && m.E == 0 && m.F == 0
}
