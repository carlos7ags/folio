// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
	xhtml "golang.org/x/net/html"
)

// TestFindHTMLLangExtractsAttribute covers the helper that walks the
// parsed document tree looking for the root <html> element's `lang`.
// Mirrors the BCP-47 tags the @font-face loader will route to
// font.ParseFontForLanguage; absent and malformed cases must yield
// an empty string so the upstream call falls back to face-0.
func TestFindHTMLLangExtractsAttribute(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"zh-CN", `<html lang="zh-CN"><body>x</body></html>`, "zh-CN"},
		{"ja", `<html lang="ja"><body>x</body></html>`, "ja"},
		{"en-US", `<html lang="en-US"><body>x</body></html>`, "en-US"},
		{"no attribute", `<html><body>x</body></html>`, ""},
		{"empty value", `<html lang=""><body>x</body></html>`, ""},
		{"doctype before html", `<!DOCTYPE html><html lang="ko"><body>x</body></html>`, "ko"},
		{"no html element (fragment)", `<body>x</body>`, ""},

		// Edge cases from the review:
		// - duplicate attributes: getAttr returns the first, so the
		//   leftmost lang wins. Pinned so a future "last wins" change
		//   is intentional.
		// - whitespace value: returned verbatim (not trimmed). The
		//   font.ParseFontForLanguage path treats unknown tags as
		//   face-0 fallback, so leading/trailing space silently
		//   degrades. Pinning the verbatim contract makes the trim
		//   decision (here or in the picker) explicit when we make it.
		// - mixed case "ZH-cn": returned verbatim. The picker
		//   case-folds tokens internally.
		// - uppercase tag/attribute names: golang.org/x/net/html
		//   normalises both to lowercase before exposing DataAtom /
		//   Key, so atom.Html and "lang" match.
		{"duplicate lang attributes — first wins", `<html lang="zh-CN" lang="ja"><body>x</body></html>`, "zh-CN"},
		{"whitespace value returned verbatim", `<html lang="  zh-CN  "><body>x</body></html>`, "  zh-CN  "},
		{"mixed case value returned verbatim", `<html lang="ZH-cn"><body>x</body></html>`, "ZH-cn"},
		{"uppercase tag and attribute names", `<HTML LANG="zh-CN"><BODY>x</BODY></HTML>`, "zh-CN"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := xhtml.Parse(strings.NewReader(tc.in))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := findHTMLLang(doc); got != tc.want {
				t.Errorf("findHTMLLang = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestConvertFullPopulatesMetadataLanguage verifies that the document
// lang reaches ConvertResult.Metadata.Language — the callable surface
// for downstream code that needs the same hint (PDF /Lang catalog
// entry, future per-element lookups, etc.).
func TestConvertFullPopulatesMetadataLanguage(t *testing.T) {
	cases := []struct {
		name string
		html string
		want string
	}{
		{
			name: "zh-CN reaches metadata",
			html: `<!DOCTYPE html><html lang="zh-CN"><body><p>测试</p></body></html>`,
			want: "zh-CN",
		},
		{
			name: "absent lang yields empty metadata",
			html: `<!DOCTYPE html><html><body><p>x</p></body></html>`,
			want: "",
		},
		{
			name: "ja reaches metadata",
			html: `<!DOCTYPE html><html lang="ja"><body><p>テスト</p></body></html>`,
			want: "ja",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ConvertFull(tc.html, nil)
			if err != nil {
				t.Fatalf("ConvertFull: %v", err)
			}
			if result.Metadata.Language != tc.want {
				t.Errorf("Metadata.Language = %q, want %q", result.Metadata.Language, tc.want)
			}
			// Conversion must still produce content. Without this
			// guard a regression that mis-routes Convert could leave
			// Metadata.Language correct while emitting no elements —
			// the test would silently green.
			if len(result.Elements) == 0 {
				t.Error("ConvertFull produced no elements; doc conversion may have aborted")
			}
		})
	}
}

// TestLoadFontFacesUsesDocLangForTTC is the wire test demanded by
// #280's acceptance criterion: HTML with <html lang="zh-CN"> + a
// pan-CJK TTC at @font-face must load the SC face, not face-0 (JP).
//
// Constructs a 2-face TTC inline (Synthetic JP + Synthetic SC),
// writes it to a tempdir, then drives loadFontFaces twice — once
// with c.metadata.Language = "ja" and once with "zh-CN" — and
// compares the resulting embedded Face.RawData() bytes. JP and SC
// faces are byte-distinct (the mutated FontFamily strings differ),
// so a regression that drops the lang wire would yield identical
// RawData for both runs.
//
// The previous tests for this PR only checked Metadata.Language,
// which is set INDEPENDENTLY of the loadFontFaces call — a regression
// that hardcoded `lang := ""` inside loadFontFaces would leave them
// green. This test plugs the gap.
func TestLoadFontFacesUsesDocLangForTTC(t *testing.T) {
	abs, err := filepath.Abs("../font/testdata/synthetic_cjk.ttf")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	ttfBytes, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	// Two faces: JP family at index 0, SC family at index 1. The
	// picker matches "JP" / "SC" tokens in NameID 1 FontFamily.
	ttc := buildLocalCJKTTC(t, ttfBytes, "Synthetic JP", "Synthetic SC")
	ttcPath := filepath.Join(t.TempDir(), "synthetic.ttc")
	if err := os.WriteFile(ttcPath, ttc, 0o644); err != nil {
		t.Fatalf("write ttc: %v", err)
	}

	loadWith := func(lang string) []byte {
		c := &converter{
			opts:          (&Options{AllowAbsolutePaths: true}).defaults(),
			logger:        loggerOrDiscard(nil),
			embeddedFonts: make(map[string]*font.EmbeddedFont),
		}
		c.metadata.Language = lang
		c.loadFontFaces([]fontFaceRule{{
			family: "synth",
			src:    ttcPath,
			weight: 400,
			style:  "normal",
			origin: "",
		}})
		ef := c.embeddedFonts["synth|400|normal"]
		if ef == nil {
			t.Fatalf("lang=%q: @font-face not loaded into embeddedFonts", lang)
		}
		return ef.Face().RawData()
	}

	jp := loadWith("ja")
	sc := loadWith("zh-CN")

	if bytes.Equal(jp, sc) {
		t.Fatal("ja and zh-CN loaded byte-identical faces; the lang wire is broken (loadFontFaces is calling ParseFontForLanguage with the same value for both runs)")
	}

	// Confirm each run picked the face that carries its expected
	// FontFamily string. The mutated TTF stores FontFamily as
	// UTF-16BE inside the name table, so we look for the encoded form.
	wantJP := utf16BE("Synthetic JP")
	wantSC := utf16BE("Synthetic SC")
	if !bytes.Contains(jp, wantJP) {
		t.Errorf("ja-loaded face does not carry the JP FontFamily; wrong face selected")
	}
	if !bytes.Contains(sc, wantSC) {
		t.Errorf("zh-CN-loaded face does not carry the SC FontFamily; wrong face selected")
	}
}

// TestLangPropagationDoesNotBreakFontFaceLoad runs the full @font-face
// pipeline with both a langless document and a lang-tagged one,
// against the same single-face TTF fixture. This is the integration
// counterpart to TestLoadFontFacesUsesDocLangForTTC above: it routes
// through ConvertFull (parsing + walking + element emission) where
// the wire test peeks at converter internals directly.
func TestLangPropagationDoesNotBreakFontFaceLoad(t *testing.T) {
	abs, err := filepath.Abs("../font/testdata/synthetic_cjk.ttf")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	for _, tc := range []struct {
		name string
		lang string
	}{
		{"no lang (back-compat)", ""},
		{"with zh-CN lang", "zh-CN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			html := buildSyntheticCJKHTML(abs, tc.lang)
			result, err := ConvertFull(html, &Options{AllowAbsolutePaths: true, StrictAssets: true})
			if err != nil {
				t.Fatalf("ConvertFull: %v", err)
			}
			if result.Metadata.Language != tc.lang {
				t.Errorf("Metadata.Language = %q, want %q", result.Metadata.Language, tc.lang)
			}
			// At least one element rendered — the @font-face load
			// did not abort the conversion.
			if len(result.Elements) == 0 {
				t.Error("ConvertFull produced no elements; @font-face load may have failed")
			}
		})
	}
}

// buildLocalCJKTTC wraps multiple copies of ttf into a TTC, mutating
// each face's name-table FontFamily (NameID 1) to one of the supplied
// family strings. Adapted from font/ttc_lang_test.go's same-named
// helper — kept private to this package because cross-package test
// fixture sharing requires either an exported helper API or a
// dedicated testing-only package, both of which would be heavier
// than this 60-line duplication.
//
// The face count equals len(families); face N carries the Nth family.
// Each family must use the SAME byte length when UTF-16BE-encoded so
// the mutation can rewrite the name string in place — Synthetic JP /
// Synthetic SC satisfy that.
func buildLocalCJKTTC(t *testing.T, ttf []byte, families ...string) []byte {
	t.Helper()
	if len(families) < 1 {
		t.Fatal("need at least 1 family")
	}
	mutated := make([][]byte, len(families))
	for i, fam := range families {
		mutated[i] = mutateTTFFontFamilyInPlace(t, ttf, fam)
	}
	headerSize := 12 + len(families)*4
	cursor := headerSize
	faceOffsets := make([]int, len(families))
	for i, m := range mutated {
		faceOffsets[i] = cursor
		cursor += len(m)
	}
	out := make([]byte, cursor)
	copy(out[0:4], "ttcf")
	binary.BigEndian.PutUint32(out[4:8], 0x00010000)
	binary.BigEndian.PutUint32(out[8:12], uint32(len(families)))
	for i, off := range faceOffsets {
		binary.BigEndian.PutUint32(out[12+i*4:], uint32(off))
	}
	for i, m := range mutated {
		off := faceOffsets[i]
		copy(out[off:off+len(m)], m)
		// Rewrite each table directory entry's offset to be absolute
		// within the TTC — extractTTCFont expects that convention.
		numTables := int(binary.BigEndian.Uint16(out[off+4 : off+6]))
		for j := 0; j < numTables; j++ {
			entry := off + 12 + j*16
			localOff := binary.BigEndian.Uint32(out[entry+8 : entry+12])
			binary.BigEndian.PutUint32(out[entry+8:entry+12], localOff+uint32(off))
		}
	}
	return out
}

// mutateTTFFontFamilyInPlace returns a copy of ttf with the `name`
// table's NameID 1 record overwritten to family. Requires the new
// family string to fit within the existing name-table region — the
// helper does not relocate the table. Callers supplying differing
// family lengths must pad to the same byte width.
func mutateTTFFontFamilyInPlace(t *testing.T, ttf []byte, family string) []byte {
	t.Helper()
	out := make([]byte, len(ttf))
	copy(out, ttf)
	numTables := int(binary.BigEndian.Uint16(out[4:6]))
	for i := 0; i < numTables; i++ {
		entry := 12 + i*16
		if string(out[entry:entry+4]) != "name" {
			continue
		}
		oldOff := int(binary.BigEndian.Uint32(out[entry+8 : entry+12]))
		oldLen := int(binary.BigEndian.Uint32(out[entry+12 : entry+16]))
		newName := buildSingleRecordNameTable(t, family)
		if len(newName) > oldLen {
			t.Fatalf("family %q name table %d bytes; old slot only %d bytes (would relocate; not implemented in this minimal helper)",
				family, len(newName), oldLen)
		}
		for j := oldOff; j < oldOff+oldLen; j++ {
			out[j] = 0
		}
		copy(out[oldOff:oldOff+len(newName)], newName)
		binary.BigEndian.PutUint32(out[entry+12:entry+16], uint32(len(newName)))
		return out
	}
	t.Fatal("ttf has no name table to mutate")
	return nil
}

// buildSingleRecordNameTable emits a name table containing exactly one
// FontFamily (NameID 1) record encoded as UTF-16BE under platform 3
// (Windows) / encoding 1 (Unicode BMP) / language 0x0409 (en-US).
func buildSingleRecordNameTable(t *testing.T, family string) []byte {
	t.Helper()
	famBytes := utf16BE(family)
	header := 6 + 12
	total := header + len(famBytes)
	out := make([]byte, total)
	binary.BigEndian.PutUint16(out[0:2], 0)
	binary.BigEndian.PutUint16(out[2:4], 1)
	binary.BigEndian.PutUint16(out[4:6], uint16(header))
	binary.BigEndian.PutUint16(out[6:8], 3)
	binary.BigEndian.PutUint16(out[8:10], 1)
	binary.BigEndian.PutUint16(out[10:12], 0x0409)
	binary.BigEndian.PutUint16(out[12:14], 1)
	binary.BigEndian.PutUint16(out[14:16], uint16(len(famBytes)))
	binary.BigEndian.PutUint16(out[16:18], 0)
	copy(out[header:], famBytes)
	return out
}

// utf16BE encodes a UTF-8 string as UTF-16BE bytes. Surrogate pairs
// are not exercised by the family strings used here but the encoder
// handles them anyway.
func utf16BE(s string) []byte {
	out := make([]byte, 0, len(s)*2)
	for _, r := range s {
		if r <= 0xFFFF {
			out = append(out, byte(r>>8), byte(r))
			continue
		}
		r -= 0x10000
		hi := 0xD800 + (r >> 10)
		lo := 0xDC00 + (r & 0x3FF)
		out = append(out, byte(hi>>8), byte(hi), byte(lo>>8), byte(lo))
	}
	return out
}

// buildSyntheticCJKHTML constructs a minimal HTML document that
// loads the synthetic CJK TTF via @font-face url() and renders one
// CJK character with it. The lang argument is omitted when empty,
// matching how a real <html> element without a lang attribute is
// emitted.
func buildSyntheticCJKHTML(ttfPath, lang string) string {
	openTag := "<html>"
	if lang != "" {
		openTag = `<html lang="` + lang + `">`
	}
	return openTag + `<head><style>
@font-face { font-family: 'Synth'; src: url('` + ttfPath + `'); }
body { font-family: 'Synth'; }
</style></head><body><p>中</p></body></html>`
}
