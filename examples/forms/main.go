// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Forms demonstrates creating a PDF with interactive AcroForm fields:
//
//   - Text fields (single-line and multi-line)
//   - Password field
//   - Checkboxes
//   - Radio button groups
//   - Dropdown (combo box)
//   - List box
//   - Read-only and required fields
//   - Styled fields (colors, borders)
//
// Open the generated PDF in Adobe Acrobat or another form-capable viewer
// to interact with the fields. Note: macOS Preview has limited form support.
//
// Usage:
//
//	go run ./examples/forms
package main

import (
	"fmt"
	"os"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/forms"
)

func main() {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Folio Forms Showcase"
	doc.Info.Author = "Folio"

	// Page 1 handles the form fields.
	p := doc.AddPage()

	// --- Title ---
	p.AddText("Folio Forms Showcase", font.HelveticaBold, 20, 72, 740)
	p.AddText("Interactive AcroForm fields — open in Adobe Acrobat to edit.", font.Helvetica, 10, 72, 722)

	// --- Text Fields ---
	y := 690.0
	p.AddText("Text Fields", font.HelveticaBold, 13, 72, y)

	y -= 22
	p.AddText("Full Name:", font.Helvetica, 10, 72, y+4)
	nameField := forms.TextField("fullName", rect(180, y, 350, y+18), 0).
		SetValue("Jane Smith").
		SetRequired().
		SetBorderColor(0.6, 0.6, 0.6).
		SetBackgroundColor(1, 1, 0.95)

	y -= 26
	p.AddText("Email:", font.Helvetica, 10, 72, y+4)
	emailField := forms.TextField("email", rect(180, y, 350, y+18), 0).
		SetBorderColor(0.6, 0.6, 0.6)

	y -= 26
	p.AddText("Password:", font.Helvetica, 10, 72, y+4)
	passwordField := forms.PasswordField("password", rect(180, y, 350, y+18), 0).
		SetBorderColor(0.6, 0.6, 0.6)

	y -= 26
	p.AddText("Read-only:", font.Helvetica, 10, 72, y+4)
	readonlyField := forms.TextField("readonly", rect(180, y, 350, y+18), 0).
		SetValue("Cannot edit this").
		SetReadOnly().
		SetBackgroundColor(0.95, 0.95, 0.95)

	// --- Multi-line Text ---
	y -= 36
	p.AddText("Multi-line Text", font.HelveticaBold, 13, 72, y)

	y -= 22
	p.AddText("Comments:", font.Helvetica, 10, 72, y+40)
	commentsField := forms.MultilineTextField("comments", rect(180, y, 450, y+58), 0).
		SetValue("Enter your comments here...\nMultiple lines supported.").
		SetBorderColor(0.4, 0.4, 0.7)

	// --- Checkboxes ---
	y -= 36
	p.AddText("Checkboxes", font.HelveticaBold, 13, 72, y)

	y -= 22
	p.AddText("Agree to terms:", font.Helvetica, 10, 72, y+4)
	termsCheckbox := forms.Checkbox("agreeTerms", rect(180, y, 196, y+16), 0, true)

	y -= 22
	p.AddText("Subscribe:", font.Helvetica, 10, 72, y+4)
	subCheckbox := forms.Checkbox("subscribe", rect(180, y, 196, y+16), 0, false)

	// --- Radio Buttons ---
	y -= 36
	p.AddText("Radio Buttons", font.HelveticaBold, 13, 72, y)

	y -= 22
	p.AddText("Preference:", font.Helvetica, 10, 72, y+4)
	p.AddText("Option A", font.Helvetica, 9, 198, y+4)
	p.AddText("Option B", font.Helvetica, 9, 278, y+4)
	p.AddText("Option C", font.Helvetica, 9, 358, y+4)
	radioGroup := forms.RadioGroup("preference", []forms.RadioOption{
		{Value: "A", Rect: rect(180, y, 196, y+16), PageIndex: 0},
		{Value: "B", Rect: rect(260, y, 276, y+16), PageIndex: 0},
		{Value: "C", Rect: rect(340, y, 356, y+16), PageIndex: 0},
	})

	// --- Dropdown ---
	y -= 36
	p.AddText("Dropdown", font.HelveticaBold, 13, 72, y)

	y -= 22
	p.AddText("Country:", font.Helvetica, 10, 72, y+4)
	dropdown := forms.Dropdown("country", rect(180, y, 350, y+20), 0,
		[]string{"United States", "Germany", "Japan", "Brazil", "Australia"}).
		SetBorderColor(0.6, 0.6, 0.6)

	// --- List Box ---
	y -= 36
	p.AddText("List Box", font.HelveticaBold, 13, 72, y)

	y -= 70
	p.AddText("Languages:", font.Helvetica, 10, 72, y+50)
	listbox := forms.ListBox("languages", rect(180, y, 350, y+60), 0,
		[]string{"Go", "Python", "Rust", "TypeScript", "Java", "C++", "Ruby"}).
		SetBorderColor(0.4, 0.6, 0.4)

	// --- Styled Fields ---
	y -= 26
	p.AddText("Styled Fields", font.HelveticaBold, 13, 72, y)

	y -= 22
	p.AddText("Blue border:", font.Helvetica, 10, 72, y+4)
	blueField := forms.TextField("blue", rect(180, y, 350, y+18), 0).
		SetBorderColor(0.2, 0.3, 0.8).
		SetBackgroundColor(0.93, 0.95, 1.0)

	y -= 26
	p.AddText("Green border:", font.Helvetica, 10, 72, y+4)
	greenField := forms.TextField("green", rect(180, y, 350, y+18), 0).
		SetBorderColor(0.2, 0.6, 0.3).
		SetBackgroundColor(0.93, 1.0, 0.95).
		SetValue("Styled input")

	// --- Signature Field ---
	y -= 36
	p.AddText("Signature Field", font.HelveticaBold, 13, 72, y)

	y -= 22
	p.AddText("Sign here:", font.Helvetica, 10, 72, y+4)
	sigField := forms.SignatureField("signature", rect(180, y, 400, y+40), 0)

	// --- Build the form ---
	form := forms.NewAcroForm()
	form.Add(nameField)
	form.Add(emailField)
	form.Add(passwordField)
	form.Add(readonlyField)
	form.Add(commentsField)
	form.Add(termsCheckbox)
	form.Add(subCheckbox)
	form.Add(radioGroup)
	form.Add(dropdown)
	form.Add(listbox)
	form.Add(sigField)
	form.Add(blueField)
	form.Add(greenField)

	doc.SetAcroForm(form)

	if err := doc.Save("forms.pdf"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Created forms.pdf — open in Adobe Acrobat to interact with fields")
}

// rect creates a field rectangle [x1, y1, x2, y2].
func rect(x1, y1, x2, y2 float64) [4]float64 {
	return [4]float64{x1, y1, x2, y2}
}
