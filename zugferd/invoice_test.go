// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package zugferd

import (
	"errors"
	"testing"
	"time"
)

func TestValidateSampleInvoiceOK(t *testing.T) {
	if err := sampleInvoice().Validate(ProfileBasic); err != nil {
		t.Fatalf("Validate(BASIC) = %v, want nil", err)
	}
	if err := sampleInvoice().Validate(ProfileMinimum); err != nil {
		t.Fatalf("Validate(MINIMUM) = %v, want nil", err)
	}
}

func TestValidateMinimumAllowsEmptyLines(t *testing.T) {
	inv := sampleInvoice()
	inv.Lines = nil
	inv.TaxTotals = nil
	if err := inv.Validate(ProfileMinimum); err != nil {
		t.Errorf("ProfileMinimum with no Lines/TaxTotals: Validate() = %v, want nil", err)
	}
}

func TestValidateBasicRejectsEmptyLines(t *testing.T) {
	inv := sampleInvoice()
	inv.Lines = nil
	err := inv.Validate(ProfileBasic)
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("Validate() = %v, want *ValidationError", err)
	}
	if verr.Field != "Lines" {
		t.Errorf("ValidationError.Field = %q, want %q", verr.Field, "Lines")
	}
}

func TestValidateMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Invoice)
		wantField string
	}{
		{"number", func(inv *Invoice) { inv.Number = "" }, "Number"},
		{"issue date", func(inv *Invoice) { inv.IssueDate = time.Time{} }, "IssueDate"},
		{"currency empty", func(inv *Invoice) { inv.Currency = "" }, "Currency"},
		{"currency lowercase", func(inv *Invoice) { inv.Currency = "eur" }, "Currency"},
		{"currency too long", func(inv *Invoice) { inv.Currency = "EURO" }, "Currency"},
		{"seller name", func(inv *Invoice) { inv.Seller.Name = "" }, "Seller.Name"},
		{"buyer name", func(inv *Invoice) { inv.Buyer.Name = "" }, "Buyer.Name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := sampleInvoice()
			tt.mutate(inv)
			err := inv.Validate(ProfileBasic)
			var verr *ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("Validate() = %v, want *ValidationError", err)
			}
			if verr.Field != tt.wantField {
				t.Errorf("ValidationError.Field = %q, want %q", verr.Field, tt.wantField)
			}
		})
	}
}

func TestValidateBasicLineFieldsRequired(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*LineItem)
	}{
		{"description", func(l *LineItem) { l.Description = "" }},
		{"quantity", func(l *LineItem) { l.Quantity = "" }},
		{"tax rate", func(l *LineItem) { l.TaxRatePercent = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := sampleInvoice()
			tt.mutate(&inv.Lines[0])
			err := inv.Validate(ProfileBasic)
			var verr *ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("Validate() = %v, want *ValidationError", err)
			}
			if verr.Profile != ProfileBasic {
				t.Errorf("ValidationError.Profile = %v, want ProfileBasic", verr.Profile)
			}
		})
	}
}

func TestValidateBasicRequiresTaxTotals(t *testing.T) {
	inv := sampleInvoice()
	inv.TaxTotals = nil
	err := inv.Validate(ProfileBasic)
	var verr *ValidationError
	if !errors.As(err, &verr) || verr.Field != "TaxTotals" {
		t.Fatalf("Validate() = %v, want *ValidationError on TaxTotals", err)
	}
}

func TestValidateTaxTotalMismatch(t *testing.T) {
	inv := sampleInvoice()
	inv.TaxTotals[0].TaxAmount = NewAmount(1, 0) // no longer sums to Totals.TaxTotal
	err := inv.Validate(ProfileBasic)
	var verr *ValidationError
	if !errors.As(err, &verr) || verr.Field != "TaxTotals" {
		t.Fatalf("Validate() = %v, want *ValidationError on TaxTotals (sum mismatch)", err)
	}
}

func TestValidateTotalsArithmeticMismatch(t *testing.T) {
	inv := sampleInvoice()
	inv.Totals.GrandTotal = NewAmount(999, 99)
	err := inv.Validate(ProfileMinimum)
	var verr *ValidationError
	if !errors.As(err, &verr) || verr.Field != "Totals" {
		t.Fatalf("Validate() = %v, want *ValidationError on Totals", err)
	}
}

func TestValidationErrorMessage(t *testing.T) {
	err := &ValidationError{Profile: ProfileBasic, Field: "Lines", Reason: "at least one line item is required for profile BASIC"}
	want := "zugferd: profile BASIC: Lines: at least one line item is required for profile BASIC"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestDuePayableDefaultsToGrandTotal(t *testing.T) {
	inv := sampleInvoice()
	inv.Totals.DuePayableAmount = 0
	if got := inv.duePayable(); got != inv.Totals.GrandTotal {
		t.Errorf("duePayable() = %v, want GrandTotal %v", got, inv.Totals.GrandTotal)
	}
}

func TestTypeCodeDefault(t *testing.T) {
	inv := sampleInvoice()
	inv.TypeCode = ""
	if got := inv.typeCode(); got != "380" {
		t.Errorf("typeCode() = %q, want \"380\"", got)
	}
	inv.TypeCode = "381" // credit note
	if got := inv.typeCode(); got != "381" {
		t.Errorf("typeCode() = %q, want \"381\"", got)
	}
}

func TestProfileString(t *testing.T) {
	if got := ProfileMinimum.String(); got != "MINIMUM" {
		t.Errorf("ProfileMinimum.String() = %q, want \"MINIMUM\"", got)
	}
	if got := ProfileBasic.String(); got != "BASIC" {
		t.Errorf("ProfileBasic.String() = %q, want \"BASIC\"", got)
	}
}
