// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
)

// GSUBFeature identifies an OpenType GSUB feature tag.
type GSUBFeature string

const (
	GSUBInit GSUBFeature = "init" // initial form
	GSUBMedi GSUBFeature = "medi" // medial form
	GSUBFina GSUBFeature = "fina" // final form
	GSUBIsol GSUBFeature = "isol" // isolated form
	GSUBLiga GSUBFeature = "liga" // standard ligatures
	GSUBRlig GSUBFeature = "rlig" // required ligatures
	GSUBClig GSUBFeature = "clig" // contextual ligatures
)

// LigatureSubst describes a single ligature substitution: a sequence of
// component glyph IDs (after the first) that, together with the first
// component used as the lookup key, are replaced by LigatureGID.
type LigatureSubst struct {
	Components  []uint16 // component GIDs after the first (may be empty)
	LigatureGID uint16
}

// GSUBSubstitutions holds parsed GSUB lookups grouped by feature tag.
//
// Single holds LookupType 1 substitutions: a per-feature map from source
// glyph ID to replacement glyph ID.
//
// Ligature holds LookupType 4 substitutions: a per-feature map keyed by
// the first component glyph ID to a slice of candidate ligatures sharing
// that prefix. Slices are ordered so that longest matches appear first,
// which matches the OpenType greedy matching rule.
type GSUBSubstitutions struct {
	Single   map[GSUBFeature]map[uint16]uint16
	Ligature map[GSUBFeature]map[uint16][]LigatureSubst
}

// ParseGSUB reads the GSUB table from raw TrueType/OpenType font bytes
// and extracts Single (LookupType 1) and Ligature (LookupType 4)
// substitutions for the Arabic positional features, the standard Latin
// ligature features, and required/contextual ligatures.
//
// Script selection: "arab", "latn", and "DFLT" (in that preference order
// for the default LangSys). Extension lookups (LookupType 7) are
// unwrapped transparently.
//
// Returns nil if the font has no GSUB table or no matching features.
//
// Reference: ISO 14496-22 §6.2, OpenType GSUB table.
func ParseGSUB(data []byte) *GSUBSubstitutions {
	gsub := findTable(data, "GSUB")
	if gsub == nil {
		return nil
	}
	if len(gsub) < 10 {
		return nil
	}

	scriptListOff := int(be16(gsub, 4))
	featureListOff := int(be16(gsub, 6))
	lookupListOff := int(be16(gsub, 8))

	if scriptListOff >= len(gsub) || featureListOff >= len(gsub) || lookupListOff >= len(gsub) {
		return nil
	}

	featureIndices := scriptFeatureIndices(gsub, scriptListOff)
	if len(featureIndices) == 0 {
		return nil
	}

	targetTags := map[string]GSUBFeature{
		"init": GSUBInit,
		"medi": GSUBMedi,
		"fina": GSUBFina,
		"isol": GSUBIsol,
		"liga": GSUBLiga,
		"rlig": GSUBRlig,
		"clig": GSUBClig,
	}
	featureToLookups := matchFeatures(gsub, featureListOff, featureIndices, targetTags)
	if len(featureToLookups) == 0 {
		return nil
	}

	result := &GSUBSubstitutions{
		Single:   make(map[GSUBFeature]map[uint16]uint16),
		Ligature: make(map[GSUBFeature]map[uint16][]LigatureSubst),
	}
	for feat, lookupIndices := range featureToLookups {
		single := make(map[uint16]uint16)
		lig := make(map[uint16][]LigatureSubst)
		parseLookups(gsub, lookupListOff, lookupIndices, single, lig)
		if len(single) > 0 {
			result.Single[feat] = single
		}
		if len(lig) > 0 {
			// Order each bucket so longest component sequences come first
			// so ApplyLigature's greedy left-to-right scan produces the
			// longest match per ISO 14496-22 §6.2.
			for k := range lig {
				sortLigsByLenDesc(lig[k])
			}
			result.Ligature[feat] = lig
		}
	}
	if len(result.Single) == 0 && len(result.Ligature) == 0 {
		return nil
	}
	return result
}

