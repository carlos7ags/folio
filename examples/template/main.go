// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Example: render a Go html/template to PDF using the tmpl package.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/carlos7ags/folio/tmpl"
)

const invoiceTemplate = `<!DOCTYPE html>
<html>
<head><style>
  body { font-family: sans-serif; padding: 40px; font-size: 11px; }
  h1 { font-size: 22px; margin-bottom: 8px; }
  table { width: 100%; border-collapse: collapse; margin-top: 16px; }
  th, td { border: 1px solid #ccc; padding: 6px 10px; text-align: left; }
  th { background: #f5f5f5; font-weight: bold; }
  .total { text-align: right; font-size: 14px; font-weight: bold; margin-top: 16px; }
</style></head>
<body>
  <h1>Invoice #{{.Number}}</h1>
  <p>Date: {{.Date}}</p>
  <p>Bill to: <strong>{{.Customer}}</strong></p>

  <table>
    <tr><th>Item</th><th>Qty</th><th>Unit Price</th><th>Amount</th></tr>
    {{range .Items}}
    <tr>
      <td>{{.Name}}</td>
      <td>{{.Qty}}</td>
      <td>${{printf "%.2f" .Price}}</td>
      <td>${{printf "%.2f" .Total}}</td>
    </tr>
    {{end}}
  </table>

  <p class="total">Total: ${{printf "%.2f" .Total}}</p>

  {{if .Notes}}
  <p><em>Notes: {{.Notes}}</em></p>
  {{end}}
</body>
</html>`

type LineItem struct {
	Name  string
	Qty   int
	Price float64
	Total float64
}

type Invoice struct {
	Number   string
	Date     string
	Customer string
	Items    []LineItem
	Total    float64
	Notes    string
}

func main() {
	inv := Invoice{
		Number:   "1042",
		Date:     time.Now().Format("January 2, 2006"),
		Customer: "Globex Corporation",
		Items: []LineItem{
			{"Consulting (40 hrs)", 1, 4800.00, 4800.00},
			{"Platform license", 12, 99.00, 1188.00},
			{"Support add-on", 1, 500.00, 500.00},
		},
		Total: 6488.00,
		Notes: "Payment due within 30 days.",
	}

	doc, err := tmpl.RenderDocument(invoiceTemplate, inv, nil)
	if err != nil {
		log.Fatal(err)
	}
	if err := doc.Save("invoice.pdf"); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Created invoice.pdf")
}
