// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build cgo && !js && !wasm

package main

/*
#include <stdint.h>
*/
import "C"
import (
	"fmt"
	"unsafe"

	"github.com/carlos7ags/folio/forms"
)

// ── Form field configuration ───────────────────────────────────────

//export folio_form_add_multiline_text_field
func folio_form_add_multiline_text_field(formH C.uint64_t, name *C.char, x1, y1, x2, y2 C.double, pageIndex C.int32_t) C.int32_t {
	af, errCode := loadForm(formH)
	if errCode != errOK {
		return errCode
	}
	rect := [4]float64{float64(x1), float64(y1), float64(x2), float64(y2)}
	af.Add(forms.NewMultilineTextField(C.GoString(name), rect, int(pageIndex)))
	return errOK
}

//export folio_form_add_password_field
func folio_form_add_password_field(formH C.uint64_t, name *C.char, x1, y1, x2, y2 C.double, pageIndex C.int32_t) C.int32_t {
	af, errCode := loadForm(formH)
	if errCode != errOK {
		return errCode
	}
	rect := [4]float64{float64(x1), float64(y1), float64(x2), float64(y2)}
	af.Add(forms.NewPasswordField(C.GoString(name), rect, int(pageIndex)))
	return errOK
}

//export folio_form_add_listbox
func folio_form_add_listbox(formH C.uint64_t, name *C.char, x1, y1, x2, y2 C.double, pageIndex C.int32_t, options **C.char, optCount C.int32_t) C.int32_t {
	af, errCode := loadForm(formH)
	if errCode != errOK {
		return errCode
	}
	rect := [4]float64{float64(x1), float64(y1), float64(x2), float64(y2)}
	n := int(optCount)
	goOpts := make([]string, n)
	if n > 0 && options != nil {
		cArray := (*[1 << 20]*C.char)(unsafe.Pointer(options))[:n:n]
		for i := 0; i < n; i++ {
			goOpts[i] = C.GoString(cArray[i])
		}
	}
	af.Add(forms.NewListBox(C.GoString(name), rect, int(pageIndex), goOpts))
	return errOK
}

//export folio_form_add_radio_group
func folio_form_add_radio_group(formH C.uint64_t, name *C.char,
	values **C.char, rects *C.double, pageIndices *C.int32_t, count C.int32_t) C.int32_t {
	af, errCode := loadForm(formH)
	if errCode != errOK {
		return errCode
	}
	n := int(count)
	if n <= 0 {
		setLastError("radio group requires at least one option")
		return errInvalidArg
	}
	cValues := (*[1 << 20]*C.char)(unsafe.Pointer(values))[:n:n]
	cRects := (*[1 << 20]C.double)(unsafe.Pointer(rects))[: n*4 : n*4]
	cPages := (*[1 << 20]C.int32_t)(unsafe.Pointer(pageIndices))[:n:n]

	opts := make([]forms.RadioOption, n)
	for i := 0; i < n; i++ {
		opts[i] = forms.RadioOption{
			Value:     C.GoString(cValues[i]),
			Rect:      [4]float64{float64(cRects[i*4]), float64(cRects[i*4+1]), float64(cRects[i*4+2]), float64(cRects[i*4+3])},
			PageIndex: int(cPages[i]),
		}
	}
	af.Add(forms.NewRadioGroup(C.GoString(name), opts))
	return errOK
}

//export folio_form_field_set_value
func folio_form_field_set_value(fieldH C.uint64_t, value *C.char) C.int32_t {
	f, errCode := loadField(fieldH)
	if errCode != errOK {
		return errCode
	}
	f.SetValue(C.GoString(value))
	return errOK
}

//export folio_form_field_set_read_only
func folio_form_field_set_read_only(fieldH C.uint64_t) C.int32_t {
	f, errCode := loadField(fieldH)
	if errCode != errOK {
		return errCode
	}
	f.SetReadOnly()
	return errOK
}

//export folio_form_field_set_required
func folio_form_field_set_required(fieldH C.uint64_t) C.int32_t {
	f, errCode := loadField(fieldH)
	if errCode != errOK {
		return errCode
	}
	f.SetRequired()
	return errOK
}

//export folio_form_field_set_background_color
func folio_form_field_set_background_color(fieldH C.uint64_t, r, g, b C.double) C.int32_t {
	f, errCode := loadField(fieldH)
	if errCode != errOK {
		return errCode
	}
	f.SetBackgroundColor(float64(r), float64(g), float64(b))
	return errOK
}

