// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/html"
)

// ---------------------------------------------------------------------------
// End-to-end HTML-to-PDF benchmarks
//
// These measure the full pipeline: HTML parse + CSS cascade + layout + PDF
// serialization. Documents are realistic production templates, not trivial
// single-line inputs.
// ---------------------------------------------------------------------------

// benchInvoiceHTML is a styled invoice with CSS Grid, flexbox, border-radius,
// alternating table rows, and multiple card components.
const benchInvoiceHTML = `<!DOCTYPE html>
<html><head><style>
body { font-family: Helvetica; font-size: 9pt; color: #333; }
.header { display: flex; justify-content: space-between; margin-bottom: 12px; }
h1 { color: #4f46e5; font-size: 22pt; margin: 0; }
.badge { display: inline-block; background: #d1fae5; color: #065f46; font-size: 7pt;
         padding: 3px 10px; border-radius: 20px; text-transform: uppercase; font-weight: bold; }
.grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 10px; }
.card { border-radius: 8px; padding: 10px; border: 1px solid #e5e7eb; }
.card-from { background: #eef2ff; }
.grid3 { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 8px; margin-bottom: 12px; }
.date-box { border: 1px solid #e5e7eb; border-radius: 6px; padding: 6px; text-align: center; }
.table-wrap { border-radius: 8px; overflow: hidden; border: 1px solid #e5e7eb; margin-bottom: 10px; }
table { width: 100%; border-spacing: 0; }
th { background: #4f46e5; color: white; padding: 7px 12px; font-size: 8pt; text-transform: uppercase; }
td { padding: 8px 12px; font-size: 9pt; border-bottom: 1px solid #e5e7eb; }
.alt { background: #f9fafb; }
.amount { text-align: right; font-weight: bold; }
.totals { display: flex; justify-content: flex-end; margin-bottom: 10px; }
.totals-box { width: 180px; }
.total-row { display: flex; justify-content: space-between; padding: 4px 0;
             border-bottom: 1px solid #f3f4f6; font-size: 8pt; color: #6b7280; }
.total-due { display: flex; justify-content: space-between; background: #4f46e5; color: white;
             border-radius: 8px; padding: 8px 12px; margin-top: 4px; font-weight: bold; }
.payment { background: #f9fafb; border: 1px solid #e5e7eb; border-radius: 8px; padding: 10px; }
</style></head>
<body style="padding: 24px">
<div class="header">
  <div><h1>INVOICE</h1><p style="font-size:8pt; color:#6b7280">Invoice #: <strong>INV-2026-0042</strong></p></div>
  <span class="badge">Due May 3, 2026</span>
</div>
<div class="grid2">
  <div class="card card-from">
    <p style="font-size:7pt; color:#6366f1; text-transform:uppercase; font-weight:bold">From</p>
    <p style="font-weight:bold; color:#4338ca">FolioPDF Inc.</p>
    <p style="font-size:7pt; color:#6b7280">123 Document Lane<br>San Francisco, CA 94107</p>
  </div>
  <div class="card">
    <p style="font-size:7pt; color:#9ca3af; text-transform:uppercase; font-weight:bold">Bill To</p>
    <p style="font-weight:bold">Acme Corporation</p>
    <p style="font-size:7pt; color:#6b7280">456 Enterprise Blvd<br>New York, NY 10001</p>
  </div>
</div>
<div class="grid3">
  <div class="date-box"><p style="font-size:6pt; color:#9ca3af">ISSUE DATE</p><p style="font-weight:bold">April 3, 2026</p></div>
  <div class="date-box"><p style="font-size:6pt; color:#9ca3af">DUE DATE</p><p style="font-weight:bold">May 3, 2026</p></div>
  <div class="date-box"><p style="font-size:6pt; color:#9ca3af">TERMS</p><p style="font-weight:bold">Net 30</p></div>
</div>
<div class="table-wrap"><table>
  <thead><tr><th style="text-align:left">Description</th><th>Qty</th><th style="text-align:right">Price</th><th style="text-align:right">Amount</th></tr></thead>
  <tbody>
    <tr><td><strong>Growth Plan</strong><br><span style="font-size:7pt;color:#9ca3af">Monthly subscription</span></td><td style="text-align:center">1</td><td style="text-align:right">$99.00</td><td class="amount">$99.00</td></tr>
    <tr class="alt"><td><strong>API Usage</strong><br><span style="font-size:7pt;color:#9ca3af">23,450 renders</span></td><td style="text-align:center">23,450</td><td style="text-align:right">$0.01</td><td class="amount">$234.50</td></tr>
    <tr><td><strong>Priority Support</strong><br><span style="font-size:7pt;color:#9ca3af">Add-on</span></td><td style="text-align:center">1</td><td style="text-align:right">$49.00</td><td class="amount">$49.00</td></tr>
  </tbody>
</table></div>
<div class="totals"><div class="totals-box">
  <div class="total-row"><span>Subtotal</span><span>$382.50</span></div>
  <div class="total-row"><span>Tax (8%)</span><span>$30.60</span></div>
  <div class="total-due"><span>Total Due</span><span>$413.10</span></div>
</div></div>
<div class="payment">
  <p style="font-size:7pt; color:#9ca3af; text-transform:uppercase; font-weight:bold; margin-bottom:6px">Payment Instructions</p>
  <div class="grid3">
    <div><p style="font-size:7pt;color:#9ca3af">Bank</p><p style="font-weight:600">Silicon Valley Bank</p></div>
    <div><p style="font-size:7pt;color:#9ca3af">Account</p><p style="font-weight:600">**** 4821</p></div>
    <div><p style="font-size:7pt;color:#9ca3af">Routing</p><p style="font-weight:600">**** 0089</p></div>
  </div>
</div>
</body></html>`

