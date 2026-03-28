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

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/forms"
)

// folio_form_new creates a new empty AcroForm and returns its handle.
//
//export folio_form_new
func folio_form_new() C.uint64_t {
	return C.uint64_t(ht.store(forms.NewAcroForm()))
}

// folio_form_add_text_field adds a text input field to the form at the given rectangle and page.
//
//export folio_form_add_text_field
func folio_form_add_text_field(formH C.uint64_t, name *C.char, x1, y1, x2, y2 C.double, pageIndex C.int32_t) C.int32_t {
	af, errCode := loadForm(formH)
	if errCode != errOK {
		return errCode
	}
	rect := [4]float64{float64(x1), float64(y1), float64(x2), float64(y2)}
	af.Add(forms.NewTextField(C.GoString(name), rect, int(pageIndex)))
	return errOK
}

// folio_form_add_checkbox adds a checkbox field to the form at the given rectangle and page.
//
//export folio_form_add_checkbox
func folio_form_add_checkbox(formH C.uint64_t, name *C.char, x1, y1, x2, y2 C.double, pageIndex C.int32_t, checked C.int32_t) C.int32_t {
	af, errCode := loadForm(formH)
	if errCode != errOK {
		return errCode
	}
	rect := [4]float64{float64(x1), float64(y1), float64(x2), float64(y2)}
	af.Add(forms.NewCheckbox(C.GoString(name), rect, int(pageIndex), checked != 0))
	return errOK
}

// folio_form_add_dropdown adds a dropdown (choice) field to the form with the given options.
//
//export folio_form_add_dropdown
func folio_form_add_dropdown(formH C.uint64_t, name *C.char, x1, y1, x2, y2 C.double, pageIndex C.int32_t, options **C.char, optCount C.int32_t) C.int32_t {
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
	af.Add(forms.NewDropdown(C.GoString(name), rect, int(pageIndex), goOpts))
	return errOK
}

// folio_form_add_signature adds a digital signature field to the form at the given rectangle and page.
//
//export folio_form_add_signature
func folio_form_add_signature(formH C.uint64_t, name *C.char, x1, y1, x2, y2 C.double, pageIndex C.int32_t) C.int32_t {
	af, errCode := loadForm(formH)
	if errCode != errOK {
		return errCode
	}
	rect := [4]float64{float64(x1), float64(y1), float64(x2), float64(y2)}
	af.Add(forms.NewSignatureField(C.GoString(name), rect, int(pageIndex)))
	return errOK
}

// folio_document_set_form attaches an AcroForm to the document.
//
//export folio_document_set_form
func folio_document_set_form(docH C.uint64_t, formH C.uint64_t) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	af, errCode := loadForm(formH)
	if errCode != errOK {
		return errCode
	}
	doc.SetAcroForm(af)
	return errOK
}

// folio_form_free removes a form handle from the handle table.
//
//export folio_form_free
func folio_form_free(formH C.uint64_t) {
	ht.delete(uint64(formH))
}

// folio_document_set_tagged enables or disables tagged PDF (accessibility) output.
//
//export folio_document_set_tagged
func folio_document_set_tagged(docH C.uint64_t, enabled C.int32_t) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	doc.SetTagged(enabled != 0)
	return errOK
}

// folio_document_set_pdfa configures PDF/A conformance at the specified level.
//
//export folio_document_set_pdfa
func folio_document_set_pdfa(docH C.uint64_t, level C.int32_t) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	doc.SetPdfA(document.PdfAConfig{Level: document.PdfALevel(level)})
	return errOK
}

// folio_document_set_encryption configures PDF encryption with user/owner passwords and an algorithm.
//
//export folio_document_set_encryption
func folio_document_set_encryption(docH C.uint64_t, userPw, ownerPw *C.char, algorithm C.int32_t) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	doc.SetEncryption(document.EncryptionConfig{
		Algorithm:     document.EncryptionAlgorithm(algorithm),
		UserPassword:  C.GoString(userPw),
		OwnerPassword: C.GoString(ownerPw),
	})
	return errOK
}

// folio_document_set_encryption_with_permissions configures PDF encryption with permissions.
// permissions is a bitmask of FOLIO_PERM_* flags.
//
//export folio_document_set_encryption_with_permissions
func folio_document_set_encryption_with_permissions(docH C.uint64_t, userPw, ownerPw *C.char, algorithm, permissions C.int32_t) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	doc.SetEncryption(document.EncryptionConfig{
		Algorithm:     document.EncryptionAlgorithm(algorithm),
		UserPassword:  C.GoString(userPw),
		OwnerPassword: C.GoString(ownerPw),
		Permissions:   core.Permission(permissions),
	})
	return errOK
}

// folio_document_set_auto_bookmarks enables or disables automatic bookmark generation from headings.
//
//export folio_document_set_auto_bookmarks
func folio_document_set_auto_bookmarks(docH C.uint64_t, enabled C.int32_t) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	doc.SetAutoBookmarks(enabled != 0)
	return errOK
}

// loadForm retrieves a *forms.AcroForm from the handle table.
func loadForm(h C.uint64_t) (*forms.AcroForm, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid form handle")
		return nil, errInvalidHandle
	}
	af, ok := v.(*forms.AcroForm)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not a form (type %T)", uint64(h), v))
		return nil, errTypeMismatch
	}
	return af, errOK
}
