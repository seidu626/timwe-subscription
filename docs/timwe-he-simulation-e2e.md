# timwe-subscription-manager — Header Enrichment (HE) Simulation + OTP E2E Test Guide

This guide describes a **staging/local-only** mechanism to **simulate Header Enrichment (HE) detection** so you can run a **full end-to-end (E2E) journey** (HE path + OTP fallback) before production rollout.

The approach is designed to:
- Work in **real browsers** (manual QA) **and** automated E2E runners (Playwright/Cypress/etc.)
- Avoid relying on real MNO network HE in non-prod
- Be **cryptographically gated**, **short-lived**, and **impossible to use in production**

---

## 0) Scope / Goals

### What HE simulation must enable
1. **HE path** (no OTP):
   - Request is treated as HE-detected
   - App receives an identity payload (at least `msisdn`, plus operator routing data)
   - Subscription flow proceeds without OTP

2. **OTP fallback path**:
   - No HE → OTP screen → OTP validation → subscription completes

### What we simulate
We simulate *detection* and *identity data*, not carrier infrastructure:
- `msisdn` (required)
- `operatorId` (or `mccmnc`) (recommended)
- optionally: `country`, `imsi`, `subscriberId`

---

## 1) Recommended Pattern (Best ROI): Signed “HE Simulation Token” in Staging

### High-level design
1. A **staging-only** endpoint `/__he/sim` lets QA/CI choose an MSISDN + operator.
2. The endpoint issues a **short-lived signed token** and sets it as an **HttpOnly cookie**.
3. Your normal request pipeline reads:
   - Real HE headers (if present) **OR**
   - The valid simulation cookie (if enabled)
4. Downstream subscription logic consumes a **single internal identity context**.

### Why this works
- Browsers cannot reliably add arbitrary “HE headers” on top-level navigation.
- Cookies *are* easy to set, and E2E tools can set cookies too.
- You keep “fake HE” away from production with strict gating.

---

## 2) Production-Safety Guardrails (Non-negotiable)

Implement **multiple layers**; do not rely on only one.

### Required
- **Hard-disable in production**:
  - `HE_SIMULATION_ENABLED=false` in prod config
  - Ideally, simulator routes not loaded at all in prod (build profile / feature gating)

- **Signature validation**:
  - Token must be signed (JWT HS256 or HMAC) using a secret present only in non-prod

- **Short TTL**:
  - e.g., 2–5 minutes expiry

### Strongly recommended
- **IP allowlist** (VPN/NAT/CI runners)
- **Auth required** for `/__he/sim` routes (Basic Auth / internal SSO / mTLS)
- **Audit logs** with explicit `he_source=SIMULATED`

### Optional (nice-to-have)
- One-time token (nonce stored server-side)
- “Allowlisted test MSISDN pool” enforced in staging

---

## 3) Configuration

### Environment variables
| Variable | Example | Purpose |
|---|---:|---|
| `HE_SIMULATION_ENABLED` | `true` (staging), `false` (prod) | Master switch |
| `HE_SIM_SECRET` | long random | Token signing secret (staging only) |
| `HE_SIM_COOKIE_NAME` | `he_sim_token` | Cookie key |
| `HE_SIM_TTL_SECONDS` | `180` | Token TTL |
| `HE_SIM_IP_ALLOWLIST` | `10.0.0.0/8,1.2.3.4/32` | Restrict simulator usage |
| `HE_SIM_BASIC_AUTH` | `user:hashedpass` | Optional protection for simulator routes |
| `HE_SIM_ALLOWED_MSISDNS` | `26133...,26134...` | Optional allowlist |

### Startup guardrail (recommended)
At service startup, fail fast if:
- `ENV=production` AND `HE_SIMULATION_ENABLED=true`

---

## 4) Internal Identity Model (single source of truth)

Define a single internal structure used by subscription logic:

```json
{
  "msisdn": "261330000001",
  "operatorId": "MNO_X",
  "mccmnc": "64601",
  "country": "MG",
  "source": "REAL | SIMULATED"
}
```