//export folio_form_field_set_border_color
func folio_form_field_set_border_color(fieldH C.uint64_t, r, g, b C.double) C.int32_t {
	f, errCode := loadField(fieldH)
	if errCode != errOK {
		return errCode
	}
	f.SetBorderColor(float64(r), float64(g), float64(b))
	return errOK
}

// ── Form field creation returning handles ──────────────────────────

//export folio_form_create_text_field
func folio_form_create_text_field(name *C.char, x1, y1, x2, y2 C.double, pageIndex C.int32_t) C.uint64_t {
	rect := [4]float64{float64(x1), float64(y1), float64(x2), float64(y2)}
	f := forms.NewTextField(C.GoString(name), rect, int(pageIndex))
	return C.uint64_t(ht.store(f))
}

//export folio_form_create_checkbox
func folio_form_create_checkbox(name *C.char, x1, y1, x2, y2 C.double, pageIndex C.int32_t, checked C.int32_t) C.uint64_t {
	rect := [4]float64{float64(x1), float64(y1), float64(x2), float64(y2)}
	f := forms.NewCheckbox(C.GoString(name), rect, int(pageIndex), checked != 0)
	return C.uint64_t(ht.store(f))
}

//export folio_form_add_field
func folio_form_add_field(formH C.uint64_t, fieldH C.uint64_t) C.int32_t {
	af, errCode := loadForm(formH)
	if errCode != errOK {
		return errCode
	}
	f, errCode := loadField(fieldH)
	if errCode != errOK {
		return errCode
	}
	af.Add(f)
	return errOK
}

//export folio_form_field_free
func folio_form_field_free(fieldH C.uint64_t) {
	ht.delete(uint64(fieldH))
}

// ── Form Filling ───────────────────────────────────────────────────

//export folio_form_filler_new
func folio_form_filler_new(readerH C.uint64_t) C.uint64_t {
	r, errCode := loadReader(readerH)
	if errCode != errOK {
		return 0
	}
	ff := forms.NewFormFiller(r)
	return C.uint64_t(ht.store(ff))
}

//export folio_form_filler_field_names
func folio_form_filler_field_names(ffH C.uint64_t) C.uint64_t {
	ff, errCode := loadFormFiller(ffH)
	if errCode != errOK {
		return 0
	}
	names, err := ff.FieldNames()
	if err != nil {
		setLastError(err.Error())
		return 0
	}
	// Join with newlines and return as buffer.
	result := ""
	for i, n := range names {
		if i > 0 {
			result += "\n"
		}
		result += n
	}
	return C.uint64_t(ht.store(newCBuffer([]byte(result))))
}

//export folio_form_filler_get_value
func folio_form_filler_get_value(ffH C.uint64_t, fieldName *C.char) C.uint64_t {
	ff, errCode := loadFormFiller(ffH)
	if errCode != errOK {
		return 0
	}
	val, err := ff.GetValue(C.GoString(fieldName))
	if err != nil {
		setLastError(err.Error())
		return 0
	}
	return C.uint64_t(ht.store(newCBuffer([]byte(val))))
}

//export folio_form_filler_set_value
func folio_form_filler_set_value(ffH C.uint64_t, fieldName, value *C.char) C.int32_t {
	ff, errCode := loadFormFiller(ffH)
	if errCode != errOK {
		return errCode
	}
	if err := ff.SetValue(C.GoString(fieldName), C.GoString(value)); err != nil {
		return setErr(errInvalidArg, err)
	}
	return errOK
}

//export folio_form_filler_set_checkbox
func folio_form_filler_set_checkbox(ffH C.uint64_t, fieldName *C.char, checked C.int32_t) C.int32_t {
	ff, errCode := loadFormFiller(ffH)
	if errCode != errOK {
		return errCode
	}
	if err := ff.SetCheckbox(C.GoString(fieldName), checked != 0); err != nil {
		return setErr(errInvalidArg, err)
	}
	return errOK
}

//export folio_form_filler_free
func folio_form_filler_free(ffH C.uint64_t) {
	ht.delete(uint64(ffH))
}

// ── Helpers ────────────────────────────────────────────────────────

func loadField(h C.uint64_t) (*forms.Field, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid field handle")
		return nil, errInvalidHandle
	}
	f, ok := v.(*forms.Field)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not a form field (type %T)", uint64(h), v))
		return nil, errTypeMismatch
	}
	return f, errOK
}

func loadFormFiller(h C.uint64_t) (*forms.FormFiller, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid form filler handle")
		return nil, errInvalidHandle
	}
	ff, ok := v.(*forms.FormFiller)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not a form filler (type %T)", uint64(h), v))
		return nil, errTypeMismatch
	}
	return ff, errOK
}
