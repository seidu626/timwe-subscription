# Ghana MNO Header Enrichment parameters (MTN, AirtelTigo/AT Ghana, Vodafone/Telecel)

This note captures **publicly verifiable** “Header Enrichment (HE)” identifiers for Ghana’s main mobile operators and explains what is **not** publicly documented (and therefore must be confirmed via operator/aggregator integration docs or empirical on‑net testing).

> **Why this matters for `timwe-subscription-manager`**  
> In most HE implementations, the downstream service uses **(a)** the subscriber’s MSISDN and **(b)** the serving network identity (**MCC/MNC**) to route the user through a frictionless on‑net subscription flow.

---

## 1) What HE typically exposes (parameter-wise)

Many HE services (especially redirect-based HE detection services) identify the subscriber’s:

- **MSISDN** (mobile number)
- **MCC** (Mobile Country Code)
- **MNC** (Mobile Network Code)

This is a common “parameter set” for HE detection flows (even if the values arrive via HTTP headers, a redirect callback, or an operator API).  
(Source example: Infomedia describes HE as detecting MSISDN + MCC + MNC.) citeturn11search17

---

## 2) Ghana: MCC/MNC values you can use for operator detection

### Ghana MCC
- **MCC = 620** for Ghana networks. citeturn14search6turn16search6

### Operator MNC mapping (Ghana)

| Operator brand (today) | Legacy / notes | MCC-MNC | Evidence |
|---|---|---:|---|
| **MTN Ghana** |  | **620-01** | Ghana NCA numbering plan lists “MTN Ghana 620-01”. citeturn14search6 |
| **Telecel Ghana** | Rebrand of **Vodafone Ghana** (Ghana Telecom Mobile 620-02) | **620-02** | Ghana NCA numbering plan lists “Ghana Telecom 620-02”. citeturn14search6; Vodafone→Telecel transition reported March 2024. citeturn16search21 |
| **AT Ghana** | Rebrand of **AirtelTigo** (Jan 2024); two MNCs still used | **620-03** and **620-06** | Ghana NCA numbering plan lists Millicom Ghana 620-03 and Airtel Ghana 620-06. citeturn14search6; Wikipedia notes AT Ghana rebrand from AirtelTigo and lists both 620-03 and 620-06. citeturn16search6 |

---

## 3) The “hard part”: exact HTTP header names per operator

### What we found (publicly)
There is **no single public, authoritative list** of the *exact* HTTP header keys that MTN Ghana / AT Ghana / Telecel Ghana inject for HE.

This aligns with industry reality:

- HE is **carrier-dependent** and may require **server IP whitelisting** or specific APN routing to receive MSISDN in headers, and some networks may pass MSISDN via query string instead. citeturn15search2
- Implementations vary by network proxy / gateway vendor and by the operator’s internal security posture (plain vs aliased/encrypted).

### Practical approach for `timwe-subscription-manager`
Implement HE parsing as:
1. **Config-driven**, per-operator profile (`mcc`, `mnc`, and candidate header names)
2. **Multi-header tolerant**, by scanning a list of common MSISDN header names

A widely used “candidate list” of MSISDN-related headers includes:  
`X-MSISDN`, `X_UP_CALLING_LINE_ID` / `X-UP-CALLING-LINE-ID`, and `X_WAP_NETWORK_CLIENT_MSISDN` (plus server/framework variants such as `HTTP_X_MSISDN`). citeturn12search27

---

## 4) Recommended simulation values (staging/dev)

Use these **publicly correct** operator identifiers in your HE simulator:

```json
{
  "country": "GH",
  "mcc": "620",
  "operators": [
    {
      "brand": "MTN Ghana",
      "mnc": ["01"],
      "msisdn_examples_e164": ["233240000000", "233540000000"],
      "candidate_msisdn_headers": ["X-MSISDN", "X-UP-CALLING-LINE-ID", "X_WAP_NETWORK_CLIENT_MSISDN"]
    },
    {
      "brand": "Telecel Ghana (ex-Vodafone)",
      "mnc": ["02"],
      "msisdn_examples_e164": ["233200000000", "233500000000"],
      "candidate_msisdn_headers": ["X-MSISDN", "X-UP-CALLING-LINE-ID", "X_WAP_NETWORK_CLIENT_MSISDN"]
    },
    {
      "brand": "AT Ghana (ex-AirtelTigo)",
      "mnc": ["03", "06"],
      "msisdn_examples_e164": ["233260000000", "233560000000"],
      "candidate_msisdn_headers": ["X-MSISDN", "X-UP-CALLING-LINE-ID", "X_WAP_NETWORK_CLIENT_MSISDN"]
    }
  ]
}
```

**Notes**
- The MSISDN prefixes above are illustrative (not authoritative) — for simulation purposes you mainly need valid E.164 formatting and correct MCC/MNC.
- In production, treat MSISDN as sensitive personal data. Log/retain minimally and comply with local regulation.

---

## 5) How to confirm the *real* header keys in Ghana (recommended)

To discover the actual header names used by each operator in production-like conditions:

1. Deploy a simple **HTTP (not HTTPS)** “header echo” endpoint (or log raw request headers at your edge) on a dedicated hostname.
2. Ensure the device uses **mobile data** (not Wi‑Fi). citeturn15search2
3. Test one SIM per operator and record which headers appear.
4. If nothing appears, request **IP/URL whitelisting** from the operator/aggregator (many networks only enrich traffic to allowlisted destinations). citeturn15search2
5. Feed the discovered header names into the per-operator config in `timwe-subscription-manager`.

---

## 6) Bottom line

- **Reliable public “parameters”** you can encode today: **MCC=620** and MNC mapping (**01 MTN**, **02 Telecel/Vodafone**, **03 & 06 AT Ghana/AirtelTigo**). citeturn14search6turn16search6
- **Exact header keys** are usually **NDA / partner-doc** material; implement parsing + simulation in a way that is tolerant and config-driven, and validate via on‑net tests.
