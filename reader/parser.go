// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"fmt"
	"math"

	"github.com/carlos7ags/folio/core"
)

// Parser builds PDF objects from a token stream.
type Parser struct {
	tok   *Tokenizer
	depth int // recursion depth guard
}

// maxParseDepth limits recursion depth to prevent stack overflow on
// deeply nested or malicious PDF objects.
const maxParseDepth = 100

// NewParser creates a parser wrapping a tokenizer.
func NewParser(tok *Tokenizer) *Parser {
	return &Parser{tok: tok}
}

// ParseObject reads the next PDF object from the token stream.
// Returns one of the core.PdfObject types, or an error.
func (p *Parser) ParseObject() (core.PdfObject, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxParseDepth {
		return nil, fmt.Errorf("reader: maximum parse depth exceeded")
	}

	tok := p.tok.Next()

	switch tok.Type {
	case TokenEOF:
		return nil, fmt.Errorf("reader: unexpected end of input")

	case TokenNumber:
		// Could be a plain number or the start of an indirect reference (1 0 R).
		return p.parseNumberOrRef(tok)

	case TokenString:
		return core.NewPdfLiteralString(tok.Value), nil

	case TokenHexString:
		return core.NewPdfHexString(tok.Value), nil

	case TokenName:
		return core.NewPdfName(tok.Value), nil

	case TokenBool:
		return core.NewPdfBoolean(tok.Value == "true"), nil

	case TokenNull:
		return core.NewPdfNull(), nil

	case TokenArrayOpen:
		return p.parseArray()

	case TokenDictOpen:
		return p.parseDictOrStream()

	default:
		return nil, fmt.Errorf("reader: unexpected token %q (type %d) at offset %d", tok.Value, tok.Type, tok.Pos)
	}
}

// parseNumberOrRef handles a number token that might be the start of
// an indirect reference (e.g. "1 0 R").
func (p *Parser) parseNumberOrRef(numTok Token) (core.PdfObject, error) {
	if !numTok.IsInt || numTok.Int < 0 {
		// Real number or negative — can't be a reference.
		if numTok.IsInt {
			return core.NewPdfInteger(int(numTok.Int)), nil
		}
		return core.NewPdfReal(numTok.Real), nil
	}

	// Save position to backtrack if it's not a reference.
	savedPos := p.tok.Pos()

	// Try to read "gen R".
	genTok := p.tok.Next()
	if genTok.Type == TokenNumber && genTok.IsInt && genTok.Int >= 0 {
		rTok := p.tok.Next()
		if rTok.Type == TokenKeyword && rTok.Value == "R" {
			// It's an indirect reference: objNum genNum R.
			return core.NewPdfIndirectReference(int(numTok.Int), int(genTok.Int)), nil
		}
	}

	// Not a reference — backtrack and return the number.
	p.tok.SetPos(savedPos)

	if numTok.IsInt {
		return core.NewPdfInteger(int(numTok.Int)), nil
	}
	return core.NewPdfReal(numTok.Real), nil
}

// parseArray reads array elements until ].
func (p *Parser) parseArray() (core.PdfObject, error) {
	arr := core.NewPdfArray()

	for {
		// Peek to check for ].
		next := p.tok.Peek()
		if next.Type == TokenArrayClose {
			p.tok.Next() // consume ]
			return arr, nil
		}
		if next.Type == TokenEOF {
			return nil, fmt.Errorf("reader: unterminated array at offset %d", next.Pos)
		}

		obj, err := p.ParseObject()
		if err != nil {
			return nil, fmt.Errorf("reader: array element: %w", err)
		}
		arr.Add(obj)
	}
}

// parseDictOrStream reads a dictionary, and checks if it's followed by
// a stream keyword (making it a PdfStream instead of PdfDictionary).
func (p *Parser) parseDictOrStream() (core.PdfObject, error) {
	dict := core.NewPdfDictionary()

	for {
		next := p.tok.Peek()
		if next.Type == TokenDictClose {
			p.tok.Next()
			break
		}
		if next.Type == TokenEOF {
			return nil, fmt.Errorf("reader: unterminated dictionary at offset %d", next.Pos)
		}

		keyTok := p.tok.Next()
		if keyTok.Type != TokenName {
			return nil, fmt.Errorf("reader: dictionary key must be a name, got %q at offset %d", keyTok.Value, keyTok.Pos)
		}

		val, err := p.ParseObject()
		if err != nil {
			return nil, fmt.Errorf("reader: dictionary value for /%s: %w", keyTok.Value, err)
		}

		dict.Set(keyTok.Value, val)
	}

	// Check if followed by "stream" keyword → PdfStream.
	p.tok.SkipWhitespace()
	if p.tok.MatchKeyword("stream") {
		return p.parseStream(dict)
	}

	return dict, nil
}

// parseStream reads the stream data following a dictionary.
func (p *Parser) parseStream(dict *core.PdfDictionary) (core.PdfObject, error) {
	// Get /Length.
	streamLen := 0
	if lengthObj := dict.Get("Length"); lengthObj != nil {
		if num, ok := lengthObj.(*core.PdfNumber); ok {
			streamLen = num.IntValue()
		}
	}

	// ReadStreamData handles: skip "stream", skip EOL, read data, skip "endstream".
	_ = p.tok.ReadStreamData(streamLen)

	// Build PdfStream — data will be read from the file by the resolver.
	stream := core.NewPdfStream(nil)
	for key, value := range dict.All() {
		stream.Dict.Set(key, value)
	}

	return stream, nil
}

// ParseIndirectObject reads "objNum genNum obj ... endobj" and returns
// the object number, generation number, and the contained object.
func (p *Parser) ParseIndirectObject() (objNum, genNum int, obj core.PdfObject, err error) {
	// Read object number.
	numTok := p.tok.Next()
	if numTok.Type != TokenNumber || !numTok.IsInt {
		return 0, 0, nil, fmt.Errorf("reader: expected object number at offset %d, got %q", numTok.Pos, numTok.Value)
	}
	if numTok.Int < 0 || numTok.Int > math.MaxInt32 {
		return 0, 0, nil, fmt.Errorf("reader: object number %d out of range at offset %d", numTok.Int, numTok.Pos)
	}
	objNum = int(numTok.Int)

	// Read generation number.
	genTok := p.tok.Next()
	if genTok.Type != TokenNumber || !genTok.IsInt {
		return 0, 0, nil, fmt.Errorf("reader: expected generation number at offset %d", genTok.Pos)
	}
	if genTok.Int < 0 || genTok.Int > math.MaxInt32 {
		return 0, 0, nil, fmt.Errorf("reader: generation number %d out of range at offset %d", genTok.Int, genTok.Pos)
	}
	genNum = int(genTok.Int)

	// Read "obj" keyword.
	objTok := p.tok.Next()
	if objTok.Type != TokenKeyword || objTok.Value != "obj" {
		return 0, 0, nil, fmt.Errorf("reader: expected 'obj' at offset %d, got %q", objTok.Pos, objTok.Value)
	}

	// Read the object.
	obj, err = p.ParseObject()
	if err != nil {
		return 0, 0, nil, fmt.Errorf("reader: object %d %d: %w", objNum, genNum, err)
	}

	// Read "endobj" keyword.
	p.tok.SkipWhitespace()
	endTok := p.tok.Next()
	// Some PDFs are lenient about endobj — don't fail hard.
	_ = endTok

	return objNum, genNum, obj, nil
}
