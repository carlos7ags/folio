// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package zugferd

import (
	"encoding/xml"
	"fmt"
)

// CII element namespaces (UN/CEFACT Cross-Industry Invoice D16B), as
// declared on the rsm:CrossIndustryInvoice root element.
const (
	nsRSM = "urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"
	nsRAM = "urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100"
	nsUDT = "urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100"
)

// The element tags below spell out their CII namespace prefix
// ("rsm:", "ram:", "udt:") directly in the Go struct tag's local name.
// encoding/xml treats an unspaced tag as an opaque local name, so it
// writes the prefix verbatim without needing to manage a prefix-to-URI
// binding itself; the xmlns:* declarations are emitted as ordinary
// attributes on the root element below.

type ciiInvoice struct {
	XMLName  xml.Name `xml:"rsm:CrossIndustryInvoice"`
	XMLNSRSM string   `xml:"xmlns:rsm,attr"`
	XMLNSRAM string   `xml:"xmlns:ram,attr"`
	XMLNSUDT string   `xml:"xmlns:udt,attr"`

	Context     ciiDocumentContext `xml:"rsm:ExchangedDocumentContext"`
	Document    ciiDocument        `xml:"rsm:ExchangedDocument"`
	Transaction ciiTransaction     `xml:"rsm:SupplyChainTradeTransaction"`
}

type ciiDocumentContext struct {
	GuidelineID string `xml:"ram:GuidelineSpecifiedDocumentContextParameter>ram:ID"`
}

type ciiDocument struct {
	ID            string          `xml:"ram:ID"`
	TypeCode      string          `xml:"ram:TypeCode"`
	IssueDateTime ciiIssueDateTag `xml:"ram:IssueDateTime"`
}

type ciiIssueDateTag struct {
	DateTimeString ciiFormattedDate `xml:"udt:DateTimeString"`
}

// ciiFormattedDate is udt:DateTimeString with a format="102" attribute
// (CII format 102 = YYYYMMDD, ISO 8601 basic date).
type ciiFormattedDate struct {
	Format string `xml:"format,attr"`
	Value  string `xml:",chardata"`
}

type ciiTransaction struct {
	LineItems  []ciiLineItem `xml:"ram:IncludedSupplyChainTradeLineItem,omitempty"`
	Agreement  ciiAgreement  `xml:"ram:ApplicableHeaderTradeAgreement"`
	Settlement ciiSettlement `xml:"ram:ApplicableHeaderTradeSettlement"`
}

type ciiAgreement struct {
	Seller ciiTradeParty `xml:"ram:SellerTradeParty"`
	Buyer  ciiTradeParty `xml:"ram:BuyerTradeParty"`
}

type ciiTradeParty struct {
	Name            string              `xml:"ram:Name"`
	TaxRegistration *ciiTaxRegistration `xml:"ram:SpecifiedTaxRegistration,omitempty"`
}

type ciiTaxRegistration struct {
	ID ciiSchemedID `xml:"ram:ID"`
}

// ciiSchemedID is a ram:ID with a schemeID attribute, e.g.
// <ram:ID schemeID="VA">DE123456789</ram:ID> for a VAT number.
type ciiSchemedID struct {
	SchemeID string `xml:"schemeID,attr"`
	Value    string `xml:",chardata"`
}

type ciiSettlement struct {
	InvoiceCurrencyCode string             `xml:"ram:InvoiceCurrencyCode"`
	ApplicableTradeTax  []ciiHeaderTax     `xml:"ram:ApplicableTradeTax,omitempty"`
	PaymentTerms        *ciiPaymentTerms   `xml:"ram:SpecifiedTradePaymentTerms,omitempty"`
	MonetarySummation   ciiMonetarySummary `xml:"ram:SpecifiedTradeSettlementHeaderMonetarySummation"`
}

type ciiHeaderTax struct {
	CalculatedAmount      string `xml:"ram:CalculatedAmount"`
	TypeCode              string `xml:"ram:TypeCode"`
	BasisAmount           string `xml:"ram:BasisAmount"`
	CategoryCode          string `xml:"ram:CategoryCode"`
	RateApplicablePercent string `xml:"ram:RateApplicablePercent"`
}

type ciiPaymentTerms struct {
	Description string `xml:"ram:Description"`
}

type ciiMonetarySummary struct {
	LineTotalAmount     string            `xml:"ram:LineTotalAmount,omitempty"`
	TaxBasisTotalAmount string            `xml:"ram:TaxBasisTotalAmount"`
	TaxTotalAmount      ciiCurrencyAmount `xml:"ram:TaxTotalAmount"`
	GrandTotalAmount    string            `xml:"ram:GrandTotalAmount"`
	DuePayableAmount    string            `xml:"ram:DuePayableAmount"`
}

// ciiCurrencyAmount is an amount with a currencyID attribute, e.g.
// <ram:TaxTotalAmount currencyID="EUR">64.13</ram:TaxTotalAmount>.
type ciiCurrencyAmount struct {
	CurrencyID string `xml:"currencyID,attr"`
	Value      string `xml:",chardata"`
}

type ciiLineItem struct {
	LineDocument ciiLineDocument   `xml:"ram:AssociatedDocumentLineDocument"`
	Product      ciiTradeProduct   `xml:"ram:SpecifiedTradeProduct"`
	Agreement    ciiLineAgreement  `xml:"ram:SpecifiedLineTradeAgreement"`
	Delivery     ciiLineDelivery   `xml:"ram:SpecifiedLineTradeDelivery"`
	Settlement   ciiLineSettlement `xml:"ram:SpecifiedLineTradeSettlement"`
}

type ciiLineDocument struct {
	LineID string `xml:"ram:LineID"`
}

type ciiTradeProduct struct {
	Name string `xml:"ram:Name"`
}

