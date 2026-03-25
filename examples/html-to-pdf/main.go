// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// HTML-to-PDF demonstrates converting a rich HTML document with CSS
// into a polished, multi-page PDF using the html.ConvertFull API.
//
// CSS features exercised:
//   - @page rules with custom margins
//   - CSS custom properties (variables with var())
//   - Linear gradient header background
//   - Flexbox layout (row, column, justify-content, gap)
//   - Tables with border-collapse and styled headers
//   - Background-color on headers and cards
//   - Border-radius on KPI cards and team cards
//   - Font sizing, weight, text-transform, text-align
//   - Page breaks (break-before: page)
//   - Progress-bar pattern (nested divs with percentage width)
//   - Callout boxes with border-left accent
//   - Multi-column layouts via flexbox
//   - Links (<a href>) with clickable PDF annotations
//   - Opacity for watermark-style text
//   - HTML metadata extraction (<title>, <meta name="author">)
//
// Usage:
//
//	go run ./examples/html-to-pdf
package main

import (
	"fmt"
	"os"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/layout"
)

const reportHTML = `<!DOCTYPE html>
<html>
<head>
  <title>Q4 2026 Quarterly Report</title>
  <meta name="author" content="Apex Capital Partners">
  <style>
    /* --- CSS Variables --- */
    :root {
      --brand: #0f172a;
      --accent: #0d9488;
      --muted: #94a3b8;
      --border: #e2e8f0;
    }

    @page {
      size: A4;
      margin: 0 0 24px 0;
      @bottom-center { content: "Page " counter(page) " of " counter(pages); }
    }
    @page :first { @bottom-center { content: ""; } }
    body { font-family: Helvetica, Arial, sans-serif; margin: 0; padding: 0; color: #2d3748; font-size: 10pt; }

    /* --- Header band with gradient --- */
    .header-band { background: linear-gradient(135deg, #0f172a, #4a6fa5); color: white; padding: 28px 2cm 24px; }
    .header-band h1 { font-size: 24pt; margin: 0 0 2px; font-weight: 700; }
    .header-band .sub { font-size: 10pt; color: var(--muted); }

    .body { padding: 24px 2cm 2cm; }

    /* --- KPI cards with flexbox + border-radius --- */
    .kpi-grid { display: flex; gap: 14px; margin-bottom: 28px; }
    .kpi { flex: 1; border: 1px solid var(--border); border-radius: 6px; padding: 14px; }
    .kpi-label { font-size: 7pt; text-transform: uppercase; color: var(--muted); margin-bottom: 6px; }
    .kpi-value { font-size: 22pt; font-weight: 700; color: var(--brand); }
    .kpi-change { font-size: 8.5pt; margin-top: 4px; }
    .up { color: #059669; }
    .down { color: #dc2626; }

    /* --- Links --- */
    a { color: var(--accent); text-decoration: underline; }

    /* --- Section headings --- */
    h2 { font-size: 12pt; font-weight: 700; color: var(--brand); margin: 24px 0 10px; padding-bottom: 6px; border-bottom: 1px solid var(--border); }

    /* --- Tables --- */
    table { width: 100%; border-collapse: collapse; margin-bottom: 20px; }
    th { padding: 7px 10px; text-align: left; font-size: 7.5pt; text-transform: uppercase; color: #64748b; border-bottom: 2px solid #e2e8f0; }
    td { padding: 7px 10px; border-bottom: 1px solid #f1f5f9; font-size: 9pt; }
    .r { text-align: right; }

    /* --- Progress bars --- */
    .progress-cell { width: 100px; }
    .progress-bar { height: 6px; background-color: #e2e8f0; width: 100%; }
    .progress-fill { height: 6px; background-color: #0f172a; }

    /* --- Two-column flex layout --- */
    .two-col { display: flex; gap: 24px; }
    .two-col > div { flex: 1; }

    /* --- Callout box --- */
    .callout { padding: 12px 16px; background-color: #fffbeb; border-left: 3px solid #f59e0b; margin: 20px 0; font-size: 9pt; }
    .callout strong { color: #0f172a; }

    /* --- Footer --- */
    .footer { margin-top: 28px; padding-top: 12px; border-top: 1px solid #e2e8f0; font-size: 7.5pt; color: #94a3b8; text-align: center; }

    /* --- Page 2 styles --- */
    .page-break { break-before: page; }
    .team-grid { display: flex; flex-wrap: wrap; gap: 16px; }
    .team-card { flex: 1; min-width: 200px; border: 1px solid var(--border); border-radius: 6px; padding: 14px; }
    .team-name { font-weight: 700; color: var(--brand); font-size: 10pt; }
    .team-role { font-size: 8pt; color: var(--accent); text-transform: uppercase; margin-bottom: 6px; }
    .team-bio { font-size: 8.5pt; color: #64748b; line-height: 1.5; }

    .milestone-list { padding-left: 18px; }
    .milestone-list li { margin-bottom: 6px; font-size: 9pt; color: #475569; }
    .milestone-list li strong { color: var(--brand); }

    /* --- Opacity for confidential watermark --- */
    .confidential { opacity: 0.4; font-size: 7pt; text-transform: uppercase; text-align: center; margin-top: 12px; }
  </style>
</head>
<body>
  <!-- ============ PAGE 1: Dashboard ============ -->
  <div class="header-band">
    <h1>Quarterly Report</h1>
    <div class="sub">Q4 2026 &mdash; Apex Capital Partners &mdash; Confidential</div>
  </div>

  <div class="body">
    <!-- KPI cards -->
    <div class="kpi-grid">
      <div class="kpi">
        <div class="kpi-label">Revenue</div>
        <div class="kpi-value">$28.3M</div>
        <div class="kpi-change up">+22% YoY</div>
      </div>
      <div class="kpi">
        <div class="kpi-label">Net Income</div>
        <div class="kpi-value">$6.1M</div>
        <div class="kpi-change up">+18% YoY</div>
      </div>
      <div class="kpi">
        <div class="kpi-label">Operating Margin</div>
        <div class="kpi-value">30.0%</div>
        <div class="kpi-change up">+3.7pp YoY</div>
      </div>
      <div class="kpi">
        <div class="kpi-label">Client Retention</div>
        <div class="kpi-value">97.2%</div>
        <div class="kpi-change down">-0.3% QoQ</div>
      </div>
    </div>

    <!-- Two-column tables -->
    <div class="two-col">
      <div>
        <h2>Revenue by Segment</h2>
        <table>
          <thead><tr><th>Segment</th><th class="r">Revenue</th><th class="r">%</th><th class="progress-cell"></th></tr></thead>
          <tbody>
            <tr><td>Advisory</td><td class="r">$14.2M</td><td class="r">50%</td><td class="progress-cell"><div class="progress-bar"><div class="progress-fill" style="width:100%"></div></div></td></tr>
            <tr><td>Asset Mgmt</td><td class="r">$8.5M</td><td class="r">30%</td><td class="progress-cell"><div class="progress-bar"><div class="progress-fill" style="width:60%"></div></div></td></tr>
            <tr><td>Research</td><td class="r">$4.2M</td><td class="r">15%</td><td class="progress-cell"><div class="progress-bar"><div class="progress-fill" style="width:30%"></div></div></td></tr>
            <tr><td>Other</td><td class="r">$1.4M</td><td class="r">5%</td><td class="progress-cell"><div class="progress-bar"><div class="progress-fill" style="width:10%"></div></div></td></tr>
          </tbody>
        </table>
      </div>
      <div>
        <h2>Regional Performance</h2>
        <table>
          <thead><tr><th>Region</th><th class="r">Revenue</th><th class="r">Growth</th></tr></thead>
          <tbody>
            <tr><td>North America</td><td class="r">$18.4M</td><td class="r up">+24.1%</td></tr>
            <tr><td>Europe</td><td class="r">$6.2M</td><td class="r up">+19.3%</td></tr>
            <tr><td>Asia-Pacific</td><td class="r">$2.8M</td><td class="r up">+31.7%</td></tr>
            <tr><td>Latin America</td><td class="r">$0.9M</td><td class="r down">-4.2%</td></tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Income statement -->
    <h2>Income Statement</h2>
    <table>
      <thead><tr><th>Metric</th><th class="r">Q4 2026</th><th class="r">Q3 2026</th><th class="r">Q4 2025</th><th class="r">YoY</th></tr></thead>
      <tbody>
        <tr><td>Total Revenue</td><td class="r">$28.3M</td><td class="r">$25.1M</td><td class="r">$23.2M</td><td class="r up">+22.0%</td></tr>
        <tr><td>Cost of Revenue</td><td class="r">$11.3M</td><td class="r">$10.4M</td><td class="r">$9.9M</td><td class="r">+14.1%</td></tr>
        <tr><td style="font-weight:700">Gross Profit</td><td class="r" style="font-weight:700">$17.0M</td><td class="r">$14.7M</td><td class="r">$13.3M</td><td class="r up">+27.8%</td></tr>
        <tr><td>Operating Expenses</td><td class="r">$8.5M</td><td class="r">$8.0M</td><td class="r">$7.2M</td><td class="r">+18.1%</td></tr>
        <tr><td style="font-weight:700">Net Income</td><td class="r" style="font-weight:700">$6.1M</td><td class="r">$4.9M</td><td class="r">$5.2M</td><td class="r up">+17.3%</td></tr>
      </tbody>
    </table>

    <div class="callout">
      <strong>Outlook:</strong> Technology integration reduced operational costs by 15%.
      Expanding into sustainable finance, digital asset custody, and cross-border
      payments in 2027 with projected $450M addressable market.
    </div>

    <!-- ============ PAGE 2: Team & Milestones ============ -->
    <div class="page-break"></div>

    <h2>Leadership Team</h2>
    <div class="team-grid">
      <div class="team-card">
        <div class="team-name">Sarah Chen</div>
        <div class="team-role">Chief Financial Officer</div>
        <div class="team-bio">20+ years in investment banking. Previously VP at Goldman Sachs. MBA from Wharton.</div>
      </div>
      <div class="team-card">
        <div class="team-name">Michael Torres</div>
        <div class="team-role">Managing Director</div>
        <div class="team-bio">Leads client advisory. Former McKinsey partner. CFA charterholder.</div>
      </div>
      <div class="team-card">
        <div class="team-name">Priya Patel</div>
        <div class="team-role">Head of Research</div>
        <div class="team-bio">PhD Economics, MIT. Published author on emerging market strategy.</div>
      </div>
    </div>

    <h2>Key Milestones</h2>
    <ul class="milestone-list">
      <li><strong>January:</strong> Launched digital asset custody platform with $2.1B AUM onboarded in first month.</li>
      <li><strong>February:</strong> Completed Meridian Dynamics acquisition due diligence. Deal closed at $340M.</li>
      <li><strong>March:</strong> Opened Singapore office. Hired 12 analysts for Asia-Pacific expansion.</li>
      <li><strong>April:</strong> Published ESG Impact Report. Achieved 97% client satisfaction in annual survey.</li>
      <li><strong>May:</strong> Signed partnership with Deutsche Bank for cross-border payment infrastructure.</li>
      <li><strong>June:</strong> Named "Top Advisory Firm" by Financial Times for third consecutive year.</li>
    </ul>

    <div class="callout">
      <strong>Next Quarter:</strong> Board approval pending for Series C fund ($500M target).
      Q1 2027 earnings call scheduled for April 15.
    </div>

    <div class="footer">
      Generated March 24, 2026 &mdash; <a href="https://github.com/carlos7ags/folio">Built with Folio</a>
    </div>

    <div class="confidential">Confidential &mdash; Do Not Distribute</div>
  </div>
</body>
</html>`

