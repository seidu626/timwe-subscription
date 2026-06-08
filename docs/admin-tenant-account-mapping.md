# Admin Tenant Account Mapping

This admin stack maps users to tenant access through Auth0 identity plus the repo-owned `tenant_admin_memberships` table. Auth0 proves who the user is; the membership table is the application source of truth for tenant workspace access when the user is not platform-scoped.

## Tenant Admin Mapping

New tenant-scoped admin accounts should be created in Auth0 and then assigned to the tenant through the admin portal membership flow. JWT tenant claims can still bootstrap or narrow access, but they are not the only source of tenant assignment.

```json
{
  "tenant_key": "tenant-a",
  "tenant_id": "tenant-a",
  "roles": ["tenant_admin"]
}
```

`webspa-admin` also accepts tenant lists from `tenants`, `tenant_options`, `tenantOptions`, or `workspaceTenants` in the Auth0 profile or token. Each entry can include `tenant_key`, `tenant_id`, and `name`. The backend membership lookup resolves active tenant access for non-platform identities when the JWT does not carry a tenant claim.

## Admin Portal Tenant Onboarding Flow

The admin portal onboarding flow should create production tenants in this order:

1. Tenant profile: stable `tenant_key`, display name, owner, country/currency defaults, and status.
2. Admin membership: tenant-scoped admin assignment from Auth0 subject/email through `tenant_admin_memberships`; platform admins must still select an active tenant workspace before tenant-scoped actions.
3. Channel: stable `channel_key`, provider mapping, enabled capabilities, callback URL, postback URL, and gateway/API exposure.
4. Credential reference: tenant/channel secret reference for provider auth material such as `TIMWE_API_KEY`, `TIMWE_PSK`, callback shared secrets, postback signing secrets, and equivalent provider credentials. Raw values must not be stored in portal-visible configuration.
5. Defaults: product, userbase, renewal cadence, billing cadence, notification, and postback defaults for the tenant/channel.
6. Validation and smoke: tenant/channel resolution, signed callback handling, enabled-capability checks, credential-reference resolution, cross-tenant refusal, and postback delivery.
7. Activation: switch the tenant/channel to active only after smoke evidence is recorded and any exposed credential-like values have been rotated.

## Platform Admin Mapping

All-tenant admin accounts are platform-scoped identities. Auth0 should assign one of these platform markers:

- role: `platform_operator`, `platform_admin`, or `super_admin`
- permission: `platform:all_tenants`, `tenants:*`, or `admin:platform`

Platform-scoped users still operate inside an active tenant workspace for tenant-specific admin routes. The frontend attaches the selected workspace as `X-Tenant-Key` and, when available, `X-Tenant-Id`. `acquisition-api` only applies those selected tenant headers after the JWT identity is already platform scoped.

## Bootstrap Admins

The following emails are the approved bootstrap all-tenant platform admins for this deployment:

- `almauricin@gmail.com`
- `seidu.abdulai@hotmail.com`

Development bootstrap config lives in `environment.adminTenantBootstrap`. Production builds fail closed and should provide the approved email and tenant catalog through `window.__ADMIN_TENANT_BOOTSTRAP__` before Angular starts:

```js
window.__ADMIN_TENANT_BOOTSTRAP__ = {
  platformAdminEmails: ["almauricin@gmail.com", "seidu.abdulai@hotmail.com"],
  tenantWorkspaces: [
    { tenant_key: "tenant-a", tenant_id: "tenant-a", name: "Tenant A" },
    { tenant_key: "tenant-b", tenant_id: "tenant-b", name: "Tenant B" }
  ]
};
```

Backend bootstrap config uses `ADMIN_BOOTSTRAP_PLATFORM_EMAILS`. If unset, no email receives bootstrap platform scope. Set it to `almauricin@gmail.com,seidu.abdulai@hotmail.com` in the target environment. The Auth0 access token must include the account `email` claim and `email_verified: true` so `acquisition-api` can recognize the bootstrap principal.

## Current Membership Contract

`tenant_admin_memberships` is the repository-owned tenant assignment table. Platform-scoped admins can list tenant catalog rows and upsert tenant members through `/v1/admin/tenants/{tenant_id}/members`. Non-platform admins get tenant context from this active membership table when their JWT has no tenant claim. A single active membership is stamped automatically; multiple active memberships require a selected `X-Tenant-Key` that matches one of the membership rows.

Activation evidence for a tenant must include the Auth0 subject/email assigned, the membership role/status, and proof that tenant-scoped admin routes resolve to the intended workspace.
