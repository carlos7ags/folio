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

// TestXSDValidation validates generated CII output against an XSD schema
// using xmllint (libxml2). FOLIO_FACTURX_XSD_DIR must point at a local,
// pre-extracted schema directory; the test skips (not fails) when the env
// var is unset or xmllint is unavailable.
//
// Two schema layouts are supported, tried in order for each profile via
// facturXPaths below:
//
//  1. The official Factur-X/ZUGFeRD package (e.g.
//     "Factur-X_1.0.7_MINIMUM/Schema/Factur-X_1.07_MINIMUM.xsd"). Not
//     committed to this repo: its redistribution terms could not be
//     confirmed (the FNFE-MPE download is email-gated and the page states
//     "All Rights Reserved" with no explicit redistribution grant).
//     Factur-X profile XSDs are restrictions of plain CII (they add
//     profile-specific cardinality/business rules on top of the base UN/
//     CEFACT model), so validating against them is the stronger check.
//
//  2. The plain UN/CEFACT Cross Industry Invoice (CII) D16B schema
//     (SCRDM, "coupled" codelists), published by UNECE and mirrored
//     verbatim by third parties (e.g. ConnectingEurope/eInvoicing-EN16931
//     on GitHub). Its root is
//     "CII/uncefact/data/standard/CrossIndustryInvoice_100pD16B.xsd"
//     under the schema dir, used for both MINIMUM and BASIC profiles
//     since CII has no Factur-X-specific profile restrictions. This is a
//     weaker check than (1): it validates that the XML is well-formed CII
//     (correct elements, types, structure) but not the tighter
//     profile-specific business rules Factur-X layers on top (e.g. which
//     elements are mandatory/forbidden per profile).
//
// Run with: FOLIO_FACTURX_XSD_DIR=/path/to/schemas go test -tags xsdvalidate ./zugferd/
func TestXSDValidation(t *testing.T) {
	xsdDir := os.Getenv("FOLIO_FACTURX_XSD_DIR")
	if xsdDir == "" {
		t.Skip("FOLIO_FACTURX_XSD_DIR not set; no XSD schema package is committed to this repo — see zugferd/xsd_test.go for supported layouts")
	}
	if _, err := exec.LookPath("xmllint"); err != nil {
		t.Skip("xmllint not installed")
	}

	const cii100pD16B = "CII/uncefact/data/standard/CrossIndustryInvoice_100pD16B.xsd"

	cases := []struct {
		profile Profile
		// candidates are tried in order; the first that exists under
		// FOLIO_FACTURX_XSD_DIR is used. This lets FOLIO_FACTURX_XSD_DIR
		// point at either a Factur-X package checkout or a plain UNECE
		// CII D16B schema extraction.
		candidates []string
	}{
		{ProfileMinimum, []string{
			"Factur-X_1.0.7_MINIMUM/Schema/Factur-X_1.07_MINIMUM.xsd",
			cii100pD16B,
		}},
		{ProfileBasic, []string{
			"Factur-X_1.0.7_BASIC/Schema/Factur-X_1.07_BASIC.xsd",
			cii100pD16B,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.candidates[0], func(t *testing.T) {
			var schema string
			for _, c := range tc.candidates {
				p := filepath.Join(xsdDir, c)
				if _, err := os.Stat(p); err == nil {
					schema = p
					break
				}
			}
			if schema == "" {
				t.Skipf("no schema found under %s among candidates %v", xsdDir, tc.candidates)
			}

			xml, err := sampleInvoice().XML(tc.profile)
			if err != nil {
				t.Fatalf("XML: %v", err)
			}
			f := filepath.Join(t.TempDir(), "invoice.xml")
			if err := os.WriteFile(f, xml, 0o644); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command("xmllint", "--noout", "--schema", schema, f).CombinedOutput()
			if err != nil {
				t.Errorf("XSD validation failed against %s:\n%s", schema, out)
			}
		})
	}
}
