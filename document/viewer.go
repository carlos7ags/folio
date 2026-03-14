// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import "github.com/carlos7ags/folio/core"

// PageLayout controls how pages are displayed when the document is opened.
type PageLayout string

const (
	LayoutSinglePage     PageLayout = "SinglePage"     // one page at a time
	LayoutOneColumn      PageLayout = "OneColumn"      // continuous scrolling
	LayoutTwoColumnLeft  PageLayout = "TwoColumnLeft"  // two columns, odd pages left
	LayoutTwoColumnRight PageLayout = "TwoColumnRight" // two columns, odd pages right
	LayoutTwoPageLeft    PageLayout = "TwoPageLeft"    // two pages, odd left
	LayoutTwoPageRight   PageLayout = "TwoPageRight"   // two pages, odd right
)

// PageMode controls what panel is visible when the document is opened.
type PageMode string

const (
	ModeNone       PageMode = "UseNone"        // no panel (default)
	ModeOutlines   PageMode = "UseOutlines"    // bookmarks panel
	ModeThumbs     PageMode = "UseThumbs"      // thumbnails panel
	ModeFullScreen PageMode = "FullScreen"     // full screen
	ModeOC         PageMode = "UseOC"          // optional content panel
	ModeAttach     PageMode = "UseAttachments" // attachments panel
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
