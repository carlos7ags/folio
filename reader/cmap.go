// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"encoding/hex"
	"strings"
	"unicode/utf16"
)

// CMap is a parsed ToUnicode CMap that maps character codes to Unicode strings.
type CMap struct {
	codeSpaceRanges []codeSpaceRange
	bfChars         map[uint32]string // code → unicode string
	bfRanges        []bfRange
}

type codeSpaceRange struct {
	low, high uint32
	bytes     int // number of bytes per code (1 or 2)
}

type bfRange struct {
	low, high uint32
	dst       string // unicode string for low; subsequent codes increment first rune
}

// ParseCMap parses a ToUnicode CMap stream into a CMap.
func ParseCMap(data []byte) *CMap {
	cm := &CMap{
		bfChars: make(map[uint32]string),
	}
	s := string(data)
	cm.parseCodeSpaceRanges(s)
	cm.parseBfChars(s)
	cm.parseBfRanges(s)

	// If no code space ranges were found, infer from the mappings.
	if len(cm.codeSpaceRanges) == 0 {
		cm.inferCodeSpace()
	}
	return cm
}

// CodeBytes returns the number of bytes per character code.
// Returns 1 for single-byte CMaps, 2 for two-byte, 0 if unknown.
func (cm *CMap) CodeBytes() int {
	if len(cm.codeSpaceRanges) > 0 {
		return cm.codeSpaceRanges[0].bytes
	}
	return 0
}

// Decode maps raw character code bytes to a Unicode string using the CMap.
func (cm *CMap) Decode(raw []byte) string {
	if cm == nil || (len(cm.bfChars) == 0 && len(cm.bfRanges) == 0) {
		return string(raw)
	}

	codeLen := cm.CodeBytes()
	if codeLen == 0 {
		codeLen = 1
	}

	var sb strings.Builder
	for i := 0; i < len(raw); {
		var code uint32
		if codeLen == 2 && i+1 < len(raw) {
			code = uint32(raw[i])<<8 | uint32(raw[i+1])
			i += 2
		} else {
			code = uint32(raw[i])
			i++
		}

		if u, ok := cm.lookupCode(code); ok {
			sb.WriteString(u)
		} else {
			// Unmapped code — try as raw rune.
			if code > 0 && code < 0x110000 {
				sb.WriteRune(rune(code))
			}
		}
	}
	return sb.String()
}

// lookupCode looks up a character code in bfChars then bfRanges.
func (cm *CMap) lookupCode(code uint32) (string, bool) {
	if s, ok := cm.bfChars[code]; ok {
		return s, true
	}
	for _, r := range cm.bfRanges {
		if code >= r.low && code <= r.high {
			offset := code - r.low
			runes := []rune(r.dst)
			if len(runes) > 0 {
				runes[0] += rune(offset)
			}
			return string(runes), true
		}
	}
	return "", false
}

// parseCodeSpaceRanges extracts begincodespacerange...endcodespacerange blocks.
func (cm *CMap) parseCodeSpaceRanges(s string) {
	for {
		idx := strings.Index(s, "begincodespacerange")
		if idx < 0 {
			return
		}
		s = s[idx+len("begincodespacerange"):]
		end := strings.Index(s, "endcodespacerange")
		if end < 0 {
			return
		}
		block := s[:end]
		s = s[end:]

		tokens := extractHexTokens(block)
		for i := 0; i+1 < len(tokens); i += 2 {
			low, lowBytes := decodeHexCode(tokens[i])
			high, _ := decodeHexCode(tokens[i+1])
			cm.codeSpaceRanges = append(cm.codeSpaceRanges, codeSpaceRange{
				low: low, high: high, bytes: lowBytes,
			})
		}
	}
}

// parseBfChars extracts beginbfchar...endbfchar blocks.
func (cm *CMap) parseBfChars(s string) {
	for {
		idx := strings.Index(s, "beginbfchar")
		if idx < 0 {
			return
		}
		s = s[idx+len("beginbfchar"):]
		end := strings.Index(s, "endbfchar")
		if end < 0 {
			return
		}
		block := s[:end]
		s = s[end:]

		tokens := extractHexTokens(block)
		for i := 0; i+1 < len(tokens); i += 2 {
			code, _ := decodeHexCode(tokens[i])
			unicode := decodeUnicodeHex(tokens[i+1])
			cm.bfChars[code] = unicode
		}
	}
}

// parseBfRanges extracts beginbfrange...endbfrange blocks.
func (cm *CMap) parseBfRanges(s string) {
	for {
		idx := strings.Index(s, "beginbfrange")
		if idx < 0 {
			return
		}
		s = s[idx+len("beginbfrange"):]
		end := strings.Index(s, "endbfrange")
		if end < 0 {
			return
		}
		block := s[:end]
		s = s[end:]

		tokens := extractHexTokens(block)
		for i := 0; i+2 < len(tokens); i += 3 {
			low, _ := decodeHexCode(tokens[i])
			high, _ := decodeHexCode(tokens[i+1])
			dst := decodeUnicodeHex(tokens[i+2])
			cm.bfRanges = append(cm.bfRanges, bfRange{
				low: low, high: high, dst: dst,
			})
		}
	}
}

// inferCodeSpace sets a default code space if none was parsed.
func (cm *CMap) inferCodeSpace() {
	// Check if any code > 255 to determine 1-byte vs 2-byte.
	maxCode := uint32(0)
	for code := range cm.bfChars {
		if code > maxCode {
			maxCode = code
		}
	}
	for _, r := range cm.bfRanges {
		if r.high > maxCode {
			maxCode = r.high
		}
	}
	if maxCode > 255 {
		cm.codeSpaceRanges = append(cm.codeSpaceRanges, codeSpaceRange{
			low: 0, high: 0xFFFF, bytes: 2,
		})
	} else {
		cm.codeSpaceRanges = append(cm.codeSpaceRanges, codeSpaceRange{
			low: 0, high: 0xFF, bytes: 1,
		})
	}
}

// extractHexTokens pulls all <XXXX> hex tokens from a block.
func extractHexTokens(block string) []string {
	var tokens []string
	for {
		start := strings.IndexByte(block, '<')
		if start < 0 {
			return tokens
		}
		block = block[start+1:]
		end := strings.IndexByte(block, '>')
		if end < 0 {
			return tokens
		}
		tokens = append(tokens, block[:end])
		block = block[end+1:]
	}
}

// decodeHexCode decodes a hex string like "0041" into a uint32 code and byte count.
func decodeHexCode(h string) (uint32, int) {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0, 1
	}
	// Pad to even length.
	if len(h)%2 != 0 {
		h += "0"
	}
	b, err := hex.DecodeString(h)
	if err != nil {
		return 0, len(h) / 2
	}
	var code uint32
	for _, bb := range b {
		code = code<<8 | uint32(bb)
	}
	return code, len(b)
}

// decodeUnicodeHex decodes a hex string into a Unicode string.
// Handles both BMP (4 hex digits → 1 rune) and surrogate pairs.
func decodeUnicodeHex(h string) string {
	h = strings.TrimSpace(h)
	if h == "" {
		return ""
	}
	if len(h)%2 != 0 {
		h += "0"
	}
	b, err := hex.DecodeString(h)
	if err != nil {
		return ""
	}

	// Interpret as big-endian UTF-16.
	if len(b)%2 != 0 {
		b = append(b, 0)
	}
	var u16 []uint16
	for i := 0; i < len(b); i += 2 {
		u16 = append(u16, uint16(b[i])<<8|uint16(b[i+1]))
	}
	return string(utf16.Decode(u16))
}
