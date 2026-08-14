// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html_test

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/document"
)

// TestNowrapEllipsisTruncatesInRenderedPDF is an end-to-end regression test
// for text-overflow:ellipsis. Ellipsis truncation used to be implemented
// only in Paragraph.Layout(); the real HTML render path goes through
// Div.PlanLayout -> Paragraph.PlanLayout, which ignored the ellipsis flag
// entirely, so overflowing text wrapped or spilled instead of being
// truncated with "...". This asserts the drawn (and PDF-extracted) text is
// actually truncated.
func TestNowrapEllipsisTruncatesInRenderedPDF(t *testing.T) {
	htmlStr := `<div style="width:100pt; white-space:nowrap; overflow:hidden; text-overflow:ellipsis">This is a very long line of text for nowrap truncation testing</div>`

	_, r := htmlRoundtrip(t, htmlStr, document.PageSizeLetter)
	text := extractAllText(t, r)

	if !strings.Contains(text, "...") {
		t.Fatalf("expected truncated text to contain an ellipsis, got %q", text)
	}
	if strings.Contains(text, "nowrap truncation testing") {
		t.Fatalf("expected text to be truncated, but full untruncated text was found: %q", text)
	}
}
