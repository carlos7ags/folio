// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package zugferd

import "time"

// sampleInvoice returns the invoice used across tests. Its header
// values (number, date, parties, totals) mirror the ones the old
// examples/zugferd/main.go example hand-rolled into a CII XML string
// literal, so xml_test.go can assert semantic equivalence against
// that literal. Lines and TaxTotals are populated (the old example
// had neither) because ProfileBasic requires them; the sums are
// chosen to stay consistent with the shared totals.
func sampleInvoice() *Invoice {
	return &Invoice{
		Number:    "2024-001",
		IssueDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		Currency:  "EUR",
		Seller:    Party{Name: "ACME Corp", VATID: "DE123456789"},
		Buyer:     Party{Name: "Example GmbH", VATID: "DE987654321"},
		Lines: []LineItem{
			{Description: "Widget A - Standard", Quantity: "10", UnitPrice: NewAmount(5, 0), LineTotal: NewAmount(50, 0), TaxRatePercent: "19.00"},
			{Description: "Widget B - Premium", Quantity: "3", UnitPrice: NewAmount(12, 50), LineTotal: NewAmount(37, 50), TaxRatePercent: "19.00"},
			{Description: "Consulting Service", Quantity: "1", UnitPrice: NewAmount(250, 0), LineTotal: NewAmount(250, 0), TaxRatePercent: "19.00"},
		},
		TaxTotals: []TaxBreakdown{
			{RatePercent: "19.00", TaxableAmount: NewAmount(337, 50), TaxAmount: NewAmount(64, 13)},
		},
		Totals: MonetarySummation{
			LineTotal:     NewAmount(337, 50),
			TaxBasisTotal: NewAmount(337, 50),
			TaxTotal:      NewAmount(64, 13),
			GrandTotal:    NewAmount(401, 63),
		},
		PaymentTerms: "30 days net",
	}
}
