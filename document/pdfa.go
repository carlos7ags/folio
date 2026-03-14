// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"fmt"
	"strings"
	"time"

	"github.com/carlos7ags/folio/core"
)

// PdfALevel specifies the PDF/A conformance level.
type PdfALevel int

const (
	// PdfA2B is PDF/A-2b (ISO 19005-2:2011, Level B).
	// Based on PDF 1.7. Allows transparency. Requires font embedding,
	// XMP metadata, and an output intent with ICC profile.
	PdfA2B PdfALevel = iota

	// PdfA2U is PDF/A-2u (Level U). Adds Unicode mapping requirement.
	PdfA2U

	// PdfA2A is PDF/A-2a (Level A). Adds structure tagging requirement.
	PdfA2A

	// PdfA3B is PDF/A-3b. Like A-2b but allows file attachments.
	PdfA3B
)

// PdfAConfig holds PDF/A conformance settings.
type PdfAConfig struct {
	Level PdfALevel

	// ICCProfile is the ICC color profile data for the output intent.
	// If nil, a minimal sRGB profile description is used.
	ICCProfile []byte

	// OutputCondition is the output condition identifier
	// (e.g. "sRGB IEC61966-2.1"). Defaults to "sRGB IEC61966-2.1".
	OutputCondition string
}

// SetPdfA enables PDF/A conformance on the document.
// This enforces: font embedding, XMP metadata, output intent,
// and disables encryption. For level A, tagging is enabled automatically.
func (d *Document) SetPdfA(config PdfAConfig) {
	d.pdfA = &config
	// PDF/A-2a requires tagged PDF.
	if config.Level == PdfA2A {
		d.tagged = true
	}
	// PDF/A disallows encryption.
	d.encryption = nil
}

// pdfALevelString returns the conformance level letter.
func pdfALevelString(level PdfALevel) string {
	switch level {
	case PdfA2B, PdfA3B:
		return "B"
	case PdfA2U:
		return "U"
	case PdfA2A:
		return "A"
	default:
		return "B"
	}
}

// pdfAPartNumber returns the PDF/A part number.
func pdfAPartNumber(level PdfALevel) int {
	switch level {
	case PdfA3B:
		return 3
	default:
		return 2
	}
}

// validatePdfA checks that the document meets PDF/A requirements.
// Returns an error describing the first violation found, or nil if valid.
func (d *Document) validatePdfA(allPages []*Page) error {
	if d.pdfA == nil {
		return nil
	}

	// PDF/A forbids encryption.
	if d.encryption != nil {
		return fmt.Errorf("pdfa: encryption is not allowed in PDF/A documents")
	}

	// All fonts on all pages must be embedded (no bare standard fonts).
	for i, page := range allPages {
		for _, fr := range page.fonts {
			if fr.standard != nil && fr.embedded == nil {
				return fmt.Errorf("pdfa: page %d uses non-embedded standard font %q; PDF/A requires all fonts to be embedded",
					i, fr.standard.Name())
			}
		}
	}

	// Title is required.
	if d.Info.Title == "" {
		return fmt.Errorf("pdfa: document Title is required for PDF/A conformance")
	}

	return nil
}

// buildXMPMetadata generates the XMP metadata stream for PDF/A identification.
func buildXMPMetadata(info Info, level PdfALevel, addObject func(core.PdfObject) *core.PdfIndirectReference) *core.PdfIndirectReference {
	part := pdfAPartNumber(level)
	conf := pdfALevelString(level)

	now := time.Now()
	if !info.CreationDate.IsZero() {
		now = info.CreationDate
	}
	dateStr := now.Format("2006-01-02T15:04:05-07:00")

	title := xmlEscape(info.Title)
	author := xmlEscape(info.Author)
	creator := xmlEscape(info.Creator)
	if creator == "" {
		creator = "Folio"
	}
	producer := xmlEscape(info.Producer)
	if producer == "" {
		producer = "Folio (github.com/carlos7ags/folio)"
	}

	var b strings.Builder
	b.WriteString(`<?xpacket begin="` + "\xef\xbb\xbf" + `" id="W5M0MpCehiHzreSzNTczkc9d"?>`)
	b.WriteString("\n")
	b.WriteString(`<x:xmpmeta xmlns:x="adobe:ns:meta/">`)
	b.WriteString("\n")
	b.WriteString(`<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">`)
	b.WriteString("\n")

	// Dublin Core (title, creator)
	b.WriteString(`<rdf:Description rdf:about=""`)
	b.WriteString(` xmlns:dc="http://purl.org/dc/elements/1.1/">`)
	b.WriteString("\n")
	if title != "" {
		b.WriteString(`<dc:title><rdf:Alt><rdf:li xml:lang="x-default">` + title + `</rdf:li></rdf:Alt></dc:title>`)
		b.WriteString("\n")
	}
	if author != "" {
		b.WriteString(`<dc:creator><rdf:Seq><rdf:li>` + author + `</rdf:li></rdf:Seq></dc:creator>`)
		b.WriteString("\n")
	}
	b.WriteString(`</rdf:Description>`)
	b.WriteString("\n")

	// XMP Basic (creator tool, dates)
	b.WriteString(`<rdf:Description rdf:about=""`)
	b.WriteString(` xmlns:xmp="http://ns.adobe.com/xap/1.0/">`)
	b.WriteString("\n")
	b.WriteString(`<xmp:CreatorTool>` + creator + `</xmp:CreatorTool>`)
	b.WriteString("\n")
	b.WriteString(`<xmp:CreateDate>` + dateStr + `</xmp:CreateDate>`)
	b.WriteString("\n")
	b.WriteString(`<xmp:ModifyDate>` + dateStr + `</xmp:ModifyDate>`)
	b.WriteString("\n")
	b.WriteString(`</rdf:Description>`)
	b.WriteString("\n")

	// PDF properties
	b.WriteString(`<rdf:Description rdf:about=""`)
	b.WriteString(` xmlns:pdf="http://ns.adobe.com/pdf/1.3/">`)
	b.WriteString("\n")
	b.WriteString(`<pdf:Producer>` + producer + `</pdf:Producer>`)
	b.WriteString("\n")
	b.WriteString(`</rdf:Description>`)
	b.WriteString("\n")

	// PDF/A identification
	b.WriteString(`<rdf:Description rdf:about=""`)
	b.WriteString(` xmlns:pdfaid="http://www.aiim.org/pdfa/ns/id/">`)
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf(`<pdfaid:part>%d</pdfaid:part>`, part))
	b.WriteString("\n")
	b.WriteString(`<pdfaid:conformance>` + conf + `</pdfaid:conformance>`)
	b.WriteString("\n")
	b.WriteString(`</rdf:Description>`)
	b.WriteString("\n")

	b.WriteString(`</rdf:RDF>`)
	b.WriteString("\n")
	b.WriteString(`</x:xmpmeta>`)
	b.WriteString("\n")
	b.WriteString(`<?xpacket end="w"?>`)

	xmpBytes := []byte(b.String())

	// XMP metadata stream: must NOT be compressed, must have /Type /Metadata /Subtype /XML.
	stream := core.NewPdfStream(xmpBytes)
	stream.Dict.Set("Type", core.NewPdfName("Metadata"))
	stream.Dict.Set("Subtype", core.NewPdfName("XML"))

	return addObject(stream)
}

