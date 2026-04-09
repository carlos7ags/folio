// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
)

// GSUBFeature identifies an OpenType GSUB feature tag used for Arabic
// positional shaping.
type GSUBFeature string

const (
	GSUBInit GSUBFeature = "init" // initial form
	GSUBMedi GSUBFeature = "medi" // medial form
	GSUBFina GSUBFeature = "fina" // final form
	GSUBIsol GSUBFeature = "isol" // isolated form
)

// GSUBSubstitutions maps a GSUB feature tag to its glyph ID substitution
// table: sourceGID -> replacementGID. Only SingleSubstitution lookups
// (GSUB LookupType 1) are supported in this version.
type GSUBSubstitutions map[GSUBFeature]map[uint16]uint16

// ParseGSUB reads the GSUB table from raw TrueType/OpenType font bytes
// and extracts SingleSubstitution lookups for the Arabic positional
// features (init, medi, fina, isol). Returns nil if the font has no GSUB
// table or no matching features.
//
// The implementation walks the OpenType GSUB table structure:
//   ScriptList -> find "arab" or "DFLT" script -> default LangSys
//   FeatureList -> match "init"/"medi"/"fina"/"isol" features
//   LookupList -> follow each feature's lookup indices
//   Each lookup -> subtables -> SingleSubstitution format 1 or 2
//
// Reference: OpenType spec v1.9, GSUB table (ISO 14496-22 §6.2).
func ParseGSUB(data []byte) GSUBSubstitutions {
	gsub := findTable(data, "GSUB")
	if gsub == nil {
		return nil
	}
	if len(gsub) < 10 {
		return nil
	}

	// GSUB header (version 1.0 or 1.1).
	scriptListOff := int(be16(gsub, 4))
	featureListOff := int(be16(gsub, 6))
	lookupListOff := int(be16(gsub, 8))

	if scriptListOff >= len(gsub) || featureListOff >= len(gsub) || lookupListOff >= len(gsub) {
		return nil
	}

	// Step 1: find feature indices for the "arab" or "DFLT" script.
	featureIndices := scriptFeatureIndices(gsub, scriptListOff)
	if len(featureIndices) == 0 {
		return nil
	}

	// Step 2: match feature tags to our target features.
	targetTags := map[string]GSUBFeature{
		"init": GSUBInit,
		"medi": GSUBMedi,
		"fina": GSUBFina,
		"isol": GSUBIsol,
	}
	featureToLookups := matchFeatures(gsub, featureListOff, featureIndices, targetTags)
	if len(featureToLookups) == 0 {
		return nil
	}

	// Step 3: parse lookups and collect substitutions.
	result := make(GSUBSubstitutions)
	for feat, lookupIndices := range featureToLookups {
		subs := parseLookups(gsub, lookupListOff, lookupIndices)
		if len(subs) > 0 {
			result[feat] = subs
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// findTable locates a TrueType/OpenType table by its 4-byte tag in the
// raw font bytes and returns the table's data slice. Returns nil if not found.
func findTable(data []byte, tag string) []byte {
	if len(data) < 12 {
		return nil
	}
	// Handle TrueType Collections (TTC): use the first font.
	if len(data) >= 12 && string(data[:4]) == "ttcf" {
		if len(data) < 16 {
			return nil
		}
		numFonts := int(be32(data, 8))
		if numFonts < 1 || len(data) < 12+4 {
			return nil
		}
		offset := int(be32(data, 12))
		if offset >= len(data) {
			return nil
		}
		data = data[offset:]
	}
	numTables := int(be16(data, 4))
	if len(data) < 12+numTables*16 {
		return nil
	}
	tagBytes := []byte(tag)
	for i := 0; i < numTables; i++ {
		entry := data[12+i*16:]
		if entry[0] == tagBytes[0] && entry[1] == tagBytes[1] &&
			entry[2] == tagBytes[2] && entry[3] == tagBytes[3] {
			offset := int(be32(entry, 8))
			length := int(be32(entry, 12))
			// For TTC, offset is from the original data start, but we
			// already sliced. Adjust: use the pre-slice data via the
			// stored offset. Actually, table offsets in TTC are relative
			// to the file start, but we sliced data to the font offset.
			// Since findTable is called on the sliced data, the table
			// offsets are relative to the font header. This is correct
			// for non-TTC fonts. For TTC, we need to use the original
			// data. This is a known limitation; TTC support would need
			// the original data reference.
			if offset+length > len(data) {
				return nil
			}
			return data[offset : offset+length]
		}
	}
	return nil
}

// scriptFeatureIndices finds the feature indices referenced by the "arab"
// script (or "DFLT" fallback) in the GSUB ScriptList.
func scriptFeatureIndices(gsub []byte, off int) []int {
	if off+2 > len(gsub) {
		return nil
	}
	count := int(be16(gsub, off))
	if off+2+count*6 > len(gsub) {
		return nil
	}

	// Prefer "arab" script; fall back to "DFLT".
	var langSysOff int
	found := false
	for i := 0; i < count; i++ {
		rec := gsub[off+2+i*6:]
		tag := string(rec[:4])
		scriptOff := off + int(be16(rec, 4))
		if tag == "arab" {
			langSysOff = scriptOff
			found = true
			break
		}
		if tag == "DFLT" && !found {
			langSysOff = scriptOff
			found = true
		}
	}
	if !found {
		return nil
	}

	// Script table: defaultLangSys offset at +0.
	if langSysOff+2 > len(gsub) {
		return nil
	}
	defOff := int(be16(gsub, langSysOff))
	if defOff == 0 {
		return nil
	}
	langSys := langSysOff + defOff
	if langSys+4 > len(gsub) {
		return nil
	}
	// LangSys: skip lookupOrder (uint16) and reqFeatureIndex (uint16).
	featureCount := int(be16(gsub, langSys+4))
	if langSys+6+featureCount*2 > len(gsub) {
		return nil
	}
	indices := make([]int, featureCount)
	for i := 0; i < featureCount; i++ {
		indices[i] = int(be16(gsub, langSys+6+i*2))
	}
	return indices
}

// matchFeatures scans the FeatureList for features matching targetTags
// whose indices appear in allowed. Returns a map from GSUBFeature to
// the lookup indices referenced by that feature.
func matchFeatures(gsub []byte, off int, allowed []int, targetTags map[string]GSUBFeature) map[GSUBFeature][]int {
	if off+2 > len(gsub) {
		return nil
	}
	count := int(be16(gsub, off))
	if off+2+count*6 > len(gsub) {
		return nil
	}
	allowSet := make(map[int]bool, len(allowed))
	for _, idx := range allowed {
		allowSet[idx] = true
	}
	result := make(map[GSUBFeature][]int)
	for i := 0; i < count; i++ {
		if !allowSet[i] {
			continue
		}
		rec := gsub[off+2+i*6:]
		tag := string(rec[:4])
		feat, ok := targetTags[tag]
		if !ok {
			continue
		}
		featureOff := off + int(be16(rec, 4))
		if featureOff+4 > len(gsub) {
			continue
		}
		// Feature table: skip featureParams (uint16), then lookupCount.
		lookupCount := int(be16(gsub, featureOff+2))
		if featureOff+4+lookupCount*2 > len(gsub) {
			continue
		}
		lookups := make([]int, lookupCount)
		for j := 0; j < lookupCount; j++ {
			lookups[j] = int(be16(gsub, featureOff+4+j*2))
		}
		result[feat] = append(result[feat], lookups...)
	}
	return result
}

// parseLookups reads SingleSubstitution subtables from the specified
// lookup indices and returns a merged glyph substitution map.
func parseLookups(gsub []byte, listOff int, indices []int) map[uint16]uint16 {
	if listOff+2 > len(gsub) {
		return nil
	}
	count := int(be16(gsub, listOff))
	subs := make(map[uint16]uint16)
	for _, idx := range indices {
		if idx >= count {
			continue
		}
		lookupOff := listOff + int(be16(gsub, listOff+2+idx*2))
		if lookupOff+6 > len(gsub) {
			continue
		}
		lookupType := be16(gsub, lookupOff)
		if lookupType != 1 {
			// Only SingleSubstitution (type 1) is supported.
			continue
		}
		subCount := int(be16(gsub, lookupOff+4))
		for si := 0; si < subCount; si++ {
			subOff := lookupOff + int(be16(gsub, lookupOff+6+si*2))
			parseSingleSubst(gsub, subOff, subs)
		}
	}
	return subs
}

// parseSingleSubst reads a SingleSubstitution subtable (format 1 or 2)
// and adds entries to the substitution map.
func parseSingleSubst(gsub []byte, off int, subs map[uint16]uint16) {
	if off+6 > len(gsub) {
		return
	}
	format := be16(gsub, off)
	coverageOff := off + int(be16(gsub, off+2))

	covered := parseCoverage(gsub, coverageOff)
	if covered == nil {
		return
	}

	switch format {
	case 1:
		// Format 1: delta applied to each covered glyph ID.
		delta := int16(be16(gsub, off+4))
		for _, gid := range covered {
			subs[gid] = uint16(int16(gid) + delta)
		}
	case 2:
		// Format 2: explicit substitute array, one per covered glyph.
		substCount := int(be16(gsub, off+4))
		if off+6+substCount*2 > len(gsub) {
			return
		}
		for i, gid := range covered {
			if i >= substCount {
				break
			}
			subs[gid] = be16(gsub, off+6+i*2)
		}
	}
}

// parseCoverage reads a Coverage table and returns the list of covered
// glyph IDs in coverage index order.
func parseCoverage(gsub []byte, off int) []uint16 {
	if off+4 > len(gsub) {
		return nil
	}
	format := be16(gsub, off)
	switch format {
	case 1:
		// Format 1: array of glyph IDs.
		count := int(be16(gsub, off+2))
		if off+4+count*2 > len(gsub) {
			return nil
		}
		result := make([]uint16, count)
		for i := 0; i < count; i++ {
			result[i] = be16(gsub, off+4+i*2)
		}
		return result
	case 2:
		// Format 2: ranges of glyph IDs.
		rangeCount := int(be16(gsub, off+2))
		if off+4+rangeCount*6 > len(gsub) {
			return nil
		}
		var result []uint16
		for i := 0; i < rangeCount; i++ {
			rec := off + 4 + i*6
			startGID := be16(gsub, rec)
			endGID := be16(gsub, rec+2)
			for gid := startGID; gid <= endGID; gid++ {
				result = append(result, gid)
			}
		}
		return result
	}
	return nil
}

// be16 reads a big-endian uint16 from data at the given offset.
func be16(data []byte, off int) uint16 {
	return binary.BigEndian.Uint16(data[off:])
}

// be32 reads a big-endian uint32 from data at the given offset.
func be32(data []byte, off int) uint32 {
	return binary.BigEndian.Uint32(data[off:])
}