type ciiLineAgreement struct {
	NetPrice ciiNetPrice `xml:"ram:NetPriceProductTradePrice"`
}

type ciiNetPrice struct {
	ChargeAmount string `xml:"ram:ChargeAmount"`
}

type ciiLineDelivery struct {
	BilledQuantity ciiQuantity `xml:"ram:BilledQuantity"`
}

// ciiQuantity is a quantity with a unitCode attribute (UN/ECE
// Recommendation 20), e.g. <ram:BilledQuantity unitCode="C62">10</ram:BilledQuantity>.
type ciiQuantity struct {
	UnitCode string `xml:"unitCode,attr"`
	Value    string `xml:",chardata"`
}

type ciiLineSettlement struct {
	ApplicableTradeTax ciiLineTax             `xml:"ram:ApplicableTradeTax"`
	MonetarySummation  ciiLineMonetarySummary `xml:"ram:SpecifiedTradeSettlementLineMonetarySummation"`
}

type ciiLineTax struct {
	TypeCode              string `xml:"ram:TypeCode"`
	CategoryCode          string `xml:"ram:CategoryCode"`
	RateApplicablePercent string `xml:"ram:RateApplicablePercent"`
}

type ciiLineMonetarySummary struct {
	LineTotalAmount string `xml:"ram:LineTotalAmount"`
}

// dateFormat102 renders a date in CII format 102 (YYYYMMDD).
func dateFormat102(inv *Invoice) string {
	return inv.IssueDate.Format("20060102")
}

// XML renders the invoice as EN 16931 CII XML for the given profile.
// The output is deterministic: field order is fixed by the struct
// layout and no data is sourced from map iteration or wall-clock time.
func (inv *Invoice) XML(p Profile) ([]byte, error) {
	if err := inv.Validate(p); err != nil {
		return nil, err
	}
	guideline, err := p.guidelineURN()
	if err != nil {
		return nil, err
	}

	doc := ciiInvoice{
		XMLNSRSM: nsRSM,
		XMLNSRAM: nsRAM,
		XMLNSUDT: nsUDT,
		Context: ciiDocumentContext{
			GuidelineID: guideline,
		},
		Document: ciiDocument{
			ID:       inv.Number,
			TypeCode: inv.typeCode(),
			IssueDateTime: ciiIssueDateTag{
				DateTimeString: ciiFormattedDate{Format: "102", Value: dateFormat102(inv)},
			},
		},
		Transaction: ciiTransaction{
			Agreement: ciiAgreement{
				Seller: tradeParty(inv.Seller),
				Buyer:  tradeParty(inv.Buyer),
			},
			Settlement: ciiSettlement{
				InvoiceCurrencyCode: inv.Currency,
				MonetarySummation: ciiMonetarySummary{
					TaxBasisTotalAmount: inv.Totals.TaxBasisTotal.String(),
					TaxTotalAmount:      ciiCurrencyAmount{CurrencyID: inv.Currency, Value: inv.Totals.TaxTotal.String()},
					GrandTotalAmount:    inv.Totals.GrandTotal.String(),
					DuePayableAmount:    inv.duePayable().String(),
				},
			},
		},
	}

	if inv.PaymentTerms != "" {
		doc.Transaction.Settlement.PaymentTerms = &ciiPaymentTerms{Description: inv.PaymentTerms}
	}

	if p == ProfileBasic {
		doc.Transaction.Settlement.MonetarySummation.LineTotalAmount = inv.Totals.LineTotal.String()

		for _, tt := range inv.TaxTotals {
			category := tt.CategoryCode
			if category == "" {
				category = "S"
			}
			doc.Transaction.Settlement.ApplicableTradeTax = append(doc.Transaction.Settlement.ApplicableTradeTax, ciiHeaderTax{
				CalculatedAmount:      tt.TaxAmount.String(),
				TypeCode:              "VAT",
				BasisAmount:           tt.TaxableAmount.String(),
				CategoryCode:          category,
				RateApplicablePercent: tt.RatePercent,
			})
		}

		for i, line := range inv.Lines {
			doc.Transaction.LineItems = append(doc.Transaction.LineItems, lineItemXML(i, line))
		}
	}

	out, err := xml.MarshalIndent(&doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("zugferd: marshal CII XML: %w", err)
	}
	return append([]byte(xml.Header), out...), nil
}

func tradeParty(party Party) ciiTradeParty {
	tp := ciiTradeParty{Name: party.Name}
	if party.VATID != "" {
		tp.TaxRegistration = &ciiTaxRegistration{ID: ciiSchemedID{SchemeID: "VA", Value: party.VATID}}
	}
	return tp
}

func lineItemXML(index int, line LineItem) ciiLineItem {
	id := line.ID
	if id == "" {
		id = fmt.Sprintf("%d", index+1)
	}
	unitCode := line.UnitCode
	if unitCode == "" {
		unitCode = "C62"
	}
	category := line.TaxCategoryCode
	if category == "" {
		category = "S"
	}
	return ciiLineItem{
		LineDocument: ciiLineDocument{LineID: id},
		Product:      ciiTradeProduct{Name: line.Description},
		Agreement: ciiLineAgreement{
			NetPrice: ciiNetPrice{ChargeAmount: line.UnitPrice.String()},
		},
		Delivery: ciiLineDelivery{
			BilledQuantity: ciiQuantity{UnitCode: unitCode, Value: line.Quantity},
		},
		Settlement: ciiLineSettlement{
			ApplicableTradeTax: ciiLineTax{
				TypeCode:              "VAT",
				CategoryCode:          category,
				RateApplicablePercent: line.TaxRatePercent,
			},
			MonetarySummation: ciiLineMonetarySummary{
				LineTotalAmount: line.LineTotal.String(),
			},
		},
	}
}
