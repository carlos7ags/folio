// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package reader provides a PDF parser that can open, read, and extract
// content from existing PDF files.
package reader

import (
	"fmt"
	"io"
	"strconv"
)

// TokenType identifies the kind of PDF token.
type TokenType int

const (
	TokenNumber     TokenType = iota // integer or real number
	TokenString                      // literal string (...)
	TokenHexString                   // hexadecimal string <...>
	TokenName                        // /Name
	TokenBool                        // true or false
	TokenNull                        // null
	TokenKeyword                     // obj, endobj, stream, endstream, xref, trailer, startxref, R, etc.
	TokenArrayOpen                   // [
	TokenArrayClose                  // ]
	TokenDictOpen                    // <<
	TokenDictClose                   // >>
	TokenEOF                         // end of input
)

// Token is a single PDF lexical token.
type Token struct {
	Type  TokenType
	Value string  // raw text value
	Int   int64   // parsed integer (for TokenNumber)
	Real  float64 // parsed float (for TokenNumber)
	IsInt bool    // true if the number is an integer
	Pos   int64   // byte offset in the input
}

// Tokenizer reads PDF tokens from a byte stream.
type Tokenizer struct {
	data []byte
	pos  int
	len  int
}

// NewTokenizer creates a tokenizer over the given byte slice.
func NewTokenizer(data []byte) *Tokenizer {
	return &Tokenizer{data: data, pos: 0, len: len(data)}
}

// NewTokenizerFromReader reads all bytes and creates a tokenizer.
func NewTokenizerFromReader(r io.Reader) (*Tokenizer, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return NewTokenizer(data), nil
}

// Pos returns the current byte position.
func (t *Tokenizer) Pos() int { return t.pos }

// SetPos seeks to a byte position.
func (t *Tokenizer) SetPos(pos int) { t.pos = pos }

// Data returns the underlying byte slice.
func (t *Tokenizer) Data() []byte { return t.data }

// Next returns the next token, skipping whitespace and comments.
func (t *Tokenizer) Next() Token {
	t.skipWhitespaceAndComments()

	if t.pos >= t.len {
		return Token{Type: TokenEOF, Pos: int64(t.pos)}
	}

	pos := int64(t.pos)
	ch := t.data[t.pos]

	switch {
	case ch == '/':
		return t.readName(pos)
	case ch == '(':
		return t.readLiteralString(pos)
	case ch == '<':
		if t.pos+1 < t.len && t.data[t.pos+1] == '<' {
			t.pos += 2
			return Token{Type: TokenDictOpen, Value: "<<", Pos: pos}
		}
		return t.readHexString(pos)
	case ch == '>':
		if t.pos+1 < t.len && t.data[t.pos+1] == '>' {
			t.pos += 2
			return Token{Type: TokenDictClose, Value: ">>", Pos: pos}
		}
		t.pos++
		return Token{Type: TokenKeyword, Value: ">", Pos: pos}
	case ch == '[':
		t.pos++
		return Token{Type: TokenArrayOpen, Value: "[", Pos: pos}
	case ch == ']':
		t.pos++
		return Token{Type: TokenArrayClose, Value: "]", Pos: pos}
	case isDigit(ch) || ch == '+' || ch == '-' || ch == '.':
		return t.readNumber(pos)
	default:
		return t.readKeywordOrBool(pos)
	}
}

// Peek returns the next token without advancing the position.
func (t *Tokenizer) Peek() Token {
	savedPos := t.pos
	tok := t.Next()
	t.pos = savedPos
	return tok
}

// readName reads a PDF name token: /Name
func (t *Tokenizer) readName(pos int64) Token {
	t.pos++ // skip /
	start := t.pos
	for t.pos < t.len {
		ch := t.data[t.pos]
		if isWhitespace(ch) || isDelimiter(ch) {
			break
		}
		t.pos++
	}
	raw := string(t.data[start:t.pos])
	// Decode #XX hex escapes.
	name := decodeName(raw)
	return Token{Type: TokenName, Value: name, Pos: pos}
}

