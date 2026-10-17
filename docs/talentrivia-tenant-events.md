# Talentrivia WAP daily vote events

Talentrivia starts an AirtelTigo daily subscription from its own web page. Its server calls `POST /v1/acquisition/transactions` with its intent reference in `click_id`, then calls `POST /v1/acquisition/transactions/{id}/confirm` with the received PIN. The configured tenant is `talentrivia`, channel `timwe-gh-airteltigo-ghana`, campaign `gh-airteltigo-talentrivia-daily-v1`, with an OTP flow and GHS 0.80 daily price. Product, pricepoint and partner role remain the values configured in the console.

The platform forwards subscription changes and successful charges. Talentrivia owns contestant selection, subscription state, votes and SMS. A first successful charge and each successful renewal add one vote at the receiver. No SMS keyword flow is implemented. MO and delivery notifications do not create tenant events. No tenant, campaign, channel or provider credential seed is installed.

## Bind the webhook in the console

Open the Talentrivia tenant, select its existing AirtelTigo channel, then open the credential value form. Choose **Tenant event webhook**. Enter the URL, signing secret, default amount and default currency, then use the existing bind action. The signing secret uses a password input and is never read back. A successful bind clears the webhook form. Credential history shows redacted references and status.

The `tenant_events` JSON blob is:

```json
{"url":"https://talentrivia.com/api/airtime/events","signing_secret":"<at least 16 characters>","default_amount":0.80,"default_currency":"GHS"}
```

The URL must be absolute HTTP or HTTPS without URL credentials or a fragment. The amount must be positive and currency must contain three uppercase letters. Unknown JSON fields are rejected. Both the admin API and dispatcher use `common/tenantevents/config.go`. The existing admin credential endpoint encrypts the JSON through `common/secretcrypto`, stores ciphertext in `tenant_channel_secrets`, and binds a `secret://` reference in `tenant_channel_credentials`. There is no new admin API.

An ACTIVE credential enables enqueueing for the tenant. Revoke it in the credential history to stop enqueueing. Pending events remain pending while the credential is unavailable, with a one-hour backoff and a clear error. Credential absence does not consume delivery attempts. After rebinding, due rows resume; an operator can bring their `next_attempt_at` forward if needed. The dispatcher resolves the ACTIVE binding on every send and caches decrypted config for 15 seconds. For a tenant with several ACTIVE channel bindings it follows the existing SMS resolver convention: highest version, then channel ID. Talentrivia uses one AirtelTigo channel. `env://` references remain supported for parity with the existing SMS resolver; console binding uses encryption.

## Mapping and evidence

| TIMWE notification | Forwarded event | Fields and evidence |
| --- | --- | --- |
| `USER_OPTIN` | `SUBSCRIBE` | Latest nonempty `click_id` for the same tenant and normalized MSISDN within the previous 60 minutes becomes `intentRef`; any campaign and entry channel qualify. `channel=APP` when found, otherwise `SMS`. `tenant_events.go`, `enqueueTenantEvent`, USER_OPTIN branch. |
| `CHARGE` | `CHARGE_SUCCESS` | `transactionRef` is `transactionUUID`. Missing UUID is recorded as skipped with `NO_TRANSACTION_UUID` and logged. Callback amount/currency are retained; missing fields stay absent in storage and are filled from the credential at send time. `tenant_events.go`, CHARGE branch; `tenant_event_dispatcher.go`, `dispatchOneTenantEvent`. |
| `USER_RENEWED` | `CHARGE_SUCCESS` | Same charge mapping and UUID requirement. The owner supplied the production observation that CHARGE occurs once for the first charge and each renewal has a distinct transaction UUID. This run used synthetic callbacks, not provider acceptance evidence. |
| `USER_OPTOUT` | `UNSUBSCRIBE` | `tenant_events.go`, USER_OPTOUT branch. |
| MO, DN and other types | None | Explicit allowlist at the start of `enqueueTenantEvent`. No MO keyword lookup or forwarding. |

