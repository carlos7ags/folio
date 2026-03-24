// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// ZUGFeRD demonstrates creating a PDF/A-3B compliant invoice with an
// embedded Factur-X/ZUGFeRD XML file attachment.
//
// PDF/A-3B is the only PDF/A level that permits file attachments
// (ISO 19005-3 §6.4). This makes it the standard for hybrid e-invoice
// formats where a machine-readable XML is carried inside a human-readable
// PDF.
//
// The example generates a minimal invoice PDF with:
//   - PDF/A-3B compliance with sRGB output intent
//   - An embedded XML attachment with AFRelationship "Alternative"
//   - XMP metadata declaring the Factur-X schema
//   - Invoice content rendered via the HTML converter (auto-embeds fonts)
//
// Usage:
//
//	go run ./examples/zugferd
package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/layout"
)

func main() {
	doc := document.NewDocument(document.PageSizeA4)
	doc.SetMargins(layout.Margins{Top: 40, Right: 40, Bottom: 40, Left: 40})
	doc.Info.Title = "Invoice 2024-001"
	doc.Info.Author = "ACME Corp"

	// --- Discover a system font for embedding (PDF/A requires it) ---
	fontPath := findSystemFont()
	if fontPath == "" {
		fmt.Fprintln(os.Stderr, "no suitable system font found for PDF/A embedding")
		os.Exit(1)
	}

	// --- Invoice content via HTML (auto-embeds fonts for PDF/A) ---
	invoiceHTML := `<html><head><style>
@font-face { font-family: 'Inv'; src: url('` + fontPath + `'); }
@font-face { font-family: 'InvBold'; font-weight: bold; src: url('` + fontPath + `'); }
body { font-family: 'Inv'; font-size: 10px; margin: 0; }
b, strong, th { font-family: 'InvBold'; }
h1 { font-size: 22px; color: #1a1a2e; text-align: center; margin-bottom: 4px; }
h2 { font-size: 13px; color: #2c3e50; margin-top: 12px; margin-bottom: 4px; }
p { margin-bottom: 3px; line-height: 1.3; }
table { width: 100%; border-collapse: collapse; margin-top: 10px; }
th { background-color: #f0f0f0; text-align: left; padding: 6px 8px;
     border-bottom: 2px solid #333; font-weight: bold; }
td { padding: 5px 8px; border-bottom: 1px solid #ddd; }
.right { text-align: right; }
.total td { border-top: 2px solid #333; font-weight: bold; }
.meta { color: #555; }
hr { margin: 8px 0; border: none; border-top: 1px solid #ccc; }
</style></head><body>

<h1>INVOICE</h1>
<hr/>

<p><b>Invoice Number:</b> 2024-001</p>
<p><b>Date:</b> 2024-01-15</p>
<p><b>Due Date:</b> 2024-02-15</p>

<h2>From</h2>
<p>ACME Corp<br/>123 Main Street<br/>Berlin, 10115, Germany<br/>VAT: DE123456789</p>

<h2>Bill To</h2>
<p>Example GmbH<br/>456 Business Ave<br/>Munich, 80331, Germany<br/>VAT: DE987654321</p>

<table>
<tr><th>Description</th><th class="right">Qty</th><th class="right">Unit Price</th><th class="right">Total</th></tr>
<tr><td>Widget A - Standard</td><td class="right">10</td><td class="right">5.00 EUR</td><td class="right">50.00 EUR</td></tr>
<tr><td>Widget B - Premium</td><td class="right">3</td><td class="right">12.50 EUR</td><td class="right">37.50 EUR</td></tr>
<tr><td>Consulting Service</td><td class="right">1</td><td class="right">250.00 EUR</td><td class="right">250.00 EUR</td></tr>
<tr class="total"><td></td><td></td><td class="right">Subtotal:</td><td class="right">337.50 EUR</td></tr>
<tr class="total"><td></td><td></td><td class="right">VAT (19%):</td><td class="right">64.13 EUR</td></tr>
<tr class="total"><td></td><td></td><td class="right"><b>Total:</b></td><td class="right"><b>401.63 EUR</b></td></tr>
</table>

<p class="meta" style="margin-top: 14px;">Payment terms: 30 days net. Bank: Deutsche Bank, IBAN: DE89 3704 0044 0532 0130 00</p>
<p class="meta">This invoice contains an embedded Factur-X XML for automated processing.</p>

</body></html>`

	elems, err := html.Convert(invoiceHTML, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "html:", err)
		os.Exit(1)
	}
	for _, e := range elems {
		doc.Add(e)
	}

	// --- PDF/A-3B with Factur-X XMP schema ---
	doc.SetPdfA(document.PdfAConfig{
		Level: document.PdfA3B,
		XMPSchemas: []document.XMPSchema{{
			Schema:       "Factur-X PDFA Extension Schema",
			NamespaceURI: "urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#",
			Prefix:       "fx",
			Properties: []document.XMPSchemaProperty{
				{Name: "DocumentFileName", ValueType: "Text", Category: "external", Description: "Name of the embedded XML invoice file"},
				{Name: "DocumentType", ValueType: "Text", Category: "external", Description: "Type of the hybrid document"},
				{Name: "Version", ValueType: "Text", Category: "external", Description: "Version of the Factur-X standard"},
				{Name: "ConformanceLevel", ValueType: "Text", Category: "external", Description: "Factur-X conformance level"},
			},
		}},
		XMPProperties: []document.XMPPropertyBlock{{
			Namespace: "urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#",
			Prefix:    "fx",
			Properties: []document.XMPProperty{
				{Name: "DocumentFileName", Value: "factur-x.xml"},
				{Name: "DocumentType", Value: "INVOICE"},
				{Name: "Version", Value: "1.0"},
				{Name: "ConformanceLevel", Value: "BASIC"},
			},
		}},
	})

	// --- Attach Factur-X XML ---
	// Minimal CrossIndustryInvoice matching the PDF content.
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rsm:CrossIndustryInvoice
  xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"
  xmlns:ram="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100"
  xmlns:udt="urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100">
  <rsm:ExchangedDocumentContext>
    <ram:GuidelineSpecifiedDocumentContextParameter>
      <ram:ID>urn:factur-x.eu:1p0:basic</ram:ID>
    </ram:GuidelineSpecifiedDocumentContextParameter>
  </rsm:ExchangedDocumentContext>
  <rsm:ExchangedDocument>
    <ram:ID>2024-001</ram:ID>
    <ram:TypeCode>380</ram:TypeCode>
    <ram:IssueDateTime>
      <udt:DateTimeString format="102">20240115</udt:DateTimeString>
    </ram:IssueDateTime>
  </rsm:ExchangedDocument>
  <rsm:SupplyChainTradeTransaction>
    <ram:ApplicableHeaderTradeAgreement>
      <ram:SellerTradeParty>
        <ram:Name>ACME Corp</ram:Name>
      </ram:SellerTradeParty>
      <ram:BuyerTradeParty>
        <ram:Name>Example GmbH</ram:Name>
      </ram:BuyerTradeParty>
    </ram:ApplicableHeaderTradeAgreement>
    <ram:ApplicableHeaderTradeSettlement>
      <ram:InvoiceCurrencyCode>EUR</ram:InvoiceCurrencyCode>
      <ram:SpecifiedTradeSettlementHeaderMonetarySummation>
        <ram:TaxBasisTotalAmount>337.50</ram:TaxBasisTotalAmount>
        <ram:TaxTotalAmount currencyID="EUR">64.13</ram:TaxTotalAmount>
        <ram:GrandTotalAmount>401.63</ram:GrandTotalAmount>
        <ram:DuePayableAmount>401.63</ram:DuePayableAmount>
      </ram:SpecifiedTradeSettlementHeaderMonetarySummation>
    </ram:ApplicableHeaderTradeSettlement>
  </rsm:SupplyChainTradeTransaction>
</rsm:CrossIndustryInvoice>`)

	doc.AttachFile(document.FileAttachment{
		FileName:       "factur-x.xml",
		MIMEType:       "application/xml",
		Description:    "Factur-X XML Invoice Data (BASIC profile)",
		AFRelationship: "Alternative",
		Data:           xmlData,
	})

	// --- Write ---
	if err := doc.Save("zugferd-invoice.pdf"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Created zugferd-invoice.pdf")
}

// findSystemFont returns the path to a TrueType font available on the
// current system. PDF/A requires all fonts to be embedded.
func findSystemFont() string {
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/System/Library/Fonts/Supplemental/Arial.ttf",
			"/System/Library/Fonts/Supplemental/Verdana.ttf",
			"/System/Library/Fonts/Supplemental/Georgia.ttf",
		}
	case "linux":
		candidates = []string{
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
			"/usr/share/fonts/dejavu/DejaVuSans.ttf",
		}
	case "windows":
		candidates = []string{
			`C:\Windows\Fonts\arial.ttf`,
			`C:\Windows\Fonts\verdana.ttf`,
		}
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