// benchReportHTML is a multi-page quarterly report with KPI cards, multiple
// tables, flexbox two-column layout, and a page break.
const benchReportHTML = `<!DOCTYPE html>
<html><head><style>
@page { size: A4; margin: 0 0 24px 0; }
body { font-family: Helvetica; margin: 0; padding: 0; color: #2d3748; font-size: 10pt; }
.header-band { background: linear-gradient(135deg, #0f172a, #4a6fa5); color: white; padding: 28px 2cm 24px; }
.header-band h1 { font-size: 24pt; margin: 0; }
.header-band .sub { font-size: 10pt; color: #94a3b8; }
.body { padding: 24px 2cm 2cm; }
.kpi-grid { display: flex; gap: 14px; margin-bottom: 28px; }
.kpi { flex: 1; border: 1px solid #e2e8f0; border-radius: 6px; padding: 14px; }
.kpi-label { font-size: 7pt; text-transform: uppercase; color: #94a3b8; margin-bottom: 6px; }
.kpi-value { font-size: 22pt; font-weight: 700; color: #0f172a; }
.kpi-change { font-size: 8.5pt; margin-top: 4px; }
.up { color: #059669; }
h2 { font-size: 12pt; font-weight: 700; color: #0f172a; margin: 24px 0 10px; padding-bottom: 6px; border-bottom: 1px solid #e2e8f0; }
table { width: 100%; border-collapse: collapse; margin-bottom: 20px; }
th { padding: 7px 10px; text-align: left; font-size: 7.5pt; text-transform: uppercase; color: #64748b; border-bottom: 2px solid #e2e8f0; }
td { padding: 7px 10px; border-bottom: 1px solid #f1f5f9; font-size: 9pt; }
.r { text-align: right; }
.two-col { display: flex; gap: 24px; }
.two-col > div { flex: 1; }
.callout { padding: 12px 16px; background-color: #fffbeb; border-left: 3px solid #f59e0b; margin: 20px 0; font-size: 9pt; }
.page-break { break-before: page; }
.team-grid { display: flex; flex-wrap: wrap; gap: 16px; }
.team-card { flex: 1; min-width: 200px; border: 1px solid #e2e8f0; border-radius: 6px; padding: 14px; }
</style></head>
<body>
<div class="header-band"><h1>Quarterly Report</h1><div class="sub">Q4 2026</div></div>
<div class="body">
  <div class="kpi-grid">
    <div class="kpi"><div class="kpi-label">Revenue</div><div class="kpi-value">$28.3M</div><div class="kpi-change up">+22% YoY</div></div>
    <div class="kpi"><div class="kpi-label">Net Income</div><div class="kpi-value">$6.1M</div><div class="kpi-change up">+18% YoY</div></div>
    <div class="kpi"><div class="kpi-label">Margin</div><div class="kpi-value">30.0%</div><div class="kpi-change up">+3.7pp</div></div>
    <div class="kpi"><div class="kpi-label">Retention</div><div class="kpi-value">97.2%</div><div class="kpi-change">-0.3%</div></div>
  </div>
  <div class="two-col">
    <div>
      <h2>Revenue by Segment</h2>
      <table><thead><tr><th>Segment</th><th class="r">Revenue</th><th class="r">%</th></tr></thead>
      <tbody>
        <tr><td>Advisory</td><td class="r">$14.2M</td><td class="r">50%</td></tr>
        <tr><td>Asset Mgmt</td><td class="r">$8.5M</td><td class="r">30%</td></tr>
        <tr><td>Research</td><td class="r">$4.2M</td><td class="r">15%</td></tr>
        <tr><td>Other</td><td class="r">$1.4M</td><td class="r">5%</td></tr>
      </tbody></table>
    </div>
    <div>
      <h2>Regional Performance</h2>
      <table><thead><tr><th>Region</th><th class="r">Revenue</th><th class="r">Growth</th></tr></thead>
      <tbody>
        <tr><td>North America</td><td class="r">$18.4M</td><td class="r up">+24.1%</td></tr>
        <tr><td>Europe</td><td class="r">$6.2M</td><td class="r up">+19.3%</td></tr>
        <tr><td>Asia-Pacific</td><td class="r">$2.8M</td><td class="r up">+31.7%</td></tr>
        <tr><td>Latin America</td><td class="r">$0.9M</td><td class="r">-4.2%</td></tr>
      </tbody></table>
    </div>
  </div>
  <h2>Income Statement</h2>
  <table><thead><tr><th>Metric</th><th class="r">Q4 2026</th><th class="r">Q3 2026</th><th class="r">Q4 2025</th><th class="r">YoY</th></tr></thead>
  <tbody>
    <tr><td>Total Revenue</td><td class="r">$28.3M</td><td class="r">$25.1M</td><td class="r">$23.2M</td><td class="r up">+22.0%</td></tr>
    <tr><td>Cost of Revenue</td><td class="r">$11.3M</td><td class="r">$10.4M</td><td class="r">$9.9M</td><td class="r">+14.1%</td></tr>
    <tr><td style="font-weight:700">Gross Profit</td><td class="r" style="font-weight:700">$17.0M</td><td class="r">$14.7M</td><td class="r">$13.3M</td><td class="r up">+27.8%</td></tr>
    <tr><td>Operating Expenses</td><td class="r">$8.5M</td><td class="r">$8.0M</td><td class="r">$7.2M</td><td class="r">+18.1%</td></tr>
    <tr><td style="font-weight:700">Net Income</td><td class="r" style="font-weight:700">$6.1M</td><td class="r">$4.9M</td><td class="r">$5.2M</td><td class="r up">+17.3%</td></tr>
  </tbody></table>
  <div class="callout"><strong>Outlook:</strong> Technology integration reduced operational costs by 15%.</div>
  <div class="page-break"></div>
  <h2>Leadership Team</h2>
  <div class="team-grid">
    <div class="team-card"><div style="font-weight:700">Sarah Chen</div><div style="font-size:8pt;color:#0d9488;text-transform:uppercase">CFO</div><div style="font-size:8.5pt;color:#64748b">20+ years in investment banking. MBA from Wharton.</div></div>
    <div class="team-card"><div style="font-weight:700">Michael Torres</div><div style="font-size:8pt;color:#0d9488;text-transform:uppercase">Managing Director</div><div style="font-size:8.5pt;color:#64748b">Former McKinsey partner. CFA charterholder.</div></div>
    <div class="team-card"><div style="font-weight:700">Priya Patel</div><div style="font-size:8pt;color:#0d9488;text-transform:uppercase">Head of Research</div><div style="font-size:8.5pt;color:#64748b">PhD Economics, MIT.</div></div>
  </div>
  <h2>Key Milestones</h2>
  <ul>
    <li><strong>January:</strong> Launched digital asset custody platform.</li>
    <li><strong>March:</strong> Opened Singapore office.</li>
    <li><strong>May:</strong> Signed partnership with Deutsche Bank.</li>
    <li><strong>June:</strong> Named "Top Advisory Firm" by Financial Times.</li>
  </ul>
</div>
</body></html>`

