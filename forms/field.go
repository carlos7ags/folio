// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package forms provides AcroForm support for creating interactive
// PDF form fields: text inputs, checkboxes, radio buttons, dropdowns,
// and signature fields.
package forms

import (
	"github.com/carlos7ags/folio/core"
)

// FieldType identifies the kind of form field.
type FieldType int

const (
	FieldText       FieldType = iota // text input
	FieldCheckbox                    // checkbox (on/off)
	FieldRadio                       // radio button group
	FieldDropdown                    // combo box / dropdown
	FieldListBox                     // list box (multi-select)
	FieldPushButton                  // push button
	FieldSignature                   // digital signature
)

// FieldFlags are bit flags for form field properties (ISO 32000 §12.7.3).
type FieldFlags uint32

const (
	FlagReadOnly FieldFlags = 1 << 0 // field cannot be edited
	FlagRequired FieldFlags = 1 << 1 // field must have a value
	FlagNoExport FieldFlags = 1 << 2 // field excluded from export

	// Text field specific flags.
	FlagMultiline       FieldFlags = 1 << 12 // multi-line text
	FlagPassword        FieldFlags = 1 << 13 // password (hidden chars)
	FlagFileSelect      FieldFlags = 1 << 20 // file path input
	FlagDoNotSpellCheck FieldFlags = 1 << 22 // disable spell check
	FlagDoNotScroll     FieldFlags = 1 << 23 // disable scrolling for long text
	FlagComb            FieldFlags = 1 << 24 // evenly spaced characters
	FlagRichText        FieldFlags = 1 << 25 // rich text value

	// Choice field specific flags.
	FlagCombo       FieldFlags = 1 << 17 // combo box (dropdown) vs list box
	FlagEdit        FieldFlags = 1 << 18 // editable combo box
	FlagSort        FieldFlags = 1 << 19 // sorted options
	FlagMultiSelect FieldFlags = 1 << 21 // allow multiple selections

	// Button specific flags.
	FlagNoToggleToOff  FieldFlags = 1 << 14 // one radio button must always be selected
	FlagRadioFlag      FieldFlags = 1 << 15 // radio button (vs checkbox)
	FlagPushButtonFlag FieldFlags = 1 << 16 // push button (no persistent value)
)

// Field represents a single form field.
type Field struct {
	Name      string     // /T — field name (required)
	Type      FieldType  // determines /FT
	Value     string     // /V — current value
	Default   string     // /DV — default value
	Flags     FieldFlags // /Ff — field flags
	Rect      [4]float64 // widget annotation rectangle [x1, y1, x2, y2]
	PageIndex int        // which page this field is on (0-based)

	// Appearance properties.
	FontSize    float64     // 0 = auto-size
	FontName    string      // default: "Helv" (Helvetica)
	TextColor   [3]float64  // RGB [0-1], default black
	BGColor     *[3]float64 // background color (nil = transparent)
	BorderColor *[3]float64 // border color (nil = no border)
	BorderWidth float64     // default 1

	// Choice field options.
	Options []string // for dropdown/listbox

	// Radio/checkbox.
	ExportValue string // /AS or /V value when checked (default "Yes")

	// children holds the individual buttons in a radio button group.
	children []*Field
}

// TextField creates a text input field.
func TextField(name string, rect [4]float64, pageIndex int) *Field {
	return &Field{
		Name:        name,
		Type:        FieldText,
		Rect:        rect,
		PageIndex:   pageIndex,
		FontSize:    12,
		FontName:    "Helv",
		BorderWidth: 1,
	}
}

// MultilineTextField creates a multi-line text area.
func MultilineTextField(name string, rect [4]float64, pageIndex int) *Field {
	f := TextField(name, rect, pageIndex)
	f.Flags |= FlagMultiline
	return f
}

// PasswordField creates a password input field.
func PasswordField(name string, rect [4]float64, pageIndex int) *Field {
	f := TextField(name, rect, pageIndex)
	f.Flags |= FlagPassword
	return f
}

// Checkbox creates a checkbox field.
func Checkbox(name string, rect [4]float64, pageIndex int, checked bool) *Field {
	f := &Field{
		Name:        name,
		Type:        FieldCheckbox,
		Rect:        rect,
		PageIndex:   pageIndex,
		ExportValue: "Yes",
		FontSize:    0,
		BorderWidth: 1,
	}
	if checked {
		f.Value = "Yes"
	} else {
		f.Value = "Off"
	}
	return f
}

// RadioGroup creates a radio button group with the given options.
// Each RadioOption becomes a child widget field representing one button.
func RadioGroup(name string, options []RadioOption) *Field {
	parent := &Field{
		Name:  name,
		Type:  FieldRadio,
		Flags: FlagRadioFlag | FlagNoToggleToOff,
	}
	for _, opt := range options {
		child := &Field{
			Name:        opt.Value,
			Type:        FieldRadio,
			Rect:        opt.Rect,
			PageIndex:   opt.PageIndex,
			ExportValue: opt.Value,
			BorderWidth: 1,
		}
		parent.children = append(parent.children, child)
	}
	return parent
}

// RadioOption defines one button in a radio group.
type RadioOption struct {
	Value     string     // export value when selected
	Rect      [4]float64 // position on page
	PageIndex int        // zero-based page index
}

// Dropdown creates a dropdown (combo box) field.
func Dropdown(name string, rect [4]float64, pageIndex int, options []string) *Field {
	return &Field{
		Name:        name,
		Type:        FieldDropdown,
		Rect:        rect,
		PageIndex:   pageIndex,
		Options:     options,
		Flags:       FlagCombo,
		FontSize:    12,
		FontName:    "Helv",
		BorderWidth: 1,
	}
}

