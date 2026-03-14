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

	// Walk the tree with the default parent style.
	parentStyle := DefaultStyle()
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

	style := ResolveStyle(node, parentStyle)

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

	switch node.Tag {
	case "g", "svg":
		// Group: just recurse into children.
		for _, child := range node.Children {
			renderNode(stream, child, style, opts)
		}
	case "rect":
		renderRect(stream, node, style)
	case "circle":
		renderCircle(stream, node, style)
	case "ellipse":
		renderEllipse(stream, node, style)
	case "line":
		renderLine(stream, node, style)
	case "polyline":
		renderPolyline(stream, node, style, false)
	case "polygon":
		renderPolyline(stream, node, style, true)
	case "path":
		renderPath(stream, node, style)
	case "text":
		renderText(stream, node, style, opts)
	default:
		// Unknown element — recurse into children in case there are
		// renderable descendants (e.g. <a>, <defs> usage, etc.).
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
func renderRect(stream *content.Stream, node *Node, style Style) {
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

	if rx == 0 && ry == 0 {
		// Sharp corners — use the PDF re operator directly.
		stream.Rectangle(x, y, w, h)
	} else {
		// Rounded corners — build the path manually with Bezier curves.
		buildRoundedRect(stream, x, y, w, h, rx, ry)
	}

	paintPath(stream, style)
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
func renderCircle(stream *content.Stream, node *Node, style Style) {
	cx := attrFloat(node, "cx", 0)
	cy := attrFloat(node, "cy", 0)
	r := attrFloat(node, "r", 0)
	if r <= 0 {
		return
	}

	applyStrokeStyle(stream, style)
	stream.Circle(cx, cy, r)
	paintPath(stream, style)
}

// renderEllipse renders an SVG <ellipse> element.
func renderEllipse(stream *content.Stream, node *Node, style Style) {
	cx := attrFloat(node, "cx", 0)
	cy := attrFloat(node, "cy", 0)
	rx := attrFloat(node, "rx", 0)
	ry := attrFloat(node, "ry", 0)
	if rx <= 0 || ry <= 0 {
		return
	}

	applyStrokeStyle(stream, style)
	stream.Ellipse(cx, cy, rx, ry)
	paintPath(stream, style)
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
func renderPolyline(stream *content.Stream, node *Node, style Style, closed bool) {
	points := parsePoints(node.Attrs["points"])
	if len(points) < 4 { // Need at least 2 points (4 values).
		return
	}

	applyStrokeStyle(stream, style)

	stream.MoveTo(points[0], points[1])
	for i := 2; i+1 < len(points); i += 2 {
		stream.LineTo(points[i], points[i+1])
	}
	if closed {
		stream.ClosePath()
	}

	if closed {
		paintPath(stream, style)
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
func renderPath(stream *content.Stream, node *Node, style Style) {
	d := node.Attrs["d"]
	if d == "" {
		return
	}

	cmds, err := ParsePathData(d)
	if err != nil || len(cmds) == 0 {
		return
	}

	applyStrokeStyle(stream, style)
	emitPathCommands(stream, cmds)
	paintPath(stream, style)
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
				cubics := ArcToCubics(curX, curY, rx, ry, xRot, largeArc, sweep, endX, endY)
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

// renderText renders an SVG <text> element (basic single-line).
func renderText(stream *content.Stream, node *Node, style Style, opts RenderOptions) {
	if opts.RegisterFont == nil {
		return
	}

	text := node.Text
	if text == "" {
		// Collect text from child <tspan> or text nodes.
		var sb strings.Builder
		collectText(node, &sb)
		text = sb.String()
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	x := attrFloat(node, "x", 0)
	y := attrFloat(node, "y", 0)

	fontSize := style.FontSize
	if fontSize <= 0 {
		fontSize = 16 // SVG default
	}

	fontName := opts.RegisterFont(style.FontFamily, style.FontWeight, style.FontStyle, fontSize)
	if fontName == "" {
		return
	}

	stream.SaveState()

	// Set fill color for text (SVG text is filled by default).
	if style.Fill != nil {
		stream.SetFillColorRGB(style.Fill.R, style.Fill.G, style.Fill.B)
	}

	stream.BeginText()
	stream.SetFont(fontName, fontSize)
	stream.MoveText(x, y)
	stream.ShowText(text)
	stream.EndText()

	stream.RestoreState()
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

// paintPath decides how to paint the current path based on the resolved style.
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
