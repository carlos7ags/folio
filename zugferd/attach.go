// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package zugferd

import (
	"fmt"

	"github.com/carlos7ags/folio/document"
)

// factur-x.xml is the fixed attachment filename Factur-X validators
// look for (ISO 19005-3 §6.4 associated file mechanism).
const attachmentFileName = "factur-x.xml"

// facturXNamespaceURI is the Factur-X PDF/A XMP extension schema
// namespace, declared under the "fx" prefix.
const facturXNamespaceURI = "urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#"

// Attach validates the invoice, renders its CII XML for the given
// profile, and wires it into doc as a PDF/A-3B associated file: it
// calls [document.Document.SetPdfA] with the Factur-X XMP extension
// schema and calls [document.Document.AttachFile] with the resulting
// XML, keeping the three Factur-X consistency rules (attachment
// filename, AFRelationship, and XMP conformance level) in one place.
//
// Attach always calls SetPdfA with PdfA3B, which replaces any PDF/A
// configuration doc already had — Attach owns PDF/A conformance for
// the document rather than merging with a caller-supplied PdfAConfig.
// Call Attach before configuring unrelated PDF/A settings, or add
// custom XMPSchemas/XMPProperties by calling SetPdfA again afterward
// (SetPdfA replaces wholesale, so the caller's second call must repeat
// the Factur-X schema block if it still wants both).
func (inv *Invoice) Attach(doc *document.Document, p Profile) error {
	xmlData, err := inv.XML(p)
	if err != nil {
		return err
	}

	doc.SetPdfA(document.PdfAConfig{
		Level: document.PdfA3B,
		XMPSchemas: []document.XMPSchema{{
			Schema:       "Factur-X PDFA Extension Schema",
			NamespaceURI: facturXNamespaceURI,
			Prefix:       "fx",
			Properties: []document.XMPSchemaProperty{
				{Name: "DocumentFileName", ValueType: "Text", Category: "external", Description: "Name of the embedded XML invoice file"},
				{Name: "DocumentType", ValueType: "Text", Category: "external", Description: "Type of the hybrid document"},
				{Name: "Version", ValueType: "Text", Category: "external", Description: "Version of the Factur-X standard"},
				{Name: "ConformanceLevel", ValueType: "Text", Category: "external", Description: "Factur-X conformance level"},
			},
		}},
		XMPProperties: []document.XMPPropertyBlock{{
			Namespace: facturXNamespaceURI,
			Prefix:    "fx",
			Properties: []document.XMPProperty{
				{Name: "DocumentFileName", Value: attachmentFileName},
				{Name: "DocumentType", Value: "INVOICE"},
				{Name: "Version", Value: "1.0"},
				{Name: "ConformanceLevel", Value: p.String()},
			},
		}},
	})

	doc.AttachFile(document.FileAttachment{
		FileName:       attachmentFileName,
		MIMEType:       "application/xml",
		Description:    fmt.Sprintf("Factur-X XML Invoice Data (%s profile)", p),
		AFRelationship: "Alternative",
		Data:           xmlData,
	})

	return nil
}
