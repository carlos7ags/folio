// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package zugferd

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
)

// oldExampleXML is the hand-rolled CII XML literal that
// examples/zugferd/main.go embedded before it was ported onto this
// package (see git history). It carries no line items and no tax
// breakdown — TestXMLSemanticEquivalenceWithOldExample compares the
// package's BASIC-profile output against it on the fields the two
// have in common, and separately asserts the package's additions.
const oldExampleXML = `<?xml version="1.0" encoding="UTF-8"?>
<rsm:CrossIndustryInvoice
  xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"
  xmlns:ram="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100"
  xmlns:udt="urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100">
  <rsm:ExchangedDocumentContext>
    <ram:GuidelineSpecifiedDocumentContextParameter>
      <ram:ID>urn:factur-x.eu:1p0:basic</ram:ID>
    </ram:GuidelineSpecifiedDocumentContextParameter>
  </rsm:ExchangedDocumentContext>
  <rsm:ExchangedDocument>
    <ram:ID>2024-001</ram:ID>
    <ram:TypeCode>380</ram:TypeCode>
    <ram:IssueDateTime>
      <udt:DateTimeString format="102">20240115</udt:DateTimeString>
    </ram:IssueDateTime>
  </rsm:ExchangedDocument>
  <rsm:SupplyChainTradeTransaction>
    <ram:ApplicableHeaderTradeAgreement>
      <ram:SellerTradeParty>
        <ram:Name>ACME Corp</ram:Name>
      </ram:SellerTradeParty>
      <ram:BuyerTradeParty>
        <ram:Name>Example GmbH</ram:Name>
      </ram:BuyerTradeParty>
    </ram:ApplicableHeaderTradeAgreement>
    <ram:ApplicableHeaderTradeSettlement>
      <ram:InvoiceCurrencyCode>EUR</ram:InvoiceCurrencyCode>
      <ram:SpecifiedTradeSettlementHeaderMonetarySummation>
        <ram:TaxBasisTotalAmount>337.50</ram:TaxBasisTotalAmount>
        <ram:TaxTotalAmount currencyID="EUR">64.13</ram:TaxTotalAmount>
        <ram:GrandTotalAmount>401.63</ram:GrandTotalAmount>
        <ram:DuePayableAmount>401.63</ram:DuePayableAmount>
      </ram:SpecifiedTradeSettlementHeaderMonetarySummation>
    </ram:ApplicableHeaderTradeSettlement>
  </rsm:SupplyChainTradeTransaction>
</rsm:CrossIndustryInvoice>`

// xnode is a generic, namespace-aware XML node tree used to compare
// documents by element identity (namespace + local name) and text
// content rather than by raw bytes — indentation and attribute/prefix
// spelling differences are not semantic.
type xnode struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Chardata string     `xml:",chardata"`
	Children []xnode    `xml:",any"`
}

func parseXNode(t *testing.T, data []byte) *xnode {
	t.Helper()
	var n xnode
	if err := xml.Unmarshal(data, &n); err != nil {
		t.Fatalf("xml.Unmarshal: %v", err)
	}
	return &n
}

// child returns the first direct child element with the given local
// name, regardless of namespace, or nil.
func (n *xnode) child(local string) *xnode {
	for i := range n.Children {
		if n.Children[i].XMLName.Local == local {
			return &n.Children[i]
		}
	}
	return nil
}

// children returns all direct child elements with the given local name.
func (n *xnode) children(local string) []*xnode {
	var out []*xnode
	for i := range n.Children {
		if n.Children[i].XMLName.Local == local {
			out = append(out, &n.Children[i])
		}
	}
	return out
}

// at navigates a dotted path of local names from n, failing the test
// if any segment is missing.
func (n *xnode) at(t *testing.T, path string) *xnode {
	t.Helper()
	cur := n
	for _, seg := range strings.Split(path, ".") {
		next := cur.child(seg)
		if next == nil {
			t.Fatalf("missing element %q under path %q", seg, path)
		}
		cur = next
	}
	return cur
}

func (n *xnode) text() string {
	return strings.TrimSpace(n.Chardata)
}