// ApplyLigature scans gids left-to-right and replaces the longest matching
// ligature sequence with the ligature glyph. Greedy longest-match per
// ISO 14496-22 §6.2. Returns a new slice; the input is not modified.
func (g *GSUBSubstitutions) ApplyLigature(gids []uint16, feature GSUBFeature) []uint16 {
	if g == nil || len(g.Ligature) == 0 || len(gids) == 0 {
		return gids
	}
	table, ok := g.Ligature[feature]
	if !ok || len(table) == 0 {
		return gids
	}
	out := make([]uint16, 0, len(gids))
	i := 0
	for i < len(gids) {
		candidates := table[gids[i]]
		matched := false
		for _, cand := range candidates {
			need := len(cand.Components)
			if i+1+need > len(gids) {
				continue
			}
			ok := true
			for j := 0; j < need; j++ {
				if gids[i+1+j] != cand.Components[j] {
					ok = false
					break
				}
			}
			if ok {
				out = append(out, cand.LigatureGID)
				i += 1 + need
				matched = true
				break
			}
		}
		if !matched {
			out = append(out, gids[i])
			i++
		}
	}
	return out
}

// sortLigsByLenDesc sorts ligatures so that longer component sequences
// come first. Insertion sort is used to keep the implementation tiny and
// because ligature buckets are typically small (single digits).
func sortLigsByLenDesc(s []LigatureSubst) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && len(s[j].Components) > len(s[j-1].Components); j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
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
			if offset+length > len(data) {
				return nil
			}
			return data[offset : offset+length]
		}
	}
	return nil
}