**Rule:** do not mutate actual request headers; store this in an internal request context:
- `req.context.identity` (Node)
- `request.setAttribute("identity", ...)` (Java)

---

## 5) Detection Precedence (REAL beats SIM)

**Always prefer real HE headers** if present.

Detection order:
1. If real HE headers exist → identity.source = `REAL`
2. Else if simulation enabled and valid cookie exists → identity.source = `SIMULATED`
3. Else → no identity → OTP path

---

## 6) Node/Express Reference Implementation

### 6.1 Middleware: resolve identity into request context

**File:** `heContextMiddleware.js`

```js
import jwt from "jsonwebtoken";

function normalizeMsisdn(v) {
  return String(v).replace(/\s+/g, "").replace(/^\+/, "");
}

function getRealHeIdentity(req) {
  // Adjust to match your vendor/carrier headers
  const msisdn = req.header("x-msisdn") || req.header("x-up-calling-line-id");
  const operatorId = req.header("x-operator-id") || req.header("x-mccmnc");
  if (!msisdn) return null;

  return {
    msisdn: normalizeMsisdn(msisdn),
    operatorId: operatorId || null,
    source: "REAL",
  };
}

export function heContextMiddleware(config) {
  return function (req, res, next) {
    // Prefer REAL
    const real = getRealHeIdentity(req);
    if (real) {
      req.context = req.context || {};
      req.context.identity = real;
      return next();
    }

    // SIM fallback
    if (!config.HE_SIMULATION_ENABLED) return next();

    const token = req.cookies?.[config.HE_SIM_COOKIE_NAME];
    if (!token) return next();

    try {
      const payload = jwt.verify(token, config.HE_SIM_SECRET, { algorithms: ["HS256"] });
      req.context = req.context || {};
      req.context.identity = {
        msisdn: normalizeMsisdn(payload.msisdn),
        operatorId: payload.operatorId || null,
        mccmnc: payload.mccmnc || null,
        country: payload.country || null,
        source: "SIMULATED",
      };
    } catch {
      // invalid/expired token → ignore and fall back to OTP
    }
    return next();
  };
}
```

### 6.2 Staging-only simulator routes

**File:** `heSimRoutes.js`

```js
import jwt from "jsonwebtoken";
import express from "express";

function isAllowedMsisdn(msisdn, allowedListCsv) {
  if (!allowedListCsv) return true;
  const allowed = allowedListCsv.split(",").map(s => s.trim()).filter(Boolean);
  return allowed.includes(msisdn);
}

export function heSimRoutes(config) {
  const router = express.Router();

  router.get("/__he/sim", (req, res) => {
    if (!config.HE_SIMULATION_ENABLED) return res.sendStatus(404);

    res.type("html").send(`
      <html>
        <body style="font-family: sans-serif; max-width: 640px; margin: 2rem auto;">
          <h2>HE Simulator (staging/local only)</h2>
          <p>This page issues a short-lived signed cookie used to simulate HE detection.</p>
          <form method="POST" action="/__he/sim">
            <label>MSISDN<br/><input name="msisdn" style="width: 100%;" /></label><br/><br/>
            <label>OperatorId (or MCCMNC)<br/><input name="operatorId" style="width: 100%;" /></label><br/><br/>
            <label>Country (optional)<br/><input name="country" style="width: 100%;" /></label><br/><br/>
            <label>Redirect (after setting cookie)<br/><input name="redirect" value="/" style="width: 100%;" /></label><br/><br/>
            <button type="submit">Start Journey with HE</button>
          </form>
        </body>
      </html>
    `);
  });

  router.post("/__he/sim", express.urlencoded({ extended: false }), (req, res) => {
    if (!config.HE_SIMULATION_ENABLED) return res.sendStatus(404);

    const msisdn = String(req.body.msisdn || "").trim().replace(/^\+/, "");
    const operatorId = String(req.body.operatorId || "").trim();
    const country = String(req.body.country || "").trim();
    const redirect = String(req.body.redirect || "/").trim();

    if (!msisdn) return res.status(400).send("msisdn required");
    if (!isAllowedMsisdn(msisdn, config.HE_SIM_ALLOWED_MSISDNS)) {
      return res.status(403).send("msisdn not allowed in staging");
    }

    const now = Math.floor(Date.now() / 1000);
    const ttl = config.HE_SIM_TTL_SECONDS ?? 180;
    const exp = now + ttl;

    const token = jwt.sign(
      { msisdn, operatorId, country, iat: now, exp },
      config.HE_SIM_SECRET,
      { algorithm: "HS256" }
    );

    res.cookie(config.HE_SIM_COOKIE_NAME, token, {
      httpOnly: true,
      secure: true,
      sameSite: "Lax",
      maxAge: ttl * 1000,
    });

    res.redirect(302, redirect);
  });

  router.post("/__he/clear", (req, res) => {
    res.clearCookie(config.HE_SIM_COOKIE_NAME, { httpOnly: true, secure: true, sameSite: "Lax" });
    res.sendStatus(204);
  });

  return router;
}
```

