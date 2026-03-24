// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package forms

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
)

func generateFormPDF(t *testing.T, form *AcroForm) []byte {
	t.Helper()
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Form Test"

	p := doc.AddPage()
	p.AddText("Form below:", font.Helvetica, 12, 72, 750)

	doc.SetAcroForm(form)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	return buf.Bytes()
}

func TestTextFieldCreation(t *testing.T) {
	form := NewAcroForm()
	form.Add(TextField("name", [4]float64{72, 700, 300, 720}, 0).SetValue("John Doe"))

	data := generateFormPDF(t, form)
	pdf := string(data)

	if !strings.Contains(pdf, "/AcroForm") {
		t.Error("expected /AcroForm in catalog")
	}
	if !strings.Contains(pdf, "/FT /Tx") {
		t.Error("expected /FT /Tx for text field")
	}
	if !strings.Contains(pdf, "John Doe") {
		t.Error("expected field value")
	}
}

func TestCheckboxCreation(t *testing.T) {
	form := NewAcroForm()
	form.Add(Checkbox("agree", [4]float64{72, 680, 92, 700}, 0, true))
	form.Add(Checkbox("newsletter", [4]float64{72, 650, 92, 670}, 0, false))

	data := generateFormPDF(t, form)
	pdf := string(data)

	if !strings.Contains(pdf, "/FT /Btn") {
		t.Error("expected /FT /Btn for checkbox")
	}
	if !strings.Contains(pdf, "/AS /Yes") {
		t.Error("expected /AS /Yes for checked checkbox")
	}
}

func TestDropdownCreation(t *testing.T) {
	form := NewAcroForm()
	form.Add(Dropdown("country", [4]float64{72, 620, 250, 640}, 0,
		[]string{"USA", "Canada", "Mexico"}).SetValue("Canada"))

	data := generateFormPDF(t, form)
	pdf := string(data)

	if !strings.Contains(pdf, "/FT /Ch") {
		t.Error("expected /FT /Ch for choice field")
	}
	if !strings.Contains(pdf, "Canada") {
		t.Error("expected dropdown value")
	}
}

func TestRadioGroupCreation(t *testing.T) {
	form := NewAcroForm()
	form.Add(RadioGroup("size", []RadioOption{
		{Value: "S", Rect: [4]float64{72, 580, 92, 600}, PageIndex: 0},
		{Value: "M", Rect: [4]float64{102, 580, 122, 600}, PageIndex: 0},
		{Value: "L", Rect: [4]float64{132, 580, 152, 600}, PageIndex: 0},
	}))

	data := generateFormPDF(t, form)
	pdf := string(data)

	if !strings.Contains(pdf, "/FT /Btn") {
		t.Error("expected /FT /Btn for radio")
	}
	if !strings.Contains(pdf, "/Kids") {
		t.Error("expected /Kids for radio group")
	}
}

// TestRadioGroupUniqueAppearances verifies that each radio widget gets its
// own /AP with a unique export value. Without this, selecting one radio
// button would visually select all of them.
func TestRadioGroupUniqueAppearances(t *testing.T) {
	form := NewAcroForm()
	form.Add(RadioGroup("pref", []RadioOption{
		{Value: "A", Rect: [4]float64{72, 580, 92, 600}, PageIndex: 0},
		{Value: "B", Rect: [4]float64{102, 580, 122, 600}, PageIndex: 0},
		{Value: "C", Rect: [4]float64{132, 580, 152, 600}, PageIndex: 0},
	}))

	data := generateFormPDF(t, form)
	pdf := string(data)

	// Each export value must appear as a Name key in an /N appearance dict.
	for _, val := range []string{"/A ", "/B ", "/C "} {
		if !strings.Contains(pdf, val) {
			t.Errorf("expected radio export value %s in PDF", val)
		}
	}
	// Every radio widget should have /AP.
	apCount := strings.Count(pdf, "/AP")
	if apCount < 3 {
		t.Errorf("expected at least 3 /AP entries (one per radio), got %d", apCount)
	}
	// All widgets start as /Off.
	offCount := strings.Count(pdf, "/AS /Off")
	if offCount < 3 {
		t.Errorf("expected 3 /AS /Off entries, got %d", offCount)
	}
}