// scriptFeatureIndices finds the feature indices referenced by the "arab"
// script, then "latn", then "DFLT" fallback in the GSUB ScriptList.
func scriptFeatureIndices(gsub []byte, off int) []int {
	if off+2 > len(gsub) {
		return nil
	}
	count := int(be16(gsub, off))
	if off+2+count*6 > len(gsub) {
		return nil
	}

	// Collect LangSys offsets from all preferred scripts so a font that
	// only lists "latn" still contributes its Latin ligature features,
	// while "arab" contributes Arabic positional lookups. Duplicates are
	// folded in matchFeatures via the allowed set.
	var langSysOffs []int
	var dfltOff int
	dfltFound := false
	for i := 0; i < count; i++ {
		rec := gsub[off+2+i*6:]
		tag := string(rec[:4])
		scriptOff := off + int(be16(rec, 4))
		switch tag {
		case "arab", "latn":
			langSysOffs = append(langSysOffs, scriptOff)
		case "DFLT":
			dfltOff = scriptOff
			dfltFound = true
		}
	}
	if len(langSysOffs) == 0 && dfltFound {
		langSysOffs = append(langSysOffs, dfltOff)
	}
	if len(langSysOffs) == 0 {
		return nil
	}

	seen := make(map[int]bool)
	var indices []int
	for _, langSysOff := range langSysOffs {
		if langSysOff+2 > len(gsub) {
			continue
		}
		defOff := int(be16(gsub, langSysOff))
		if defOff == 0 {
			continue
		}
		langSys := langSysOff + defOff
		if langSys+6 > len(gsub) {
			continue
		}
		featureCount := int(be16(gsub, langSys+4))
		if langSys+6+featureCount*2 > len(gsub) {
			continue
		}
		for i := 0; i < featureCount; i++ {
			idx := int(be16(gsub, langSys+6+i*2))
			if !seen[idx] {
				seen[idx] = true
				indices = append(indices, idx)
			}
		}
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
		feat, ok := targetTags[string(rec[:4])]
		if !ok {
			continue
		}
		featureOff := off + int(be16(rec, 4))
		if featureOff+4 > len(gsub) {
			continue
		}
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

// parseLookups walks each referenced lookup and dispatches its subtables
// to the appropriate LookupType parser. Extension lookups (type 7) are
// unwrapped; nested extensions are not expected by the spec and are
// ignored if encountered.
func parseLookups(gsub []byte, listOff int, indices []int, single map[uint16]uint16, lig map[uint16][]LigatureSubst) {
	if listOff+2 > len(gsub) {
		return
	}
	count := int(be16(gsub, listOff))
	for _, idx := range indices {
		if idx >= count {
			continue
		}
		lookupOff := listOff + int(be16(gsub, listOff+2+idx*2))
		parseLookup(gsub, lookupOff, single, lig)
	}
}

// parseLookup reads a single Lookup table, following each subtable offset
// and calling the appropriate subtable parser for supported lookup types.
func parseLookup(gsub []byte, lookupOff int, single map[uint16]uint16, lig map[uint16][]LigatureSubst) {
	if lookupOff+6 > len(gsub) {
		return
	}
	lookupType := be16(gsub, lookupOff)
	subCount := int(be16(gsub, lookupOff+4))
	if lookupOff+6+subCount*2 > len(gsub) {
		return
	}
	for si := 0; si < subCount; si++ {
		subOff := lookupOff + int(be16(gsub, lookupOff+6+si*2))
		switch lookupType {
		case 1:
			parseSingleSubst(gsub, subOff, single)
		case 4:
			parseLigatureSubst(gsub, subOff, lig)
		case 7:
			// Extension table: format(2), extensionLookupType(2),
			// extensionOffset(4, relative to the extension subtable start).
			if subOff+8 > len(gsub) {
				continue
			}
			extType := be16(gsub, subOff+2)
			extOff := subOff + int(be32(gsub, subOff+4))
			if extOff >= len(gsub) {
				continue
			}
			switch extType {
			case 1:
				parseSingleSubst(gsub, extOff, single)
			case 4:
				parseLigatureSubst(gsub, extOff, lig)
			}
		}
	}
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
		delta := int16(be16(gsub, off+4))
		for _, gid := range covered {
			subs[gid] = uint16(int16(gid) + delta)
		}
	case 2:
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

// parseLigatureSubst reads a LigatureSubstFormat1 subtable and appends
// every ligature into lig keyed by its first component.
//
// Subtable layout (ISO 14496-22 §6.2 LookupType 4):
//
//	format           uint16  (always 1)
//	coverageOffset   Offset16
//	ligatureSetCount uint16
//	ligatureSetOffsets[ligatureSetCount] Offset16
//
// Each LigatureSet:
//
//	ligatureCount      uint16
//	ligatureOffsets[]  Offset16 (relative to LigatureSet)
//
// Each Ligature:
//
//	ligatureGlyph      uint16
//	componentCount     uint16
//	componentGlyphIDs[componentCount-1] uint16
func parseLigatureSubst(gsub []byte, off int, lig map[uint16][]LigatureSubst) {
	if off+6 > len(gsub) {
		return
	}
	format := be16(gsub, off)
	if format != 1 {
		return
	}
	coverageOff := off + int(be16(gsub, off+2))
	ligSetCount := int(be16(gsub, off+4))
	if off+6+ligSetCount*2 > len(gsub) {
		return
	}
	covered := parseCoverage(gsub, coverageOff)
	if covered == nil {
		return
	}
	for i, firstGID := range covered {
		if i >= ligSetCount {
			break
		}
		setOff := off + int(be16(gsub, off+6+i*2))
		if setOff+2 > len(gsub) {
			continue
		}
		ligCount := int(be16(gsub, setOff))
		if setOff+2+ligCount*2 > len(gsub) {
			continue
		}
		for j := 0; j < ligCount; j++ {
			ligOff := setOff + int(be16(gsub, setOff+2+j*2))
			if ligOff+4 > len(gsub) {
				continue
			}
			ligGlyph := be16(gsub, ligOff)
			compCount := int(be16(gsub, ligOff+2))
			if compCount == 0 {
				continue
			}
			rest := compCount - 1
			if ligOff+4+rest*2 > len(gsub) {
				continue
			}
			var comps []uint16
			if rest > 0 {
				comps = make([]uint16, rest)
				for k := 0; k < rest; k++ {
					comps[k] = be16(gsub, ligOff+4+k*2)
				}
			}
			lig[firstGID] = append(lig[firstGID], LigatureSubst{
				Components:  comps,
				LigatureGID: ligGlyph,
			})
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
		// Format 2: RangeRecord[] where each record gives
		// startGlyphID, endGlyphID, startCoverageIndex. The coverage
		// index order is the one implied by startCoverageIndex, so
		// ranges must be placed at their declared index to preserve the
		// correspondence with Format 1 used by callers that index the
		// returned slice positionally (e.g. LigatureSubstFormat1).
		rangeCount := int(be16(gsub, off+2))
		if off+4+rangeCount*6 > len(gsub) {
			return nil
		}
		// First pass: compute total length from the highest end index.
		total := 0
		for i := 0; i < rangeCount; i++ {
			rec := off + 4 + i*6
			startGID := be16(gsub, rec)
			endGID := be16(gsub, rec+2)
			startCov := int(be16(gsub, rec+4))
			end := startCov + int(endGID-startGID) + 1
			if end > total {
				total = end
			}
		}
		result := make([]uint16, total)
		for i := 0; i < rangeCount; i++ {
			rec := off + 4 + i*6
			startGID := be16(gsub, rec)
			endGID := be16(gsub, rec+2)
			startCov := int(be16(gsub, rec+4))
			for gid := startGID; gid <= endGID; gid++ {
				idx := startCov + int(gid-startGID)
				if idx < len(result) {
					result[idx] = gid
				}
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