The callback route handlers in `services/notification/internal/handler/http.go` establish these TIMWE type names. The receiver's unchanged field, header and response contract is `/home/xper626/workspace/apps/talentrivia/.harness/runs/airtime-daily-vote-20261007/contract.md`. Its old sentence about an environment secret is superseded by credential storage here; the signed HTTP contract is unchanged. No failed-charge callback is inferred from a renewal attempt, and this mapper emits no `CHARGE_FAILED` or `RESUBSCRIBE` event.

`operator` comes from MCC/MNC: 620/03 and 620/06 map to AT, 620/01 to MTN, and 620/02 to TELECEL. When either code is missing, the active channel's operator or key supplies the fallback; `timwe-gh-airteltigo-ghana` maps to AT. Unsupported complete codes or invalid numbers produce skipped rows. `normalizeEventMSISDN` strips nondigits and converts Ghana local forms to country-code digits. The same normalization is used for acquisition matching. `occurredAt` uses a valid RFC 3339 callback `occurredAt`, then `timestamp`, then database receipt time. Non-opt-in events retain the existing entry-channel mapping: web/WAP/HE/landing markers map to APP, otherwise SMS. Opt-in classification depends only on finding its click ID.

Event IDs are `timwe:<TYPE>:<transactionUUID or externalTxId>`. The port retains a deterministic hash fallback for non-charge notifications with neither identifier and bounds IDs to 128 characters. The outbox deduplicates event IDs, source notification IDs, and charge transaction references across CHARGE and USER_RENEWED. A charge without its UUID is never delivered even if it has an external transaction ID.

## Delivery and schema

`NotificationRepository.ProcessNotification` inserts the notification and its tenant event in the same transaction. Enqueue checks ACTIVE credential existence without decrypting it. The notification process starts one event dispatcher with a three-second polling interval. Each tick attempts one due event and one due opt-out. Due rows use `FOR UPDATE SKIP LOCKED` with a one-minute sending lease. An earlier pending or sending row for the same tenant and normalized MSISDN blocks later rows, ordered by creation time and source notification ID. Delivered, skipped, failed and dead rows do not block.

The dispatcher posts directly to the configured URL. It sends `Content-Type: application/json`, `X-Timwe-Contract-Version: tenant-channel-v1.0.0`, `X-Timwe-Timestamp` in Unix seconds, and `X-Timwe-Event-Id` matching the body. `X-Timwe-Signature` is `sha256=` followed by lowercase hex HMAC-SHA256 of `<timestamp>.<eventId>.<raw body>`, keyed by `signing_secret`. Charge defaults are applied before serializing and signing the transmitted body.

Any 2xx delivers the row and stores at most 2048 response bytes. Network errors and 5xx retry after 30 seconds, 2 minutes, 10 minutes, 30 minutes, then hourly. Attempt 12 ends in `dead`. Other HTTP failures, including 400 and 401, become `failed` without retry. A 2xx JSON response with `action: optout` queues a separate opt-out attempt. The dispatcher calls `${SUBSCRIPTION_EXTERNAL_BASE_URL}/api/v1/subscription-external/partners/optout` with tenant/channel query parameters, the callback's product and MSISDN, optional MCC/MNC, and `X-Gateway-Trust` from `GATEWAY_TRUST_SECRET`. Opt-out attempts have separate status, retry count and outcome columns.

`services/acquisition-api/migrations/add_tenant_event_outbox.sql` creates only the ledger, its delivery/opt-out columns and indexes. It creates no endpoint table. Acquisition API's admin schema runner and `scripts/compose-db-bootstrap.sh` register it. The compose base now creates `notifications`, which the existing notification migrations require. Bootstrap also applies the existing product SMS template migration 022 and notification dedup migration 032. Apply the schema before starting the updated notification process. No gateway or production compose changes are required.

## Local run

