// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Invoice example — generates a professional invoice PDF from HTML+CSS.
//
// Demonstrates:
//   - Rounded table headers via wrapper Div with overflow:hidden
//   - CSS Grid for From/To cards and date boxes
//   - Flexbox for header row and totals alignment
//   - Border-radius on Divs (cards, badges, payment box)
//   - Per-cell background colors (alternating rows)
//   - Header/footer element decorators with page numbers
//   - Using a remote Tailwind CSS v2 stylesheet (optional, via --tailwind flag)
//
// Usage:
//
//	go run ./examples/invoice/                    # plain CSS
//	go run ./examples/invoice/ --tailwind         # with Tailwind v2 CDN
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/layout"
)

var useTailwind = flag.Bool("tailwind", false, "fetch Tailwind CSS v2 from CDN (2.9MB, slower)")

const invoiceCSS = `
body { font-family: Helvetica; font-size: 9pt; color: #333; line-height: 1.3; }
p { margin: 0; }

/* Header row */
.header-row { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 12px; }
h1 { color: #4f46e5; font-size: 22pt; margin: 0 0 2px 0; }
.badge { display: inline-block; background: #d1fae5; color: #065f46; font-size: 7pt; padding: 3px 10px; border-radius: 20px; text-transform: uppercase; font-weight: bold; }

/* From/To cards */
.grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 10px; }
.card { border-radius: 8px; padding: 10px; border: 1px solid #e5e7eb; }
.card-from { background: #eef2ff; border-color: #c7d2fe; }
.card-to { background: #f9fafb; }
.card-label { font-size: 7pt; color: #6366f1; text-transform: uppercase; letter-spacing: 1px; font-weight: bold; margin-bottom: 3px; }
.card-to .card-label { color: #9ca3af; }
.card-name { font-size: 10pt; font-weight: bold; margin-bottom: 2px; }
.card-addr { font-size: 7pt; color: #6b7280; line-height: 1.4; }

/* Date boxes */
.grid3 { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 8px; margin-bottom: 12px; }
.date-box { border: 1px solid #e5e7eb; border-radius: 6px; padding: 6px; text-align: center; }
.date-label { font-size: 6pt; color: #9ca3af; text-transform: uppercase; letter-spacing: 1px; }
.date-value { font-size: 9pt; font-weight: bold; }

/* Table: wrapped in a rounded Div for clean outer border */
.table-wrap { border-radius: 8px; overflow: hidden; border: 1px solid #e5e7eb; margin-bottom: 10px; }
table { width: 100%; border-spacing: 0; }
th { background: #4f46e5; color: white; padding: 7px 12px; font-size: 8pt; text-transform: uppercase; letter-spacing: 0.5px; }
td { padding: 8px 12px; font-size: 9pt; border-bottom: 1px solid #e5e7eb; }
tr:last-child td { border-bottom: none; }
.alt { background: #f9fafb; }
.amount { text-align: right; font-weight: bold; }
.rlabel { text-align: right; }
.desc-sub { font-size: 7pt; color: #9ca3af; }

/* Totals */
.totals { display: flex; justify-content: flex-end; margin-bottom: 10px; }
.totals-box { width: 180px; }
.total-row { display: flex; justify-content: space-between; padding: 4px 0; border-bottom: 1px solid #f3f4f6; font-size: 8pt; color: #6b7280; }
.total-row .val { font-weight: 600; color: #374151; }
.total-due { display: flex; justify-content: space-between; background: #4f46e5; color: white; border-radius: 8px; padding: 8px 12px; margin-top: 4px; font-weight: bold; font-size: 10pt; }

/* Payment box */
.payment { background: #f9fafb; border: 1px solid #e5e7eb; border-radius: 8px; padding: 10px; margin-bottom: 8px; }
.payment-title { font-size: 7pt; color: #9ca3af; text-transform: uppercase; letter-spacing: 1px; font-weight: bold; margin-bottom: 6px; }
.payment-label { font-size: 7pt; color: #9ca3af; }
.payment-value { font-size: 9pt; font-weight: 600; }

/* Footer note */
.footer-note { font-size: 7pt; color: #9ca3af; border-top: 1px dashed #e5e7eb; padding-top: 6px; line-height: 1.4; }
.footer-note strong { color: #6b7280; }
.footer-note .link { color: #6366f1; }
`

