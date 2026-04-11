// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import "github.com/carlos7ags/folio/core"

// StructNode represents a node in the PDF structure tree.
type StructNode struct {
	Tag        string        `json:"tag"`                    // structure type (e.g. "P", "H1", "Table", "Span")
	MCID       int           `json:"mcid"`                   // marked content identifier (-1 if not a leaf)
	PageObjNum int           `json:"page_obj_num,omitempty"` // page object number for this MCID
	Children   []*StructNode `json:"children,omitempty"`     // child nodes
}

// StructureTree represents the parsed PDF structure tree.
type StructureTree struct {
	Root *StructNode `json:"root"`
}

// parseStructureTree extracts the structure tree from a PDF catalog.
// Returns nil if the document is not tagged (no /MarkInfo or /StructTreeRoot).
func parseStructureTree(catalog *core.PdfDictionary, res *resolver) *StructureTree {
	if catalog == nil {
		return nil
	}

	// Check /MarkInfo → /Marked true.
	markInfoObj := catalog.Get("MarkInfo")
	if markInfoObj != nil {
		resolved, err := res.ResolveDeep(markInfoObj)
		if err == nil {
			if markDict, ok := resolved.(*core.PdfDictionary); ok {
				markedObj := markDict.Get("Marked")
				if markedObj != nil {
					if boolVal, ok := markedObj.(*core.PdfBoolean); ok {
						if !boolVal.Bool() {
							return nil
						}
					}
				} else {
					return nil
				}
			}
		}
	} else {
		return nil
	}

	// Get /StructTreeRoot.
	structTreeObj := catalog.Get("StructTreeRoot")
	if structTreeObj == nil {
		return nil
	}
	resolved, err := res.ResolveDeep(structTreeObj)
	if err != nil {
		return nil
	}
	structTreeDict, ok := resolved.(*core.PdfDictionary)
	if !ok {
		return nil
	}

	root := &StructNode{
		Tag:  "StructTreeRoot",
		MCID: -1,
	}

	// Walk /K (kids) of the structure tree root.
	kObj := structTreeDict.Get("K")
	if kObj != nil {
		parseStructKids(kObj, root, res)
	}

	return &StructureTree{Root: root}
}

// parseStructKids parses the /K entry of a structure element and
// appends child nodes to parent.
func parseStructKids(kObj core.PdfObject, parent *StructNode, res *resolver) {
	resolved, err := res.ResolveDeep(kObj)
	if err != nil {
		return
	}

	switch v := resolved.(type) {
	case *core.PdfNumber:
		// Integer MCID — this is a leaf node referencing marked content.
		parent.MCID = v.IntValue()

	case *core.PdfDictionary:
		parseStructElement(v, parent, res)

	case *core.PdfArray:
		for _, elem := range v.All() {
			parseStructKids(elem, parent, res)
		}
	}
}

// parseStructElement parses a single structure element dictionary.
func parseStructElement(dict *core.PdfDictionary, parent *StructNode, res *resolver) {
	// Check if this is a marked content reference (MCR) dict.
	// MCR dicts have /Type /MCR and /MCID integer.
	typeObj := dict.Get("Type")
	if typeObj != nil {
		if name, ok := typeObj.(*core.PdfName); ok && name.Value == "MCR" {
			mcidObj := dict.Get("MCID")
			if mcidObj != nil {
				if num, ok := mcidObj.(*core.PdfNumber); ok {
					pageObjNum := 0
					pgObj := dict.Get("Pg")
					if pgObj != nil {
						if ref, ok := pgObj.(*core.PdfIndirectReference); ok {
							pageObjNum = ref.Num()
						}
					}
					// MCR is a leaf — attach MCID to parent directly.
					child := &StructNode{
						Tag:        parent.Tag,
						MCID:       num.IntValue(),
						PageObjNum: pageObjNum,
					}
					parent.Children = append(parent.Children, child)
				}
			}
			return
		}
		// /Type /OBJR — object reference, skip for now.
		if name, ok := typeObj.(*core.PdfName); ok && name.Value == "OBJR" {
			return
		}
	}

	// It's a structure element with /S (structure type) and /K (kids).
	tag := ""
	sObj := dict.Get("S")
	if sObj != nil {
		resolved, err := res.ResolveDeep(sObj)
		if err == nil {
			if name, ok := resolved.(*core.PdfName); ok {
				tag = name.Value
			}
		}
	}

	// Extract page reference if present.
	pageObjNum := 0
	pgObj := dict.Get("Pg")
	if pgObj != nil {
		if ref, ok := pgObj.(*core.PdfIndirectReference); ok {
			pageObjNum = ref.Num()
		}
	}

	child := &StructNode{
		Tag:        tag,
		MCID:       -1,
		PageObjNum: pageObjNum,
	}

	// Recurse into /K.
	kObj := dict.Get("K")
	if kObj != nil {
		resolved, err := res.ResolveDeep(kObj)
		if err == nil {
			switch kv := resolved.(type) {
			case *core.PdfNumber:
				// Direct MCID integer — this element is a leaf.
				child.MCID = kv.IntValue()
			case *core.PdfDictionary:
				parseStructElement(kv, child, res)
			case *core.PdfArray:
				for _, elem := range kv.All() {
					parseStructKids(elem, child, res)
				}
			}
		}
	}

	parent.Children = append(parent.Children, child)
}
