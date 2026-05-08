# Auth0 Production Deployment Checklist

This guide helps you deploy Auth0 JWT authentication to your production droplet.

## Prerequisites

- SSH access to production droplet
- Docker and Docker Compose installed
- Auth0 tenant configured with:
  - Domain: `dev-chliep5q.auth0.com`
  - API Audience: `https://dev-chliep5q.auth0.com/api/v2/`
  - Application configured for SPA (Single Page Application)

## Step 1: Add Environment Variables to Droplet

SSH into your production droplet and navigate to your project directory:

```bash
ssh user@your-droplet-ip
cd /path/to/project  # Where docker-compose.prod-do.yml is located
```

Add the following to your `.env` file:

```bash
cat >> .env << 'EOF'

# Auth0 Configuration for Admin APIs
ADMIN_AUTH0_DOMAIN=dev-chliep5q.auth0.com
ADMIN_AUTH0_AUDIENCE=https://dev-chliep5q.auth0.com/api/v2/

# CORS origins for admin frontend
ACQUISITION_ADMIN_CORS_ORIGINS=https://admin.nouveauricheglobalgroup.com
CADENCE_ADMIN_CORS_ORIGINS=https://admin.nouveauricheglobalgroup.com
EOF
```

Verify the variables were added:

```bash
grep -E "AUTH0|CORS_ORIGINS" .env
```

## Step 2: Rebuild and Restart Services

Pull the latest images and restart services:

```bash
# Pull latest images (if using remote registry)
docker compose -f docker-compose.prod-do.yml pull

# Restart services to pick up new environment variables
docker compose -f docker-compose.prod-do.yml down
docker compose -f docker-compose.prod-do.yml up -d
```

## Step 3: Verify Services Are Running

Check service status:

```bash
docker compose -f docker-compose.prod-do.yml ps
```

All services should show `Up` status.

## Step 4: Check Logs for Configuration

Verify Auth0 configuration is loaded correctly:

```bash
# Check acquisition-api logs
docker compose -f docker-compose.prod-do.yml logs acquisition-api | grep -i "auth0\|admin"

# Check cadence-engine logs
docker compose -f docker-compose.prod-do.yml logs cadence-engine | grep -i "auth0\|admin"
```

**Expected behavior:**
- No "Admin access not configured" errors (503)
- Services should start without Auth0-related errors

## Step 5: Test Authentication Flow

1. **Open the admin frontend:** `https://admin.nouveauricheglobalgroup.com`
2. **Login with Auth0:** Click "Login with Auth0" and complete authentication
3. **Test an API call:** Navigate to Dashboard or Reports
4. **Check for 401 errors:** If you see 401, check the logs:

```bash
# Watch logs in real-time during a test request
docker compose -f docker-compose.prod-do.yml logs -f acquisition-api
```

Look for log lines like:
```
admin auth failed (acquisition-api): remote_ip=... err=invalid token: issuer mismatch (got "..." want "...")
admin auth failed (acquisition-api): remote_ip=... err=invalid token: audience mismatch (got [...] want "...")
```

## Troubleshooting

### Issue: "Admin access not configured" (503)

**Cause:** Environment variables not set or services not restarted.

**Fix:**
1. Verify `.env` file has `ADMIN_AUTH0_DOMAIN` and `ADMIN_AUTH0_AUDIENCE`
2. Restart services: `docker compose -f docker-compose.prod-do.yml restart acquisition-api cadence-engine`

### Issue: "Unauthorized" (401) after login

**Cause:** JWT validation failing (issuer/audience mismatch, expired token, or JWKS fetch failure).

**Diagnosis:**
Check the logs for specific error messages:
```bash
docker compose -f docker-compose.prod-do.yml logs acquisition-api | grep "admin auth failed"
```

**Common fixes:**

1. **Issuer mismatch:**
   - Verify `ADMIN_AUTH0_DOMAIN=dev-chliep5q.auth0.com` (no `https://` prefix)
   - Backend expects issuer: `https://dev-chliep5q.auth0.com/`

2. **Audience mismatch:**
   - Verify `ADMIN_AUTH0_AUDIENCE=https://dev-chliep5q.auth0.com/api/v2/` (exact match, including trailing `/`)
   - Check Auth0 API configuration matches this exact value

3. **JWKS fetch failure:**
   - Ensure droplet can reach `https://dev-chliep5q.auth0.com/.well-known/jwks.json`
   - Test: `curl https://dev-chliep5q.auth0.com/.well-known/jwks.json`

4. **Token expired:**
   - Auth0 tokens typically expire after 24 hours
   - User needs to log in again

### Issue: CORS errors in browser

**Cause:** Frontend origin not in allowed CORS list.

**Fix:**
- Verify `ACQUISITION_ADMIN_CORS_ORIGINS` includes `https://admin.nouveauricheglobalgroup.com`
- Restart acquisition-api service

## Verification Checklist

- [ ] `.env` file contains `ADMIN_AUTH0_DOMAIN` and `ADMIN_AUTH0_AUDIENCE`
- [ ] Services restarted after adding environment variables
- [ ] No "Admin access not configured" errors in logs
- [ ] Can log in via Auth0 in frontend
- [ ] Dashboard/Reports load without 401 errors
- [ ] Logs show successful JWT validation (no "admin auth failed" messages)

## Next Steps

After successful deployment:
1. Monitor logs for any authentication issues
2. Verify all admin endpoints are accessible
3. Test logout and re-login flow
4. Document any custom Auth0 rules or permissions needed

## Support

If issues persist:
1. Check service logs: `docker compose -f docker-compose.prod-do.yml logs [service-name]`
2. Verify Auth0 tenant configuration matches expected values
3. Test JWT token manually using [jwt.io](https://jwt.io) to verify claims
4. Ensure network connectivity to Auth0 endpoints from droplet
