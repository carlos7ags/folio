// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import "github.com/carlos7ags/folio/core"

// StructTag is a standard PDF structure type (ISO 32000 §14.8.4).
type StructTag string

const (
	TagDocument   StructTag = "Document"
	TagPart       StructTag = "Part"
	TagSection    StructTag = "Sect"
	TagH1         StructTag = "H1"
	TagH2         StructTag = "H2"
	TagH3         StructTag = "H3"
	TagH4         StructTag = "H4"
	TagH5         StructTag = "H5"
	TagH6         StructTag = "H6"
	TagP          StructTag = "P"
	TagSpan       StructTag = "Span"
	TagTable      StructTag = "Table"
	TagTR         StructTag = "TR"
	TagTH         StructTag = "TH"
	TagTD         StructTag = "TD"
	TagTHead      StructTag = "THead"
	TagTBody      StructTag = "TBody"
	TagL          StructTag = "L"     // list
	TagLI         StructTag = "LI"    // list item
	TagLbl        StructTag = "Lbl"   // list label (bullet/number)
	TagLBody      StructTag = "LBody" // list item body
	TagFigure     StructTag = "Figure"
	TagCaption    StructTag = "Caption"
	TagLink       StructTag = "Link"
	TagBlockQuote StructTag = "BlockQuote"
	TagDiv        StructTag = "Div"
)

// structNode is a node in the document structure tree.
// It represents either a structure element (with children) or
// a marked content reference (leaf pointing to page content).
type structNode struct {
	tag      StructTag          // structure type (e.g. "P", "H1", "Table")
	children []*structNode      // child structure elements
	mcids    []markedContentRef // marked content references (leaf content)
	altText  string             // alternative text for figures/images
}

// markedContentRef links a structure node to content on a specific page.
type markedContentRef struct {
	mcid      int // marked content identifier
	pageIndex int // which page this content is on (0-based)
}

// structTree builds and manages the PDF structure tree for tagged PDF.
type structTree struct {
	root     *structNode // document root element
	nextMCID []int       // per-page MCID counter
}

// newStructTree creates a new structure tree with a Document root.
func newStructTree() *structTree {
	return &structTree{
		root: &structNode{tag: TagDocument},
	}
}

// addElement adds a structure element under the document root and returns it.
func (st *structTree) addElement(tag StructTag) *structNode {
	node := &structNode{tag: tag}
	st.root.children = append(st.root.children, node)
	return node
}

// addChild adds a child structure element under the given parent.
func (st *structTree) addChild(parent *structNode, tag StructTag) *structNode {
	node := &structNode{tag: tag}
	parent.children = append(parent.children, node)
	return node
}

// allocMCID allocates the next MCID for the given page.
func (st *structTree) allocMCID(pageIndex int) int {
	for len(st.nextMCID) <= pageIndex {
		st.nextMCID = append(st.nextMCID, 0)
	}
	mcid := st.nextMCID[pageIndex]
	st.nextMCID[pageIndex]++
	return mcid
}

// markContent assigns a marked content ID to a node for a specific page.
func (st *structTree) markContent(node *structNode, pageIndex int) int {
	mcid := st.allocMCID(pageIndex)
	node.mcids = append(node.mcids, markedContentRef{mcid: mcid, pageIndex: pageIndex})
	return mcid
}

// isEmpty reports whether the structure tree has any content.
func (st *structTree) isEmpty() bool {
	return st.root == nil || (len(st.root.children) == 0 && len(st.root.mcids) == 0)
}

// buildPdfObjects serializes the structure tree into PDF objects.
// Returns the StructTreeRoot reference to set on the catalog.
func (st *structTree) buildPdfObjects(
	pageRefs []*core.PdfIndirectReference,
	addObject func(core.PdfObject) *core.PdfIndirectReference,
) *core.PdfIndirectReference {
	if st.isEmpty() {
		return nil
	}

	// Create the StructTreeRoot dictionary.
	rootDict := core.NewPdfDictionary()
	rootDict.Set("Type", core.NewPdfName("StructTreeRoot"))
	rootRef := addObject(rootDict)

	// Create the Document structure element (top-level container).
	docElem := core.NewPdfDictionary()
	docElem.Set("Type", core.NewPdfName("StructElem"))
	docElem.Set("S", core.NewPdfName(string(st.root.tag)))
	docElem.Set("P", rootRef)
	docElemRef := addObject(docElem)
	rootDict.Set("K", docElemRef)

	// Build all structure element objects recursively under the Document element.
	var parentEntries []parentEntry
	kids := st.buildChildren(st.root, docElemRef, pageRefs, addObject, &parentEntries)

	if kids.Len() == 1 {
		docElem.Set("K", kids.Elements[0])
	} else if kids.Len() > 0 {
		docElem.Set("K", kids)
	}

	// Build the ParentTree (number tree mapping MCIDs to struct elem refs).
	if len(parentEntries) > 0 {
		parentTree := buildParentTree(parentEntries, pageRefs, addObject)
		rootDict.Set("ParentTree", parentTree)
	}

	return rootRef
}

