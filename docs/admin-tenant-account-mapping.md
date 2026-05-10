# Admin Tenant Account Mapping

This admin stack maps users to tenant access through Auth0 identity claims first. The backend does not currently own a database membership table for admin users.

## Tenant Admin Mapping

New tenant-scoped admin accounts should be assigned in Auth0 with at least one tenant claim:

```json
{
  "tenant_key": "tenant-a",
  "tenant_id": "tenant-a",
  "roles": ["tenant_admin"]
}
```

`webspa-admin` also accepts tenant lists from `tenants`, `tenant_options`, `tenantOptions`, or `workspaceTenants` in the Auth0 profile or token. Each entry can include `tenant_key`, `tenant_id`, and `name`.

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

## Current Limitation

There is no repository-owned admin membership table yet. For new users, Auth0 remains the source of truth for tenant and platform assignment. A future membership slice can add database-backed mappings by Auth0 subject/email if the platform needs self-service admin invitations or per-tenant admin provisioning inside this application.
