// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import "github.com/carlos7ags/folio/core"

// PageLayout controls how pages are displayed when the document is opened.
type PageLayout string

const (
	// LayoutSinglePage displays one page at a time.
	LayoutSinglePage PageLayout = "SinglePage"
	// LayoutOneColumn displays pages in a single continuous scrolling column.
	LayoutOneColumn PageLayout = "OneColumn"
	// LayoutTwoColumnLeft displays pages in two columns with odd pages on the left.
	LayoutTwoColumnLeft PageLayout = "TwoColumnLeft"
	// LayoutTwoColumnRight displays pages in two columns with odd pages on the right.
	LayoutTwoColumnRight PageLayout = "TwoColumnRight"
	// LayoutTwoPageLeft displays two pages at a time with odd pages on the left.
	LayoutTwoPageLeft PageLayout = "TwoPageLeft"
	// LayoutTwoPageRight displays two pages at a time with odd pages on the right.
	LayoutTwoPageRight PageLayout = "TwoPageRight"
)

// PageMode controls what panel is visible when the document is opened.
type PageMode string

const (
	// ModeNone shows no panel when the document is opened (default).
	ModeNone PageMode = "UseNone"
	// ModeOutlines shows the bookmarks panel when the document is opened.
	ModeOutlines PageMode = "UseOutlines"
	// ModeThumbs shows the page thumbnails panel when the document is opened.
	ModeThumbs PageMode = "UseThumbs"
	// ModeFullScreen opens the document in full-screen mode.
	ModeFullScreen PageMode = "FullScreen"
	// ModeOC shows the optional content panel when the document is opened.
	ModeOC PageMode = "UseOC"
	// ModeAttach shows the attachments panel when the document is opened.
	ModeAttach PageMode = "UseAttachments"
)

// ViewerPreferences controls how the PDF viewer displays the document.
type ViewerPreferences struct {
	// How pages are arranged.
	PageLayout PageLayout

	// What panel is visible on open.
	PageMode PageMode

	// Hide viewer UI elements.
	HideToolbar  bool
	HideMenubar  bool
	HideWindowUI bool

	// Fit the window to the first page.
	FitWindow bool

	// Center the window on screen.
	CenterWindow bool

	// Display the document title (vs filename) in the title bar.
	DisplayDocTitle bool

	// Page to display on open (0-based). -1 = not set.
	OpenPage int

	// Zoom on open: "Fit", "FitH", "FitV", "FitB", or a percentage (e.g. 100).
	// Empty string = viewer default.
	OpenZoom string
}

// SetViewerPreferences configures how viewers display the document.
func (d *Document) SetViewerPreferences(vp ViewerPreferences) {
	d.viewerPrefs = &vp
}

// buildViewerPreferences creates the /ViewerPreferences dictionary
// and sets /PageLayout and /PageMode on the catalog.
func buildViewerPreferences(vp *ViewerPreferences, catalog *core.PdfDictionary) {
	if vp == nil {
		return
	}

	// /PageLayout on catalog.
	if vp.PageLayout != "" {
		catalog.Set("PageLayout", core.NewPdfName(string(vp.PageLayout)))
	}

	// /PageMode on catalog.
	if vp.PageMode != "" {
		catalog.Set("PageMode", core.NewPdfName(string(vp.PageMode)))
	}

	// /ViewerPreferences dictionary.
	prefs := core.NewPdfDictionary()
	hasPrefs := false

	if vp.HideToolbar {
		prefs.Set("HideToolbar", core.NewPdfBoolean(true))
		hasPrefs = true
	}
	if vp.HideMenubar {
		prefs.Set("HideMenubar", core.NewPdfBoolean(true))
		hasPrefs = true
	}
	if vp.HideWindowUI {
		prefs.Set("HideWindowUI", core.NewPdfBoolean(true))
		hasPrefs = true
	}
	if vp.FitWindow {
		prefs.Set("FitWindow", core.NewPdfBoolean(true))
		hasPrefs = true
	}
	if vp.CenterWindow {
		prefs.Set("CenterWindow", core.NewPdfBoolean(true))
		hasPrefs = true
	}
	if vp.DisplayDocTitle {
		prefs.Set("DisplayDocTitle", core.NewPdfBoolean(true))
		hasPrefs = true
	}

	if hasPrefs {
		catalog.Set("ViewerPreferences", prefs)
	}
}
