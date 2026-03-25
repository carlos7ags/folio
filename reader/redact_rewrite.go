// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"fmt"
	"math"
	"strings"
)

// Standard typographic proportions relative to the font em-square, used
// when actual font metrics (ascent/descent) are unavailable. These values
// are typical for Latin fonts (Helvetica, Times, Courier) and match the
// defaults in ISO 32000 §9.2.4 and CSS Fonts Module Level 3.
const (
	// typAscender is the ascent above the baseline as a fraction of em.
	typAscender = 0.8

	// typDescender is the descent below the baseline as a fraction of em.
	typDescender = 0.2
)

// SerializeContentOps converts parsed content stream operators back to
// valid PDF content stream bytes. This is the inverse of ParseContentStream.
func SerializeContentOps(ops []ContentOp) []byte {
	var b strings.Builder
	for i, op := range ops {
		if i > 0 {
			b.WriteByte('\n')
		}
		for j, tok := range op.Operands {
			if j > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(serializeToken(tok))
		}
		if len(op.Operands) > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(op.Operator)
	}
	if len(ops) > 0 {
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

// serializeToken converts a Token back to its PDF text representation.
func serializeToken(tok Token) string {
	switch tok.Type {
	case TokenNumber:
		if tok.IsInt {
			return fmt.Sprintf("%d", tok.Int)
		}
		return serializeReal(tok.Real)
	case TokenString:
		return "(" + escapePDFString(tok.Value) + ")"
	case TokenHexString:
		return "<" + tok.Value + ">"
	case TokenName:
		return "/" + tok.Value
	case TokenBool:
		return tok.Value
	case TokenNull:
		return "null"
	case TokenArrayOpen:
		return "["
	case TokenArrayClose:
		return "]"
	case TokenDictOpen:
		return "<<"
	case TokenDictClose:
		return ">>"
	case TokenKeyword:
		return tok.Value
	default:
		return tok.Value
	}
}

// serializeReal formats a float64 compactly for PDF output.
func serializeReal(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "0"
	}
	if v == float64(int64(v)) && math.Abs(v) < 1e15 {
		return fmt.Sprintf("%.1f", v)
	}
	s := fmt.Sprintf("%.6f", v)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

// escapePDFString escapes special characters in a PDF literal string.
func escapePDFString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			b.WriteString(`\\`)
		case '(':
			b.WriteString(`\(`)
		case ')':
			b.WriteString(`\)`)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// rewriteState tracks the minimal graphics state needed for computing
// text bounding boxes during content stream rewriting.
type rewriteState struct {
	ctm      [6]float64 // current transformation matrix
	tm       [6]float64 // text matrix
	lm       [6]float64 // line matrix (start of current line)
	fontName string
	fontSize float64
	stack    [][6]float64 // CTM save stack
}

// newRewriteState creates an identity-state rewriter.
func newRewriteState() *rewriteState {
	return &rewriteState{
		ctm: [6]float64{1, 0, 0, 1, 0, 0},
		tm:  [6]float64{1, 0, 0, 1, 0, 0},
		lm:  [6]float64{1, 0, 0, 1, 0, 0},
	}
}

// rectsOverlap reports whether two boxes overlap.
func rectsOverlap(a, b Box) bool {
	if a.IsZero() || b.IsZero() {
		return false
	}
	return a.X1 < b.X2 && a.X2 > b.X1 && a.Y1 < b.Y2 && a.Y2 > b.Y1
}

// updateState processes an operator and updates the rewrite state.
// Returns true if the operator is a text-showing operator.
func (rs *rewriteState) updateState(op ContentOp) bool {
	switch op.Operator {
	case "q":
		rs.stack = append(rs.stack, rs.ctm)
	case "Q":
		if len(rs.stack) > 0 {
			rs.ctm = rs.stack[len(rs.stack)-1]
			rs.stack = rs.stack[:len(rs.stack)-1]
		}
	case "cm":
		if len(op.Operands) >= 6 {
			m := extractRewriteMatrix(op.Operands)
			rs.ctm = multiplyMatrix(m, rs.ctm)
		}
	case "BT":
		rs.tm = [6]float64{1, 0, 0, 1, 0, 0}
		rs.lm = rs.tm
	case "Tf":
		if len(op.Operands) >= 2 {
			rs.fontName = op.Operands[0].Value
			rs.fontSize = tokenFloat(op.Operands[1])
		}
	case "Tm":
		if len(op.Operands) >= 6 {
			rs.tm = extractRewriteMatrix(op.Operands)
			rs.lm = rs.tm
		}
	case "Td", "TD":
		if len(op.Operands) >= 2 {
			tx := tokenFloat(op.Operands[0])
			ty := tokenFloat(op.Operands[1])
			rs.tm = multiplyMatrix([6]float64{1, 0, 0, 1, tx, ty}, rs.lm)
			rs.lm = rs.tm
		}
	case "T*":
		rs.tm = multiplyMatrix([6]float64{1, 0, 0, 1, 0, -rs.fontSize}, rs.lm)
		rs.lm = rs.tm
	case "Tj", "TJ", "'", `"`:
		return true
	}
	return false
}

// extractRewriteMatrix reads 6 float operands into an affine matrix.
func extractRewriteMatrix(operands []Token) [6]float64 {
	var m [6]float64
	for i := 0; i < 6 && i < len(operands); i++ {
		m[i] = tokenFloat(operands[i])
	}
	return m
}

// rewriteContentStream removes text that overlaps any redaction rectangle
// at character-level precision. For Tj and TJ operators, each character's
// bounding box is tested individually; characters outside all redaction
// rects are preserved. Non-text operators pass through unchanged.
func rewriteContentStream(data []byte, rects []Box, fonts FontCache) []byte {
	if len(rects) == 0 {
		return data
	}
	ops := ParseContentStream(data)
	if len(ops) == 0 {
		return data
	}

	rs := newRewriteState()
	var out []ContentOp

	for _, op := range ops {
		isTextOp := rs.updateState(op)

		if !isTextOp {
			out = append(out, op)
			continue
		}

		// Character-level splitting for text operators.
		split := rs.splitTextOp(op, rects, fonts)
		out = append(out, split...)
	}

	return SerializeContentOps(out)
}

// splitTextOp processes a text-showing operator (Tj, TJ, ', ") and returns
// replacement operators with redacted characters removed. If no characters
// overlap any rect, the original operator is returned unchanged. If all
// characters overlap, nothing is returned.
func (rs *rewriteState) splitTextOp(op ContentOp, rects []Box, fonts FontCache) []ContentOp {
	switch op.Operator {
	case "Tj", "'":
		return rs.splitTj(op, rects, fonts)
	case "TJ":
		return rs.splitTJ(op, rects, fonts)
	case `"`:
		// " takes aw ac string — treat the string part like Tj.
		if len(op.Operands) >= 3 {
			// Emit the spacing parameters as separate ops, then split the text.
			fakeOp := ContentOp{
				Operator: "Tj",
				Operands: []Token{op.Operands[2]},
			}
			return rs.splitTj(fakeOp, rects, fonts)
		}
	}
	return nil
}

// splitTj splits a Tj operator at character boundaries. Characters that
// overlap any redaction rect are removed; remaining characters are emitted
// as one or more Tj operators with Td moves to maintain correct positioning.
func (rs *rewriteState) splitTj(op ContentOp, rects []Box, fonts FontCache) []ContentOp {
	if len(op.Operands) == 0 {
		return nil
	}
	raw := []byte(op.Operands[0].Value)
	isHex := op.Operands[0].Type == TokenHexString

	charWidths := rs.charWidths(raw, fonts)
	if len(charWidths) == 0 {
		return nil
	}

	// Test each character against the redaction rects.
	type charInfo struct {
		b       byte
		width   float64 // in text space
		redact  bool
		xOffset float64 // cumulative x offset from start of operator
	}
	chars := make([]charInfo, len(raw))
	cumX := 0.0
	for i, b := range raw {
		w := charWidths[i]
		bounds := rs.charBounds(cumX, w)
		redacted := false
		for _, rect := range rects {
			if rectsOverlap(bounds, rect) {
				redacted = true
				break
			}
		}
		chars[i] = charInfo{b: b, width: w, redact: redacted, xOffset: cumX}
		cumX += w
	}

	// Build output: group consecutive non-redacted characters into Tj ops.
	// Insert Td moves to skip over redacted gaps.
	var result []ContentOp
	totalAdvance := 0.0 // tracks how much we've advanced in text space

	i := 0
	for i < len(chars) {
		// Skip redacted characters.
		if chars[i].redact {
			i++
			continue
		}

		// Collect consecutive non-redacted characters.
		start := i
		for i < len(chars) && !chars[i].redact {
			i++
		}

		// If there's a gap before this segment (redacted chars were skipped),
		// emit a Td to move the text position.
		segmentX := chars[start].xOffset
		if segmentX != totalAdvance {
			gap := segmentX - totalAdvance
			result = append(result, ContentOp{
				Operator: "Td",
				Operands: []Token{
					{Type: TokenNumber, Real: gap * rs.fontSize},
					{Type: TokenNumber, Int: 0, IsInt: true},
				},
			})
		}

		// Emit the kept characters as a Tj.
		var kept []byte
		segmentWidth := 0.0
		for j := start; j < i; j++ {
			kept = append(kept, chars[j].b)
			segmentWidth += chars[j].width
		}

		tokType := TokenString
		if isHex {
			tokType = TokenHexString
		}
		result = append(result, ContentOp{
			Operator: op.Operator,
			Operands: []Token{{Type: tokType, Value: string(kept)}},
		})
		totalAdvance = segmentX + segmentWidth
	}

	// Advance the text matrix by the full original width so subsequent
	// operators are positioned correctly.
	rs.advanceTextPosition(cumX)

	return result
}

// splitTJ splits a TJ array operator at character boundaries. Each string
// element in the array is processed like Tj; numeric elements (kerning
// adjustments) are preserved if they fall between non-redacted segments.
func (rs *rewriteState) splitTJ(op ContentOp, rects []Box, fonts FontCache) []ContentOp {
	// Collect all "fragments" from the TJ array: strings and kern values.
	type fragment struct {
		isString bool
		raw      []byte
		isHex    bool
		kern     float64 // for number fragments, in thousandths
	}
	var fragments []fragment
	for _, tok := range op.Operands {
		switch tok.Type {
		case TokenString:
			fragments = append(fragments, fragment{isString: true, raw: []byte(tok.Value)})
		case TokenHexString:
			fragments = append(fragments, fragment{isString: true, raw: []byte(tok.Value), isHex: true})
		case TokenNumber:
			fragments = append(fragments, fragment{kern: tokenFloat(tok)})
		}
	}

	// Build a flat list of items with positions.
	type item struct {
		isByte  bool
		b       byte
		isHex   bool
		width   float64
		kern    float64
		redact  bool
		xOffset float64
	}
	var items []item
	cumX := 0.0
	for _, frag := range fragments {
		if !frag.isString {
			// Kern adjustment: negative values move right, positive move left.
			items = append(items, item{kern: frag.kern, xOffset: cumX})
			cumX -= frag.kern / 1000.0
			continue
		}
		widths := rs.charWidths(frag.raw, fonts)
		for j, b := range frag.raw {
			w := 0.0
			if j < len(widths) {
				w = widths[j]
			}
			bounds := rs.charBounds(cumX, w)
			redacted := false
			for _, rect := range rects {
				if rectsOverlap(bounds, rect) {
					redacted = true
					break
				}
			}
			items = append(items, item{
				isByte: true, b: b, isHex: frag.isHex,
				width: w, redact: redacted, xOffset: cumX,
			})
			cumX += w
		}
	}

	// Rebuild TJ array: keep non-redacted bytes and kern values between them.
	var tjTokens []Token
	hasContent := false

	for _, it := range items {
		if !it.isByte {
			// Kern value — keep if we have content around it.
			if hasContent {
				tjTokens = append(tjTokens, Token{Type: TokenNumber, Real: it.kern})
			}
			continue
		}
		if it.redact {
			// Replace redacted char with a position adjustment to maintain spacing.
			if hasContent {
				tjTokens = append(tjTokens, Token{
					Type: TokenNumber,
					Real: -it.width * 1000.0,
				})
			}
			continue
		}
		// Non-redacted character.
		tokType := TokenString
		if it.isHex {
			tokType = TokenHexString
		}
		// Merge with previous string token if possible.
		if len(tjTokens) > 0 {
			last := &tjTokens[len(tjTokens)-1]
			if last.Type == tokType {
				last.Value += string(it.b)
				hasContent = true
				continue
			}
		}
		tjTokens = append(tjTokens, Token{Type: tokType, Value: string(it.b)})
		hasContent = true
	}

	if !hasContent {
		rs.advanceTextPosition(cumX)
		return nil
	}

	// Wrap in TJ array markers.
	var operands []Token
	operands = append(operands, Token{Type: TokenArrayOpen})
	operands = append(operands, tjTokens...)
	operands = append(operands, Token{Type: TokenArrayClose})

	rs.advanceTextPosition(cumX)

	return []ContentOp{{Operator: "TJ", Operands: operands}}
}

// charWidths returns the width of each byte in text space (unscaled by fontSize).
func (rs *rewriteState) charWidths(raw []byte, fonts FontCache) []float64 {
	widths := make([]float64, len(raw))
	for i, b := range raw {
		w := 0.5 // fallback
		if fonts != nil && rs.fontName != "" {
			if fe, ok := fonts[rs.fontName]; ok && fe != nil {
				cw := fe.CharWidth(int(b))
				if cw > 0 {
					w = float64(cw) / 1000.0
				}
			}
		}
		widths[i] = w
	}
	return widths
}

// charBounds computes a tight bounding box for a single character at the
// given offset from the current text position. The box uses a conservative
// height estimate: baseline - descender to baseline + ascender.
func (rs *rewriteState) charBounds(xOffset, charWidth float64) Box {
	trm := multiplyMatrix(rs.tm, rs.ctm)
	height := math.Abs(rs.fontSize * trm[3])
	if height == 0 {
		height = math.Abs(rs.fontSize * trm[0])
	}
	scaleX := math.Abs(trm[0])

	x := trm[4] + xOffset*rs.fontSize*scaleX
	y := trm[5]
	w := charWidth * rs.fontSize * scaleX

	return Box{
		X1: x,
		Y1: y - height*typDescender,
		X2: x + w,
		Y2: y + height*typAscender,
	}
}

// advanceTextPosition moves the text matrix forward by the given width
// in text space units.
func (rs *rewriteState) advanceTextPosition(textSpaceWidth float64) {
	rs.tm[4] += textSpaceWidth * rs.fontSize
}

