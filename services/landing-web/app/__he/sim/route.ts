/**
 * HE Simulator Route - Staging/Local Only
 *
 * GET  /__he/sim - Display simulator form
 * POST /__he/sim - Generate simulation token and set cookie
 *
 * SECURITY: Returns 404 when HE_SIMULATION_ENABLED !== 'true'
 */

import { NextRequest, NextResponse } from 'next/server'
import {
  getHESimConfig,
  createSimulationToken,
  normalizeMsisdn,
  isValidMsisdn,
  GHANA_OPERATORS,
} from '@/lib/he-simulation'

/**
 * GET /__he/sim - Display simulator form
 */
export async function GET() {
  const config = getHESimConfig()

  if (!config.enabled) {
    return new NextResponse('Not Found', { status: 404 })
  }

  const html = `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>HE Simulator (Staging)</title>
  <style>
    * { box-sizing: border-box; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      max-width: 600px;
      margin: 2rem auto;
      padding: 1rem;
      background: #f5f5f5;
    }
    .container {
      background: white;
      padding: 2rem;
      border-radius: 8px;
      box-shadow: 0 2px 4px rgba(0,0,0,0.1);
    }
    h1 {
      color: #333;
      margin-top: 0;
      font-size: 1.5rem;
    }
    .warning {
      background: #fff3cd;
      border: 1px solid #ffc107;
      padding: 1rem;
      border-radius: 4px;
      margin-bottom: 1.5rem;
      font-size: 0.875rem;
    }
    .warning strong { color: #856404; }
    label {
      display: block;
      margin-bottom: 0.5rem;
      font-weight: 500;
      color: #333;
    }
    input, select {
      width: 100%;
      padding: 0.75rem;
      border: 1px solid #ddd;
      border-radius: 4px;
      font-size: 1rem;
      margin-bottom: 1rem;
    }
    input:focus, select:focus {
      outline: none;
      border-color: #007bff;
      box-shadow: 0 0 0 2px rgba(0,123,255,0.25);
    }
    button {
      width: 100%;
      padding: 1rem;
      background: #007bff;
      color: white;
      border: none;
      border-radius: 4px;
      font-size: 1rem;
      font-weight: 500;
      cursor: pointer;
    }
    button:hover { background: #0056b3; }
    .help {
      font-size: 0.75rem;
      color: #666;
      margin-top: -0.75rem;
      margin-bottom: 1rem;
    }
    .operator-info {
      background: #f8f9fa;
      padding: 1rem;
      border-radius: 4px;
      font-size: 0.875rem;
      margin-bottom: 1rem;
    }
    .operator-info h3 {
      margin: 0 0 0.5rem 0;
      font-size: 0.875rem;
    }
    .operator-info code {
      background: #e9ecef;
      padding: 0.125rem 0.375rem;
      border-radius: 3px;
      font-size: 0.75rem;
    }
  </style>
</head>
<body>
  <div class="container">
    <h1>🧪 HE Simulator (Staging/Local)</h1>

    <div class="warning">
      <strong>⚠️ Testing Only</strong><br>
      This tool simulates Header Enrichment detection for testing subscription flows.
      The token expires after ${config.ttlSeconds} seconds.
    </div>

    <div class="operator-info">
      <h3>Ghana Operators (MCC 620)</h3>
      <p>
        <strong>MTN:</strong> MNC <code>01</code> | Example: <code>233240000001</code><br>
        <strong>Telecel:</strong> MNC <code>02</code> | Example: <code>233200000001</code><br>
        <strong>AT Ghana:</strong> MNC <code>03</code> or <code>06</code> | Example: <code>233260000001</code>
      </p>
    </div>

    <form method="POST" action="/__he/sim">
      <label for="msisdn">MSISDN (Phone Number)</label>
      <input
        type="text"
        id="msisdn"
        name="msisdn"
        placeholder="233240000001"
        pattern="[0-9]{9,15}"
        required
      />
      <p class="help">Enter without + prefix (e.g., 233240000001 for Ghana)</p>

      <label for="operator">Operator</label>
      <select id="operator" name="operator">
        <option value="MTN">MTN Ghana (MCC 620, MNC 01)</option>
        <option value="TELECEL">Telecel Ghana (MCC 620, MNC 02)</option>
        <option value="AT_03">AT Ghana (MCC 620, MNC 03)</option>
        <option value="AT_06">AT Ghana (MCC 620, MNC 06)</option>
        <option value="custom">Custom...</option>
      </select>

      <div id="custom-fields" style="display: none;">
        <label for="mcc">MCC (Mobile Country Code)</label>
        <input type="text" id="mcc" name="mcc" placeholder="620" />

        <label for="mnc">MNC (Mobile Network Code)</label>
        <input type="text" id="mnc" name="mnc" placeholder="01" />
      </div>

      <label for="redirect">Redirect To (after setting cookie)</label>
      <input
        type="text"
        id="redirect"
        name="redirect"
        value="/"
        placeholder="/lp/campaign-slug"
      />
      <p class="help">Landing page path to redirect to after simulation starts</p>

      <button type="submit">Start HE Simulation →</button>
    </form>
  </div>

  <script>
    document.getElementById('operator').addEventListener('change', function() {
      const customFields = document.getElementById('custom-fields');
      const mccInput = document.getElementById('mcc');
      const mncInput = document.getElementById('mnc');

      if (this.value === 'custom') {
        customFields.style.display = 'block';
      } else {
        customFields.style.display = 'none';
        // Pre-fill MCC/MNC based on selection
        const operators = {
          MTN: { mcc: '620', mnc: '01' },
          TELECEL: { mcc: '620', mnc: '02' },
          AT_03: { mcc: '620', mnc: '03' },
          AT_06: { mcc: '620', mnc: '06' }
        };
        const op = operators[this.value];
        if (op) {
          mccInput.value = op.mcc;
          mncInput.value = op.mnc;
        }
      }
    });
    // Trigger initial selection
    document.getElementById('operator').dispatchEvent(new Event('change'));
  </script>
</body>
</html>
`

  return new NextResponse(html, {
    headers: { 'Content-Type': 'text/html; charset=utf-8' },
  })
}

