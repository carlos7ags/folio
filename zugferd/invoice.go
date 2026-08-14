// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package zugferd generates Factur-X/ZUGFeRD hybrid e-invoices: a
// typed [Invoice] renders to UN/CEFACT Cross-Industry Invoice (CII)
// XML and attaches to a [github.com/carlos7ags/folio/document.Document]
// as a PDF/A-3 embedded file, using the primitives in that package
// ([document.FileAttachment], [document.PdfAConfig]).
//
// Only the MINIMUM and BASIC Factur-X 1.0 profiles are supported.
// EN 16931 (COMFORT) and EXTENDED are not implemented.
package zugferd

import (
	"fmt"
	"regexp"
	"time"
)

// Profile selects the Factur-X conformance profile, which determines
// the guideline URN written into the XML and which Invoice fields
// are required.
type Profile int

const (
	// ProfileMinimum carries only header-level identification and
	// totals; no line items or tax breakdown are required.
	ProfileMinimum Profile = iota

	// ProfileBasic adds line items and a tax breakdown to MINIMUM.
	ProfileBasic
)

// String returns the profile's conformance-level name as used in the
// Factur-X XMP extension schema (e.g. "MINIMUM", "BASIC").
func (p Profile) String() string {
	switch p {
	case ProfileMinimum:
		return "MINIMUM"
	case ProfileBasic:
		return "BASIC"
	default:
		return "UNKNOWN"
	}
}

// guidelineURN returns the Factur-X 1.0 guideline URN written into
// ram:GuidelineSpecifiedDocumentContextParameter/ram:ID for the profile.
func (p Profile) guidelineURN() (string, error) {
	switch p {
	case ProfileMinimum:
		return "urn:factur-x.eu:1p0:minimum", nil
	case ProfileBasic:
		return "urn:factur-x.eu:1p0:basic", nil
	default:
		return "", fmt.Errorf("zugferd: unknown profile %d", int(p))
	}
}

// Party is a seller or buyer trade party.
type Party struct {
	// Name is the party's registered name. Required.
	Name string

	// VATID is the party's VAT identification number (e.g.
	// "DE123456789"). Optional; omitted from the XML when empty.
	VATID string
}

// LineItem is one invoice line (CII ram:IncludedSupplyChainTradeLineItem).
// Required for ProfileBasic; ProfileMinimum ignores Lines entirely.
type LineItem struct {
	// ID is the line number (e.g. "1"). If empty, XML assigns the
	// 1-based position in Lines.
	ID string

	// Description is the billed product or service name. Required.
	Description string

	// Quantity is the billed quantity as a plain decimal string (e.g.
	// "10", "2.5"). Required. Not validated arithmetically against
	// UnitPrice/LineTotal — see package-level limitations.
	Quantity string

	// UnitCode is the UN/ECE Recommendation 20 unit code (e.g. "C62"
	// for "piece", "HUR" for hour). Defaults to "C62" when empty.
	UnitCode string

	// UnitPrice is the net price per unit.
	UnitPrice Amount

	// LineTotal is the net line total (Quantity x UnitPrice,
	// pre-computed by the caller — not derived). Required.
	LineTotal Amount

	// TaxCategoryCode is the UNTDID 5305 category code (e.g. "S" for
	// standard rate). Defaults to "S" when empty.
	TaxCategoryCode string

	// TaxRatePercent is the VAT rate as a decimal string (e.g. "19.00").
	// Required.
	TaxRatePercent string
}

// TaxBreakdown is one entry in the header tax summary (CII
// ram:ApplicableTradeTax), grouping line items by rate and category.
// Required (at least one entry) for ProfileBasic.
type TaxBreakdown struct {
	// CategoryCode is the UNTDID 5305 category code (e.g. "S").
	// Defaults to "S" when empty.
	CategoryCode string

	// RatePercent is the VAT rate as a decimal string (e.g. "19.00").
	RatePercent string

	// TaxableAmount is the taxable basis for this rate/category.
	TaxableAmount Amount

	// TaxAmount is the tax due for this rate/category.
	TaxAmount Amount
}

// MonetarySummation is the header-level totals block (CII
// ram:SpecifiedTradeSettlementHeaderMonetarySummation).
type MonetarySummation struct {
	// LineTotal is the sum of all line net totals. Only meaningful
	// (and emitted) when Lines is non-empty.
	LineTotal Amount

	// TaxBasisTotal is the total taxable amount (sum of TaxTotals'
	// TaxableAmount, or the invoice net total for MINIMUM).
	TaxBasisTotal Amount

	// TaxTotal is the total tax due.
	TaxTotal Amount

	// GrandTotal is TaxBasisTotal + TaxTotal.
	GrandTotal Amount

	// DuePayableAmount is the amount still owed. Defaults to
	// GrandTotal when zero (no prior payments/prepayments modeled).
	DuePayableAmount Amount
}

