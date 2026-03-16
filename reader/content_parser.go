// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

// ContentOp is a single PDF content stream operator with its operands.
type ContentOp struct {
	Operator string  // e.g. "BT", "Tf", "Tj", "cm", "re", "f"
	Operands []Token // operand tokens preceding the operator
}

// ParseContentStream parses a decompressed content stream into a sequence
// of operators. Each operator is returned with its preceding operands.
//
// Content stream syntax:
//
//	operand1 operand2 ... operator
//	e.g.: /F1 12 Tf     (set font F1 at 12pt)
//	      100 700 Td     (move to x=100, y=700)
//	      (Hello) Tj     (show text "Hello")
func ParseContentStream(data []byte) []ContentOp {
	tok := NewTokenizer(data)
	var ops []ContentOp
	var operands []Token

	for {
		token := tok.Next()
		if token.Type == TokenEOF {
			break
		}

		switch token.Type {
		case TokenKeyword:
			// Keywords are operators (BT, ET, Tf, Tj, cm, re, f, etc.)
			// Special case: "BI" starts an inline image — skip until "EI".
			if token.Value == "BI" {
				skipInlineImage(tok)
				operands = nil
				continue
			}

			ops = append(ops, ContentOp{
				Operator: token.Value,
				Operands: operands,
			})
			operands = nil

		default:
			// Everything else is an operand (numbers, strings, names, arrays, bools).
			operands = append(operands, token)
		}
	}

	return ops
}

// skipInlineImage skips an inline image (BI ... ID <data> EI).
func skipInlineImage(tok *Tokenizer) {
	// Skip until ID keyword.
	for {
		t := tok.Next()
		if t.Type == TokenEOF {
			return
		}
		if t.Type == TokenKeyword && t.Value == "ID" {
			break
		}
	}
	// After "ID", the spec requires a single white-space byte before image
	// data. However, some producers emit "\r\n" (two bytes). Consume up to
	// two EOL bytes so we don't treat them as image data.
	if !tok.AtEnd() {
		b := tok.Data()[tok.Pos()]
		if b == '\r' {
			tok.Skip(1)
			// Consume optional \n after \r.
			if !tok.AtEnd() && tok.Data()[tok.Pos()] == '\n' {
				tok.Skip(1)
			}
		} else {
			// Single whitespace byte (space, \n, \t, etc.).
			tok.Skip(1)
		}
	}

	// Scan for "EI" preceded by whitespace. Limit the scan to prevent
	// runaway searches through the rest of a large content stream.
	const maxScan = 10 * 1024 * 1024 // 10 MB safety limit
	scanned := 0
	for !tok.AtEnd() && scanned < maxScan {
		if tok.MatchKeyword("EI") {
			// Check that the byte before is whitespace.
			pos := tok.Pos()
			if pos > 0 {
				prev := tok.Data()[pos-1]
				if isWhitespace(prev) {
					tok.Skip(2) // skip "EI"
					return
				}
			}
		}
		tok.Skip(1)
		scanned++
	}
}

// ExtractText extracts plain text from a content stream.
// Returns concatenated text from Tj and TJ operators.
// This is a simple extraction — it doesn't handle font encoding,
// character mapping, or text positioning.
func ExtractText(data []byte) string {
	return ExtractTextWithFonts(data, nil)
}

// textState tracks the PDF text state machine during extraction.
type textState struct {
	fonts       FontCache
	currentFont *FontEntry
	fontSize    float64 // from Tf operator

	// Full 6-element text matrices for correct handling of rotated/scaled/skewed text.
	textMatrix     [6]float64 // current text matrix (Tm)
	textLineMatrix [6]float64 // line start matrix (set by Td/TD/Tm, used by T*)

	// Leading for T* and ' operators.
	leading float64

	// Previous text end position for gap detection.
	prevEndX  float64 // estimated x position where previous text ended
	prevY     float64 // y position of previous text
	hadText   bool    // whether we've output any text yet
	inBT      bool    // inside a BT/ET block
	btHadText bool    // whether current BT block has rendered text
}

// wordGapThreshold is the fraction of fontSize that constitutes a word gap.
// If horizontal distance between estimated text end and next text start
// exceeds fontSize * this, insert a space.
const wordGapThreshold = 0.25

// tjKernThreshold is the TJ adjustment value (in thousandths of a unit) that
// indicates a word space rather than kerning.
const tjKernThreshold = -200