// TestCheckboxAppearancesHaveBorder verifies that checkbox appearance
// streams draw visible borders (not empty content).
func TestCheckboxAppearancesHaveBorder(t *testing.T) {
	form := NewAcroForm()
	form.Add(Checkbox("test", [4]float64{72, 680, 92, 700}, 0, false))

	data := generateFormPDF(t, form)
	pdf := string(data)

	// Should have /AP dictionary.
	if !strings.Contains(pdf, "/AP") {
		t.Error("expected /AP dictionary on checkbox widget")
	}
	// The Off appearance should have content (border drawing).
	// Before the fix, the Off stream was empty.
	if strings.Count(pdf, "/AP") < 1 {
		t.Error("checkbox missing appearance dictionary")
	}
	// The appearance streams should contain drawing operators.
	// "re S" = rectangle stroke (the border).
	if !strings.Contains(pdf, "re S") {
		t.Error("expected border drawing (re S) in checkbox appearance")
	}
}

// TestDropdownOptions verifies that the /Opt array contains all options.
func TestDropdownOptions(t *testing.T) {
	opts := []string{"USA", "Canada", "Mexico", "Brazil"}
	form := NewAcroForm()
	form.Add(Dropdown("country", [4]float64{72, 620, 250, 640}, 0, opts))

	data := generateFormPDF(t, form)
	pdf := string(data)

	if !strings.Contains(pdf, "/Opt") {
		t.Error("expected /Opt array in dropdown")
	}
	for _, opt := range opts {
		if !strings.Contains(pdf, opt) {
			t.Errorf("expected option %q in PDF", opt)
		}
	}
}

// TestListBoxOptions verifies that list box has /Opt array and is not
// a combo box (no /Combo flag).
func TestListBoxOptions(t *testing.T) {
	opts := []string{"Go", "Rust", "Python"}
	form := NewAcroForm()
	form.Add(ListBox("lang", [4]float64{72, 400, 250, 500}, 0, opts))

	data := generateFormPDF(t, form)
	pdf := string(data)

	if !strings.Contains(pdf, "/FT /Ch") {
		t.Error("expected /FT /Ch for list box")
	}
	if !strings.Contains(pdf, "/Opt") {
		t.Error("expected /Opt array")
	}
	for _, opt := range opts {
		if !strings.Contains(pdf, opt) {
			t.Errorf("expected option %q in PDF", opt)
		}
	}
	// Combo box flag (bit 17 = 0x20000) should NOT be set for a list box.
	if strings.Contains(pdf, "/Ff 131072") {
		t.Error("list box should not have combo flag")
	}
}

func TestMultilineTextField(t *testing.T) {
	form := NewAcroForm()
	f := MultilineTextField("comments", [4]float64{72, 500, 400, 600}, 0)
	f.SetValue("Line 1\nLine 2")
	form.Add(f)

	data := generateFormPDF(t, form)
	pdf := string(data)

	// Should have multiline flag (bit 12 = 4096).
	if !strings.Contains(pdf, "/Ff") {
		t.Error("expected /Ff flags")
	}
}

func TestPasswordField(t *testing.T) {
	f := PasswordField("secret", [4]float64{72, 460, 300, 480}, 0)
	if f.Flags&FlagPassword == 0 {
		t.Error("expected password flag")
	}
}

func TestListBoxCreation(t *testing.T) {
	form := NewAcroForm()
	form.Add(ListBox("items", [4]float64{72, 400, 250, 500}, 0,
		[]string{"Apple", "Banana", "Cherry"}))

	data := generateFormPDF(t, form)
	if !strings.Contains(string(data), "/FT /Ch") {
		t.Error("expected choice field type")
	}
}

func TestSignatureField(t *testing.T) {
	form := NewAcroForm()
	form.Add(SignatureField("sig", [4]float64{72, 100, 250, 150}, 0))

	data := generateFormPDF(t, form)
	if !strings.Contains(string(data), "/FT /Sig") {
		t.Error("expected signature field type")
	}
}

