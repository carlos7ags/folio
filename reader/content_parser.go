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
	// Skip one whitespace byte after ID.
	tok.Skip(1)
	// Scan for EI preceded by whitespace.
	for !tok.AtEnd() {
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

	// Text matrix components — we track tx, ty (translation) for positioning.
	// These are set by Tm and updated by Td/TD/T*.
	tmX, tmY float64 // current text position
	lineX    float64 // line start x (set by Tm, updated by T*/TD)
	lineY    float64 // line start y

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
			ts.tmX, ts.tmY = 0, 0
			ts.lineX, ts.lineY = 0, 0
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
			// e = x translation, f = y translation
			if len(op.Operands) >= 6 {
				ts.tmX = tokenFloat(op.Operands[4])
				ts.tmY = tokenFloat(op.Operands[5])
				ts.lineX = ts.tmX
				ts.lineY = ts.tmY
			}

		case "Td":
			// Move text position: tx ty Td
			if len(op.Operands) >= 2 {
				tx := tokenFloat(op.Operands[0])
				ty := tokenFloat(op.Operands[1])
				ts.tmX = ts.lineX + tx
				ts.tmY = ts.lineY + ty
				ts.lineX = ts.tmX
				ts.lineY = ts.tmY
			}

		case "TD":
			// Move text position and set leading: tx ty TD (equivalent to -ty TL; tx ty Td)
			if len(op.Operands) >= 2 {
				tx := tokenFloat(op.Operands[0])
				ty := tokenFloat(op.Operands[1])
				ts.leading = -ty
				ts.tmX = ts.lineX + tx
				ts.tmY = ts.lineY + ty
				ts.lineX = ts.tmX
				ts.lineY = ts.tmY
			}

		case "T*":
			// Move to start of next line (equivalent to 0 -leading Td).
			ts.tmX = ts.lineX
			ts.tmY = ts.lineY - ts.leading
			ts.lineX = ts.tmX
			ts.lineY = ts.tmY

		case "Tj":
			// Check position change right before rendering text.
			result = ts.emitPositionChange(result)
			if len(op.Operands) > 0 {
				text := decodeTextOperand(op.Operands[0], ts.currentFont)
				result = append(result, text...)
				ts.advanceX(text)
			}

		case "'":
			// Move to next line and show text.
			ts.tmX = ts.lineX
			ts.tmY = ts.lineY - ts.leading
			ts.lineX = ts.tmX
			ts.lineY = ts.tmY
			result = ts.emitPositionChange(result)
			if len(op.Operands) > 0 {
				text := decodeTextOperand(op.Operands[0], ts.currentFont)
				result = append(result, text...)
				ts.advanceX(text)
			}

		case "\"":
			// Set word/char spacing, move to next line, show text.
			ts.tmX = ts.lineX
			ts.tmY = ts.lineY - ts.leading
			ts.lineX = ts.tmX
			ts.lineY = ts.tmY
			result = ts.emitPositionChange(result)
			if len(op.Operands) > 2 {
				text := decodeTextOperand(op.Operands[2], ts.currentFont)
				result = append(result, text...)
				ts.advanceX(text)
			}

		case "TJ":
			// Check position before the TJ array.
			result = ts.emitPositionChange(result)
			// Text array: mix of strings and kerning adjustments.
			for _, operand := range op.Operands {
				if operand.Type == TokenString || operand.Type == TokenHexString {
					text := decodeTextOperand(operand, ts.currentFont)
					result = append(result, text...)
					ts.advanceX(text)
				} else if operand.Type == TokenNumber {
					// Negative = move right (kern tighter), positive = move left.
					// Large negative values indicate word spaces.
					adj := tokenFloat(operand)
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

	dy := ts.tmY - ts.prevY
	if dy < 0 {
		dy = -dy
	}

	lineHeight := ts.fontSize
	if lineHeight <= 0 {
		lineHeight = 12
	}

	// Significant Y change → line break.
	if dy > lineHeight*0.5 {
		return appendNewlineIfNeeded(result)
	}

	// Same line gap detection. Two strategies:
	//
	// 1. Between BT/ET blocks (new BT, no text yet in this block):
	//    If font encoding is available, spaces come from the text data itself
	//    (CMap decoding or literal strings), so skip automatic space insertion.
	//    If no font encoding (standard fonts, our own PDFs), insert a space
	//    since the writer relies on positioning rather than space characters.
	//
	// 2. Within the same BT block: use estimated end position to detect gaps.
	if !ts.btHadText {
		// Between BT blocks: insert space only when no font encoding.
		if ts.currentFont == nil {
			return appendSpaceIfNeeded(result)
		}
		return result
	}

	// Within BT: check gap between estimated text end and current position.
	gap := ts.tmX - ts.prevEndX
	if gap > lineHeight*wordGapThreshold {
		return appendSpaceIfNeeded(result)
	}

	return result
}

// advanceX marks that text was output and estimates where the text ends.
func (ts *textState) advanceX(text []byte) {
	if len(text) > 0 {
		ts.hadText = true
		ts.btHadText = true
		ts.prevY = ts.tmY
		charCount := len([]rune(string(text)))
		// Estimate end position using average character width.
		// When font encoding is available (CMap/named encoding), use a generous
		// estimate to avoid false word-gap detection within BT blocks.
		// When no encoding, use a narrower estimate for between-BT gap detection.
		widthFactor := 0.45
		if ts.currentFont != nil {
			widthFactor = 0.7
		}
		ts.prevEndX = ts.tmX + float64(charCount)*ts.fontSize*widthFactor
	}
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
