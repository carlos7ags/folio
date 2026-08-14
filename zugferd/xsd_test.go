// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build xsdvalidate

package zugferd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestXSDValidation validates generated CII output against the official
// Factur-X XSD for each supported profile using xmllint (libxml2).
//
// The official Factur-X/ZUGFeRD schema package is not committed to this
// repository: its redistribution terms could not be confirmed (the
// FNFE-MPE download is email-gated and the page states "All Rights
// Reserved" with no explicit redistribution grant — see
// plans/zugferd-20260813-xsd-validation.md STOP conditions). Point
// FOLIO_FACTURX_XSD_DIR at a local checkout of the schema package to run
// this test; each entry in root below is resolved relative to that
// directory. The test skips (not fails) when the env var is unset or
// xmllint is unavailable.
//
// Run with: FOLIO_FACTURX_XSD_DIR=/path/to/schemas go test -tags xsdvalidate ./zugferd/
func TestXSDValidation(t *testing.T) {
	xsdDir := os.Getenv("FOLIO_FACTURX_XSD_DIR")
	if xsdDir == "" {
		t.Skip("FOLIO_FACTURX_XSD_DIR not set; the Factur-X schema package is not committed to this repo (unclear redistribution terms) — see plans/zugferd-20260813-xsd-validation.md")
	}
	if _, err := exec.LookPath("xmllint"); err != nil {
		t.Skip("xmllint not installed")
	}
	cases := []struct {
		profile Profile
		xsd     string // root XSD relative to FOLIO_FACTURX_XSD_DIR
	}{
		{ProfileMinimum, "Factur-X_1.0.7_MINIMUM/Schema/Factur-X_1.07_MINIMUM.xsd"},
		{ProfileBasic, "Factur-X_1.0.7_BASIC/Schema/Factur-X_1.07_BASIC.xsd"},
	}
	for _, tc := range cases {
		t.Run(tc.xsd, func(t *testing.T) {
			xml, err := sampleInvoice().XML(tc.profile)
			if err != nil {
				t.Fatalf("XML: %v", err)
			}
			f := filepath.Join(t.TempDir(), "invoice.xml")
			if err := os.WriteFile(f, xml, 0o644); err != nil {
				t.Fatal(err)
			}
			schema := filepath.Join(xsdDir, tc.xsd)
			if _, err := os.Stat(schema); err != nil {
				t.Skipf("schema not found at %s: %v", schema, err)
			}
			out, err := exec.Command("xmllint", "--noout", "--schema", schema, f).CombinedOutput()
			if err != nil {
				t.Errorf("XSD validation failed:\n%s", out)
			}
		})
	}
}