// readLiteralString reads a literal string: (...)
// Handles nested parentheses and escape sequences.
// If the string is unterminated (EOF before closing paren), returns what was read.
func (t *Tokenizer) readLiteralString(pos int64) Token {
	t.pos++ // skip (
	var result []byte
	depth := 1

	for t.pos < t.len && depth > 0 {
		ch := t.data[t.pos]
		switch ch {
		case '(':
			depth++
			result = append(result, ch)
		case ')':
			depth--
			if depth > 0 {
				result = append(result, ch)
			}
		case '\\':
			t.pos++
			if t.pos >= t.len {
				break
			}
			esc := t.data[t.pos]
			switch esc {
			case 'n':
				result = append(result, '\n')
			case 'r':
				result = append(result, '\r')
			case 't':
				result = append(result, '\t')
			case 'b':
				result = append(result, '\b')
			case 'f':
				result = append(result, '\f')
			case '(', ')', '\\':
				result = append(result, esc)
			case '\r':
				// Escaped CR or CRLF: line continuation.
				if t.pos+1 < t.len && t.data[t.pos+1] == '\n' {
					t.pos++
				}
			case '\n':
				// Escaped LF: line continuation.
			default:
				// Octal escape: up to 3 digits.
				if esc >= '0' && esc <= '7' {
					val := int(esc - '0')
					for i := 0; i < 2 && t.pos+1 < t.len; i++ {
						next := t.data[t.pos+1]
						if next >= '0' && next <= '7' {
							t.pos++
							val = val*8 + int(next-'0')
						} else {
							break
						}
					}
					result = append(result, byte(val))
				} else {
					result = append(result, esc)
				}
			}
		default:
			result = append(result, ch)
		}
		t.pos++
	}

	return Token{Type: TokenString, Value: string(result), Pos: pos}
}

// readHexString reads a hexadecimal string: <hex>
func (t *Tokenizer) readHexString(pos int64) Token {
	t.pos++ // skip <
	var hex []byte
	for t.pos < t.len && t.data[t.pos] != '>' {
		ch := t.data[t.pos]
		if !isWhitespace(ch) {
			hex = append(hex, ch)
		}
		t.pos++
	}
	if t.pos < t.len {
		t.pos++ // skip >
	}
	// Odd number of hex digits: append trailing 0.
	if len(hex)%2 != 0 {
		hex = append(hex, '0')
	}
	// Decode hex pairs.
	result := make([]byte, len(hex)/2)
	for i := 0; i < len(hex); i += 2 {
		hi := hexVal(hex[i])
		lo := hexVal(hex[i+1])
		result[i/2] = hi<<4 | lo
	}
	return Token{Type: TokenHexString, Value: string(result), Pos: pos}
}

// readNumber reads an integer or real number.
func (t *Tokenizer) readNumber(pos int64) Token {
	start := t.pos
	hasDecimal := false

	if t.data[t.pos] == '+' || t.data[t.pos] == '-' {
		t.pos++
	}
	for t.pos < t.len {
		ch := t.data[t.pos]
		if ch == '.' {
			if hasDecimal {
				break
			}
			hasDecimal = true
			t.pos++
		} else if isDigit(ch) {
			t.pos++
		} else {
			break
		}
	}

	raw := string(t.data[start:t.pos])
	tok := Token{Type: TokenNumber, Value: raw, Pos: pos}

	if hasDecimal {
		tok.Real, _ = strconv.ParseFloat(raw, 64)
		tok.IsInt = false
	} else {
		tok.Int, _ = strconv.ParseInt(raw, 10, 64)
		tok.Real = float64(tok.Int)
		tok.IsInt = true
	}

	return tok
}

// readKeywordOrBool reads a keyword (obj, endobj, true, false, null, etc).
func (t *Tokenizer) readKeywordOrBool(pos int64) Token {
	start := t.pos
	for t.pos < t.len {
		ch := t.data[t.pos]
		if isWhitespace(ch) || isDelimiter(ch) {
			break
		}
		t.pos++
	}
	word := string(t.data[start:t.pos])

	switch word {
	case "true":
		return Token{Type: TokenBool, Value: word, Pos: pos}
	case "false":
		return Token{Type: TokenBool, Value: word, Pos: pos}
	case "null":
		return Token{Type: TokenNull, Value: word, Pos: pos}
	default:
		return Token{Type: TokenKeyword, Value: word, Pos: pos}
	}
}

// skipWhitespaceAndComments skips whitespace and % comments.
func (t *Tokenizer) skipWhitespaceAndComments() {
	for t.pos < t.len {
		ch := t.data[t.pos]
		if isWhitespace(ch) {
			t.pos++
		} else if ch == '%' {
			// Skip until end of line.
			for t.pos < t.len && t.data[t.pos] != '\n' && t.data[t.pos] != '\r' {
				t.pos++
			}
		} else {
			break
		}
	}
}

