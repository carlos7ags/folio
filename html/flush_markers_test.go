// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strings"
	"testing"
)

// TestFlushListMarkersOption verifies the document-wide Options.FlushListMarkers
// toggle reaches the layout: it changes how a nested ordered list renders while
// leaving the numbering intact. The precise flush geometry (all markers in one
// left column) is asserted at the layout level in TestNestedMarkerFlush; here
// we only confirm the option is wired through Convert.
func TestFlushListMarkersOption(t *testing.T) {
	const doc = `<ol><li>One<ol><li>Inner</li></ol></li><li>Two</li></ol>`

	nested, err := Convert(doc, &Options{})
	if err != nil {
		t.Fatal(err)
	}
	flush, err := Convert(doc, &Options{FlushListMarkers: true})
	if err != nil {
		t.Fatal(err)
	}

	nStream := renderStreamText(t, nested)
	fStream := renderStreamText(t, flush)

	// Numbering renders in both modes (top-level 1./2., nested 1.).
	for _, want := range []string{"(1.) Tj", "(2.) Tj"} {
		if !strings.Contains(nStream, want) {
			t.Errorf("default render missing marker %q", want)
		}
		if !strings.Contains(fStream, want) {
			t.Errorf("flush render missing marker %q", want)
		}
	}

	// The option must change the layout (the nested marker moves from its
	// indented column to the container's left column); otherwise it is not
	// wired through Options.
	if nStream == fStream {
		t.Error("FlushListMarkers had no effect on the rendered stream — option not wired")
	}
}
