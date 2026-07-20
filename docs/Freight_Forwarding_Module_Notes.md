# FREIGHT FORWARDING MODULE — NOTES & PLANNING DOC

> **Status:** Idea / pre-discovery. No requirements gathered from the client. No code written. Not yet an entry in `docs/Project_Instruction.md` Section 9.
> **Created:** 2026-07-19, from a planning conversation with Claude.
> **Purpose:** A standalone reference so this entire line of thinking can be picked back up later without re-deriving any of it. This doc is deliberately self-contained — it does not assume you remember the conversation that produced it.
> **Relationship to other docs:** This sits alongside `docs/Project_Instruction.md` and `Capture_Agent_Prompt.md` but is not part of either. Nothing here should be treated as decided or queued until Section 9 (Discovery Checklist) has actually been done with the client.

---

## 0. TL;DR — read this first

- **What:** A possible new vertical/module — freight forwarding — for BusinessSAAS. Trigger: a personal connection at a freight forwarding company who might buy the product.
- **Status:** Idea only. Nothing committed on either side.
- **Scope reality:** Freight forwarding is a mature, complex domain — comparable in size to CRM + HRM combined if built out fully. Treat it as a new vertical, not "a module you bolt on."
- **Before any code:** Run the discovery conversation in Section 9. Do not design the schema from assumptions about how freight forwarding "generally" works — audit the client's actual process, exactly the same discipline already used for the rest of BusinessSAAS (audit real source, don't trust assumptions).
- **When it's time to build:** Start with the Phase 1 MVP only (Section 15) — shipment tracking + basic billing, reusing `platform/contacts` and `platform/engagement`. New module lives at `internal/freight/`.
- **Sequencing:** Do not start until the CRM Capture Fix Pass (A/B) + capture frontend are shipped. One workstream at a time — this project has already seen what happens when that discipline slips (the capture module's r11 audit).
- **Biggest architectural insight to remember:** If/when the CRM Layer 2 custom fields engine gets built, consider generalizing it (`internal/platform/customfields/` with an `entity_type` column) instead of keeping it CRM-only — freight cargo attributes (weight, volume, HS code, DG class) need exactly the same kind of engine.

---

## 1. WHY THIS DOCUMENT EXISTS

This came out of a "give me everything" conversation about whether/how to add freight forwarding to BusinessSAAS. Nothing is being built right now. This doc exists purely so that when the time comes — whether that's in two weeks or eight months — all of the domain research, architectural thinking, and open questions are in one place instead of buried in a chat transcript.

---

## 2. TRIGGER / BUSINESS CONTEXT

- Mridha has a personal connection at a freight forwarding company and believes there's a real chance of selling BusinessSAAS to them.
- **Assumption (unconfirmed):** given Mridha is based in Dhaka, this is very likely a Bangladesh-based freight forwarder. Section 16 is written on that assumption — if it turns out wrong, skip that section.
- **Nothing has been validated yet.** "I can sell it to them" is an opportunity, not a signed requirement. Section 9 exists specifically to convert this from a hunch into a real spec.
- One nice coincidence: migration `00048_seed_data_vertex.sql` already seeds a fictional demo tenant called "Vertex Logistics," a freight-forwarding-flavored company, using the *existing generic CRM tables* (`crm_deals`, `crm_leads`, `platform_activities`, etc.) with a freight narrative (companies like "Atlas Maritime," "CargoLink"; deal titles like "Atlas Maritime Transpacific Fleet"; tasks like "Verify Bill of Lading with Atlas"). **This is not a real freight module** — no shipment, container, or B/L tables exist. But it's a genuinely good demo asset: the existing CRM already "looks like" a freight forwarding CRM out of the box, which could be useful for a first conversation/demo with the client before any freight-specific code exists.

---

## 3. REALITY CHECK

Freight forwarding software is a mature, competitive, well-funded market (see Section 8). This doesn't mean don't do it — it means:

1. A personal connection expressing interest is not the same as a validated requirement. Treat it the way the capture module's own audit discipline treats documentation: audit the real thing, don't build from assumptions.
2. This is architecturally closer to a **new vertical SaaS product** than a CRM feature. Full scope (documents, rates, billing, compliance, network/agent management) is arguably bigger than CRM + HRM combined.
3. Because of (2), the existing project discipline — **sequential workstreams, no parallel branches, one thing fully shipped before the next starts** — matters even more here than usual. The capture module's r11 audit ("architecture sound, not yet functional") is the cautionary tale for what happens when too much starts at once.
4. Smaller/regional players do carve out real space next to giants like CargoWise (see NuevaFlo, Linbis in Section 8) — so "the market has incumbents" is not a reason not to do this. It's a reason to be deliberate about who exactly this is for.

---

## 4. FREIGHT FORWARDING DOMAIN PRIMER

### 4.1 What a freight forwarder actually is

A freight forwarder is an **intermediary/coordinator**, not a carrier. They typically don't own ships, planes, or trucks — they use relationships with carriers to move a client's cargo, and they handle the paperwork, customs, and coordination in between. This is the single most important distinction to keep in mind when modeling the domain: the forwarder's product is *coordination + documentation + trust*, not transport capacity itself.

### 4.2 Key actors

| Actor | Role |
|---|---|
| **Shipper / Exporter** | Sends the goods — the forwarder's client, usually |
| **Consignee / Importer** | Receives the goods |
| **Notify Party** | Party notified on arrival (often = consignee, sometimes a bank in Letter of Credit deals) |
| **Carrier** | Actual transport provider — ocean line, airline, trucking company, rail operator |
| **Freight Forwarder** | The intermediary/coordinator — this is the client in question |
| **Customs Broker / C&F Agent** | Handles customs clearance — sometimes the same company as the forwarder, sometimes a separate licensed partner (this varies a lot by country — see Section 16 for Bangladesh specifics) |
| **Overseas Agent / Partner Forwarder** | Freight forwarding is fundamentally a **network business** — forwarders in different countries partner to handle each end of a shipment |
| **NVOCC** (Non-Vessel Operating Common Carrier) | A forwarder that issues its own House Bill of Lading and consolidates cargo, acting like a carrier to shippers without owning vessels |

### 4.3 Transport modes & shipment types

- **Ocean:** FCL (Full Container Load — one shipper, one container) vs. LCL (Less than Container Load — multiple shippers' cargo consolidated in one container at a CFS). Also breakbulk / project cargo / RoRo for non-containerized freight.
- **Air:** General cargo, express, charter; consolidated (co-load) vs. direct.
- **Road:** FTL (Full Truck Load) vs. LTL (Less than Truck Load); cross-border trucking matters for regional trade lanes.
- **Rail:** Less relevant for Bangladesh specifically, more relevant on corridors like China–Europe.
- **Multimodal:** Combinations of the above — e.g. door-to-door combining trucking + ocean + trucking.

### 4.4 End-to-end shipment lifecycle

The four-stage version (matches the diagram shown in chat):

```
Quote & Book  →  Origin  →  Main Carriage  →  Destination
```

The fuller, more granular version:

1. **Inquiry / RFQ** — shipper requests a quote for moving cargo from A to B
2. **Quotation** — forwarder checks rates (own contracted carrier rates, or spot rates) and quotes the shipper — all-in or itemized
3. **Booking** — shipper confirms; forwarder books space with the carrier (or an NVOCC/co-loader for LCL/consolidated air)
4. **Origin pickup / haulage** — cargo picked up from the shipper's warehouse, trucked to port/airport/CFS
5. **Export customs clearance** — export declaration filed
6. **Loading** — container stuffing (FCL) or CFS consolidation (LCL), or ULD build-up for air; loaded onto vessel/aircraft
7. **Main carriage** — the actual transit; visibility milestones (departed, in-transit, transshipment)
8. **Documents issued** — carrier issues Master B/L or Master AWB to the forwarder; forwarder issues its own House B/L or House AWB to the shipper if acting as NVOCC/consolidator
9. **Arrival at destination** — import customs clearance
10. **Destination handling** — deconsolidation (LCL), devanning, delivery order issuance
11. **Final delivery** — last-mile trucking to the consignee
12. **POD** (Proof of Delivery) — signed confirmation
13. **Invoicing & settlement** — forwarder invoices the shipper for freight + all accessorial charges, pays the carrier/agents; **margin = sell rate − buy rate**, and this is where the forwarder actually makes money

---

## 5. GLOSSARY

| Term | Meaning |
|---|---|
| FCL | Full Container Load — one shipper's cargo fills a whole container |
| LCL | Less than Container Load — multiple shippers consolidated into one container |
| FTL / LTL | Full / Less than Truck Load (road freight equivalent of FCL/LCL) |
| NVOCC | Non-Vessel Operating Common Carrier — forwarder that issues its own B/L without owning ships |
| B/L (MBL / HBL) | Bill of Lading — ocean freight's title document. Master (carrier→forwarder) vs. House (forwarder→shipper) |
| AWB (MAWB / HAWB) | Air Waybill — air freight's equivalent of a B/L, but never a negotiable title document |
| THC | Terminal Handling Charge — charged at origin and destination |
| CFS | Container Freight Station — where LCL cargo gets consolidated/deconsolidated |
| Incoterms | Standardized trade terms (EXW, FOB, CIF, DDP, etc.) defining who pays for and bears risk on which leg of the journey |
| BAF / CAF | Bunker / Currency Adjustment Factor — ocean freight surcharges |
| Demurrage | Charge for a container sitting at the port too long |
| Detention | Charge for a container sitting with the shipper/consignee too long |
| ISF | Importer Security Filing — US-specific advance filing for ocean imports |
| AMS / ENS | Advance Manifest Filings — US/EU security filings |
| HS Code | Harmonized System code — commodity classification used for customs |
| DG (Dangerous Goods) | Hazardous cargo classification — IMDG Code (ocean), IATA DGR (air) |
| C&F Agent | Clearing & Forwarding Agent — a customs clearance role, separately licensed in some countries (including Bangladesh) |
| DO | Delivery Order — document that authorizes cargo release at destination |
| POD | Proof of Delivery |

---

## 6. CORE DOCUMENTS & THE PAPER TRAIL

The core product of a freight forwarding system isn't really "shipment tracking" — it's **guaranteeing the right documents and money move correctly**. Document types the data model needs to account for:

| Document | Issued by | Purpose |
|---|---|---|
| Commercial Invoice | Shipper | States goods value; used for customs |
| Packing List | Shipper | Itemized packing detail (weights, dimensions, quantities) |
| Bill of Lading (ocean) | Carrier (Master) / Forwarder (House, if NVOCC) | Title document. Original = negotiable, needed for cargo release. Telex Release / Seaway Bill = non-negotiable, faster release. |
| Air Waybill (air) | Carrier (Master) / Forwarder (House) | Never a negotiable/title document, unlike ocean B/L |
| Certificate of Origin | Chamber of commerce / relevant authority | States country of manufacture — needed for tariffs and trade agreements (very relevant for Bangladesh RMG exports to EU under GSP, see Section 16) |
| Letter of Credit | Bank | Trade finance instrument; document compliance is critical when this is the payment method |
| Insurance Certificate | Insurer | Cargo insurance |
| Customs Declaration (export/import) | Forwarder / C&F Agent | Terminology varies by country (e.g., Bill of Entry / Shipping Bill) |
| Delivery Order | Carrier/agent at destination | Authorizes cargo release |
| Dangerous Goods Declaration | Shipper/forwarder | Only if DG cargo — IATA DGR (air) / IMDG (ocean) |

---

## 7. CHARGES & THE ECONOMICS OF FORWARDING

Typical line items on a freight invoice:

- Ocean/Air freight (base transport cost)
- THC — origin and destination
- BAF/CAF (ocean surcharges)
- Documentation fee
- Customs clearance fee
- Trucking / drayage (pre-carriage and on-carriage)
- CFS/handling charges (LCL)
- Insurance premium
- Demurrage & detention
- Storage charges
- ISF / AMS / ENS fees (US/EU security filings, only relevant on those lanes)

**Core mechanic:** every charge line has a **buy rate** (cost from the carrier) and a **sell rate** (billed to the customer). Margin tracking per shipment / per trade lane / per customer is the forwarder's actual profitability lever — this needs to be a first-class concept in the data model, not an afterthought.

**Multi-currency is not optional.** A single shipment can have freight cost in USD, local trucking cost in BDT, and an invoice to the customer in either — the system needs currency at the line-item level, plus an org-level base currency for reporting.

**Credit terms matter.** Most B2B freight relationships run on 30/60/90-day credit, so AR aging and credit control are relevant reporting needs, not nice-to-haves.

---

## 8. COMPETITIVE SOFTWARE LANDSCAPE (as of July 2026)

This is a real, established market — worth knowing before assuming there's no competition.

| Platform | Position |
|---|---|
| **CargoWise** (WiseTech Global) | Dominant enterprise player. Reportedly serves 17,000+ logistics organizations across 193 countries, including 24 of the top 25 global freight forwarders and 47 of the top 50 3PLs. Single global database architecture — one system for multi-office, multi-country, multi-currency operations. Known for being expensive and for a steep implementation curve; moved to a new "Value Packs" pricing model in Dec 2025 that reportedly raised costs for some customers. Australia's competition regulator (ACCC) required WiseTech to divest a competing product (Expedient) following an investigation into a 2026 acquisition. |
| **Magaya** | The closest direct mid-market competitor to CargoWise, especially strong for forwarders that also run their own warehouse operations (integrated WMS). Faster implementation than CargoWise; the "Magaya Network" helps onboard overseas agents/partners. Interface considered somewhat dated by some reviewers. Pricing is mid-tier and not published (one source put entry pricing around $200/user/month, another cited a $3,000+ entry point — treat these as rough signals, not confirmed figures). |
| **GoFreight** | Cloud-native, strong for ocean/air forwarding with good customer-facing portals; positioned as a lighter alternative to CargoWise for brokers. |
| **Descartes** | Enterprise-grade, compliance-heavy, modular rather than monolithic — good fit for forwarders wanting separate transportation/EDI/customs modules. |
| **Regional/smaller players** | NuevaFlo and Linbis (Latin America-focused), FreightPOP, FreightPath, Freightview, and others serve SMB/regional niches. This matters as a comfort point: smaller, regionally-focused platforms clearly do carve out real space next to the giants — this space is not winner-take-all. |

**Pricing bands (reported, directional only):**
- Mid-market FMS: roughly $100–400 per user/month
- Enterprise FMS: roughly $500–2,000+ per user/month, plus $50,000–100,000+ in implementation fees

**Takeaway:** the enterprise tier is expensive and slow to implement — which is exactly the gap a focused, fast-to-deploy, regionally-relevant tool can sit in. That's a legitimate wedge, provided the requirements are validated first (Section 9).

---

## 9. DISCOVERY CHECKLIST — DO THIS BEFORE WRITING ANY CODE

Literally bring this list to the conversation with the contact. The goal is to replace assumptions with their actual current process.

- [ ] What do they use today? (Excel? WhatsApp? Some existing software — which one?)
- [ ] Which transport modes do they actually handle — ocean only, air too, road/trucking?
- [ ] Ocean: FCL, LCL, or both? Typical container sizes/volumes?
- [ ] Do they do their own customs clearance as a licensed C&F agent, or is that handled by a separate partner?
- [ ] Do they need to generate legally significant documents (B/L, AWB), or is internal tracking + invoicing enough for v1?
- [ ] Single office, or multiple branches/locations?
- [ ] Roughly how many shipments per month? (needed for scale/performance planning)
- [ ] What share of the business is RMG/garments export vs. other cargo types?
- [ ] Do they already have an overseas agent network, or is that informal/ad hoc?
- [ ] What currencies do they invoice in? Do they hold foreign currency accounts?
- [ ] What's the single biggest pain point in their current process right now?
- [ ] Can you get 2–3 sanitized real shipment records or documents to look at?
- [ ] Who would actually use this day to day, and how many seats/roles? (ops staff, sales, accounts)
- [ ] Is there real budget/timeline pressure, or is this still exploratory on their end too?

---

## 10. HOW THIS MAPS ONTO BUSINESSSAAS ARCHITECTURE

The good news: the platform-layer separation already in place makes a lot of this reusable.

**Reuse as-is:**

- **`platform/contacts`** — Shipper, Consignee, Notify Party are all just `platform_companies` / `platform_contacts` records linked by role. No duplicate contact system needed.
- **`platform/engagement`** — shipment-related notes/tasks/activities/emails belong here, tagged `module = "freight"`. **Watch out:** this project has already hit the "wrong/missing module tag" bug class twice (capture module's dedup notes used the wrong tag; `capture.*` permissions were never added to `permissionGroups.ts`). Do both correctly from day one for freight — don't repeat it a third time.
- **RBAC roles** (owner/admin/manager/member/viewer) — no new role architecture needed, just new `freight.*` permission keys (Section 14).
- **Planned platform notification system + platform scheduler** (queued under CRM Layer 1) — directly useful for freight too (e.g. "shipment X customs delay," "demurrage day-3 warning"). Another argument for keeping these genuinely generic in `internal/platform/`, per the project's own existing principle.

**Key insight — generalize the custom fields engine:** CRM Layer 2 plans a custom fields engine (`crm_field_definitions`, JSONB). Cargo attributes (weight, volume, HS code, DG class, temperature control) need exactly the same kind of flexible schema. **Recommendation: if/when that engine gets built, put it in `internal/platform/customfields/` with an `entity_type` column instead of making it CRM-specific.** This is a direct extension of the existing "platform-layer placement for shared infrastructure" principle — a small amount of extra generality now avoids building the same engine twice later.

**Architectural parallel worth remembering:** RFQ → Quote → Booking is structurally the same shape as CRM's Lead → Deal conversion. Shipment milestone tracking (Booked → Pickup → Export Customs → Departed → In Transit → Import Customs → Delivered) is conceptually similar to a Pipeline/Deal board — but recommend modeling it as a **fixed status enum + a `freight_shipment_status_history` table** (mirroring the CRM Layer 1 "deal stage history" plan) rather than a fully configurable pipeline, since shipment milestones are fairly universal and don't need to be as customizable as a sales process per org.

---

## 11. PROPOSED BACKEND MODULE STRUCTURE

```
internal/freight/
  shipments/    ← core shipment/booking CRUD, status, mode (ocean/air/road), type (FCL/LCL/FTL/LTL)
  parties/      ← shipment ↔ platform_companies/contacts linking (role: shipper/consignee/notify_party)
  carriers/     ← carrier master data (ocean lines, airlines, truckers) + rate contracts
  agents/       ← overseas partner-forwarder network
  documents/    ← B/L, AWB, invoice, packing list — metadata + storage reference, House vs Master linkage
  charges/      ← per-shipment charge line items, buy/sell rate, margin calculation
  containers/   ← FCL container tracking (container no, seal no, size/type)
  quotations/   ← RFQ → quote → booking conversion flow
  reports/      ← volume by lane, margin by customer, AR aging — mirrors crm/reports pattern
```

This mirrors the existing `internal/capture/{apikeys,public,email,social,visitors}` and HRM's Group A–E sub-package pattern — consistent with how the codebase already organizes multi-part domains.

---

## 12. PROPOSED DATABASE SCHEMA (DRAFT — validate against Section 9 findings before writing any migration)

This is a first-pass sketch for planning purposes only. Same rule as everywhere else in this project: **audit real requirements before trusting a design that was never checked against actual source (in this case, the client's actual process) — don't build from this sketch blind.**

```sql
freight_shipments
  id UUID PK
  org_id UUID FK
  shipment_number TEXT        -- human-readable reference, org-scoped unique
  mode TEXT                   -- ocean | air | road | rail
  type TEXT                   -- FCL | LCL | FTL | LTL | breakbulk
  incoterm TEXT                -- EXW, FOB, CIF, DDP, ...
  origin_port TEXT, origin_country TEXT
  destination_port TEXT, destination_country TEXT
  status TEXT                  -- current status; history in table below
  etd DATE, eta DATE, atd DATE, ata DATE
  owner_id UUID, created_by UUID
  custom_fields JSONB          -- if/when the generic custom fields engine exists (see Section 10)
  created_at, updated_at

freight_shipment_status_history
  id UUID PK
  shipment_id UUID FK
  status TEXT
  note TEXT
  changed_by UUID
  changed_at TIMESTAMPTZ

freight_shipment_parties
  id UUID PK
  shipment_id UUID FK
  party_role TEXT              -- shipper | consignee | notify_party
  company_id UUID FK -> platform_companies
  contact_id UUID FK -> platform_contacts (nullable)

freight_carriers
  id UUID PK, org_id UUID FK
  name TEXT, scac_code TEXT, mode TEXT
  contact_info JSONB

freight_agents
  id UUID PK, org_id UUID FK
  name TEXT, country TEXT, contact_info JSONB
  network TEXT                 -- e.g. WCA membership, nullable

freight_containers
  id UUID PK, shipment_id UUID FK
  container_no TEXT, seal_no TEXT, size_type TEXT   -- 20GP, 40HC, ...

freight_documents
  id UUID PK, shipment_id UUID FK
  doc_type TEXT                -- MBL | HBL | MAWB | HAWB | commercial_invoice | packing_list | certificate_of_origin | ...
  file_reference TEXT
  issued_at TIMESTAMPTZ, created_by UUID

freight_charges
  id UUID PK, shipment_id UUID FK
  charge_type TEXT             -- freight | thc_origin | thc_destination | documentation | customs | trucking | insurance | demurrage | ...
  buy_amount NUMERIC, buy_currency TEXT
  sell_amount NUMERIC, sell_currency TEXT

freight_rate_cards
  id UUID PK, org_id UUID FK, carrier_id UUID FK
  origin TEXT, destination TEXT, mode TEXT
  rate NUMERIC, currency TEXT, valid_from DATE, valid_to DATE

freight_quotations
  id UUID PK, org_id UUID FK
  status TEXT                  -- draft | sent | accepted | rejected | expired | converted
  shipment_id UUID FK (nullable — set on conversion)
  created_by UUID, created_at, updated_at
```

~10 new tables for a reasonably complete MVP+ scope. Phase 1 (Section 15) only needs `freight_shipments`, `freight_shipment_status_history`, `freight_shipment_parties`, and `freight_charges`.

---

## 13. PROPOSED FRONTEND STRUCTURE

```
frontend/src/app/(dashboard)/[orgId]/freight/
  shipments/
  quotations/
  carriers/
  documents/
  reports/
```

Plus: `lib/freight/` (API helpers, mirroring `lib/crm/`), `types/freight.ts`, `components/freight/`. Same TanStack Query + React Hook Form + Zod conventions as the rest of the dashboard — no new frontend patterns needed.

---

## 14. PROPOSED PERMISSIONS

Following the existing `module.resource.action` convention:

```
freight.shipments.view / .create / .update / .delete
freight.quotations.view / .create / .update / .delete / .convert
freight.documents.view / .create / .delete
freight.carriers.view / .create / .update / .delete
freight.agents.view / .create / .update / .delete
freight.charges.view / .create / .update
freight.reports.view
```

**Do not forget:** add the `freight.*` group to `lib/permissionGroups.ts` at the same time the permissions are seeded — not as a follow-up. This exact gap already happened once with `capture.*`.

---

## 15. PHASED BUILD PLAN

**Phase 0 — Discovery, no code.** Run Section 9. Get real sample documents.

**Phase 1 — MVP: shipment tracking + basic billing.**
`freight_shipments` + status history, reuse `platform/contacts` for shipper/consignee, basic charge line items, simple PDF invoice generation (via the pdf skill). This alone replaces Excel/WhatsApp — it's the actual sellable value; everything past this is a bonus.

**Phase 2 — Documents & rates.**
B/L / AWB generation (House vs. Master distinction). These are legally sensitive documents — start from an established template and a simple fill approach rather than building complex generation logic from scratch. Rate cards, quotation workflow.

**Phase 3 — Network & advanced.**
Carrier/agent master data, container tracking, customer-facing track-and-trace portal (a read-only "customer" role via the existing per-member permission override system — no new RBAC architecture needed).

**Phase 4 — Integrations, only if genuinely needed.**
Real carrier tracking API integration, customs system integration, sanctions/denied-party screening, DG compliance tooling. These are expensive and usually not worth it for a v1.

---

## 16. BANGLADESH-SPECIFIC CONTEXT (unconfirmed assumption — validate this applies)

If the contact's company is Bangladesh-based, this is directly relevant. (Facts below current as of July 2026 — see Sources, Section 20.)

- Bangladesh customs operates under the **National Board of Revenue (NBR)**, the country's apex tax authority under the Ministry of Finance's Internal Resources Division (established 1972).
- The core customs declaration system is **ASYCUDA World**, part of a broader **Bangladesh Single Window** initiative that also integrates certificates/licenses/permits from other agencies (DOE, DGDA, EPB, DOEX, BNACWC, BEZA, BEPZA).
- This is an actively evolving system, not a static one: in April 2026, NBR and Bangladesh Bank launched a real-time digital interconnection between the Foreign Exchange Transaction Management System (FxTMS) and ASYCUDA World, ending manual commercial invoice verification.
- **If the client's business is significantly RMG/garments-export-focused** (very likely, given garments is Bangladesh's dominant export sector and a huge share of local freight forwarding activity serves it): as of January 2026, NBR integrated BGMEA's electronic Utilisation Declaration (e-UD) platform with ASYCUDA World, enabling real-time online verification of bonded raw material usage for export-oriented garment factories — a live, currently-modernizing piece of the puzzle, and a natural area where deeper future integration might make sense.
- **C&F (Clearing & Forwarding) Agent is a separately licensed role in Bangladesh.** Some companies are both freight forwarder and C&F agent; some keep them separate. This is exactly why it's on the discovery checklist (Section 9) rather than assumed.
- **Risk note:** customs operations in Bangladesh are not immune to disruption — there was a significant NBR strike in May–June 2025 that suspended customs services and trade at ports nationwide before being resolved. Worth knowing as background/operational risk context for a business that depends on customs continuity, not something the software itself needs to solve.

---

## 17. SECURITY & COMPLIANCE NOTES

- Freight documents (B/L, invoices) carry commercially sensitive data (rates, customer details) — the existing file/avatar storage pattern extends fine here; no new storage layer needed.
- Decide exchange rate source and rounding rules for multi-currency **before** writing the schema — this is much harder to retrofit than to design in from the start.
- If any carrier tracking API or webhook integration ever happens, apply the same lesson already learned from the capture module: signature verification from day one, not as a Fix Pass B added later.
- Sanctions/denied-party screening and dangerous goods classification are only relevant if US/EU trade lanes or hazardous cargo are actually in scope — reasonable to skip entirely for a v1 serving smaller/domestic-focused shipments.

---

## 18. OPEN QUESTIONS / DECISIONS TO MAKE LATER

- Is this being built as a one-off customization for a single client, or a genuinely generalizable module for multiple future freight forwarder customers? This materially changes how "generic vs. specific" the design should be — worth deciding explicitly rather than drifting into one or the other.
- Does the custom fields engine get generalized now (alongside CRM Layer 2) or built CRM-only for now and generalized later? (Leaning toward generalizing now — see Section 10 — but not yet decided.)
- Carriers vs. agents — one table or two? (Current lean: two, since they represent different relationship types — a carrier sells transport capacity, an agent is a peer/partner forwarder.)
- Does document generation need pixel-perfect legal templates from day one, or is a "good enough" PDF acceptable for v1? (Current lean: good-enough for v1, revisit for Phase 2.)
- Multi-branch support — introduce a "location" concept within an org, or treat each branch as fully out of scope until it comes up? (Not yet needed — defer until discovery surfaces it.)

---

## 19. RECOMMENDATION & NEXT STEPS

1. Do not start coding. Finish the Capture Fix Pass A/B + capture frontend first.
2. Run the Section 9 discovery conversation with the contact — replace every assumption in this doc with a real answer.
3. When ready to build, start with Phase 1 MVP only (Section 15).
4. Decide the custom-fields-engine generalization question (Section 10 / 18) whenever CRM Layer 2 work actually starts — don't let it get built CRM-only by default without a conscious choice.
5. Once there's a real decision to build, promote this into an actual `⚪ QUEUED` entry in `docs/Project_Instruction.md` Section 9, in that document's normal format — this standalone doc is not a substitute for that, just the research that feeds it.

---

## 20. SOURCES CONSULTED (July 2026 — re-verify before relying on specifics later)

- GoFreight — "Best Freight Management Software 2026": https://gofreight.com/blog/best-freight-management-software
- SoftwareConnect — "Best Freight Forwarding Software on the Market (2026)": https://softwareconnect.com/roundups/best-freight-forwarding-software/
- Magaya — "Magaya vs. CargoWise: What the Industry Is Actually Saying": https://www.magaya.com/magaya-vs-cargowise-what-the-industry-is-actually-saying/
- Guideflow — "6 best freight management software for 2026": https://www.guideflow.com/blog/freight-management-software
- Ubico — "The 7 best CargoWise alternatives for freight forwarders in 2026": https://www.ubico.io/post/the-7-best-cargowise-alternatives-for-freight-forwarders
- Samyak — "Top Freight Forwarding Software Solutions 2026": https://www.samyak.com/feeds/blog/freight-forwarder-software
- NuevaFlo — "5 Best Magaya Alternatives for Freight Forwarders (2026)": https://www.nuevaflo.com/en/blog/magaya-alternatives-freight-forwarders/
- Bangladesh Customs / NBR (official): https://bangladeshcustoms.gov.bd/
- BSS News — "NBR, BB launch digital interconnection to streamline customs...": https://www.bssnews.net/business/375444
- Apparel Resources — "Bangladesh Integrates BGMEA Platform with ASYCUDA...": https://apparelresources.com/business-news/trade-business-news/bangladesh-integrates-bgmea-platform-asycuda-speed-customs-clearance/
- NBR — Medium-and-Long-Term Revenue Strategy FY2025-26 to FY2034-35: https://nbr.gov.bd/uploads/publications/MLTRS_E_Book_Version.pdf
- Wikipedia — "Bangladesh Customs": https://en.wikipedia.org/wiki/Bangladesh_Customs
- Wikipedia — "2025 NBR strike": https://en.wikipedia.org/wiki/2025_NBR_strike