All evidence in the 2026-10-16 run was local and synthetic. The HTTP capture listener used port 8085; acquisition used 8084; notification used 8082; a throwaway TLS JWKS issuer used 8083. PostgreSQL is `timwe-wap-pg` at 127.0.0.1:15434 and Redis is `timwe-wap-redis` at 127.0.0.1:16379. Both containers were left running. Service and listener processes were stopped. Evidence and temporary local keys are under `/tmp/timwe-wap-evidence`.

The following commands reproduce that path. For the real local Talentrivia receiver, start its API separately with `AIRTIME_EVENTS_SECRET=local-airtime-dev-secret` at `http://127.0.0.1:8090`, and change only the `url` value in the bind command below to `http://127.0.0.1:8090/api/airtime/events`. Keep the local listener running at 8085 for subscription-external opt-out capture and the TLS issuer. The receiver's actual vote behavior was not exercised by this platform run.

### Build

```sh
cd /home/xper626/workspace/worktrees/timwe-subscription/talentrivia-wap-events
mkdir -p /tmp/timwe-wap-evidence
(cd services/acquisition-api && go build ./... && go build -o /tmp/timwe-wap-evidence/acquisition-api ./cmd && go vet ./...)
(cd services/notification && GOFLAGS=-mod=mod go build ./... && GOFLAGS=-mod=mod go build -o /tmp/timwe-wap-evidence/notification ./cmd && GOFLAGS=-mod=mod go vet ./...)
(cd frontend/webspa-admin && npm ci && npm run build:prod)
```

`npm ci` was needed because this worktree had no `node_modules`; use it only when missing. No dependency declarations or lockfiles changed. Notification's checked-in vendor tree lacks existing `common/pii` and the current shared crypto/config packages; module mode uses its existing `replace .../common => ../../common`. No vendor regeneration or module addition was performed. The admin build passed with the existing campaign SCSS size and selector warnings; npm reported the installed Node 24 version is outside the package's Node 18/20 engine range.

### Disposable database and local rows

These reset commands remove only the two explicitly named disposable containers.

```sh
podman rm -f --ignore timwe-wap-pg
podman run -d --name timwe-wap-pg -e POSTGRES_USER=wap -e POSTGRES_PASSWORD=local-airtime-dev-secret -e POSTGRES_DB=wap -p 127.0.0.1:15434:5432 docker.io/library/postgres:16-alpine
podman rm -f --ignore timwe-wap-redis
podman run -d --name timwe-wap-redis -p 127.0.0.1:16379:6379 docker.io/library/redis:7-alpine
PGHOST=127.0.0.1 PGPORT=15434 PGUSER=wap PGPASSWORD=local-airtime-dev-secret PGDATABASE=wap COMPOSE_DB_BOOTSTRAP_WORKSPACE="$PWD" sh scripts/compose-db-bootstrap.sh
PGPASSWORD=local-airtime-dev-secret psql -h 127.0.0.1 -p 15434 -U wap -d wap -v ON_ERROR_STOP=1 <<'SQL'
INSERT INTO tenants (id,tenant_key,name,default_country) VALUES ('11111111-1111-4111-8111-111111111111','talentrivia','Talentrivia local','GH');
INSERT INTO tenant_channels(id,tenant_id,channel_key,provider,country,operator,capabilities) VALUES ('22222222-2222-4222-8222-222222222222','11111111-1111-4111-8111-111111111111','timwe-gh-airteltigo-ghana','timwe','GH','AirtelTigo',ARRAY['optin','confirm','mt','charge']);
INSERT INTO campaigns(slug,country,operator,offer_product_id,pricepoint_id,partner_role_id,flow_type,price,billing_cycle,tenant_id,channel_id) VALUES ('gh-airteltigo-talentrivia-daily-v1','GH','AirtelTigo',32535,800,2117,'OTP',0.80,'daily','11111111-1111-4111-8111-111111111111','22222222-2222-4222-8222-222222222222');
INSERT INTO acquisition_transactions(correlation_id,campaign_slug,msisdn,click_id,tenant_id,created_at) VALUES (gen_random_uuid(),'gh-airteltigo-talentrivia-daily-v1','233261234567','wap-intent-older','11111111-1111-4111-8111-111111111111',now()-interval '10 minutes'),(gen_random_uuid(),'another-campaign-local','0261234567','wap-intent-latest','11111111-1111-4111-8111-111111111111',now()-interval '1 minute');
SQL
```

