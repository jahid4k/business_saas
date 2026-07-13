# ADR-0019: HRM document templates — Markdown body, browser PDF, in-app acknowledgement

**Date:** 2026-07-07
**Status:** Accepted
**Deciders:** Mridha

---

## Context

HRM Group A4 (Document Template Engine) must support two workflows:

1. **Template-generated documents** — HR defines a template (offer letter, warning letter,
   promotion letter) with placeholders like `{{employee.first_name}}`. When an event occurs
   (a warning is issued, a promotion is approved), the system fills the placeholders and
   produces a document for the employee.

2. **Direct uploads** — HR uploads an existing file (a passport copy, a manually-signed NDA,
   a certificate scan) to the employee's record without using a template.

Both must support:
- Requiring the employee to formally acknowledge receipt
- Expiry tracking (visas, certifications, fixed-term contract end dates)
- Versioning (when a document is superseded by a newer version)

Three additional decisions must be made for template-generated documents:
1. What format should templates be stored in?
2. How is the PDF produced?
3. How is employee acknowledgement collected?

---

## Decision

**Template body format:** Markdown with `{{placeholder}}` syntax.

**PDF production:** Server stores the filled Markdown/HTML; the browser renders and prints
to PDF client-side. No server-side PDF generation library.

**Employee acknowledgement:** In-app acknowledgement via the shared `AcknowledgementRecord`
mechanism (C4). External e-signature services (DocuSign, HelloSign) are deferred.

**Document versioning:** Supersede-based — uploading a new version sets the old document's
`status = superseded` and links the old to the new via `superseded_by`. No binary diff,
no git-style history.

---

## Reasoning

### Markdown over HTML/WYSIWYG

**Markdown** is the best balance of expressiveness and safety for HR document templates:

- It is rich enough to produce professional-looking documents with headers, bold text,
  bullet lists, and tables — everything a standard offer letter or warning letter requires
- It is a text format — stored in a `TEXT` column, indexed for search, diffable in version
  control, readable in a raw API response without parsing
- The frontend renders it with a markdown parser (`marked`, `remark`, etc.) for preview
  and for the fill/print step — zero extra dependencies on the backend
- It avoids the XSS risk of storing raw HTML in the database

WYSIWYG rich text editors produce HTML that varies wildly by editor (Quill, Tiptap, Slate)
and is hard to sanitise server-side. Storing editor-specific HTML creates a vendor lock-in
to the editor library. Markdown avoids this.

### Placeholder syntax

`{{employee.first_name}}` uses double-brace syntax because it is visually distinct from
Markdown syntax and familiar to developers. The set of available placeholder paths is
stored as `available_variables TEXT[]` on the template, so the UI can show a tooltip
list of valid placeholders when the HR admin is writing the template body.

At document generation time, the backend performs a simple string replace — no expression
evaluation, no logic in templates. If conditional content is needed ("Dear Mr./Ms."),
the HR admin writes two variants of the letter rather than embedding logic. This is
intentional: document templates are not programs.

### Browser-side PDF instead of server-side generation

Server-side PDF generation is a major operational complexity:

| Library | Problem |
|---------|---------|
| wkhtmltopdf | Requires a headless Chromium install in the Docker image; adds ~400MB to the image; render bugs on complex CSS |
| Chromium/Puppeteer | Same image size problem; requires a separate process or a sidecar; security surface |
| gopdf / unipdf | Go-native but limited CSS support; pixel-perfect layout requires significant effort |
| LaTeX | Powerful but requires TeX installation and LaTeX syntax from HR admins |

The browser already knows how to render HTML+CSS and print to PDF via `window.print()`.
This is the mechanism used by every web-based invoice generator, payslip viewer, and
offer letter tool in production today.

**Flow:**
1. Backend generates the filled Markdown content and stores it as `generated_content` on
   `EmployeeDocument`
2. Frontend renders `generated_content` as HTML in a print-optimised layout
3. HR or employee clicks "Download PDF" → browser's print-to-PDF dialog opens
4. The resulting PDF file is stored client-side (or uploaded back via a future "save PDF"
   flow if needed)

This produces high-quality PDFs using the browser's native rendering engine, requires zero
server-side libraries, and works on any device with a browser. The tradeoff is that the
server does not hold a canonical PDF — only the rendered Markdown. This is acceptable: the
canonical record is the filled Markdown, and the PDF is a presentation layer.

