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

	"github.com/carlos7ags/folio/barcode"
	"github.com/carlos7ags/folio/layout"
)

// folio_barcode_qr generates a QR code barcode and returns its handle.
//
//export folio_barcode_qr
func folio_barcode_qr(data *C.char) C.uint64_t {
	bc, err := barcode.NewQR(C.GoString(data))
	if err != nil {
		setLastError(err.Error())
		return 0
	}
	return C.uint64_t(ht.store(bc))
}

// folio_barcode_qr_ecc generates a QR code with the specified error correction level.
//
//export folio_barcode_qr_ecc
func folio_barcode_qr_ecc(data *C.char, level C.int32_t) C.uint64_t {
	bc, err := barcode.NewQRWithECC(C.GoString(data), barcode.ECCLevel(level))
	if err != nil {
		setLastError(err.Error())
		return 0
	}
	return C.uint64_t(ht.store(bc))
}

// folio_barcode_code128 generates a Code 128 barcode and returns its handle.
//
//export folio_barcode_code128
func folio_barcode_code128(data *C.char) C.uint64_t {
	bc, err := barcode.NewCode128(C.GoString(data))
	if err != nil {
		setLastError(err.Error())
		return 0
	}
	return C.uint64_t(ht.store(bc))
}

// folio_barcode_ean13 generates an EAN-13 barcode and returns its handle.
//
//export folio_barcode_ean13
func folio_barcode_ean13(data *C.char) C.uint64_t {
	bc, err := barcode.NewEAN13(C.GoString(data))
	if err != nil {
		setLastError(err.Error())
		return 0
	}
	return C.uint64_t(ht.store(bc))
}

// folio_barcode_width returns the pixel width of a barcode.
//
//export folio_barcode_width
func folio_barcode_width(bcH C.uint64_t) C.int32_t {
	bc, errCode := loadBarcode(bcH)
	if errCode != errOK {
		return 0
	}
	return C.int32_t(bc.Width())
}

// folio_barcode_height returns the pixel height of a barcode.
//
//export folio_barcode_height
func folio_barcode_height(bcH C.uint64_t) C.int32_t {
	bc, errCode := loadBarcode(bcH)
	if errCode != errOK {
		return 0
	}
	return C.int32_t(bc.Height())
}

// folio_barcode_element_new wraps a barcode in a layout element with the given display width.
//
//export folio_barcode_element_new
func folio_barcode_element_new(bcH C.uint64_t, width C.double) C.uint64_t {
	bc, errCode := loadBarcode(bcH)
	if errCode != errOK {
		return 0
	}
	be := layout.NewBarcodeElement(bc, float64(width))
	return C.uint64_t(ht.store(be))
}

// folio_barcode_element_set_height sets the display height of a barcode element in points.
//
//export folio_barcode_element_set_height
func folio_barcode_element_set_height(beH C.uint64_t, height C.double) C.int32_t {
	be, errCode := loadBarcodeElement(beH)
	if errCode != errOK {
		return errCode
	}
	be.SetHeight(float64(height))
	return errOK
}

// folio_barcode_element_set_align sets the alignment of a barcode element.
//
//export folio_barcode_element_set_align
func folio_barcode_element_set_align(beH C.uint64_t, align C.int32_t) C.int32_t {
	be, errCode := loadBarcodeElement(beH)
	if errCode != errOK {
		return errCode
	}
	be.SetAlign(layout.Align(align))
	return errOK
}

// folio_barcode_free removes a barcode handle from the handle table.
//
//export folio_barcode_free
func folio_barcode_free(bcH C.uint64_t) {
	ht.delete(uint64(bcH))
}

// folio_barcode_element_free removes a barcode element handle from the handle table.
//
//export folio_barcode_element_free
func folio_barcode_element_free(beH C.uint64_t) {
	ht.delete(uint64(beH))
}

func loadBarcode(h C.uint64_t) (*barcode.Barcode, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid barcode handle")
		return nil, errInvalidHandle
	}
	bc, ok := v.(*barcode.Barcode)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not a barcode (type %T)", uint64(h), v))
		return nil, errTypeMismatch
	}
	return bc, errOK
}

func loadBarcodeElement(h C.uint64_t) (*layout.BarcodeElement, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid barcode element handle")
		return nil, errInvalidHandle
	}
	be, ok := v.(*layout.BarcodeElement)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not a barcode element (type %T)", uint64(h), v))
		return nil, errTypeMismatch
	}
	return be, errOK
}