// Invoice is a Factur-X/ZUGFeRD invoice: enough structured data to
// render EN 16931 CII XML and attach it to a PDF/A-3 document via
// [Invoice.Attach]. The rendered PDF content (the human-readable
// invoice) is the caller's responsibility — Invoice only produces the
// machine-readable counterpart.
type Invoice struct {
	// Number is the invoice number (CII ExchangedDocument/ID). Required.
	Number string

	// TypeCode is the UNTDID 1001 document type code. Defaults to
	// "380" (commercial invoice) when empty.
	TypeCode string

	// IssueDate is the invoice issue date. Required (non-zero).
	IssueDate time.Time

	// Currency is the ISO 4217 currency code (e.g. "EUR"). Required.
	Currency string

	// Seller and Buyer are the trade parties. Both Name fields are required.
	Seller, Buyer Party

	// Lines is the invoice line items. Empty is only valid for
	// ProfileMinimum; ProfileBasic requires at least one line.
	Lines []LineItem

	// TaxTotals is the header tax breakdown. Empty is only valid for
	// ProfileMinimum; ProfileBasic requires at least one entry, and
	// the sum of TaxAmount must equal Totals.TaxTotal.
	TaxTotals []TaxBreakdown

	// Totals is the header monetary summation. Required; TaxBasisTotal
	// + TaxTotal must equal GrandTotal.
	Totals MonetarySummation

	// PaymentTerms is a free-text payment terms description (e.g.
	// "30 days net"). Optional.
	PaymentTerms string
}

// ValidationError names the invoice field and profile that failed
// validation.
type ValidationError struct {
	Profile Profile
	Field   string
	Reason  string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("zugferd: profile %s: %s: %s", e.Profile, e.Field, e.Reason)
}

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// Validate checks the invoice against the field-presence and
// arithmetic-consistency rules for the given profile. It does not
// validate Quantity/UnitPrice/LineTotal arithmetic (see package docs)
// or perform full EN 16931 Schematron-equivalent validation.
func (inv *Invoice) Validate(p Profile) error {
	if inv.Number == "" {
		return &ValidationError{p, "Number", "required"}
	}
	if inv.IssueDate.IsZero() {
		return &ValidationError{p, "IssueDate", "required"}
	}
	if !currencyPattern.MatchString(inv.Currency) {
		return &ValidationError{p, "Currency", "required, must be a 3-letter ISO 4217 code"}
	}
	if inv.Seller.Name == "" {
		return &ValidationError{p, "Seller.Name", "required"}
	}
	if inv.Buyer.Name == "" {
		return &ValidationError{p, "Buyer.Name", "required"}
	}

	if p == ProfileBasic {
		if len(inv.Lines) == 0 {
			return &ValidationError{p, "Lines", "at least one line item is required for profile BASIC"}
		}
		for i, line := range inv.Lines {
			if line.Description == "" {
				return &ValidationError{p, fmt.Sprintf("Lines[%d].Description", i), "required"}
			}
			if line.Quantity == "" {
				return &ValidationError{p, fmt.Sprintf("Lines[%d].Quantity", i), "required"}
			}
			if line.TaxRatePercent == "" {
				return &ValidationError{p, fmt.Sprintf("Lines[%d].TaxRatePercent", i), "required"}
			}
		}
		if len(inv.TaxTotals) == 0 {
			return &ValidationError{p, "TaxTotals", "at least one entry is required for profile BASIC"}
		}
		var taxSum Amount
		for _, tt := range inv.TaxTotals {
			if tt.RatePercent == "" {
				return &ValidationError{p, "TaxTotals[].RatePercent", "required"}
			}
			sum, ok := taxSum.AddChecked(tt.TaxAmount)
			if !ok {
				return &ValidationError{p, "TaxTotals", "sum of TaxAmount overflows"}
			}
			taxSum = sum
		}
		if taxSum != inv.Totals.TaxTotal {
			return &ValidationError{p, "TaxTotals", "sum of TaxAmount must equal Totals.TaxTotal"}
		}
	}

	sum, ok := inv.Totals.TaxBasisTotal.AddChecked(inv.Totals.TaxTotal)
	if !ok {
		return &ValidationError{p, "Totals", "TaxBasisTotal + TaxTotal overflows"}
	}
	if sum != inv.Totals.GrandTotal {
		return &ValidationError{p, "Totals", "TaxBasisTotal + TaxTotal must equal GrandTotal"}
	}

	return nil
}

// typeCode returns inv.TypeCode, defaulting to "380" (commercial invoice).
func (inv *Invoice) typeCode() string {
	if inv.TypeCode == "" {
		return "380"
	}
	return inv.TypeCode
}

// duePayable returns Totals.DuePayableAmount, defaulting to GrandTotal.
func (inv *Invoice) duePayable() Amount {
	if inv.Totals.DuePayableAmount == 0 {
		return inv.Totals.GrandTotal
	}
	return inv.Totals.DuePayableAmount
}