func (n *xnode) attr(local string) string {
	for _, a := range n.Attrs {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

// TestXMLSemanticEquivalenceWithOldExample is the spike's acceptance
// test (see package doc): it asserts the package's BASIC-profile CII
// output carries the same values, in the same structural positions,
// as the string literal examples/zugferd/main.go used to hand-roll
// before it was ported onto this package. Byte equality is not
// required — only element identity (namespace + local name) and
// text/attribute content, which is what a validator actually checks.
func TestXMLSemanticEquivalenceWithOldExample(t *testing.T) {
	got, err := sampleInvoice().XML(ProfileBasic)
	if err != nil {
		t.Fatalf("XML: %v", err)
	}

	oldRoot := parseXNode(t, []byte(oldExampleXML))
	newRoot := parseXNode(t, got)

	// --- Common subset: every field the old literal carried. ---
	if got, want := oldRoot.at(t, "ExchangedDocumentContext.GuidelineSpecifiedDocumentContextParameter.ID").text(),
		newRoot.at(t, "ExchangedDocumentContext.GuidelineSpecifiedDocumentContextParameter.ID").text(); got != want {
		t.Errorf("GuidelineID = %q, want %q", want, got)
	}
	if got, want := oldRoot.at(t, "ExchangedDocument.ID").text(), newRoot.at(t, "ExchangedDocument.ID").text(); got != want {
		t.Errorf("ExchangedDocument.ID = %q, want %q", want, got)
	}
	if got, want := oldRoot.at(t, "ExchangedDocument.TypeCode").text(), newRoot.at(t, "ExchangedDocument.TypeCode").text(); got != want {
		t.Errorf("TypeCode = %q, want %q", want, got)
	}
	oldDate := oldRoot.at(t, "ExchangedDocument.IssueDateTime.DateTimeString")
	newDate := newRoot.at(t, "ExchangedDocument.IssueDateTime.DateTimeString")
	if oldDate.text() != newDate.text() {
		t.Errorf("IssueDateTime text = %q, want %q", newDate.text(), oldDate.text())
	}
	if oldDate.attr("format") != newDate.attr("format") {
		t.Errorf("IssueDateTime format = %q, want %q", newDate.attr("format"), oldDate.attr("format"))
	}

	oldAgreement := oldRoot.at(t, "SupplyChainTradeTransaction.ApplicableHeaderTradeAgreement")
	newAgreement := newRoot.at(t, "SupplyChainTradeTransaction.ApplicableHeaderTradeAgreement")
	if got, want := oldAgreement.at(t, "SellerTradeParty.Name").text(), newAgreement.at(t, "SellerTradeParty.Name").text(); got != want {
		t.Errorf("SellerTradeParty.Name = %q, want %q", want, got)
	}
	if got, want := oldAgreement.at(t, "BuyerTradeParty.Name").text(), newAgreement.at(t, "BuyerTradeParty.Name").text(); got != want {
		t.Errorf("BuyerTradeParty.Name = %q, want %q", want, got)
	}

	oldSettlement := oldRoot.at(t, "SupplyChainTradeTransaction.ApplicableHeaderTradeSettlement")
	newSettlement := newRoot.at(t, "SupplyChainTradeTransaction.ApplicableHeaderTradeSettlement")
	if got, want := oldSettlement.at(t, "InvoiceCurrencyCode").text(), newSettlement.at(t, "InvoiceCurrencyCode").text(); got != want {
		t.Errorf("InvoiceCurrencyCode = %q, want %q", want, got)
	}

	oldTotals := oldSettlement.at(t, "SpecifiedTradeSettlementHeaderMonetarySummation")
	newTotals := newSettlement.at(t, "SpecifiedTradeSettlementHeaderMonetarySummation")
	for _, field := range []string{"TaxBasisTotalAmount", "GrandTotalAmount", "DuePayableAmount"} {
		if got, want := oldTotals.at(t, field).text(), newTotals.at(t, field).text(); got != want {
			t.Errorf("%s = %q, want %q", field, want, got)
		}
	}
	oldTax := oldTotals.at(t, "TaxTotalAmount")
	newTax := newTotals.at(t, "TaxTotalAmount")
	if oldTax.text() != newTax.text() {
		t.Errorf("TaxTotalAmount = %q, want %q", newTax.text(), oldTax.text())
	}
	if oldTax.attr("currencyID") != newTax.attr("currencyID") {
		t.Errorf("TaxTotalAmount currencyID = %q, want %q", newTax.attr("currencyID"), oldTax.attr("currencyID"))
	}

	// --- Additions: BASIC-profile elements the old literal lacked. ---
	if newTotals.child("LineTotalAmount") == nil {
		t.Error("new XML missing LineTotalAmount (BASIC addition over the old MINIMUM-ish example)")
	}
	newTransaction := newRoot.at(t, "SupplyChainTradeTransaction")
	if got := len(newTransaction.children("IncludedSupplyChainTradeLineItem")); got != 3 {
		t.Errorf("IncludedSupplyChainTradeLineItem count = %d, want 3 (old example had none)", got)
	}
	if got := len(newSettlement.children("ApplicableTradeTax")); got != 1 {
		t.Errorf("ApplicableTradeTax count = %d, want 1 (old example had none)", got)
	}
}

// TestXMLDeterministic asserts two generations from the same Invoice
// produce byte-identical XML — required so folio's deterministic
// document output isn't broken by embedding this attachment.
func TestXMLDeterministic(t *testing.T) {
	inv := sampleInvoice()
	a, err := inv.XML(ProfileBasic)
	if err != nil {
		t.Fatalf("XML: %v", err)
	}
	b, err := inv.XML(ProfileBasic)
	if err != nil {
		t.Fatalf("XML: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Error("two XML generations from the same Invoice differ")
	}
}

// TestXMLDateFormat102 asserts IssueDateTime uses CII format 102
// (YYYYMMDD).
func TestXMLDateFormat102(t *testing.T) {
	inv := sampleInvoice()
	out, err := inv.XML(ProfileBasic)
	if err != nil {
		t.Fatalf("XML: %v", err)
	}
	if !strings.Contains(string(out), `format="102">20240115<`) {
		t.Errorf("expected format=\"102\">20240115< in output, got:\n%s", out)
	}
}

// TestXMLMinimumProfileOmitsLinesAndTax asserts ProfileMinimum ignores
// Lines/TaxTotals entirely, even when present on the Invoice, and uses
// the MINIMUM guideline URN.
func TestXMLMinimumProfileOmitsLinesAndTax(t *testing.T) {
	out, err := sampleInvoice().XML(ProfileMinimum)
	if err != nil {
		t.Fatalf("XML: %v", err)
	}
	root := parseXNode(t, out)
	if got := root.at(t, "ExchangedDocumentContext.GuidelineSpecifiedDocumentContextParameter.ID").text(); got != "urn:factur-x.eu:1p0:minimum" {
		t.Errorf("GuidelineID = %q, want the MINIMUM URN", got)
	}
	transaction := root.at(t, "SupplyChainTradeTransaction")
	if n := len(transaction.children("IncludedSupplyChainTradeLineItem")); n != 0 {
		t.Errorf("IncludedSupplyChainTradeLineItem count = %d, want 0 for ProfileMinimum", n)
	}
	settlement := transaction.at(t, "ApplicableHeaderTradeSettlement")
	if n := len(settlement.children("ApplicableTradeTax")); n != 0 {
		t.Errorf("ApplicableTradeTax count = %d, want 0 for ProfileMinimum", n)
	}
}

// childIndex returns the index, in document order among all direct
// children of n, of the first child with the given local name, or -1
// if absent. Used to assert relative element order against the
// XSD-mandated xs:sequence for the elements this package emits.
func (n *xnode) childIndex(local string) int {
	for i := range n.Children {
		if n.Children[i].XMLName.Local == local {
			return i
		}
	}
	return -1
}

// assertOrder fails the test unless the given local names appear, among
// n's direct children, in strictly increasing index order (each must be
// present).
func assertOrder(t *testing.T, n *xnode, names ...string) {
	t.Helper()
	prevIdx := -1
	prevName := ""
	for _, name := range names {
		idx := n.childIndex(name)
		if idx < 0 {
			t.Fatalf("missing child %q (expected after %q)", name, prevName)
		}
		if idx <= prevIdx {
			t.Errorf("element order: %q (index %d) does not come after %q (index %d)", name, idx, prevName, prevIdx)
		}
		prevIdx, prevName = idx, name
	}
}

// TestXMLElementOrderMatchesSchema is a tag-free, schema-shaped
// regression net: it asserts the XSD-mandated child order (xs:sequence)
// of the elements this package emits, for the subset of the CII schema
// it covers. It complements (does not replace) the byte/semantic
// comparisons in TestXMLSemanticEquivalenceWithOldExample.
//
// The official Factur-X XSD is not committed to this repo (see
// xsd_test.go); the expected order below is the well-known UN/CEFACT
// Cross-Industry Invoice (CII) D16B sequence for
// rsm:CrossIndustryInvoice, rsm:SupplyChainTradeTransaction, and
// ram:SpecifiedTradeSettlementHeaderMonetarySummation.
func TestXMLElementOrderMatchesSchema(t *testing.T) {
	out, err := sampleInvoice().XML(ProfileBasic)
	if err != nil {
		t.Fatalf("XML: %v", err)
	}
	root := parseXNode(t, out)

	// Root: ExchangedDocumentContext, ExchangedDocument,
	// SupplyChainTradeTransaction.
	assertOrder(t, root,
		"ExchangedDocumentContext",
		"ExchangedDocument",
		"SupplyChainTradeTransaction")

	// SupplyChainTradeTransaction: IncludedSupplyChainTradeLineItem*,
	// ApplicableHeaderTradeAgreement, ApplicableHeaderTradeDelivery (not
	// emitted by this package), ApplicableHeaderTradeSettlement.
	transaction := root.at(t, "SupplyChainTradeTransaction")
	if transaction.childIndex("IncludedSupplyChainTradeLineItem") < 0 {
		t.Fatalf("sampleInvoice's BASIC output has no IncludedSupplyChainTradeLineItem to order-check")
	}
	assertOrder(t, transaction,
		"IncludedSupplyChainTradeLineItem",
		"ApplicableHeaderTradeAgreement",
		"ApplicableHeaderTradeSettlement")

	// SpecifiedTradeSettlementHeaderMonetarySummation: LineTotalAmount
	// ... GrandTotalAmount ... DuePayableAmount, as the XSD sequences
	// them (this package does not emit ChargeTotalAmount/
	// AllowanceTotalAmount, which the schema places between
	// TaxTotalAmount and GrandTotalAmount).
	summation := transaction.at(t, "ApplicableHeaderTradeSettlement.SpecifiedTradeSettlementHeaderMonetarySummation")
	assertOrder(t, summation,
		"LineTotalAmount",
		"TaxBasisTotalAmount",
		"TaxTotalAmount",
		"GrandTotalAmount",
		"DuePayableAmount")
}