func main() {
	result, err := html.ConvertFull(reportHTML, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "convert:", err)
		os.Exit(1)
	}

	doc := document.NewDocument(document.PageSizeA4)
	doc.SetMargins(layout.Margins{Top: 0, Right: 0, Bottom: 0, Left: 0})
	doc.Info.Title = "Q4 2026 Quarterly Report"
	doc.Info.Author = "Apex Capital Partners"
	doc.SetAutoBookmarks(true)

	// Apply @page configuration (margins, margin boxes for page numbers).
	if pc := result.PageConfig; pc != nil {
		if pc.HasMargins {
			doc.SetMargins(layout.Margins{
				Top: pc.MarginTop, Right: pc.MarginRight,
				Bottom: pc.MarginBottom, Left: pc.MarginLeft,
			})
		}
		if len(pc.MarginBoxes) > 0 {
			boxes := make(map[string]layout.MarginBox)
			for name, mbc := range pc.MarginBoxes {
				boxes[name] = layout.MarginBox{Content: mbc.Content, FontSize: mbc.FontSize, Color: mbc.Color}
			}
			doc.SetMarginBoxes(boxes)
		}
		if pc.First != nil && len(pc.First.MarginBoxes) > 0 {
			boxes := make(map[string]layout.MarginBox)
			for name, mbc := range pc.First.MarginBoxes {
				boxes[name] = layout.MarginBox{Content: mbc.Content, FontSize: mbc.FontSize, Color: mbc.Color}
			}
			doc.SetFirstMarginBoxes(boxes)
		}
	}

	for _, e := range result.Elements {
		doc.Add(e)
	}

	// Apply metadata from <title> and <meta> tags.
	if result.Metadata.Title != "" {
		doc.Info.Title = result.Metadata.Title
	}
	if result.Metadata.Author != "" {
		doc.Info.Author = result.Metadata.Author
	}

	if err := doc.Save("report.pdf"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Created report.pdf")
}