### 6.3 App wiring

```js
import cookieParser from "cookie-parser";
import { heContextMiddleware } from "./heContextMiddleware.js";
import { heSimRoutes } from "./heSimRoutes.js";

app.use(cookieParser());
app.use(heContextMiddleware(config));
app.use(heSimRoutes(config));
```

### 6.4 Subscription flow usage

```js
app.post("/subscribe", async (req, res) => {
  const identity = req.context?.identity;

  if (identity?.msisdn) {
    // HE path
    // Proceed without OTP
  } else {
    // OTP path
    // Trigger OTP challenge
  }
});
```

---

## 7) Java/Spring Boot Reference (Profiles-based)

### 7.1 Filter to resolve identity

```java
public class HeContextFilter extends OncePerRequestFilter {
  private final boolean simEnabled;
  private final String simSecret;
  private final String simCookieName;

  public HeContextFilter(boolean simEnabled, String simSecret, String simCookieName) {
    this.simEnabled = simEnabled;
    this.simSecret = simSecret;
    this.simCookieName = simCookieName;
  }

  @Override
  protected void doFilterInternal(HttpServletRequest request, HttpServletResponse response, FilterChain chain)
      throws ServletException, IOException {

    Identity identity = RealHeParser.parse(request); // read real vendor headers
    if (identity != null) {
      identity.setSource("REAL");
      request.setAttribute("identity", identity);
      chain.doFilter(request, response);
      return;
    }

    if (simEnabled) {
      String token = CookieUtils.getCookieValue(request, simCookieName);
      if (token != null) {
        try {
          SimPayload payload = JwtUtils.verify(token, simSecret); // HS256 verify + exp check
          identity = new Identity(payload.getMsisdn(), payload.getOperatorId(), "SIMULATED");
          request.setAttribute("identity", identity);
        } catch (Exception ignored) {
          // invalid/expired token → ignore
        }
      }
    }
    chain.doFilter(request, response);
  }
}
```

### 7.2 Staging-only simulator controller

```java
@Profile({"staging","local"})
@RestController
@RequestMapping("/__he")
public class HeSimController {
  @PostMapping("/sim")
  public ResponseEntity<Void> simulate(@RequestParam String msisdn,
                                       @RequestParam(required=false) String operatorId,
                                       @RequestParam(defaultValue="/") String redirect,
                                       HttpServletResponse response) {

    String token = JwtUtils.sign(msisdn, operatorId, 180 /*seconds*/, simSecret);
    Cookie cookie = new Cookie("he_sim_token", token);
    cookie.setHttpOnly(true);
    cookie.setSecure(true);
    cookie.setPath("/");
    response.addCookie(cookie);

    return ResponseEntity.status(302).header("Location", redirect).build();
  }
}
```

**Important:** Using `@Profile` ensures the endpoint is not even registered in production.

---

## 8) Alternative: Reverse Proxy Header Injection (More “Realistic”, More Infra)

If you want the app to literally receive HE headers in staging:
- Put Nginx/Envoy before the app in staging
- Proxy injects `X-MSISDN`/`X-OPERATOR-ID` when a signed cookie/query param is present

