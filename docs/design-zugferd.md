# zugferd: Factur-X/ZUGFeRD e-invoice generation

Package `github.com/carlos7ags/folio/zugferd` turns a typed `Invoice`
into an EN 16931 UN/CEFACT Cross-Industry Invoice (CII) XML document
and attaches it to a `document.Document` as a PDF/A-3B associated
file — the Factur-X/ZUGFeRD hybrid e-invoice format. It builds on two
primitives folio already ships: PDF/A-3 conformance with custom XMP
extension schemas (`document/pdfa.go`) and embedded files with
`/AFRelationship` (`document/attachment.go`).

## Standards background

Factur-X 1.0 (France) and ZUGFeRD 2.x (Germany) are the same
technical standard under two names. The embedded XML is UN/CEFACT
Cross-Industry Invoice (CII) D16B; EN 16931 defines the semantic
data model the XML carries. The standard defines five ascending
profiles: MINIMUM, BASIC WL, BASIC, EN 16931 (COMFORT), EXTENDED.
The guideline URN in `ExchangedDocumentContext` names the profile a
given document conforms to; a validator reads that URN and checks the
document against the matching rule set.

Three consistency rules span the PDF, the XMP metadata, and the XML,
and a hand-rolled string template cannot enforce any of them: the
attachment filename must be `factur-x.xml` (or `zugferd-invoice.xml`
for older ZUGFeRD 1.x), `AFRelationship` must be `Alternative` or
`Data` depending on profile, and the XMP `fx:ConformanceLevel` value
must match the profile encoded in the XML's guideline URN. `Invoice.Attach`
exists to keep those three in lockstep from one call site.

## Package API

```go
package zugferd

type Profile int

const (
	ProfileMinimum Profile = iota
	ProfileBasic
)

type Party struct {
	Name  string
	VATID string
}

type LineItem struct {
	ID              string
	Description     string
	Quantity        string
	UnitCode        string
	UnitPrice       Amount
	LineTotal       Amount
	TaxCategoryCode string
	TaxRatePercent  string
}

type TaxBreakdown struct {
	CategoryCode  string
	RatePercent   string
	TaxableAmount Amount
	TaxAmount     Amount
}

type MonetarySummation struct {
	LineTotal        Amount
	TaxBasisTotal    Amount
	TaxTotal         Amount
	GrandTotal       Amount
	DuePayableAmount Amount
}

type Invoice struct {
	Number       string
	TypeCode     string
	IssueDate    time.Time
	Currency     string
	Seller, Buyer Party
	Lines        []LineItem
	TaxTotals    []TaxBreakdown
	Totals       MonetarySummation
	PaymentTerms string
}

func (inv *Invoice) Validate(p Profile) error
func (inv *Invoice) XML(p Profile) ([]byte, error)
func (inv *Invoice) Attach(doc *document.Document, p Profile) error
```

This matches the shape sketched when the package was scoped, with two
adjustments the prototype forced:

**Amount is a minor-units integer type, not a string or a decimal
dependency.** `type Amount int64` stores cents; `NewAmount(units,
cents)` and `ParseAmount("401.63")` construct one, `.String()` renders
the fixed two-decimal form CII expects, `.Add` composes totals. This
keeps every money computation exact and keeps `Amount.String()`
output byte-stable, at the cost of only supporting two-decimal
currencies (EUR, USD, GBP, ...) — three-decimal (BHD) and zero-decimal
(JPY) currencies are out of scope for this spike. A plain validated
string was the other option considered; it was rejected because it
gives up the `.Add` arithmetic the totals-consistency validation rule
needs, without saving meaningful complexity over the minor-units type.

**CII XML uses struct tags with literal prefixed local names, not a
manual token-stream encoder.** A field tag like `` `xml:"rsm:ExchangedDocumentContext"` ``
has no space before the colon, so `encoding/xml`'s encoder treats the
whole string as an opaque local name and writes it verbatim — it never
tries to resolve `rsm` as a namespace prefix itself. The root element
separately declares `xmlns:rsm`, `xmlns:ram`, `xmlns:udt` as literal
attributes (tagged `xml:"xmlns:rsm,attr"`), so the emitted document is
namespace-valid XML even though the encoder isn't aware it's producing
namespaced elements. Decoding is the interesting asymmetry: `encoding/xml`'s
*decoder* is genuinely namespace-aware, so parsing this same output back
resolves `<rsm:ExchangedDocumentContext>` to `XMLName{Space: nsRSM, Local:
"ExchangedDocumentContext"}` — the prefix is stripped and the URI is
resolved for real. `zugferd/xml_test.go`'s semantic-equivalence test
relies on exactly this: it decodes both the old hand-rolled XML and the
package's output into a generic namespace-aware node tree and compares
by `(Space, Local)` identity, which is prefix-agnostic by construction.
No `xml.Encoder`/token-stream fallback was needed — struct tags produced
correct output on the first attempt, so the stdlib namespace-prefix
limitation flagged as a risk when this package was scoped did not
materialize for CII's three-namespace, non-mixed-content structure.

