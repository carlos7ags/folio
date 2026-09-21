// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/html"
)

// fontFaceFS builds an in-memory FS holding the repo's synthetic CJK TTF
// under the name the test documents reference from @font-face. The fixture
// covers only CJK codepoints, which is what makes it useful here: a
// standard PDF-14 face cannot encode them, so a fallback to one is
// unambiguous rather than a subtle difference in letterforms.
func fontFaceFS(t *testing.T) fstest.MapFS {
	t.Helper()
	ttf, err := os.ReadFile("../font/testdata/synthetic_cjk.ttf")
	if err != nil {
		t.Fatalf("read fixture font: %v", err)
	}
	return fstest.MapFS{"Test-Regular.ttf": &fstest.MapFile{Data: ttf}}
}

// renderWithFontFace converts htmlStr with the fixture FS mounted and
// returns the serialized PDF.
func renderWithFontFace(t *testing.T, htmlStr string) []byte {
	t.Helper()
	doc := document.NewDocument(document.PageSizeLetter)
	opts := &html.Options{
		PageWidth:  document.PageSizeLetter.Width,
		PageHeight: document.PageSizeLetter.Height,
		BaseFS:     fontFaceFS(t),
	}
	if err := doc.AddHTML(htmlStr, opts); err != nil {
		t.Fatalf("AddHTML: %v", err)
	}
	pdf, err := doc.ToBytes()
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	return pdf
}

const fontFaceHead = `<html><head><style>
	@font-face { font-family: 'TestFace'; src: url('Test-Regular.ttf') format('truetype'); }
	body { font-family: 'TestFace', Helvetica, sans-serif; }
</style></head><body>`

// Every rune below is covered by font/testdata/synthetic_cjk.ttf. All
// rendered text in these documents is CJK so that any standard-14 fallback
// shows up as a /BaseFont /Helvetica the fixture could never have produced.
const (
	cjkBody = "中华人民"
	cjkLink = "共和国是"
	cjkDest = "一个历史"
)

// TestLinkUsesDocumentFace pins that link text is set in the document's
// declared @font-face, not a standard PDF-14 substitute. The block-level
// <a> path built its paragraph from resolveFont, which only ever returns
// one of the 14 standard faces, so a link inside a fully embedded document
// silently fell back to non-embedded Helvetica.
func TestLinkUsesDocumentFace(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"block anchor", `<a href="#target" style="display:block">` + cjkLink + `</a>`},
		{"block external anchor", `<a href="https://example.com/" style="display:block">` + cjkLink + `</a>`},
		{"inline anchor", `<p>` + cjkBody + `<a href="#target">` + cjkLink + `</a></p>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pdf := string(renderWithFontFace(t, fontFaceHead+`<p>`+cjkBody+`</p>`+tc.body+
				`<div id="target">`+cjkDest+`</div></body></html>`))

			// A subset-embedded font is written as /BaseFont /ABCDEF+Name.
			if !regexp.MustCompile(`/FontFile2`).MatchString(pdf) {
				t.Fatal("document embeds no font program at all — fixture is wrong")
			}
			if strings.Contains(pdf, "/BaseFont /Helvetica") {
				t.Error("link text fell back to non-embedded Helvetica; " +
					"it must use the document's embedded face")
			}
		})
	}
}

// TestAnchorWrappingOnlyAnImage covers an <a> whose entire content is an
// image — a thumbnail or icon that navigates. Two things must survive: the
// image itself, and a link annotation over it. The block-level path used to
// discard the whole subtree when the anchor held no text, and the inline
// path measured the image into a word that never carried the link target.
func TestAnchorWrappingOnlyAnImage(t *testing.T) {
	// A 34x34 flat-colour PNG, inlined so the case needs no asset FS.
	const png = `data:image/png;base64,` +
		`iVBORw0KGgoAAAANSUhEUgAAACIAAAAiCAIAAAC1JZyVAAAAKklEQVR4nO3NMQ0AAAgDsAmb/y` +
		`ALDTxcTfo30z6IRqPRaDQajUaj0WguFlw+pUwg5gZxAAAAAElFTkSuQmCC`

	cases := []struct {
		name string
		body string
	}{
		{"block anchor", `<a href="#target" style="display:block"><img src="` + png + `" width="34" height="34" alt=""></a>`},
		{"inline anchor", `<p><a href="#target"><img src="` + png + `" width="34" height="34" alt=""></a></p>`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			htmlStr := `<html><body>` + tc.body + `<h2 id="target">Target</h2></body></html>`
			pdf, r := htmlRoundtrip(t, htmlStr, document.PageSizeLetter)

			if !strings.Contains(string(pdf), "/Subtype /Image") {
				t.Error("the image inside the anchor was dropped from the output")
			}

			annots := collectLinkAnnots(t, r)
			assertNoFragmentURI(t, annots)
			if len(annots) == 0 {
				t.Fatal("an anchor wrapping an image produced no /Link annotation")
			}
			for i, a := range annots {
				if !a.followable() {
					t.Errorf("annotation %d navigates nowhere (uri=%q destName=%q hasDest=%v)",
						i, a.uri, a.destName, a.hasDest)
				}
			}
			if !annots[0].hasDest && annots[0].destName == "" {
				t.Error("annotation carries no destination")
			}
		})
	}
}