// buildTableHeavyHTML generates an HTML document with a styled table of n rows
// and 5 columns, with alternating row backgrounds and right-aligned numbers.
func buildTableHeavyHTML(rows int) string {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html>
<html><head><style>
body { font-family: Helvetica; font-size: 9pt; }
h1 { font-size: 14pt; color: #1e293b; }
table { width: 100%; border-collapse: collapse; }
th { background: #1e293b; color: white; padding: 6px 10px; font-size: 7pt; text-transform: uppercase; }
td { padding: 6px 10px; border-bottom: 1px solid #e2e8f0; font-size: 8pt; }
.alt { background: #f8fafc; }
.r { text-align: right; }
</style></head>
<body style="padding: 24px">
<h1>Transaction Ledger</h1>
<table><thead><tr><th>ID</th><th>Description</th><th>Category</th><th class="r">Amount</th><th class="r">Balance</th></tr></thead><tbody>`)
	for i := range rows {
		cls := ""
		if i%2 == 1 {
			cls = ` class="alt"`
		}
		fmt.Fprintf(&sb,
			`<tr%s><td>TXN-%04d</td><td>Payment for service item %d</td><td>Operations</td><td class="r">$%d.00</td><td class="r">$%d.00</td></tr>`,
			cls, i+1, i+1, (i+1)*47%500+50, 50000-(i+1)*47%500)
	}
	sb.WriteString(`</tbody></table></body></html>`)
	return sb.String()
}

func BenchmarkHTMLInvoice(b *testing.B) {
	opts := &html.Options{
		PageWidth:  PageSizeLetter.Width,
		PageHeight: PageSizeLetter.Height,
	}
	for range b.N {
		doc := NewDocument(PageSizeLetter)
		_ = doc.AddHTML(benchInvoiceHTML, opts)
		_, _ = doc.WriteTo(io.Discard)
	}
}

func BenchmarkHTMLTableHeavy100(b *testing.B) {
	tableHTML := buildTableHeavyHTML(100)
	opts := &html.Options{
		PageWidth:  PageSizeLetter.Width,
		PageHeight: PageSizeLetter.Height,
	}
	b.ResetTimer()
	for range b.N {
		doc := NewDocument(PageSizeLetter)
		_ = doc.AddHTML(tableHTML, opts)
		_, _ = doc.WriteTo(io.Discard)
	}
}

func BenchmarkHTMLReport(b *testing.B) {
	opts := &html.Options{
		PageWidth:  PageSizeA4.Width,
		PageHeight: PageSizeA4.Height,
	}
	for range b.N {
		doc := NewDocument(PageSizeA4)
		_ = doc.AddHTML(benchReportHTML, opts)
		_, _ = doc.WriteTo(io.Discard)
	}
}