func TestFieldSetters(t *testing.T) {
	f := TextField("test", [4]float64{0, 0, 100, 20}, 0)
	f.SetReadOnly().SetRequired()
	f.SetBackgroundColor(1, 1, 0.8)
	f.SetBorderColor(0, 0, 0)

	if f.Flags&FlagReadOnly == 0 {
		t.Error("expected readonly flag")
	}
	if f.Flags&FlagRequired == 0 {
		t.Error("expected required flag")
	}
	if f.BGColor == nil {
		t.Error("expected background color")
	}
	if f.BorderColor == nil {
		t.Error("expected border color")
	}
}

func TestMultipleFieldsOnPage(t *testing.T) {
	form := NewAcroForm()
	form.Add(TextField("first_name", [4]float64{72, 700, 250, 720}, 0))
	form.Add(TextField("last_name", [4]float64{260, 700, 450, 720}, 0))
	form.Add(TextField("email", [4]float64{72, 670, 450, 690}, 0))
	form.Add(Checkbox("agree", [4]float64{72, 640, 92, 660}, 0, false))
	form.Add(Dropdown("role", [4]float64{72, 610, 250, 630}, 0,
		[]string{"Developer", "Designer", "Manager"}))

	data := generateFormPDF(t, form)

	// Count field references in AcroForm.
	pdf := string(data)
	fieldCount := strings.Count(pdf, "/FT")
	if fieldCount < 5 {
		t.Errorf("expected at least 5 field types, found %d", fieldCount)
	}
}

func TestAcroFormBuild(t *testing.T) {
	form := NewAcroForm()
	form.Add(TextField("test", [4]float64{0, 0, 100, 20}, 0))

	pageRefs := []*core.PdfIndirectReference{
		core.NewPdfIndirectReference(1, 0),
	}

	addObj := func(obj core.PdfObject) *core.PdfIndirectReference {
		return core.NewPdfIndirectReference(99, 0)
	}

	formRef, widgets := form.Build(addObj, pageRefs)
	if formRef == nil {
		t.Fatal("expected form reference")
	}
	if len(widgets) == 0 {
		t.Error("expected widget annotations")
	}
	if _, ok := widgets[0]; !ok {
		t.Error("expected widgets on page 0")
	}
}

func TestEmptyFormBuild(t *testing.T) {
	form := NewAcroForm()
	addObj := func(obj core.PdfObject) *core.PdfIndirectReference {
		return core.NewPdfIndirectReference(1, 0)
	}
	ref, widgets := form.Build(addObj, nil)
	if ref != nil {
		t.Error("empty form should return nil")
	}
	if widgets != nil {
		t.Error("empty form should have no widgets")
	}
}

func TestFieldDA(t *testing.T) {
	f := TextField("test", [4]float64{0, 0, 100, 20}, 0)
	f.FontSize = 14
	f.TextColor = [3]float64{1, 0, 0}
	da := buildDA(f)
	if !strings.Contains(da, "/Helv 14 Tf") {
		t.Errorf("DA = %q, expected /Helv 14 Tf", da)
	}
	if !strings.Contains(da, "1 0 0 rg") {
		t.Errorf("DA = %q, expected red color", da)
	}
}

func TestFormQpdfCheck(t *testing.T) {
	qpdfPath, err := exec.LookPath("qpdf")
	if err != nil {
		t.Skip("qpdf not installed")
	}

	form := NewAcroForm()
	form.Add(TextField("name", [4]float64{72, 700, 300, 720}, 0).SetValue("Test User"))
	form.Add(Checkbox("agree", [4]float64{72, 670, 92, 690}, 0, true))
	form.Add(Dropdown("role", [4]float64{72, 640, 250, 660}, 0,
		[]string{"Dev", "Design", "PM"}).SetValue("Dev"))

	data := generateFormPDF(t, form)

	tmpFile := filepath.Join(t.TempDir(), "form.pdf")
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(qpdfPath, "--check", tmpFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("qpdf --check failed: %v\n%s", err, out)
	}
}
