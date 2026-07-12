// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import "fmt"

// parsedFont holds the per-table data parsed directly from a TrueType
// or OpenType font's raw bytes. Replaces the previous dependency on
// golang.org/x/image/font/sfnt for metric reads.
//
// Parsers for each table live in their own file: head.go, hhea.go,
// maxp.go, hmtx.go, os2.go, name.go, cmap.go. The orchestrator
// parseAllTables stitches them together.
//
// Spec: ISO/IEC 14496-22 (Open Font Format) §5.
type parsedFont struct {
	// rawTables maps 4-byte table tag → table bytes within the source
	// font. Used for tables we don't decode here (post, kern, GSUB,
	// GPOS, glyf, loca, cmap, etc.) and consulted by feature-specific
	// parsers (gsub, gpos, kern, subset).
	rawTables map[string][]byte

	os2 *os2 // optional — older TrueType fonts omit OS/2.

	// cmap is the Unicode → GID lookup parsed from the cmap table.
	cmap cmapTable

	// nameTable holds resolved name records keyed by NameID. Only the
	// IDs Folio actually consults are kept (PostScript, Full).
	postScriptName string
	fullName       string

	// hmtx: longHorMetric advances + LSBs. len == numGlyphs once
	// parseHmtx fills it. Glyphs beyond numberOfHMetrics share the
	// last advance with their own LSB.
	advances []uint16
	lsbs     []int16

	head head
	hhea hhea
	maxp maxp
}

// parseAllTables parses the table directory and decodes every table
// Folio's metric path needs from a TrueType / OpenType font's raw
// bytes. CFF outline tables are not decoded — Folio's subset code
// only handles glyf/loca outlines, so a CFF font's CFF table is left
// as an opaque blob in rawTables for embedding.
//
// Spec: ISO/IEC 14496-22 §5 (table directory) and the per-table
// clauses cited in each parser.
func parseAllTables(data []byte) (*parsedFont, error) {
	tables, err := parseTTFTables(data)
	if err != nil {
		return nil, err
	}

	pf := &parsedFont{rawTables: tables}

	headData, ok := tables["head"]
	if !ok {
		return nil, fmt.Errorf("font: missing head table: %w", ErrMissingTable)
	}
	if pf.head, err = parseHead(headData); err != nil {
		return nil, err
	}

	hheaData, ok := tables["hhea"]
	if !ok {
		return nil, fmt.Errorf("font: missing hhea table: %w", ErrMissingTable)
	}
	if pf.hhea, err = parseHhea(hheaData); err != nil {
		return nil, err
	}

	maxpData, ok := tables["maxp"]
	if !ok {
		return nil, fmt.Errorf("font: missing maxp table: %w", ErrMissingTable)
	}
	if pf.maxp, err = parseMaxp(maxpData); err != nil {
		return nil, err
	}

	hmtxData, ok := tables["hmtx"]
	if !ok {
		return nil, fmt.Errorf("font: missing hmtx table: %w", ErrMissingTable)
	}
	pf.advances, pf.lsbs, err = parseHmtx(hmtxData, int(pf.hhea.numberOfHMetrics), int(pf.maxp.numGlyphs))
	if err != nil {
		return nil, err
	}

	if os2Data, ok := tables["OS/2"]; ok {
		o, err := parseOS2(os2Data)
		if err != nil {
			return nil, err
		}
		pf.os2 = o
	}

	if nameData, ok := tables["name"]; ok {
		nm, err := parseName(nameData)
		if err != nil {
			return nil, err
		}
		pf.postScriptName = nm.postScript
		pf.fullName = nm.full
	}

	cmapData, ok := tables["cmap"]
	if !ok {
		return nil, fmt.Errorf("font: missing cmap table: %w", ErrMissingTable)
	}
	if pf.cmap, err = parseCmapTable(cmapData); err != nil {
		return nil, err
	}

	return pf, nil
}