### In-app acknowledgement over external e-signature

External e-signature services (DocuSign, HelloSign, PandaDoc) provide:
- Legally binding digital signatures in most jurisdictions
- Tamper-evident signature certificates
- Audit trails hosted by the provider

In-app acknowledgement provides:
- A timestamp in our own database: "Employee X clicked Acknowledge at 14:32:10 UTC"
- A record of the employee's IP and session at the time of acknowledgement (available via
  the request context)
- No external dependency, no per-signature cost, no vendor relationship required

For most HR documents — policy acknowledgements, warning letter receipts, promotion letter
confirmations — an in-app timestamp is legally sufficient and operationally simpler.

For documents where a legally binding signature is required (employment contracts, NDAs,
equity agreements in regulated industries), external e-signature integration is the correct
choice. This is an explicitly deferred feature. The `EmployeeDocument` schema has a
`document_type` field — a future `requires_legal_signature` flag on the template, combined
with an e-signature provider integration, can be added without schema changes.

### Declined acknowledgement as dispute mechanism

Employees can decline to acknowledge a document rather than acknowledging it. The
`AcknowledgementRecord.status = declined` with a free-text `declined_reason` creates a
formal record of the disagreement without requiring a separate dispute workflow. This pattern
is also used for payslip disputes (ADR-0017).

### Versioning via supersede, not history table

A history table (one row per version) was considered. Supersede-based versioning (same table,
`superseded_by` FK, `status = superseded`) was chosen because:
- All documents remain queryable in the same table with no JOIN to a history table
- The version chain is navigable: `WHERE superseded_by IS NULL` finds the current version
- Access control applies uniformly — old versions are visible to the same users as new ones
- Implementation complexity is lower

### Bulk send via batch record

When HR sends a policy document to 200 employees, creating 200 `EmployeeDocument` rows
individually in a loop introduces 200 round trips. The service layer uses a single batch
`INSERT ... SELECT` from the recipient list to create all rows in one query, linked by a
shared `bulk_send_batch_id` UUID. The `DocumentBulkSend` table records the operation for
HR's tracking purposes (pending/completed counts).

---

## Alternatives considered

| Option | Reason rejected |
|--------|----------------|
| HTML templates | XSS risk if raw HTML is stored; editor vendor lock-in; harder to sanitise |
| WYSIWYG editor format | Vendor lock-in to the editor library; non-portable across frontend frameworks |
| Server-side PDF (wkhtmltopdf) | +400MB Docker image; headless browser operational complexity; render bugs |
| Server-side PDF (gopdf) | Limited CSS; complex layout requires significant effort |
| External e-signature from day one | Per-signature cost; external vendor dependency; not required for most HR documents |
| History table for versions | Extra JOIN; same access control must be duplicated; supersede pattern is simpler |
| Template logic (conditionals, loops) | Document templates should not be programs; HR admins are not developers |

---

## Consequences

**Positive:**
- No server-side PDF generation dependency — Docker image stays small
- Markdown templates are readable as plain text, editable without a WYSIWYG editor, and
  portable across any future frontend framework changes
- In-app acknowledgement is zero-cost, zero-dependency, and sufficient for most HR use cases
- The supersede pattern makes document version management simple to implement and query

**Negative:**
- Server does not hold a canonical PDF — the PDF is produced client-side on demand; if a
  document needs to be attached to an email or stored in a filing system, a "save PDF" upload
  flow must be built in the future
- In-app acknowledgement is not legally equivalent to a wet signature or a provider-certified
  digital signature in all jurisdictions; HR must be informed of this limitation
- Complex document layouts (multi-column, precise positioning, custom fonts beyond browser
  defaults) are harder to achieve with Markdown → browser print than with a dedicated PDF engine
- External e-signature integration, when eventually needed, will require a new ADR and likely
  a schema change for signature certificate storage

---

## Related decisions

- [ADR-0014](0014-hrm-extended-architecture.md) — Document templates are Group A4; warning types
  (A3) and contracts (A7) depend on this existing first (FK constraint)
- [ADR-0016](0016-hrm-approval-chains.md) — Some templates require approval before being sent to
  an employee; the approval engine is reused
