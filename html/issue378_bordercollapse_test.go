// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"bytes"
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// TestIssue378ThreadTbodyBorderConflictResolved reproduces issue #378's
// reduction: a thead cell's wider, nonzero border-bottom must win over an
// explicit 0pt border-top declared on the adjacent tbody cell, per CSS2.1
// §17.6.2. Previously the collapse code favored row order over the actual
// declared widths, so the header's border silently vanished.
func TestIssue378ThreadTbodyBorderConflictResolved(t *testing.T) {
	src := `<html><head><style>
		table { border-collapse: collapse; }
		table thead tr:last-child th { border-bottom: .25pt solid black; }
		table tbody tr:first-child td { border-top: 0pt solid black; }
	</style></head><body><table>
		<thead><tr><th>H</th></tr></thead>
		<tbody><tr><td>B</td></tr></tbody>
	</table></body></html>`

	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := layout.NewRenderer(612, 792, layout.Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	for _, e := range elems {
		r.Add(e)
	}
	pages := r.Render()
	stream := pages[0].Stream.Bytes()

	// The header's .25pt border must win (it is the only nonzero
	// declaration on this edge) — its line-width operator must appear.
	if !bytes.Contains(stream, []byte("0.25 w")) {
		t.Errorf("expected the header's .25pt border to win the thead/tbody edge conflict; stream:\n%s", stream)
	}
}

// TestIssue378StylePriorityWinsAtEqualWidth locks in the equal-width
// style-priority tier of the conflict resolver: at equal width, double
// beats solid (see borderPriority) — a boundary an author can hit
// alongside the width-based reduction above.
func TestIssue378StylePriorityWinsAtEqualWidth(t *testing.T) {
	src := `<html><head><style>
		table { border-collapse: collapse; }
		table thead tr:last-child th { border-bottom: 2pt double black; }
		table tbody tr:first-child td { border-top: 2pt solid black; }
	</style></head><body><table>
		<thead><tr><th>H</th></tr></thead>
		<tbody><tr><td>B</td></tr></tbody>
	</table></body></html>`

	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := layout.NewRenderer(612, 792, layout.Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	for _, e := range elems {
		r.Add(e)
	}
	pages := r.Render()
	stream := pages[0].Stream.Bytes()

	// double draws two parallel strokes per edge, solid draws one. This
	// table declares no other borders, so the total stroke count on the
	// shared edge alone distinguishes the winner.
	if strokes := countStrokeOps(stream); strokes != 2 {
		t.Errorf("expected the equal-width double border to win (2 strokes), got %d; stream:\n%s", strokes, stream)
	}
}

// countStrokeOps counts standalone "S" (stroke) operators in a PDF
// content stream.
func countStrokeOps(stream []byte) int {
	count := 0
	for _, line := range bytes.Split(stream, []byte("\n")) {
		if string(bytes.TrimSpace(line)) == "S" {
			count++
		}
	}
	return count
}
