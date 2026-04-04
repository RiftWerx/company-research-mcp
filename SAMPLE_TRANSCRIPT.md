# Conversation Transcript — Lloyds Bank Accounts Review
**Date:** 3 April 2026

---

**User:** What documents and filings are available for Lloyds Bank?

**Assistant:** company-research - search_company (MCP)(query: "Lloyds Bank")

company-research - list_filings (MCP)(ch_number: "00002065", limit: 30)

Here's what's available for Lloyds Bank PLC (CH# 00002065):

  Accounts (Group)

  ┌────────────┬─────────────────────────────────────┐
  │    Date    │             Description             │
  ├────────────┼─────────────────────────────────────┤
  │ 2026-03-19 │ Annual group accounts (most recent) │
  ├────────────┼─────────────────────────────────────┤
  │ 2025-04-01 │ Annual group accounts               │
  ├────────────┼─────────────────────────────────────┤
  │ 2024-04-15 │ Annual group accounts               │
  ├────────────┼─────────────────────────────────────┤
  │ 2023-04-15 │ Annual group accounts               │
  ├────────────┼─────────────────────────────────────┤
  │ 2022-04-07 │ Annual group accounts               │
  └────────────┴─────────────────────────────────────┘

  Confirmation Statements

  ┌────────────┐
  │    Date    │
  ├────────────┤
  │ 2025-05-06 │
  ├────────────┤
  │ 2024-05-20 │
  ├────────────┤
  │ 2023-05-09 │
  ├────────────┤
  │ 2022-05-09 │
  └────────────┘

  Director Changes

  - 2025-11-03 — Director termination
  - 2025-06-16 — Director appointment
  - 2024-08-08 — Director appointment
  - 2024-05-17 — Two director terminations
  - 2022-11-03, 2022-08-04 — Director appointments
  - 2022-05-12 — Director termination

  Mortgage/Charge Registrations (MR01)

  Several between 2022–2025 (new charge registrations, routine banking activity).

  Other

  - 2022-03-16 — Memorandum & Articles

  The most useful for your LLOY thesis would be the 2026-03-19 group accounts (FY2025 results). Would you like me to fetch and review that filing?

---

**User:** Yes please, let's fetch and review the latest group accounts.

**Assistant:** company-research - fetch_filing (MCP)(ch_number: "00002065", document_url: "https://document-api.company-information.service.gov.uk/document

Large PDF (~10MB). Let me read it in sections — starting with the front matter to orient the structure.

Good start. Let me now read the key financial statements — income statement, balance sheet, and dividend notes.

Good data captured. Let me grab the dividend note and motor finance provision details.

I have everything I need. Here's the full review:

  ---
  Lloyds Bank plc — Annual Report & Accounts FY2025

  Income Statement

  ┌─────────────────────┬────────────┬────────────┬────────────┐
  │                     │    2025    │    2024    │    2023    │
  ├─────────────────────┼────────────┼────────────┼────────────┤
  │ Net interest income │ £13,363m   │ £12,592m   │ £13,709m   │
  ├─────────────────────┼────────────┼────────────┼────────────┤
  │ Other income        │ £5,066m    │ £4,479m    │ £4,658m    │
  ├─────────────────────┼────────────┼────────────┼────────────┤
  │ Total income        │ £18,429m   │ £17,071m   │ £18,367m   │
  ├─────────────────────┼────────────┼────────────┼────────────┤
  │ Operating expenses  │ (£12,165m) │ (£11,927m) │ (£10,968m) │
  ├─────────────────────┼────────────┼────────────┼────────────┤
  │ Impairment          │ (£792m)    │ (£456m)    │ (£343m)    │
  ├─────────────────────┼────────────┼────────────┼────────────┤
  │ Profit before tax   │ £5,472m    │ £4,688m    │ £7,056m    │
  ├─────────────────────┼────────────┼────────────┼────────────┤
  │ Profit after tax    │ £3,856m    │ £3,486m    │ £5,207m    │
  └─────────────────────┴────────────┴────────────┴────────────┘

  PBT up 17% YoY — solid recovery from the 2024 trough. However impairment charges nearly doubled (£792m vs £456m), and remain elevated vs 2023. Expenses grew 2% — contained but still rising.

  ---
  Balance Sheet (31 Dec 2025)

  ┌───────────────────────────────┬───────────┬───────────┐
  │                               │   2025    │   2024    │
  ├───────────────────────────────┼───────────┼───────────┤
  │ Total assets                  │ £631,335m │ £611,213m │
  ├───────────────────────────────┼───────────┼───────────┤
  │ Loans & advances to customers │ £461,504m │ £441,907m │
  ├───────────────────────────────┼───────────┼───────────┤
  │ Customer deposits             │ £465,207m │ £451,794m │
  ├───────────────────────────────┼───────────┼───────────┤
  │ Total equity                  │ £41,980m  │ £39,747m  │
  └───────────────────────────────┴───────────┴───────────┘

  Loan book grew £19.6bn (+4.4%), funded largely by deposit growth of £13.4bn. Healthy balance sheet expansion.

  ---
  Capital Ratios

  ┌─────────────────────┬───────┬───────┐
  │                     │ 2025  │ 2024  │
  ├─────────────────────┼───────┼───────┤
  │ CET1 ratio          │ 13.6% │ 13.7% │
  ├─────────────────────┼───────┼───────┤
  │ Total capital ratio │ 20.1% │ 19.9% │
  ├─────────────────────┼───────┼───────┤
  │ UK leverage ratio   │ 5.2%  │ 5.4%  │
  └─────────────────────┴───────┴───────┘

  CET1 ticked down slightly — motor finance provisions and RWA growth (up £7.3bn) are the drivers. Still a robust buffer above regulatory requirements.

  ---
  Motor Finance — The Key Risk (Note 26)

  - Total provision at 31 Dec 2025: £1,950m (£1,150m prior + £800m added in Q3 2025 following the FCA's October 2025 announcement of a formal redress scheme)
  - The Supreme Court ruling (Johnson v FirstRand, Aug 2025) confirmed unfair commission arrangements, widening scope back to 2007
  - FCA's final scheme rules expected end of March 2026 (i.e. imminent / just issued)
  - Complaints pause lifts 31 May 2026; Court of Appeal considering omnibus claims April 2026
  - The Group explicitly warns the ultimate impact could materially differ from the £1,950m provision

  This remains the single biggest uncertainty for the thesis. The provision is credible but the tail is unquantified.

  ---
  Dividends (Note 31)

  Note: these are intercompany dividends from Lloyds Bank plc to its parent, Lloyds Banking Group plc — not the shareholder dividend you receive. Nonetheless they reflect cash generation capacity:

  ┌──────┬─────────────────────────┐
  │ Year │ Dividends paid upstream │
  ├──────┼─────────────────────────┤
  │ 2025 │ £2,390m                 │
  ├──────┼─────────────────────────┤
  │ 2024 │ £3,990m                 │
  ├──────┼─────────────────────────┤
  │ 2023 │ £4,700m                 │
  └──────┴─────────────────────────┘

  The sharp drop in 2025 (vs 2024) reflects motor finance provisioning absorbing distributable profits. A further £480m interim was approved in February 2026 and paid 16 Feb 2026 — suggesting the parent is still receiving cash despite the
  provisions.

  The shareholder dividend (what matters for your ISA) is declared at the Lloyds Banking Group plc level — record date 10 April per your thesis flag.

  ---
  Auditor Sign-Off

  Deloitte (Michael Lloyd, Senior Statutory Auditor) issued an unqualified opinion on 26 February 2026. Key audit matter flagged was Expected Credit Losses methodology. No material misstatements identified.

  ---
  Thesis Implications (Scenario B / Stagflation)

  ┌─────────────────────────┬───────────────────────────────────────────────────────────────────────────────────┐
  │         Factor          │                                      Signal                                       │
  ├─────────────────────────┼───────────────────────────────────────────────────────────────────────────────────┤
  │ NII recovery            │ Positive — margin holding, lending growing                                        │
  ├─────────────────────────┼───────────────────────────────────────────────────────────────────────────────────┤
  │ Impairment trend        │ Amber — nearly doubled, stagflation scenario risks further pressure               │
  ├─────────────────────────┼───────────────────────────────────────────────────────────────────────────────────┤
  │ Motor finance           │ Amber/Red — £1,950m provisioned, tail risk remains; FCA scheme rules just landing │
  ├─────────────────────────┼───────────────────────────────────────────────────────────────────────────────────┤
  │ Capital                 │ Amber — CET1 slightly declining, adequate but not building                        │
  ├─────────────────────────┼───────────────────────────────────────────────────────────────────────────────────┤
  │ Dividend sustainability │ Positive — upstream cash flow maintained, £480m paid Feb 2026                     │
  ├─────────────────────────┼───────────────────────────────────────────────────────────────────────────────────┤
  │ Cost discipline         │ Neutral — 2% growth, not runaway but no improvement                               │
  └─────────────────────────┴───────────────────────────────────────────────────────────────────────────────────┘

  Overall: Underlying results are better than 2024, but motor finance is still the live wildcard. With the FCA scheme rules expected imminently and the Court of Appeal considering omnibus claims in April, the next few weeks are material
  for the thesis. The 10 April record date comes before the picture is fully clear.

---

**User:** Let's save our summary of the Lloyds accounts in the Summaries folder as lloyds_group_accounts_2026-03-19.md

**Assistant:** Saved summary to Summaries/lloyds_group_accounts_2026-03-19.md