These product and pricepoint IDs are local fixtures. Nothing is inserted into a real tenant. The second transaction intentionally uses another campaign and a local number format to exercise unrestricted intent matching. Send callbacks within 60 minutes of inserting it.

### Throwaway issuer and local environment

Admin endpoints require a valid JWT. This run used an issuer bound to loopback, a self-signed certificate trusted only by these processes, and an OpenSSL-signed local admin token. No Auth0 or provider credentials were used.

```sh
openssl req -x509 -newkey rsa:2048 -nodes -keyout /tmp/timwe-wap-evidence/local.key -out /tmp/timwe-wap-evidence/local.crt -days 2 -subj /CN=127.0.0.1 -addext subjectAltName=IP:127.0.0.1 >/tmp/timwe-wap-evidence/openssl.log 2>&1
python3 - <<'PY'
import base64,json,subprocess,time
from pathlib import Path
p=Path('/tmp/timwe-wap-evidence')
modulus=subprocess.check_output(['openssl','rsa','-in',str(p/'local.key'),'-noout','-modulus'],stderr=subprocess.DEVNULL).decode().strip().split('=')[1]
def b64(v): return base64.urlsafe_b64encode(v).decode().rstrip('=')
(p/'jwks.json').write_text(json.dumps({'keys':[{'kty':'RSA','kid':'wap-local','use':'sig','alg':'RS256','n':b64(bytes.fromhex(modulus)),'e':'AQAB'}]}))
unsigned=b64(json.dumps({'alg':'RS256','kid':'wap-local','typ':'JWT'}).encode())+'.'+b64(json.dumps({'iss':'https://127.0.0.1:8083/','aud':'wap-local','sub':'wap-local-admin','roles':['platform_admin'],'iat':int(time.time()),'exp':int(time.time())+14400}).encode())
sig=subprocess.check_output(['openssl','dgst','-sha256','-sign',str(p/'local.key')],input=unsigned.encode())
(p/'admin.token').write_text(unsigned+'.'+b64(sig))
(p/'local.env').write_text('export TENANT_SECRET_MASTER_KEY='+base64.b64encode(subprocess.check_output(['openssl','rand','32'])).decode()+'\nexport APP_DATABASE_POSTGRESQL_HOST=127.0.0.1 APP_DATABASE_POSTGRESQL_PORT=15434 APP_DATABASE_POSTGRESQL_USER=wap APP_DATABASE_POSTGRESQL_PASSWORD=local-airtime-dev-secret APP_DATABASE_POSTGRESQL_DB_NAME=wap APP_DATABASE_POSTGRESQL_SSL_MODE=disable\nexport APP_CACHE_REDIS_HOST=127.0.0.1 APP_CACHE_REDIS_PORT=16379\nexport JWT_SECRET=local-airtime-dev-secret INTERNAL_API_SECRET=local-airtime-dev-secret GATEWAY_TRUST_SECRET=local-airtime-dev-secret GATEWAY_TRUST_REQUIRED=true\nexport ADMIN_AUTH0_DOMAIN=127.0.0.1:8083 ADMIN_AUTH0_AUDIENCE=wap-local SSL_CERT_FILE=/tmp/timwe-wap-evidence/local.crt\nexport SUBSCRIPTION_EXTERNAL_BASE_URL=http://127.0.0.1:8085 SUBSCRIPTION_EXTERNAL_URL=http://127.0.0.1:8085\n')
for f in ['local.key','admin.token','local.env']: (p/f).chmod(0o600)
PY
```

