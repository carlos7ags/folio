// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func xobjectStream(dict, data string) string {
	return fmt.Sprintf("<< %s /Length %d >>\nstream\n%s\nendstream", dict, len(data), data)
}

// formChainPDF returns a page drawing form X0, where each of n forms draws
// the next one twice and the last strokes a line and shows "leaf": 2^(n-1)
// leaf draws from a file that grows linearly in n.
func formChainPDF(n int) []byte {
	var names []string
	for i := 0; i < n; i++ {
		names = append(names, fmt.Sprintf("/X%d %d 0 R", i, 6+i))
	}
	res := fmt.Sprintf("/Font << /F1 5 0 R >> /XObject << %s >>", strings.Join(names, " "))
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Contents 4 0 R /Resources << %s >> >>", res),
		xobjectStream("", "/X0 Do"),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
	}
	for i := 0; i < n; i++ {
		body := "0 0 m 1 1 l S BT /F1 1 Tf (leaf) Tj ET"
		if i < n-1 {
			body = fmt.Sprintf("/X%d Do /X%d Do", i+1, i+1)
		}
		objs = append(objs, xobjectStream(fmt.Sprintf("/Type /XObject /Subtype /Form /BBox [0 0 1 1] /Resources << %s >>", res), body))
	}
	return buildPDFFromObjects(objs)
}

func firstPage(t *testing.T, data []byte) *PageInfo {
	t.Helper()
	r, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	return page
}

func lowerFormBudget(t *testing.T, n int) {
	t.Helper()
	saved := maxFormOps
	maxFormOps = n
	t.Cleanup(func() { maxFormOps = saved })
}

// Without a budget, 40 forms mean 2^39 leaf draws: the call would not return.
func TestFormFanOutExceedsBudget(t *testing.T) {
	lowerFormBudget(t, 100_000)
	page := firstPage(t, formChainPDF(40))

	if _, err := page.TextSpans(); !errors.Is(err, ErrFormBudgetExceeded) {
		t.Errorf("TextSpans: got %v, want ErrFormBudgetExceeded", err)
	}
	if _, err := page.PathOps(); !errors.Is(err, ErrFormBudgetExceeded) {
		t.Errorf("PathOps: got %v, want ErrFormBudgetExceeded", err)
	}
	if _, err := page.ExtractTextWithStrategy(&SimpleStrategy{}); !errors.Is(err, ErrFormBudgetExceeded) {
		t.Errorf("ExtractTextWithStrategy: got %v, want ErrFormBudgetExceeded", err)
	}
}

func TestFormFanOutWithinBudget(t *testing.T) {
	lowerFormBudget(t, 100_000)
	page := firstPage(t, formChainPDF(6)) // 32 leaves

	spans, err := page.TextSpans()
	if err != nil {
		t.Fatalf("TextSpans: %v", err)
	}
	if len(spans) != 32 {
		t.Errorf("got %d spans, want 32", len(spans))
	}
	paths, err := page.PathOps()
	if err != nil {
		t.Fatalf("PathOps: %v", err)
	}
	if len(paths) != 64 { // a move and a line segment per leaf
		t.Errorf("got %d path segments, want 64", len(paths))
	}
}

// The budget counts per Process call: a processor that stopped on one
// content stream processes the next one normally.
func TestFormBudgetResetsPerProcess(t *testing.T) {
	lowerFormBudget(t, 10)
	form := ParseContentStream([]byte("/Fm1 Do /Fm1 Do"))
	leaf := ParseContentStream([]byte("BT (x) Tj ET"))
	proc := NewContentProcessor(nil)
	proc.SetFormResolver(func(name string) []ContentOp {
		if name == "Fm1" {
			return form
		}
		return leaf
	})

	proc.Process(ParseContentStream([]byte("/Fm1 Do")))
	if !errors.Is(proc.Err(), ErrFormBudgetExceeded) {
		t.Fatalf("self-drawing form: got %v, want ErrFormBudgetExceeded", proc.Err())
	}
	spans := proc.Process(ParseContentStream([]byte("BT (top) Tj ET /Leaf Do")))
	if proc.Err() != nil {
		t.Fatalf("next stream: %v", proc.Err())
	}
	if len(spans) != 2 {
		t.Errorf("got %d spans, want 2", len(spans))
	}
}