// ReadLine reads bytes until the next line ending, used for xref parsing.
func (t *Tokenizer) ReadLine() string {
	start := t.pos
	for t.pos < t.len && t.data[t.pos] != '\n' && t.data[t.pos] != '\r' {
		t.pos++
	}
	line := string(t.data[start:t.pos])
	// Skip the line ending.
	if t.pos < t.len && t.data[t.pos] == '\r' {
		t.pos++
	}
	if t.pos < t.len && t.data[t.pos] == '\n' {
		t.pos++
	}
	return line
}

// ReadBytes reads n bytes from the current position.
func (t *Tokenizer) ReadBytes(n int) ([]byte, error) {
	if t.pos+n > t.len {
		return nil, fmt.Errorf("reader: unexpected end of data at offset %d", t.pos)
	}
	result := make([]byte, n)
	copy(result, t.data[t.pos:t.pos+n])
	t.pos += n
	return result, nil
}

// MatchKeyword checks if the bytes at the current position match the keyword.
// Does not advance the position.
func (t *Tokenizer) MatchKeyword(kw string) bool {
	end := t.pos + len(kw)
	return end <= t.len && string(t.data[t.pos:end]) == kw
}

// Skip advances the position by n bytes.
func (t *Tokenizer) Skip(n int) {
	t.pos += n
	if t.pos > t.len {
		t.pos = t.len
	}
}

// SkipWhitespace advances past whitespace and comments.
func (t *Tokenizer) SkipWhitespace() {
	t.skipWhitespaceAndComments()
}

// SkipByte advances past a specific byte if it matches.
func (t *Tokenizer) SkipByte(b byte) bool {
	if t.pos < t.len && t.data[t.pos] == b {
		t.pos++
		return true
	}
	return false
}

// ReadStreamData reads stream data: skips the "stream" keyword and EOL,
// reads `length` bytes of data, then skips EOL + "endstream".
// If length <= 0, scans for "endstream" marker.
func (t *Tokenizer) ReadStreamData(length int) []byte {
	// Skip "stream" keyword.
	if t.MatchKeyword("stream") {
		t.Skip(6)
	}

	// Skip EOL after "stream" (CR, LF, or CRLF).
	t.SkipByte('\r')
	t.SkipByte('\n')

	var data []byte
	if length > 0 && t.pos+length <= t.len {
		data = make([]byte, length)
		copy(data, t.data[t.pos:t.pos+length])
		t.pos += length
	} else {
		// Scan for "endstream".
		start := t.pos
		marker := []byte("endstream")
		for t.pos < t.len-len(marker)+1 {
			if t.data[t.pos] == 'e' && string(t.data[t.pos:t.pos+len(marker)]) == "endstream" {
				break
			}
			t.pos++
		}
		// Trim trailing whitespace from data.
		end := t.pos
		for end > start && (t.data[end-1] == '\n' || t.data[end-1] == '\r') {
			end--
		}
		data = make([]byte, end-start)
		copy(data, t.data[start:end])
	}

	// Skip whitespace + "endstream".
	t.skipWhitespaceAndComments()
	if t.MatchKeyword("endstream") {
		t.Skip(9)
	}

	return data
}

// AtEnd reports whether all input has been consumed.
func (t *Tokenizer) AtEnd() bool {
	return t.pos >= t.len
}

// --- Helper functions ---

func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '\f' || ch == 0
}

func isDelimiter(ch byte) bool {
	switch ch {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	}
	return false
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func hexVal(ch byte) byte {
	switch {
	case ch >= '0' && ch <= '9':
		return ch - '0'
	case ch >= 'a' && ch <= 'f':
		return ch - 'a' + 10
	case ch >= 'A' && ch <= 'F':
		return ch - 'A' + 10
	default:
		return 0
	}
}

// decodeName decodes #XX hex escapes in a PDF name.
func decodeName(raw string) string {
	if len(raw) == 0 {
		return raw
	}
	var result []byte
	for i := 0; i < len(raw); i++ {
		if raw[i] == '#' && i+2 < len(raw) {
			hi := hexVal(raw[i+1])
			lo := hexVal(raw[i+2])
			result = append(result, hi<<4|lo)
			i += 2
		} else {
			result = append(result, raw[i])
		}
	}
	return string(result)
}