**`Attach` owns `SetPdfA` outright; it does not attempt to merge with a
caller's existing `PdfAConfig`.** `document.SetPdfA` takes a whole
`PdfAConfig` value and replaces whatever was there
(`d.pdfA = &config`), so `Attach` calling it a second time after the
caller's own call would silently discard the caller's `XMPSchemas`/
`XMPProperties`. A merge is possible in principle — concatenate the
`XMPSchemas` and `XMPProperties` slices, keep the caller's
`ICCProfile`/`OutputCondition` if set — but it requires `Document` to
expose its current `*PdfAConfig` (it doesn't; the field is unexported),
and doing that merge blind, without knowing whether the caller's schema
list already has a same-prefixed `fx` entry, risks producing invalid
duplicate schema declarations. For this spike `Attach` documents the
overwrite behavior in its doc comment and requires callers who need
both to call `SetPdfA` again *after* `Attach`, repeating the Factur-X
block themselves. A `PdfAConfig`-merging helper, or an `Attach` variant
that takes the caller's extra `XMPSchemas`/`XMPProperties` as
arguments, is open-question material (see below) rather than something
this spike hacks around.

## Profile strategy

Only MINIMUM and BASIC are implemented. Field-presence matrix:

| Field | MINIMUM | BASIC |
|---|---|---|
| `Number`, `IssueDate`, `Currency`, `Seller.Name`, `Buyer.Name` | required | required |
| `Totals` (`TaxBasisTotal`, `TaxTotal`, `GrandTotal`) | required, arithmetic-checked | required, arithmetic-checked |
| `Lines` | ignored even if set | required, ≥1 entry |
| `TaxTotals` | ignored even if set | required, ≥1 entry, sum must equal `Totals.TaxTotal` |
| `PaymentTerms` | optional | optional |

Guideline URNs (Factur-X 1.0):

| Profile | URN |
|---|---|
| MINIMUM | `urn:factur-x.eu:1p0:minimum` |
| BASIC | `urn:factur-x.eu:1p0:basic` |

`Attach` sets `fx:ConformanceLevel` to `p.String()` ("MINIMUM" /
"BASIC"), matching the profile passed to it — this is one of the three
consistency rules `Attach` exists to hold together.

## Validation strategy

Three options were on the table: (a) no validation — garbage in,
garbage out; (b) hand-written struct validation covering required
fields and totals arithmetic; (c) full XSD/Schematron validation
against the official Factur-X schema set.

(c) is rejected outright — it needs a schema-validation dependency,
and the repo's dependency invariant limits this module to
`golang.org/x/{image,net,text}` plus the stdlib. (a) produces XML that
silently violates a validator's required-field rules, defeating the
whole point of a typed package over the string-template status quo.

The package implements (b), in `Invoice.Validate`, with this exact
rule list:

- `Number`, `Currency` (3-letter, uppercase, ISO-4217-shaped —
  regex-checked, not looked up against the ISO 4217 table),
  `Seller.Name`, `Buyer.Name` non-empty; `IssueDate` non-zero — for
  both profiles.
- `Totals.TaxBasisTotal + Totals.TaxTotal == Totals.GrandTotal` —
  for both profiles.
- BASIC only: `Lines` non-empty; each line's `Description`,
  `Quantity`, `TaxRatePercent` non-empty; `TaxTotals` non-empty; sum
  of `TaxTotals[].TaxAmount == Totals.TaxTotal`.

Deliberately **not** validated in this spike: `Quantity * UnitPrice ==
LineTotal` arithmetic (Quantity is a caller-supplied decimal string,
not a typed decimal — see Open questions), currency-code lookup
against the real ISO 4217 list (regex shape only), and any
cross-field EN 16931 business rule beyond the two arithmetic checks
above (e.g. the full BR-CO series). This is explicitly the "schema-lite"
tier — enough to catch the mistakes a copy-paste string template
invites, not a Schematron replacement. `ValidationError` names the
`Profile` and the offending `Field` so a caller (or a test) can act on
the specific failure rather than pattern-matching an error string.

## Determinism

`Invoice.XML` output is byte-identical across repeated calls with the
same `Invoice` value: field order comes from Go struct field order
(fixed at compile time, not map iteration), `Amount.String()` is a
pure function of the integer minor-units value, and no field is
sourced from wall-clock time — `IssueDate` is caller-supplied.
`zugferd/xml_test.go`'s `TestXMLDeterministic` asserts this directly
by generating twice and comparing bytes. This preserves the
deterministic-output invariant the rest of folio's document output
already holds (see `document/deterministic_test.go`).

## Honest demand note

This package's priority is inferred from the EU/German B2B e-invoicing
regulatory trend (Factur-X/ZUGFeRD adoption, and the broader mandatory
e-invoicing rollout across several European jurisdictions), not from
measured user demand — folio has not received a feature request for
this. It was built because the PDF-side primitives (PDF/A-3, custom
XMP schemas, associated files) were already done and tested, and the
only existing e-invoice code path (`examples/zugferd/main.go`) was a
37-line hand-rolled XML string that any consuming project would have
had to copy and get right on their own. Treat the profile scope
(MINIMUM/BASIC only) and the validation depth as a starting point sized
to that inferred need, not a commitment shaped by real integration
feedback.

## Open questions

- **XRechnung**: German public-sector e-invoicing uses CII with a
  different (stricter) guideline URN and additional mandatory fields
  (e.g. a "Leitweg-ID" routing identifier). Does it belong in this
  package as another `Profile` constant, or is it different enough
  (different validation rule set, different mandatory fields) to
  warrant its own type?
- **Order-X / Delivery-X**: the same PDF/A-3 + embedded-XML hybrid
  mechanism, applied to purchase orders and delivery notes instead of
  invoices. If those are added, does the package name `zugferd`
  survive, or should the package be renamed to something
  document-type-agnostic (e.g. `einvoice`) with Factur-X as the first
  supported format? Renaming after a public release is a breaking
  change, so this should be decided before this package's first
  tagged release.
- **Validation depth**: is the hand-written rule list in `Validate`
  worth extending toward full EN 16931 business-rule coverage (the
  BR-CO/BR-S/BR-* series), or should the package stay schema-lite
  permanently and document a pointer to an external validator (e.g.
  running the official Schematron rules out-of-process) for callers
  who need certainty?
- **Profile roadmap**: EN 16931 (COMFORT) and EXTENDED are the
  natural next profiles. COMFORT adds delivery information, multiple
  payment means, and allowance/charge lines; EXTENDED adds further
  optional business terms. Who actually needs them, and does adding
  them require new `Invoice` fields that are unpopulated (and
  therefore silently absent from the XML) for existing MINIMUM/BASIC
  callers — i.e. is the field addition additive-only, or does it risk
  reshaping fields like `LineItem` in a breaking way?
- **Guideline URN pinning**: the package hard-codes the Factur-X 1.0
  URNs (`urn:factur-x.eu:1p0:*`). Should the guideline URN become
  caller-overridable (e.g. an `Invoice.GuidelineURN` override field)
  for forward compatibility with a future Factur-X/ZUGFeRD revision,
  or does that just let callers produce XML whose URN and structure
  disagree?
- **`Attach`'s ownership of `SetPdfA`**: covered above under Package
  API — does the package need a merge-aware variant, or an `Attach`
  overload that accepts the caller's additional `XMPSchemas`/
  `XMPProperties` to fold in, once `document.Document` exposes a way
  to read back its current `PdfAConfig`?
- **Reading side**: extracting and parsing `factur-x.xml` back out of
  a received invoice PDF needs embedded-file extraction on the reader
  side, which does not exist yet (`reader.PdfReader` has no
  `EmbeddedFiles()`/`Attachments()` accessor — `zugferd/attach_test.go`
  had to walk `Catalog()["AF"]` by hand via `ResolveObject` to test the
  write side). This is a separate, larger piece of work: a generic
  `reader` attachment-extraction API that `zugferd` could then build a
  CII-XML-parsing layer on top of.

## Spike limitations (not open questions — known gaps in this pass)

- Two-decimal currencies only (`Amount` assumes cents; JPY/BHD-style
  currencies are unsupported).
- `LineItem.Quantity` is an unvalidated decimal string; `Quantity *
  UnitPrice == LineTotal` is not checked.
- Currency codes are shape-validated (3 uppercase letters), not looked
  up against the real ISO 4217 list.
- No delivery-party/delivery-date block (`ApplicableHeaderTradeDelivery`)
  is emitted — invoices with a distinct ship-to party need it and this
  package doesn't produce it yet.
- No support for multiple tax breakdown entries interacting with
  allowances/charges (BR-CO discount rules) — `TaxBreakdown` entries
  are summed and checked against the header tax total, nothing more.