// ListBox creates a list box field.
func ListBox(name string, rect [4]float64, pageIndex int, options []string) *Field {
	return &Field{
		Name:        name,
		Type:        FieldListBox,
		Rect:        rect,
		PageIndex:   pageIndex,
		Options:     options,
		FontSize:    12,
		FontName:    "Helv",
		BorderWidth: 1,
	}
}

// SignatureField creates a digital signature field.
func SignatureField(name string, rect [4]float64, pageIndex int) *Field {
	return &Field{
		Name:        name,
		Type:        FieldSignature,
		Rect:        rect,
		PageIndex:   pageIndex,
		BorderWidth: 1,
	}
}

// SetValue sets the field's value.
func (f *Field) SetValue(v string) *Field {
	f.Value = v
	return f
}

// SetReadOnly marks the field as read-only.
func (f *Field) SetReadOnly() *Field {
	f.Flags |= FlagReadOnly
	return f
}

// SetRequired marks the field as required.
func (f *Field) SetRequired() *Field {
	f.Flags |= FlagRequired
	return f
}

// SetBackgroundColor sets the field's background color.
func (f *Field) SetBackgroundColor(r, g, b float64) *Field {
	f.BGColor = &[3]float64{r, g, b}
	return f
}

// SetBorderColor sets the field's border color.
func (f *Field) SetBorderColor(r, g, b float64) *Field {
	f.BorderColor = &[3]float64{r, g, b}
	return f
}

// ftName returns the PDF /FT name for the given FieldType.
func ftName(ft FieldType) string {
	switch ft {
	case FieldText:
		return "Tx"
	case FieldCheckbox, FieldRadio, FieldPushButton:
		return "Btn"
	case FieldDropdown, FieldListBox:
		return "Ch"
	case FieldSignature:
		return "Sig"
	default:
		return "Tx"
	}
}

// ToDict converts the field to a PDF dictionary and registers it via addObject.
// It returns the field's indirect reference and widget info for page annotation placement.
func (f *Field) ToDict(addObject func(core.PdfObject) *core.PdfIndirectReference, pageRefs []*core.PdfIndirectReference) (*core.PdfIndirectReference, []*widgetInfo) {
	dict := core.NewPdfDictionary()
	dict.Set("T", core.NewPdfLiteralString(f.Name))
	dict.Set("FT", core.NewPdfName(ftName(f.Type)))

	if f.Flags != 0 {
		dict.Set("Ff", core.NewPdfInteger(int(f.Flags)))
	}
	if f.Value != "" {
		dict.Set("V", core.NewPdfLiteralString(f.Value))
	}
	if f.Default != "" {
		dict.Set("DV", core.NewPdfLiteralString(f.Default))
	}

	// Choice options.
	if len(f.Options) > 0 {
		optArr := core.NewPdfArray()
		for _, opt := range f.Options {
			optArr.Add(core.NewPdfLiteralString(opt))
		}
		dict.Set("Opt", optArr)
	}

	fieldRef := addObject(dict)
	var widgets []*widgetInfo

	// Radio button group: children are separate widgets.
	if f.Type == FieldRadio && len(f.children) > 0 {
		kids := core.NewPdfArray()
		for _, child := range f.children {
			wDict := buildWidgetDict(child, fieldRef, pageRefs)
			wRef := addObject(wDict)
			kids.Add(wRef)
			if child.PageIndex >= 0 && child.PageIndex < len(pageRefs) {
				widgets = append(widgets, &widgetInfo{ref: wRef, pageIndex: child.PageIndex})
			}
		}
		dict.Set("Kids", kids)
		return fieldRef, widgets
	}

	// Single widget merged into the field dict.
	if f.PageIndex >= 0 && f.PageIndex < len(pageRefs) {
		dict.Set("Type", core.NewPdfName("Annot"))
		dict.Set("Subtype", core.NewPdfName("Widget"))
		dict.Set("Rect", core.NewPdfArray(
			core.NewPdfReal(f.Rect[0]),
			core.NewPdfReal(f.Rect[1]),
			core.NewPdfReal(f.Rect[2]),
			core.NewPdfReal(f.Rect[3]),
		))
		dict.Set("P", pageRefs[f.PageIndex])

		// Default appearance string.
		if f.FontName != "" {
			da := buildDA(f)
			dict.Set("DA", core.NewPdfLiteralString(da))
		}

		// Border.
		if f.BorderColor != nil {
			bc := f.BorderColor
			mkDict := core.NewPdfDictionary()
			mkDict.Set("BC", core.NewPdfArray(
				core.NewPdfReal(bc[0]), core.NewPdfReal(bc[1]), core.NewPdfReal(bc[2]),
			))
			if f.BGColor != nil {
				bg := f.BGColor
				mkDict.Set("BG", core.NewPdfArray(
					core.NewPdfReal(bg[0]), core.NewPdfReal(bg[1]), core.NewPdfReal(bg[2]),
				))
			}
			dict.Set("MK", mkDict)
		}

		// Appearance for checkboxes.
		if f.Type == FieldCheckbox {
			ap := buildCheckboxAppearance(f, addObject)
			dict.Set("AP", ap)
			if f.Value == "Yes" {
				dict.Set("AS", core.NewPdfName("Yes"))
			} else {
				dict.Set("AS", core.NewPdfName("Off"))
			}
		}

		widgets = append(widgets, &widgetInfo{ref: fieldRef, pageIndex: f.PageIndex})
	}

	return fieldRef, widgets
}

// widgetInfo pairs a widget annotation's indirect reference with the page it belongs to.
type widgetInfo struct {
	// ref is the indirect reference to the widget annotation object.
	ref *core.PdfIndirectReference
	// pageIndex is the zero-based index of the page containing this widget.
	pageIndex int
}