// ExtractTextWithFonts extracts text from a content stream using font encoding
// information and text positioning to produce properly spaced Unicode text.
func ExtractTextWithFonts(data []byte, fonts FontCache) string {
	ops := ParseContentStream(data)
	var result []byte
	ts := textState{fonts: fonts, fontSize: 12}

	for _, op := range ops {
		switch op.Operator {
		case "BT":
			// Begin text object — reset text matrix.
			ts.textMatrix = identityMatrix
			ts.textLineMatrix = identityMatrix
			ts.inBT = true
			ts.btHadText = false

		case "ET":
			ts.inBT = false

		case "Tf":
			// Set font and size: /FontName size Tf
			if len(op.Operands) > 1 {
				if op.Operands[0].Type == TokenName && fonts != nil {
					ts.currentFont = fonts[op.Operands[0].Value]
				}
				if op.Operands[1].Type == TokenNumber {
					ts.fontSize = op.Operands[1].Real
					if ts.fontSize == 0 && op.Operands[1].IsInt {
						ts.fontSize = float64(op.Operands[1].Int)
					}
					if ts.fontSize < 0 {
						ts.fontSize = -ts.fontSize
					}
				}
			}

		case "TL":
			// Set leading: leading TL
			if len(op.Operands) > 0 && op.Operands[0].Type == TokenNumber {
				ts.leading = tokenFloat(op.Operands[0])
			}

		case "Tm":
			// Set text matrix: a b c d e f Tm
			if len(op.Operands) >= 6 {
				m := [6]float64{
					tokenFloat(op.Operands[0]), tokenFloat(op.Operands[1]),
					tokenFloat(op.Operands[2]), tokenFloat(op.Operands[3]),
					tokenFloat(op.Operands[4]), tokenFloat(op.Operands[5]),
				}
				ts.textMatrix = m
				ts.textLineMatrix = m
			}

		case "Td":
			// Move text position: tx ty Td
			if len(op.Operands) >= 2 {
				tx := tokenFloat(op.Operands[0])
				ty := tokenFloat(op.Operands[1])
				ts.textLineMatrix = multiplyMatrix([6]float64{1, 0, 0, 1, tx, ty}, ts.textLineMatrix)
				ts.textMatrix = ts.textLineMatrix
			}

		case "TD":
			// Move text position and set leading: tx ty TD (equivalent to -ty TL; tx ty Td)
			if len(op.Operands) >= 2 {
				tx := tokenFloat(op.Operands[0])
				ty := tokenFloat(op.Operands[1])
				ts.leading = -ty
				ts.textLineMatrix = multiplyMatrix([6]float64{1, 0, 0, 1, tx, ty}, ts.textLineMatrix)
				ts.textMatrix = ts.textLineMatrix
			}

		case "T*":
			// Move to start of next line (equivalent to 0 -leading Td).
			ts.textLineMatrix = multiplyMatrix([6]float64{1, 0, 0, 1, 0, -ts.leading}, ts.textLineMatrix)
			ts.textMatrix = ts.textLineMatrix

		case "Tj":
			// Check position change right before rendering text.
			result = ts.emitPositionChange(result)
			if len(op.Operands) > 0 {
				raw := []byte(op.Operands[0].Value)
				text := decodeTextOperand(op.Operands[0], ts.currentFont)
				result = append(result, text...)
				ts.advanceX(raw)
			}

		case "'":
			// Move to next line and show text.
			ts.textLineMatrix = multiplyMatrix([6]float64{1, 0, 0, 1, 0, -ts.leading}, ts.textLineMatrix)
			ts.textMatrix = ts.textLineMatrix
			result = ts.emitPositionChange(result)
			if len(op.Operands) > 0 {
				raw := []byte(op.Operands[0].Value)
				text := decodeTextOperand(op.Operands[0], ts.currentFont)
				result = append(result, text...)
				ts.advanceX(raw)
			}

		case "\"":
			// Set word/char spacing, move to next line, show text.
			ts.textLineMatrix = multiplyMatrix([6]float64{1, 0, 0, 1, 0, -ts.leading}, ts.textLineMatrix)
			ts.textMatrix = ts.textLineMatrix
			result = ts.emitPositionChange(result)
			if len(op.Operands) > 2 {
				raw := []byte(op.Operands[2].Value)
				text := decodeTextOperand(op.Operands[2], ts.currentFont)
				result = append(result, text...)
				ts.advanceX(raw)
			}

		case "TJ":
			// Check position before the TJ array.
			result = ts.emitPositionChange(result)
			// Text array: mix of strings and kerning adjustments.
			for _, operand := range op.Operands {
				if operand.Type == TokenString || operand.Type == TokenHexString {
					raw := []byte(operand.Value)
					text := decodeTextOperand(operand, ts.currentFont)
					result = append(result, text...)
					ts.advanceX(raw)
				} else if operand.Type == TokenNumber {
					// Negative = move right (kern tighter), positive = move left.
					// Large negative values indicate word spaces.
					adj := tokenFloat(operand)
					ts.textMatrix[4] -= adj / 1000 * ts.fontSize
					if adj < float64(tjKernThreshold) {
						result = appendSpaceIfNeeded(result)
					}
				}
			}
		}
	}

	return string(result)
}