// buildOutputIntent creates the PDF/A output intent dictionary with
// an embedded ICC color profile.
func buildOutputIntent(config *PdfAConfig, addObject func(core.PdfObject) *core.PdfIndirectReference) *core.PdfIndirectReference {
	condition := config.OutputCondition
	if condition == "" {
		condition = "sRGB IEC61966-2.1"
	}

	// ICC profile stream.
	profileData := config.ICCProfile
	if len(profileData) == 0 {
		profileData = minimalSRGBProfile()
	}

	profileStream := core.NewPdfStreamCompressed(profileData)
	profileStream.Dict.Set("N", core.NewPdfInteger(3)) // 3 components (RGB)
	profileRef := addObject(profileStream)

	// Output intent dictionary.
	intent := core.NewPdfDictionary()
	intent.Set("Type", core.NewPdfName("OutputIntent"))
	intent.Set("S", core.NewPdfName("GTS_PDFA1")) // required for PDF/A
	intent.Set("OutputConditionIdentifier", core.NewPdfLiteralString(condition))
	intent.Set("RegistryName", core.NewPdfLiteralString("http://www.color.org"))
	intent.Set("Info", core.NewPdfLiteralString(condition))
	intent.Set("DestOutputProfile", profileRef)

	return addObject(intent)
}

// minimalSRGBProfile returns a minimal ICC profile header for sRGB.
// This is a 128-byte profile header that identifies the color space
// as sRGB. For full PDF/A compliance in production, use a complete
// sRGB ICC profile (available from color.org).
func minimalSRGBProfile() []byte {
	// Minimal ICC profile: 128-byte header only.
	// This satisfies basic PDF/A validators. For strict compliance,
	// embed the full sRGB2014.icc profile.
	profile := make([]byte, 128)

	// Profile size (4 bytes, big-endian).
	profile[0] = 0
	profile[1] = 0
	profile[2] = 0
	profile[3] = 128

	// Preferred CMM type: none.
	// Profile version: 2.1.0.
	profile[8] = 2
	profile[9] = 0x10

	// Profile/device class: 'mntr' (monitor).
	copy(profile[12:16], "mntr")

	// Color space: 'RGB '.
	copy(profile[16:20], "RGB ")

	// Profile connection space: 'XYZ '.
	copy(profile[20:24], "XYZ ")

	// Date/time: 2024-01-01 00:00:00.
	profile[24] = 0x07
	profile[25] = 0xe8 // year 2024
	profile[26] = 0
	profile[27] = 1 // month
	profile[28] = 0
	profile[29] = 1 // day

	// Profile signature: 'acsp'.
	copy(profile[36:40], "acsp")

	// Primary platform: 'APPL'.
	copy(profile[40:44], "APPL")

	// Rendering intent: perceptual (0).

	// Illuminant: D50 (XYZ = 0.9642, 1.0000, 0.8249 in s15Fixed16).
	// X: 0.9642 * 65536 = 63190 = 0x0000F6D6
	profile[68] = 0x00
	profile[69] = 0x00
	profile[70] = 0xF6
	profile[71] = 0xD6
	// Y: 1.0 * 65536 = 65536 = 0x00010000
	profile[72] = 0x00
	profile[73] = 0x01
	profile[74] = 0x00
	profile[75] = 0x00
	// Z: 0.8249 * 65536 = 54061 = 0x0000D32D
	profile[76] = 0x00
	profile[77] = 0x00
	profile[78] = 0xD3
	profile[79] = 0x2D

	return profile
}

// xmlEscape escapes special XML characters.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