Generate the master key once per fresh database. Regenerating it after binding makes the existing ciphertext unreadable. Regenerate the local token if its four-hour lifetime expires.

### Capture listener

This is the temporary Python listener used for verification. It stores exact request bodies and headers in `capture.jsonl`, returns 500 once for the first charge and returns `action: optout` once for the opt-in. GET serves only the local public JWKS.

```sh
cat > /tmp/timwe-wap-evidence/listener.py <<'PY'
import json, ssl, threading, time
from pathlib import Path
from http.server import ThreadingHTTPServer, BaseHTTPRequestHandler
root = Path('/tmp/timwe-wap-evidence')
class Handler(BaseHTTPRequestHandler):
    def log_message(self, *args): pass
    def do_GET(self):
        body = (root / 'jwks.json').read_bytes()
        self.send_response(200); self.send_header('Content-Type','application/json'); self.end_headers(); self.wfile.write(body)
    def do_POST(self):
        body = self.rfile.read(int(self.headers.get('Content-Length','0')))
        event_id = self.headers.get('X-Timwe-Event-Id','')
        status, reply = 200, {'ok':True,'outcome':'applied'}
        mode = root / 'responses.json'
        with lock:
            rules = json.loads(mode.read_text()) if mode.exists() else {}
            if event_id and rules.get('fail_once') == event_id:
                status, reply = 500, {'ok':False}; rules.pop('fail_once')
            elif event_id and rules.get('optout_once') == event_id:
                reply = {'ok':True,'outcome':'rejected','action':'optout'}; rules.pop('optout_once')
            mode.write_text(json.dumps(rules))
            entry = {'time':time.time(),'path':self.path,'headers':dict(self.headers),'body':body.decode(),'status':status,'reply':reply}
            with (root / 'capture.jsonl').open('a') as out: out.write(json.dumps(entry)+'\n')
            print(json.dumps(entry), flush=True)
        self.send_response(status); self.send_header('Content-Type','application/json'); self.end_headers(); self.wfile.write(json.dumps(reply).encode())
lock = threading.Lock()
tls = ThreadingHTTPServer(('127.0.0.1',8083), Handler)
ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER); ctx.load_cert_chain(str(root/'local.crt'), str(root/'local.key'))
tls.socket = ctx.wrap_socket(tls.socket, server_side=True)
threading.Thread(target=tls.serve_forever, daemon=True).start()
ThreadingHTTPServer(('127.0.0.1',8085), Handler).serve_forever()
PY
printf '%s' '{"fail_once":"timwe:CHARGE:local-first-charge","optout_once":"timwe:USER_OPTIN:local-optin"}' > /tmp/timwe-wap-evidence/responses.json
python3 -u /tmp/timwe-wap-evidence/listener.py > /tmp/timwe-wap-evidence/listener.log 2>&1
```

Run the listener in a separate terminal. Its exception responses apply only when it receives events on port 8085. A real receiver at 8090 determines its own responses.

### Start services and bind

Run these in separate terminals after the listener has started:

```sh
cd /home/xper626/workspace/worktrees/timwe-subscription/talentrivia-wap-events/services/acquisition-api
. /tmp/timwe-wap-evidence/local.env
APP_APPLICATION_PORT=8084 /tmp/timwe-wap-evidence/acquisition-api > /tmp/timwe-wap-evidence/acquisition-runtime.log 2>&1
```

```sh
cd /home/xper626/workspace/worktrees/timwe-subscription/talentrivia-wap-events/services/notification
. /tmp/timwe-wap-evidence/local.env
APPLICATION_PORT=8082 NOTIFICATION_REQUIRE_TENANT_CONTEXT=true /tmp/timwe-wap-evidence/notification > /tmp/timwe-wap-evidence/notification-runtime.log 2>&1
```