/**
 * POST /__he/sim - Generate token and set cookie
 */
export async function POST(request: NextRequest) {
  const config = getHESimConfig()

  if (!config.enabled) {
    return new NextResponse('Not Found', { status: 404 })
  }

  if (!config.secret) {
    return new NextResponse('HE_SIM_SECRET not configured', { status: 500 })
  }

  // Parse form data
  const formData = await request.formData()
  const msisdn = formData.get('msisdn') as string
  const operator = formData.get('operator') as string
  const customMcc = formData.get('mcc') as string
  const customMnc = formData.get('mnc') as string
  const redirect = (formData.get('redirect') as string) || '/'

  // Validate MSISDN
  if (!msisdn || !isValidMsisdn(msisdn)) {
    return new NextResponse('Invalid MSISDN format', { status: 400 })
  }

  // Resolve operator MCC/MNC
  let mcc: string
  let mnc: string
  let operatorId: string

  if (operator === 'custom') {
    mcc = customMcc || '620'
    mnc = customMnc || '01'
    operatorId = `${mcc}-${mnc}`
  } else {
    const opConfig = GHANA_OPERATORS[operator as keyof typeof GHANA_OPERATORS]
    if (opConfig) {
      mcc = opConfig.mcc
      mnc = opConfig.mnc
      operatorId = opConfig.name
    } else {
      mcc = '620'
      mnc = '01'
      operatorId = 'Unknown'
    }
  }

  // Create signed token
  const token = await createSimulationToken(
    {
      msisdn: normalizeMsisdn(msisdn),
      operatorId,
      mcc,
      mnc,
      country: 'GH',
    },
    config
  )

  // Create redirect response with cookie
  const response = NextResponse.redirect(new URL(redirect, request.url), 302)

  response.cookies.set(config.cookieName, token, {
    httpOnly: true,
    secure: process.env.NODE_ENV === 'production',
    sameSite: 'lax',
    maxAge: config.ttlSeconds,
    path: '/',
  })

  return response
}
