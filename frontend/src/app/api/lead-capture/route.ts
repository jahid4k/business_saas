// // src/app/api/lead-capture/route.ts
// //
// // Server-side proxy for the marketing homepage's "Book a Demo" / "Join Early
// // Access" form. Runs on the server only, so the capture API key never
// // reaches the browser — the page's client component calls this route, and
// // this route calls the real backend.
// //
// // Setup required before this works:
// //   1. Generate an org API key with the `capture:leads` scope. The capture
// //      frontend (Section 8 → CRM — CAPTURE) isn't built yet, so for now
// //      create one directly:
// //        POST /api/v1/organizations/:orgId/capture/apikeys
// //        (Authorization: Bearer <your JWT>, scope: "capture:leads")
// //      Copy the raw `bs_live_...` value — it's shown exactly once.
// //   2. Set these in the frontend's .env (never commit the real values):
// //        CAPTURE_API_KEY=bs_live_...
// //        NEXT_PUBLIC_API_URL=https://your-api-host/api/v1
// //
// // One thing to verify before relying on this in production: the payload
// // below is built from the confirmed `CreateLeadRequest` shape used by the
// // authenticated /crm/leads endpoint (first_name, last_name, email, phone,
// // company_name, source) plus `capture_source` / `capture_metadata`, which
// // migration 00058 adds to crm_leads. I could not confirm the exact public
// // /pub/leads request struct in internal/capture/public/handler.go from what
// // I had available — if the public handler names those two fields
// // differently (or nests them differently), adjust the `body` object below
// // to match. Everything else here should be an exact match.
// //
// // Also: /pub/* routes don't have rate limiting yet (Fix Pass B, Section 5 →
// // CAPTURE). A honeypot field or a simple per-IP limit here would be cheap
// // insurance until that ships — not added here so as not to guess at your
// // infra (Redis client, etc.) without seeing it.

// import { NextRequest, NextResponse } from "next/server";

// interface LeadCapturePayload {
//   name?: string;
//   email?: string;
//   company?: string;
//   intent?: "demo" | "waitlist";
// }

// export async function POST(req: NextRequest) {
//   let body: LeadCapturePayload;
//   try {
//     body = await req.json();
//   } catch {
//     return NextResponse.json(
//       { success: false, error: "Invalid request." },
//       { status: 400 },
//     );
//   }

//   const { name, email, company, intent } = body;

//   if (!name?.trim() || !email?.trim()) {
//     return NextResponse.json(
//       { success: false, error: "Name and email are required." },
//       { status: 400 },
//     );
//   }

//   const apiKey = process.env.CAPTURE_API_KEY;
//   const apiBase = process.env.NEXT_PUBLIC_API_URL;

//   if (!apiKey || !apiBase) {
//     console.error(
//       "[lead-capture] Missing CAPTURE_API_KEY or NEXT_PUBLIC_API_URL — see setup notes at the top of this file.",
//     );
//     return NextResponse.json(
//       {
//         success: false,
//         error: "Lead capture isn't configured yet. Please try again shortly.",
//       },
//       { status: 500 },
//     );
//   }

//   const [firstName, ...rest] = name.trim().split(/\s+/);
//   const lastName = rest.join(" ") || undefined;

//   try {
//     const upstream = await fetch(`${apiBase}/pub/leads`, {
//       method: "POST",
//       headers: {
//         "Content-Type": "application/json",
//         "X-API-Key": apiKey,
//       },
//       body: JSON.stringify({
//         first_name: firstName,
//         last_name: lastName,
//         email: email.trim(),
//         company_name: company?.trim() || undefined,
//         source: "marketing_site",
//         capture_source: "marketing_site",
//         capture_metadata: { intent: intent ?? "waitlist" },
//       }),
//       // This is a public marketing form — don't let a slow/unreachable
//       // backend hang the request indefinitely.
//       signal: AbortSignal.timeout(10_000),
//     });

//     if (!upstream.ok) {
//       const text = await upstream.text().catch(() => "");
//       console.error("[lead-capture] upstream error", upstream.status, text);
//       return NextResponse.json(
//         {
//           success: false,
//           error: "Could not submit right now. Please try again.",
//         },
//         { status: 502 },
//       );
//     }

//     return NextResponse.json({ success: true });
//   } catch (err) {
//     console.error("[lead-capture] request to backend failed", err);
//     return NextResponse.json(
//       {
//         success: false,
//         error: "Could not reach the server. Please try again.",
//       },
//       { status: 500 },
//     );
//   }
// }

console.log("lead-capture: disabled");