Bind through the same admin endpoint used by the console. For the real local API, replace `http://127.0.0.1:8085/api/airtime/events` below with `http://127.0.0.1:8090/api/airtime/events`.

```sh
python3 - <<'PY'
import json,urllib.request
from pathlib import Path
p=Path('/tmp/timwe-wap-evidence')
config={'url':'http://127.0.0.1:8085/api/airtime/events','signing_secret':'local-airtime-dev-secret','default_amount':0.80,'default_currency':'GHS'}
(p/'credential.json').write_text(json.dumps({'purpose':'tenant_events','secret_value':json.dumps(config)}))
req=urllib.request.Request('http://127.0.0.1:8084/v1/admin/channels/22222222-2222-4222-8222-222222222222/credentials',data=(p/'credential.json').read_bytes(),headers={'Authorization':'Bearer '+(p/'admin.token').read_text(),'Content-Type':'application/json','X-Tenant-Key':'talentrivia'})
with urllib.request.urlopen(req) as response:
    raw=response.read(); (p/'credential-response.json').write_bytes(raw); print(response.status,raw.decode())
PY
PGPASSWORD=local-airtime-dev-secret psql -h 127.0.0.1 -p 15434 -U wap -d wap <<'SQL'
SELECT c.purpose,c.status,left(c.secret_ref,9) AS scheme,octet_length(s.ciphertext) AS encrypted_bytes
FROM tenant_channel_credentials c JOIN tenant_channel_secrets s ON c.secret_ref='secret://'||s.id::text;
SQL
```

Observed: HTTP 201, ACTIVE, `secret://[REDACTED]`, and 175 ciphertext bytes for this local blob. The response contains no signing secret.

### Post the callback sequence

`checkGatewayTrust` verifies HMAC-SHA256 of the fixed message `gateway-trust-marker`, keyed by `GATEWAY_TRUST_SECRET`. The body type is taken from the route. These calls exercise the actual handler and database transaction:

```sh
python3 - <<'PY'
import json,urllib.request,hmac,hashlib
from pathlib import Path
p=Path('/tmp/timwe-wap-evidence')
trust=hmac.new(b'local-airtime-dev-secret',b'gateway-trust-marker',hashlib.sha256).hexdigest()
notifications=[('user-optin','local-optin'),('charge','local-first-charge'),('charge','local-first-charge'),('user-renewed','local-renewal'),('user-optout','local-optout'),('mo','local-mo')]
for route,txid in notifications:
    body={'productId':32535,'pricepointId':800,'msisdn':'233261234567','transactionUUID':txid,'externalTxId':'external-'+txid,'mcc':'620','mnc':'06','entryChannel':'OTHER','message':'TNT N1' if route=='mo' else ''}
    req=urllib.request.Request('http://127.0.0.1:8082/api/v1/notification/'+route+'/2117?tenant_key=talentrivia&channel_key=timwe-gh-airteltigo-ghana',data=json.dumps(body).encode(),headers={'Content-Type':'application/json','X-Gateway-Trust':trust})
    with urllib.request.urlopen(req) as res: print(route,txid,res.status,res.read().decode())
PY
PGPASSWORD=local-airtime-dev-secret psql -h 127.0.0.1 -p 15434 -U wap -d wap <<'SQL'
SELECT event_id,status,attempts,last_http_status,last_error,
 round(extract(epoch from next_attempt_at-updated_at)) AS delay_seconds,
 payload->>'intentRef' AS intent,payload->>'amount' AS stored_amount,
 payload->>'currency' AS stored_currency,optout_status,optout_outcome
FROM tenant_event_outbox ORDER BY created_at,source_notification_id;
SQL
```