// emitPositionChange decides whether to insert a space or newline based on
// position change from the previous text output location.
func (ts *textState) emitPositionChange(result []byte) []byte {
	if !ts.hadText {
		return result
	}

	dy := ts.textMatrix[5] - ts.prevY
	if dy < 0 {
		dy = -dy
	}

	lineHeight := ts.fontSize
	if lineHeight <= 0 {
		lineHeight = 12
	}

	// Significant Y change -> line break.
	if dy > lineHeight*0.5 {
		return appendNewlineIfNeeded(result)
	}

	// Same line gap detection: check estimated text end vs current position.
	// If the gap exceeds a threshold, insert a space.
	// Use font-aware space width when available.
	gap := ts.textMatrix[4] - ts.prevEndX
	threshold := ts.fontSize * wordGapThreshold // default
	if ts.currentFont != nil {
		sw := ts.currentFont.SpaceWidth()
		if sw > 0 {
			threshold = float64(sw) / 1000.0 * ts.fontSize * 0.5
		}
	}
	if gap > threshold {
		return appendSpaceIfNeeded(result)
	}

	return result
}

// advanceX marks that text was output and computes the text end position
// for gap-based word space detection.
//
// When the font has glyph width data (from /Widths in the PDF or from the
// standard 14 font metrics), the width is computed exactly. Otherwise a
// heuristic estimate is used (0.45 em per character).
func (ts *textState) advanceX(rawBytes []byte) {
	if len(rawBytes) == 0 {
		return
	}
	ts.hadText = true
	ts.btHadText = true
	ts.prevY = ts.textMatrix[5]

	textWidth := 0.0
	if ts.currentFont != nil {
		textWidth = ts.computeTextWidth(rawBytes)
	} else {
		// No font info at all — rough estimate.
		charCount := len([]rune(string(rawBytes)))
		textWidth = float64(charCount) * ts.fontSize * 0.45
	}
	ts.prevEndX = ts.textMatrix[4] + textWidth
	// Advance the text matrix.
	ts.textMatrix[4] += textWidth
}

// computeTextWidth calculates the exact width of raw character code bytes
// using the current font's glyph width data.
func (ts *textState) computeTextWidth(raw []byte) float64 {
	fe := ts.currentFont
	total := 0
	if fe.isType0 {
		// CIDFont: 2-byte character codes.
		for i := 0; i+1 < len(raw); i += 2 {
			code := int(raw[i])<<8 | int(raw[i+1])
			total += fe.CharWidth(code)
		}
	} else {
		// Simple font: 1-byte character codes.
		for _, b := range raw {
			total += fe.CharWidth(int(b))
		}
	}
	return float64(total) / 1000.0 * ts.fontSize
}

// decodeTextOperand converts a string/hex-string token to Unicode text
// using the current font's encoding.
func decodeTextOperand(tok Token, fe *FontEntry) []byte {
	raw := []byte(tok.Value)
	if fe != nil {
		return []byte(fe.Decode(raw))
	}
	return raw
}

// tokenFloat extracts a float64 from a number token.
func tokenFloat(t Token) float64 {
	if t.IsInt {
		return float64(t.Int)
	}
	return t.Real
}

func appendSpaceIfNeeded(b []byte) []byte {
	if len(b) > 0 && b[len(b)-1] != ' ' && b[len(b)-1] != '\n' {
		return append(b, ' ')
	}
	return b
}

func appendNewlineIfNeeded(b []byte) []byte {
	if len(b) > 0 && b[len(b)-1] != '\n' {
		return append(b, '\n')
	}
	return b
}