const invoiceBody = `
<div class="header-row">
  <div>
    <h1>INVOICE</h1>
    <p style="font-size:8pt; color:#6b7280">Invoice #: <strong style="color:#374151">INV-2026-0042</strong></p>
  </div>
  <span class="badge">Due May 3, 2026</span>
</div>

<div class="grid2">
  <div class="card card-from">
    <p class="card-label">From</p>
    <p class="card-name" style="color:#4338ca">FolioPDF Inc.</p>
    <p class="card-addr">123 Document Lane<br>San Francisco, CA 94107<br>billing@foliopdf.dev</p>
  </div>
  <div class="card card-to">
    <p class="card-label">Bill To</p>
    <p class="card-name">Acme Corporation</p>
    <p class="card-addr">456 Enterprise Blvd<br>New York, NY 10001<br>accounts@acme.com</p>
  </div>
</div>

<div class="grid3">
  <div class="date-box"><p class="date-label">Issue Date</p><p class="date-value">April 3, 2026</p></div>
  <div class="date-box"><p class="date-label">Due Date</p><p class="date-value">May 3, 2026</p></div>
  <div class="date-box"><p class="date-label">Payment Terms</p><p class="date-value">Net 30</p></div>
</div>

<div class="table-wrap">
  <table>
    <thead><tr>
      <th style="text-align:left">Description</th>
      <th style="text-align:center">Qty</th>
      <th style="text-align:right">Unit Price</th>
      <th style="text-align:right">Amount</th>
    </tr></thead>
    <tbody>
      <tr>
        <td><strong>Growth Plan (monthly)</strong><br><span class="desc-sub">Monthly subscription — April 2026</span></td>
        <td style="text-align:center; color:#6b7280">1</td>
        <td class="rlabel" style="color:#6b7280">$99.00</td>
        <td class="amount">$99.00</td>
      </tr>
      <tr class="alt">
        <td><strong>API Usage</strong><br><span class="desc-sub">23,450 renders @ $0.01 per render</span></td>
        <td style="text-align:center; color:#6b7280">23,450</td>
        <td class="rlabel" style="color:#6b7280">$0.01</td>
        <td class="amount">$234.50</td>
      </tr>
      <tr>
        <td><strong>Priority Support</strong><br><span class="desc-sub">Dedicated support add-on — April 2026</span></td>
        <td style="text-align:center; color:#6b7280">1</td>
        <td class="rlabel" style="color:#6b7280">$49.00</td>
        <td class="amount">$49.00</td>
      </tr>
    </tbody>
  </table>
</div>

<div class="totals">
  <div class="totals-box">
    <div class="total-row"><span>Subtotal</span><span class="val">$382.50</span></div>
    <div class="total-row"><span>Tax (8%)</span><span class="val">$30.60</span></div>
    <div class="total-due"><span>Total Due</span><span>$413.10</span></div>
  </div>
</div>

<div class="payment">
  <p class="payment-title">Payment Instructions</p>
  <div class="grid3">
    <div><p class="payment-label">Bank Name</p><p class="payment-value">Silicon Valley Bank</p></div>
    <div><p class="payment-label">Account Number</p><p class="payment-value">•••• 4821</p></div>
    <div><p class="payment-label">Routing Number</p><p class="payment-value">•••• 0089</p></div>
  </div>
</div>

<p class="footer-note">
  Please include invoice number <strong>INV-2026-0042</strong> in your payment reference.
  Late payments may incur a 1.5% monthly fee. Questions? Contact <span class="link">billing@foliopdf.dev</span>.
</p>
`

func main() {
	flag.Parse()

	// Build the full HTML document.
	var cssBlock string
	if *useTailwind {
		// Tailwind v2 CDN: pre-built CSS with all utility classes (2.9MB).
		// Tailwind v3+ requires a build step — no pre-built CDN file available.
		cssBlock = `<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/tailwindcss@2.2.19/dist/tailwind.min.css">`
		fmt.Println("Fetching Tailwind CSS v2 from CDN (2.9MB)...")
	}

	htmlStr := fmt.Sprintf(`<!DOCTYPE html>
<html><head>
%s
<style>%s</style>
</head><body style="padding: 24px">%s</body></html>`,
		cssBlock, invoiceCSS, invoiceBody)

	// Create document.
	doc := document.NewDocument(document.PageSizeLetter)
	doc.SetMargins(layout.Margins{Top: 24, Right: 30, Bottom: 24, Left: 30})
	doc.Info.Title = "Invoice INV-2026-0042"
	doc.Info.Author = "FolioPDF Inc."

	// Convert HTML and add to document.
	if err := doc.AddHTML(htmlStr, &html.Options{
		PageWidth:  document.PageSizeLetter.Width,
		PageHeight: document.PageSizeLetter.Height,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "AddHTML: %v\n", err)
		os.Exit(1)
	}

	// Footer with page numbers.
	doc.SetFooterElement(func(ctx document.PageContext) layout.Element {
		return layout.NewParagraph("Thank you for your business!", font.HelveticaOblique, 7).
			SetAlign(layout.AlignCenter)
	})

	// Write PDF.
	f, err := os.Create("invoice.pdf")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Create: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = f.Close() }()

	n, err := doc.WriteTo(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WriteTo: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated invoice.pdf (%d bytes)\n", n)
}