All six calls returned 200. Four events were created. Opt-in carried `wap-intent-latest`, channel APP and operator AT. Stored charge defaults were absent; transmitted values were 0.8 GHS. During the first charge's 500 backoff it had attempts=1, delay_seconds=30; renewal and unsubscribe both had attempts=0. The first charge succeeded on its second attempt at 17:21:40 UTC. Renewal followed at 17:21:43 and unsubscribe at 17:21:46. The first opt-in response triggered a local subscription-external opt-out with MCC 620, MNC 06, product 32535 and the matching tenant/channel keys. Inspect the query while the first charge is pending, then again after its real 30-second backoff and polling ticks.

Recompute each captured signature with OpenSSL, independently of the Go signer:

```sh
python3 - <<'PY'
import json,subprocess
from pathlib import Path
for line in Path('/tmp/timwe-wap-evidence/capture.jsonl').read_text().splitlines():
    x=json.loads(line); h={k.lower():v for k,v in x['headers'].items()}
    if 'x-timwe-event-id' not in h: continue
    data=(h['x-timwe-timestamp']+'.'+h['x-timwe-event-id']+'.'+x['body']).encode()
    digest=subprocess.check_output(['openssl','dgst','-sha256','-hmac','local-airtime-dev-secret'],input=data).decode().strip().split()[-1]
    print(h['x-timwe-event-id'],x['status'],'MATCH' if 'sha256='+digest==h['x-timwe-signature'] else 'MISMATCH')
PY
```

All eight event requests captured across the baseline and additional cases matched, including the failed attempts. The subscription-external trust header also matched an independent OpenSSL digest of `gateway-trust-marker`.

### Revocation and pending delivery

The run additionally sent an opt-in for an unmatched number without MCC/MNC, a charge with only an external ID, a charge with explicit amount `0.95`, and a third number's charge whose capture response was 500. Observed: unmatched opt-in used SMS and channel fallback AT; the missing-UUID charge was skipped and logged; 0.95 was preserved. The third number's event ID was `timwe:CHARGE:local-paused-charge`. Its one-time listener override was written as follows, then that UUID was posted through the same charge route:

```sh
printf '%s' '{"fail_once":"timwe:CHARGE:local-paused-charge"}' > /tmp/timwe-wap-evidence/responses.json
```

After its 500, revoke with the existing admin API:

```sh
python3 - <<'PY'
import json,urllib.request
from pathlib import Path
p=Path('/tmp/timwe-wap-evidence')
credential=json.loads((p/'credential-response.json').read_text())['credential_id']
req=urllib.request.Request('http://127.0.0.1:8084/v1/admin/channels/22222222-2222-4222-8222-222222222222/credentials/'+credential,method='DELETE',headers={'Authorization':'Bearer '+(p/'admin.token').read_text(),'X-Tenant-Key':'talentrivia'})
with urllib.request.urlopen(req) as res: print(res.status,res.read().decode())
PY
PGPASSWORD=local-airtime-dev-secret psql -h 127.0.0.1 -p 15434 -U wap -d wap <<'SQL'
UPDATE tenant_event_outbox SET next_attempt_at=now() WHERE event_id='timwe:CHARGE:local-paused-charge';
SELECT event_id,status,attempts,last_error,round(extract(epoch from next_attempt_at-updated_at)) AS delay_seconds
FROM tenant_event_outbox WHERE event_id='timwe:CHARGE:local-paused-charge';
SQL
```

The UPDATE deliberately brought only this local pending row due so the missing-credential behavior could be observed immediately. Requery after a dispatcher tick. Observed: pending, attempts=1, `tenant_events credential missing or revoked`, delay_seconds=3600. Then a synthetic USER_RENEWED with UUID `local-after-revoke` returned 200 and was stored in `notifications`, with zero matching rows in `tenant_event_outbox`.

Stop the three foreground processes with Ctrl-C when finished. Keep the two named containers running for inspection. The run did not exercise real OTP initiation/confirmation, a carrier charge, the real Talentrivia API or vote updates, HTTP 400/401 delivery outcomes, a network failure, all twelve retry attempts, concurrent dispatcher processes, or browser interaction with the console. No test suites or review passes were run.