Pros:
- Mimics production header-based detection more closely  
Cons:
- Requires infra + careful configuration

---

## 9) OTP Simulation (Completes full E2E)

A full E2E requires OTP to be testable without real SMS.

### Option A (recommended): OTP Sink in staging
- Your OTP sending adapter writes OTP to a store (DB/Redis) in staging
- Expose a staging-only endpoint:

`GET /__otp/latest?msisdn=261330000001`

Return:
```json
{ "msisdn": "261330000001", "otp": "483920", "createdAt": "..." }
```

Security:
- staging-only profile
- IP allowlist + auth
- optionally only for allowlisted test MSISDN pool

### Option B: Vendor sandbox (if supported)
Use provider test numbers / sandbox mode and query OTP via provider tools.

### Option C: Test-number deterministic OTP (least preferred)
- For allowlisted test MSISDNs: OTP always `123456` in staging  
⚠️ Avoid global/static OTP for all numbers; restrict heavily.

---

## 10) E2E Test Flows (Playwright/Cypress)

### 10.1 HE Path
1. Navigate to `/__he/sim`
2. Submit `msisdn` + `operatorId`, redirect into normal journey
3. Verify OTP step is skipped
4. Verify subscription success

### 10.2 OTP Fallback Path
1. Start normal journey (no HE sim cookie)
2. Verify OTP requested
3. Fetch OTP from sink (`/__otp/latest`)
4. Submit OTP
5. Verify success

### 10.3 Negative & edge cases
- Expired sim token → OTP fallback
- Invalid sim token signature → OTP fallback
- Unknown operatorId → error/route/fallback as per business rules
- Session continuity (refresh/back button)
- Multiple tabs/devices behavior

---

## 11) Logging, Observability, and Auditing

Log these fields on each journey:
- `journey_id` / `correlation_id`
- `he_detected` boolean
- `he_source`: `REAL|SIMULATED|NONE`
- `msisdn_hash` (hash only, avoid plain msisdn in logs unless explicitly allowed)
- `operatorId` / `mccmnc`
- OTP events: sent/validated/failed/expired/throttled

Alerting:
- Any `he_source=SIMULATED` outside staging/local should alert immediately.

---

## 12) CI/CD Guardrails

Add tests:
- Simulator endpoints return **404** when `HE_SIMULATION_ENABLED=false`
- Startup fails if `ENV=prod` and `HE_SIMULATION_ENABLED=true`
- Token TTL enforced
- Signature required

Deployment:
- Separate secrets between environments
- Ensure `HE_SIM_SECRET` is not present in prod secret stores

---

## 13) Rollout Plan (Practical)

1. Implement simulation and OTP sink in **local + staging**
2. Add E2E tests for both branches
3. Run manual QA using `/__he/sim`
4. Confirm logs and dashboards show correct `he_source`
5. Production rollout:
   - simulation disabled
   - verify real HE works using real carrier environment / controlled pilot

---

## 14) Quick Checklist

- [ ] `HE_SIMULATION_ENABLED` default false in prod
- [ ] `/__he/sim` protected (profile + auth + IP allowlist)
- [ ] Signed token with short TTL
- [ ] Real HE precedence over sim
- [ ] OTP sink or sandbox implemented
- [ ] E2E tests cover HE + OTP flows
- [ ] Logs include `he_source`
- [ ] Alerts for simulated HE outside staging

---

## Appendix A — Token payload recommendation

JWT/HMAC payload (example):
```json
{
  "msisdn": "261330000001",
  "operatorId": "MNO_X",
  "mccmnc": "64601",
  "country": "MG",
  "iat": 1730000000,
  "exp": 1730000180,
  "nonce": "random"
}
```

---

## Appendix B — Threat model notes (why these safeguards matter)

If simulation were accessible in prod, an attacker could:
- impersonate an MSISDN
- bypass OTP and subscribe/charge incorrectly

Therefore:
- disable at build/runtime in prod
- require signature + TTL
- restrict access to simulator endpoints
- keep secrets strictly separated

---

**End of document**