// parentEntry maps an MCID on a page to the struct element that owns it.
type parentEntry struct {
	pageIndex int
	mcid      int
	elemRef   *core.PdfIndirectReference
}

// buildChildren recursively builds StructElem objects for a node's children.
func (st *structTree) buildChildren(
	node *structNode,
	parentRef *core.PdfIndirectReference,
	pageRefs []*core.PdfIndirectReference,
	addObject func(core.PdfObject) *core.PdfIndirectReference,
	parentEntries *[]parentEntry,
) *core.PdfArray {
	kids := core.NewPdfArray()

	for _, child := range node.children {
		elemDict := core.NewPdfDictionary()
		elemDict.Set("Type", core.NewPdfName("StructElem"))
		elemDict.Set("S", core.NewPdfName(string(child.tag)))
		elemDict.Set("P", parentRef)
		elemRef := addObject(elemDict)

		// Add alt text for figures.
		if child.altText != "" {
			elemDict.Set("Alt", core.NewPdfLiteralString(child.altText))
		}

		// Build this element's content references (MCIDs).
		elemKids := core.NewPdfArray()
		for _, mcr := range child.mcids {
			if mcr.pageIndex < len(pageRefs) {
				// Marked content reference dictionary.
				mcrDict := core.NewPdfDictionary()
				mcrDict.Set("Type", core.NewPdfName("MCR"))
				mcrDict.Set("Pg", pageRefs[mcr.pageIndex])
				mcrDict.Set("MCID", core.NewPdfInteger(mcr.mcid))
				elemKids.Add(mcrDict)

				*parentEntries = append(*parentEntries, parentEntry{
					pageIndex: mcr.pageIndex,
					mcid:      mcr.mcid,
					elemRef:   elemRef,
				})
			}
		}

		// Recurse into child structure elements.
		childKids := st.buildChildren(child, elemRef, pageRefs, addObject, parentEntries)
		for _, ck := range childKids.Elements {
			elemKids.Add(ck)
		}

		if elemKids.Len() == 1 {
			elemDict.Set("K", elemKids.Elements[0])
		} else if elemKids.Len() > 0 {
			elemDict.Set("K", elemKids)
		}

		// Set /Pg on the element if all MCIDs are on the same page.
		if pg := singlePage(child, pageRefs); pg != nil {
			elemDict.Set("Pg", pg)
		}

		kids.Add(elemRef)
	}

	return kids
}

// singlePage returns the page ref if all MCIDs in the node are on the same page.
func singlePage(node *structNode, pageRefs []*core.PdfIndirectReference) *core.PdfIndirectReference {
	if len(node.mcids) == 0 {
		return nil
	}
	pg := node.mcids[0].pageIndex
	for _, mcr := range node.mcids[1:] {
		if mcr.pageIndex != pg {
			return nil
		}
	}
	if pg < len(pageRefs) {
		return pageRefs[pg]
	}
	return nil
}

// buildParentTree constructs the /ParentTree number tree.
// The ParentTree maps page-index to an array of struct elem refs,
// indexed by MCID within that page.
func buildParentTree(
	entries []parentEntry,
	pageRefs []*core.PdfIndirectReference,
	addObject func(core.PdfObject) *core.PdfIndirectReference,
) *core.PdfIndirectReference {
	// Group entries by page.
	pageMap := make(map[int][]parentEntry)
	for _, e := range entries {
		pageMap[e.pageIndex] = append(pageMap[e.pageIndex], e)
	}

	// Build the Nums array: [pageIndex0 [ref0 ref1 ...] pageIndex1 [...] ...]
	nums := core.NewPdfArray()
	for pageIdx := range len(pageRefs) {
		pes, ok := pageMap[pageIdx]
		if !ok {
			continue
		}
		// Find max MCID to size the array.
		maxMCID := 0
		for _, pe := range pes {
			if pe.mcid > maxMCID {
				maxMCID = pe.mcid
			}
		}
		// Build the per-page array indexed by MCID.
		arr := core.NewPdfArray()
		refs := make([]*core.PdfIndirectReference, maxMCID+1)
		for _, pe := range pes {
			refs[pe.mcid] = pe.elemRef
		}
		for _, ref := range refs {
			if ref != nil {
				arr.Add(ref)
			} else {
				arr.Add(core.NewPdfNull())
			}
		}

		nums.Add(core.NewPdfInteger(pageIdx))
		arrRef := addObject(arr)
		nums.Add(arrRef)
	}

	parentTreeDict := core.NewPdfDictionary()
	parentTreeDict.Set("Nums", nums)
	return addObject(parentTreeDict)
}
