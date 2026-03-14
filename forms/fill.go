// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package forms

import (
	"fmt"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/reader"
)

// FormFiller reads form fields from an existing PDF and allows
// modifying their values before saving.
type FormFiller struct {
	reader *reader.PdfReader
}

// NewFormFiller creates a filler from a parsed PDF.
func NewFormFiller(r *reader.PdfReader) *FormFiller {
	return &FormFiller{reader: r}
}

// FieldNames returns the names of all form fields in the document.
func (ff *FormFiller) FieldNames() ([]string, error) {
	fields, err := ff.getFieldDicts()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, fd := range fields {
		if t := fd.Get("T"); t != nil {
			if s, ok := t.(*core.PdfString); ok {
				names = append(names, s.Value)
			}
		}
	}
	return names, nil
}

// GetValue returns the current value of a form field.
func (ff *FormFiller) GetValue(fieldName string) (string, error) {
	fields, err := ff.getFieldDicts()
	if err != nil {
		return "", err
	}
	for _, fd := range fields {
		if nameMatch(fd, fieldName) {
			return fieldValue(fd), nil
		}
	}
	return "", fmt.Errorf("forms: field %q not found", fieldName)
}

// SetValue sets the value of a form field.
// The change is applied to the in-memory PDF objects.
// Call reader.Merge to save the modified document.
func (ff *FormFiller) SetValue(fieldName, value string) error {
	fields, err := ff.getFieldDicts()
	if err != nil {
		return err
	}
	for _, fd := range fields {
		if nameMatch(fd, fieldName) {
			fd.Set("V", core.NewPdfLiteralString(value))
			return nil
		}
	}
	return fmt.Errorf("forms: field %q not found", fieldName)
}

// SetCheckbox sets a checkbox field's value.
// checked=true sets it to the export value, false sets to "Off".
func (ff *FormFiller) SetCheckbox(fieldName string, checked bool) error {
	fields, err := ff.getFieldDicts()
	if err != nil {
		return err
	}
	for _, fd := range fields {
		if nameMatch(fd, fieldName) {
			if checked {
				// Use the export value from /AP/N keys, or default "Yes".
				exportVal := "Yes"
				if ap := fd.Get("AP"); ap != nil {
					if apDict, ok := ap.(*core.PdfDictionary); ok {
						if n := apDict.Get("N"); n != nil {
							if nDict, ok := n.(*core.PdfDictionary); ok {
								for _, entry := range nDict.Entries {
									if entry.Key.Value != "Off" {
										exportVal = entry.Key.Value
										break
									}
								}
							}
						}
					}
				}
				fd.Set("V", core.NewPdfName(exportVal))
				fd.Set("AS", core.NewPdfName(exportVal))
			} else {
				fd.Set("V", core.NewPdfName("Off"))
				fd.Set("AS", core.NewPdfName("Off"))
			}
			return nil
		}
	}
	return fmt.Errorf("forms: field %q not found", fieldName)
}

// getFieldDicts extracts all field dictionaries from the AcroForm.
func (ff *FormFiller) getFieldDicts() ([]*core.PdfDictionary, error) {
	catalog := ff.reader.Catalog()
	acroFormObj := catalog.Get("AcroForm")
	if acroFormObj == nil {
		return nil, fmt.Errorf("forms: document has no AcroForm")
	}

	acroForm, err := ff.reader.ResolveObject(acroFormObj)
	if err != nil {
		return nil, err
	}
	formDict, ok := acroForm.(*core.PdfDictionary)
	if !ok {
		return nil, fmt.Errorf("forms: AcroForm is not a dictionary")
	}

	fieldsObj := formDict.Get("Fields")
	if fieldsObj == nil {
		return nil, fmt.Errorf("forms: AcroForm has no Fields")
	}
	fieldsResolved, err := ff.reader.ResolveObject(fieldsObj)
	if err != nil {
		return nil, err
	}
	fieldsArr, ok := fieldsResolved.(*core.PdfArray)
	if !ok {
		return nil, fmt.Errorf("forms: Fields is not an array")
	}

	var result []*core.PdfDictionary
	for _, fieldRef := range fieldsArr.Elements {
		fieldObj, err := ff.reader.ResolveObject(fieldRef)
		if err != nil {
			continue
		}
		if fd, ok := fieldObj.(*core.PdfDictionary); ok {
			result = append(result, fd)
			// Also collect children (for radio groups).
			if kids := fd.Get("Kids"); kids != nil {
				kidsResolved, err := ff.reader.ResolveObject(kids)
				if err == nil {
					if kidsArr, ok := kidsResolved.(*core.PdfArray); ok {
						for _, kidRef := range kidsArr.Elements {
							kidObj, err := ff.reader.ResolveObject(kidRef)
							if err == nil {
								if kd, ok := kidObj.(*core.PdfDictionary); ok {
									result = append(result, kd)
								}
							}
						}
					}
				}
			}
		}
	}

	return result, nil
}

// nameMatch checks if a field dictionary has the given name.
func nameMatch(fd *core.PdfDictionary, name string) bool {
	t := fd.Get("T")
	if t == nil {
		return false
	}
	if s, ok := t.(*core.PdfString); ok {
		return s.Value == name
	}
	return false
}

// fieldValue extracts the string value from a field dictionary.
func fieldValue(fd *core.PdfDictionary) string {
	v := fd.Get("V")
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case *core.PdfString:
		return val.Value
	case *core.PdfName:
		return val.Value
	default:
		return ""
	}
}
