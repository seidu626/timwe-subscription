import { HttpBackend, HttpClient } from '@angular/common/http';
import { Injectable, Injector } from '@angular/core';
import { AuthService, User } from '@auth0/auth0-angular';
import { BehaviorSubject, combineLatest, Observable, of } from 'rxjs';
import { catchError, map, switchMap } from 'rxjs/operators';
import { environment } from '../../../environments/environment';

const TENANT_SELECTION_STORAGE_KEY = 'webspa-admin.selected-tenant';

export interface TenantWorkspaceOption {
  identifier: string;
  tenantId: string;
  tenantKey: string;
  label: string;
}

export interface TenantWorkspaceState {
  authenticated: boolean;
  loading: boolean;
  platformScoped: boolean;
  currentTenant: TenantWorkspaceOption | null;
  availableTenants: TenantWorkspaceOption[];
  canSwitchTenant: boolean;
  status: 'loading' | 'unauthenticated' | 'missing-tenant' | 'selection-required' | 'invalid-selection' | 'ready';
  reason: string | null;
}

interface ClaimSnapshot {
  tenantId: string | null;
  tenantKey: string | null;
  tenantOptions: TenantWorkspaceOption[];
  platformScoped: boolean;
}

interface AdminTenantBootstrapConfig {
  platformAdminEmails?: unknown;
  platformAdminSubjects?: unknown;
  tenantWorkspaces?: unknown;
}

interface BackendWorkspaceSnapshot {
  platformScoped: boolean;
  tenantOptions: TenantWorkspaceOption[];
}

interface AdminTenantWorkspaceResponse {
  platform_scoped?: boolean;
  platformScoped?: boolean;
  tenants?: unknown;
}

@Injectable({
  providedIn: 'root'
})
export class TenantWorkspaceService {
  private readonly selectionSubject = new BehaviorSubject<string | null>(this.readStoredSelection());
  private readonly workspaceSubject = new BehaviorSubject<TenantWorkspaceState>(this.createLoadingState());
  private readonly backendWorkspaceSubject = new BehaviorSubject<BackendWorkspaceSnapshot | null | undefined>(undefined);
  private readonly workspaceEndpoint = `${environment.acquisitionApiEndpoint}/v1/admin/tenants/workspaces`;
  private backendHttp: HttpClient | null = null;
  private workspaceFetchInFlight = false;

  readonly workspace$: Observable<TenantWorkspaceState> = this.workspaceSubject.asObservable();

  constructor(
    private readonly auth: AuthService,
    private readonly injector: Injector
  ) {
    combineLatest([
      this.auth.isLoading$,
      this.auth.isAuthenticated$,
      this.auth.user$,
      this.selectionSubject,
      this.backendWorkspaceSubject
    ]).pipe(
      map(([loading, authenticated, user, selection, backend]) => this.resolveWorkspace(loading, authenticated, user, selection, backend))
    ).subscribe((state) => {
      this.workspaceSubject.next(state);
    });

    combineLatest([this.auth.isLoading$, this.auth.isAuthenticated$]).subscribe(([loading, authenticated]) => {
      if (loading) {
        return;
      }
      if (!authenticated) {
        this.backendWorkspaceSubject.next(null);
        return;
      }
      this.refreshBackendWorkspace();
    });
  }

  selectTenant(identifier: string): boolean {
    const normalized = this.normalizeIdentifier(identifier);

    if (!normalized) {
      return false;
    }

    const workspace = this.workspaceSubject.value;
    const option = this.findTenantOption(workspace.availableTenants, normalized);

    if (!option) {
      return false;
    }

    this.selectionSubject.next(option.identifier);
    sessionStorage.setItem(TENANT_SELECTION_STORAGE_KEY, option.identifier);
    return true;
  }

  /**
   * Re-fetches tenant workspaces from the backend. The automatic fetch runs
   * only on auth transitions, so a single failure (expired Auth0 session,
   * transient API error) would otherwise pin the workspace on
   * 'missing-tenant' until the user signs out and back in.
   */
  refreshWorkspace(): void {
    this.refreshBackendWorkspace();
  }

  clearTenantSelection(): void {
    this.selectionSubject.next(null);
    sessionStorage.removeItem(TENANT_SELECTION_STORAGE_KEY);
  }

  getCurrentWorkspace(): TenantWorkspaceState {
    return this.workspaceSubject.value;
  }

  isWorkspaceRequest(url: string): boolean {
    if (url.startsWith(this.workspaceEndpoint)) {
      return false;
    }
    return [
      this.getApiBaseUrl(),
      this.getSubscriptionApiUrl(),
      this.getSubscriptionExternalAdminApiUrl(),
      this.getNotificationApiUrl(),
      this.getAcquisitionApiUrl(),
      this.getCadenceEngineUrl()
    ].some((baseUrl) => baseUrl.length > 0 && url.startsWith(baseUrl)) || url.includes('/api/');
  }

  private resolveWorkspace(
    loading: boolean,
    authenticated: boolean,
    user: User | null | undefined,
    selection: string | null,
    backend: BackendWorkspaceSnapshot | null | undefined
  ): TenantWorkspaceState {
    if (loading) {
      return this.createLoadingState();
    }

    if (!authenticated || !user) {
      return {
        authenticated: false,
        loading: false,
        platformScoped: false,
        currentTenant: null,
        availableTenants: [],
        canSwitchTenant: false,
        status: 'unauthenticated',
        reason: null
      };
    }

    const claimSnapshot = this.extractClaims(user, backend ?? null);

    if (backend === undefined && !claimSnapshot.tenantOptions.length) {
      return this.createLoadingState();
    }

    const currentTenant = this.resolveCurrentTenant(claimSnapshot, selection);
    const canSwitchTenant = claimSnapshot.tenantOptions.length > 1;
    const invalidSelection = Boolean(selection) && claimSnapshot.tenantOptions.length > 1 && !currentTenant;

    if (!claimSnapshot.tenantOptions.length) {
      return {
        authenticated: true,
        loading: false,
        platformScoped: claimSnapshot.platformScoped,
        currentTenant: null,
        availableTenants: [],
        canSwitchTenant: false,
        status: 'missing-tenant',
        reason: 'missing-tenant'
      };
    }

    if (invalidSelection) {
      return {
        authenticated: true,
        loading: false,
        platformScoped: claimSnapshot.platformScoped,
        currentTenant: null,
        availableTenants: claimSnapshot.tenantOptions,
        canSwitchTenant,
        status: 'invalid-selection',
        reason: 'invalid-selection'
      };
    }

    if (!currentTenant) {
      const requiresSelection = claimSnapshot.tenantOptions.length > 1;

      return {
        authenticated: true,
        loading: false,
        platformScoped: claimSnapshot.platformScoped,
        currentTenant: null,
        availableTenants: claimSnapshot.tenantOptions,
        canSwitchTenant,
        status: requiresSelection ? 'selection-required' : 'missing-tenant',
        reason: requiresSelection ? 'selection-required' : 'missing-tenant'
      };
    }

    return {
      authenticated: true,
      loading: false,
      platformScoped: claimSnapshot.platformScoped,
      currentTenant,
      availableTenants: claimSnapshot.tenantOptions,
      canSwitchTenant,
      status: 'ready',
      reason: null
    };
  }

  private resolveCurrentTenant(snapshot: ClaimSnapshot, selection: string | null): TenantWorkspaceOption | null {
    const bySelection = selection
      ? this.findTenantOption(snapshot.tenantOptions, selection)
      : null;

    if (bySelection) {
      return bySelection;
    }

    if (!snapshot.platformScoped) {
      if (selection && snapshot.tenantOptions.length > 1) {
        return null;
      }

      if (snapshot.tenantId || snapshot.tenantKey) {
        return this.findTenantOption(snapshot.tenantOptions, snapshot.tenantKey ?? snapshot.tenantId ?? '');
      }

      return snapshot.tenantOptions.length === 1 ? snapshot.tenantOptions[0] : null;
    }

    return snapshot.tenantOptions.length === 1 ? snapshot.tenantOptions[0] : null;
  }

  private extractClaims(user: User, backend: BackendWorkspaceSnapshot | null): ClaimSnapshot {
    const record = this.asRecord(user);
    const metadata = this.asRecord(record['app_metadata']) ?? this.asRecord(record['user_metadata']) ?? {};
    const source = {
      ...metadata,
      ...record
    };

    const tenantIds = this.collectStrings(source, ['tenant_id', 'tenantId', 'tenant_ids', 'tenantIds']);
    const tenantKeys = this.collectStrings(source, ['tenant_key', 'tenantKey', 'tenant_keys', 'tenantKeys']);
    const orgIds = this.collectStrings(source, ['org_id', 'orgId', 'org_ids', 'orgIds']);
    const tenants = this.collectTenantOptions(source);
    const bootstrap = this.resolveAdminTenantBootstrap(source);
    const platformScoped = this.isPlatformScoped(source) || bootstrap.platformScoped || Boolean(backend?.platformScoped);

    const tenantOptions = this.uniqueOptions([
      ...(backend?.tenantOptions ?? []),
      ...bootstrap.tenantOptions,
      ...tenants,
      ...tenantKeys.map((tenantKey) => this.toTenantOption(tenantKey, tenantKey, tenantKey)),
      ...tenantIds.map((tenantId) => this.toTenantOption(tenantId, tenantId, tenantId)),
      ...orgIds.map((orgId) => this.toTenantOption(orgId, orgId, orgId))
    ].filter((option): option is TenantWorkspaceOption => Boolean(option)));

    return {
      tenantId: tenantIds[0] ?? null,
      tenantKey: tenantKeys[0] ?? null,
      tenantOptions,
      platformScoped
    };
  }

  private refreshBackendWorkspace(): void {
    const http = this.getBackendHttp();

    if (!http) {
      this.backendWorkspaceSubject.next(null);
      return;
    }

    if (this.workspaceFetchInFlight) {
      return;
    }
    this.workspaceFetchInFlight = true;

    // A failed fetch leaves null in the subject; flip it back to undefined so
    // guards treat the workspace as loading until this retry resolves instead
    // of denying on the stale failure.
    if (this.backendWorkspaceSubject.value === null) {
      this.backendWorkspaceSubject.next(undefined);
    }

    this.auth.getAccessTokenSilently().pipe(
      switchMap((token) => http.get<AdminTenantWorkspaceResponse>(this.workspaceEndpoint, {
        headers: {
          Authorization: `Bearer ${token}`
        }
      })),
      catchError((error: unknown) => {
        if (this.requiresInteractiveLogin(error)) {
          // The Auth0 session can no longer mint tokens silently; only a
          // fresh interactive login recovers, so start it rather than
          // dead-ending the user on the 403 page.
          this.auth.loginWithRedirect({
            appState: { target: this.currentRouterTarget() }
          });
        }
        return of(null);
      })
    ).subscribe((response) => {
      this.workspaceFetchInFlight = false;
      this.backendWorkspaceSubject.next(this.toBackendWorkspaceSnapshot(response));
    });
  }

  private requiresInteractiveLogin(error: unknown): boolean {
    const code = this.isRecord(error)
      ? this.extractSingleString(error['error'] ?? error['code'])
      : null;
    return code !== null && [
      'login_required',
      'consent_required',
      'interaction_required',
      'mfa_required',
      'missing_refresh_token',
      'invalid_grant'
    ].includes(code);
  }

  private currentRouterTarget(): string {
    // Hash-based routing: the hash fragment is the router URL.
    const hash = typeof window === 'undefined' ? '' : window.location.hash;
    return hash.startsWith('#') && hash.length > 1 ? hash.slice(1) : '/';
  }

  private getBackendHttp(): HttpClient | null {
    if (this.backendHttp) {
      return this.backendHttp;
    }

    const backend = this.injector.get(HttpBackend, null);

    if (!backend) {
      return null;
    }

    this.backendHttp = new HttpClient(backend);
    return this.backendHttp;
  }

  private toBackendWorkspaceSnapshot(response: AdminTenantWorkspaceResponse | null): BackendWorkspaceSnapshot | null {
    if (!response) {
      return null;
    }
    return {
      platformScoped: Boolean(response.platform_scoped ?? response.platformScoped),
      tenantOptions: this.collectTenantOptions({ tenantOptions: response.tenants })
    };
  }

  private resolveAdminTenantBootstrap(source: Record<string, unknown>): { platformScoped: boolean; tenantOptions: TenantWorkspaceOption[] } {
    const bootstrap = this.getAdminTenantBootstrapConfig();
    const platformAdminEmails = this.extractStrings(bootstrap.platformAdminEmails)
      .map((email) => email.trim().toLowerCase())
      .filter((email) => email.length > 0);
    const platformAdminSubjects = this.extractStrings(bootstrap.platformAdminSubjects)
      .map((subject) => subject.trim())
      .filter((subject) => subject.length > 0);
    const userEmails = this.collectStrings(source, ['email', 'https://platform/email']);
    const userSubjects = this.extractStrings(source['sub'] ?? source['user_id'] ?? source['userId'] ?? source['https://platform/sub'])
      .map((subject) => subject.trim())
      .filter((subject) => subject.length > 0);
    const emailVerified = this.resolveOptionalEmailVerified(source);
    const subjectScoped = userSubjects.some((subject) => platformAdminSubjects.includes(subject));
    const emailScoped = emailVerified && userEmails.some((email) => platformAdminEmails.includes(email));
    const platformScoped = subjectScoped || emailScoped;

    if (!platformScoped) {
      return { platformScoped: false, tenantOptions: [] };
    }

    return {
      platformScoped: true,
      tenantOptions: this.collectTenantOptions({ tenantOptions: bootstrap.tenantWorkspaces })
    };
  }

  private resolveOptionalEmailVerified(source: Record<string, unknown>): boolean {
    const value = [
      source['email_verified'],
      source['emailVerified'],
      source['https://platform/email_verified']
    ].find((candidate) => candidate !== undefined && candidate !== null && String(candidate).trim().length > 0);

    return value === undefined ? true : this.isTruthy(value);
  }

  private getAdminTenantBootstrapConfig(): AdminTenantBootstrapConfig {
    const configured = this.asRecord(environment['adminTenantBootstrap' as keyof typeof environment]);
    const runtime = typeof window === 'undefined'
      ? {}
      : this.asRecord((window as unknown as Record<string, unknown>)['__ADMIN_TENANT_BOOTSTRAP__']);

    return {
      ...configured,
      ...runtime
    };
  }

  private collectTenantOptions(source: Record<string, unknown>): TenantWorkspaceOption[] {
    const options: TenantWorkspaceOption[] = [];
    const tenantCollection = source['tenants'] ?? source['tenant_options'] ?? source['tenantOptions'] ?? source['workspaceTenants'];

    if (Array.isArray(tenantCollection)) {
      tenantCollection.forEach((entry, index) => {
        if (typeof entry === 'string') {
          const option = this.toTenantOption(entry, entry, entry);
          if (option) {
            options.push(option);
          }
          return;
        }

        if (this.isRecord(entry)) {
          const option = this.toTenantOptionFromRecord(entry, index);
          if (option) {
            options.push(option);
          }
        }
      });
    }

    return options;
  }

  private toTenantOptionFromRecord(value: Record<string, unknown>, fallbackIndex: number): TenantWorkspaceOption | null {
    const tenantId = this.firstString(value, ['tenant_id', 'tenantId', 'id', 'org_id', 'orgId']);
    const tenantKey = this.firstString(value, ['tenant_key', 'tenantKey', 'key', 'slug']);
    const label = this.firstString(value, ['name', 'label', 'display_name', 'displayName', 'title']) ?? tenantKey ?? tenantId;

    return this.toTenantOption(tenantKey ?? tenantId ?? `tenant-${fallbackIndex + 1}`, tenantId, tenantKey, label);
  }

  private toTenantOption(
    identifier: string | null | undefined,
    tenantId: string | null | undefined,
    tenantKey: string | null | undefined,
    label?: string | null
  ): TenantWorkspaceOption | null {
    const normalizedIdentifier = this.normalizeIdentifier(identifier);
    const normalizedTenantId = this.normalizeIdentifier(tenantId);
    const normalizedTenantKey = this.normalizeIdentifier(tenantKey);

    if (!normalizedIdentifier && !normalizedTenantId && !normalizedTenantKey) {
      return null;
    }

    const finalIdentifier = normalizedIdentifier ?? normalizedTenantKey ?? normalizedTenantId ?? '';

    return {
      identifier: finalIdentifier,
      tenantId: normalizedTenantId ?? finalIdentifier,
      tenantKey: normalizedTenantKey ?? finalIdentifier,
      label: label?.trim() || normalizedTenantKey || normalizedTenantId || finalIdentifier
    };
  }

  private uniqueOptions(options: TenantWorkspaceOption[]): TenantWorkspaceOption[] {
    const seen = new Set<string>();
    return options.filter((option) => {
      if (seen.has(option.identifier)) {
        return false;
      }
      seen.add(option.identifier);
      return true;
    });
  }

  private findTenantOption(options: TenantWorkspaceOption[], identifier: string): TenantWorkspaceOption | null {
    const normalized = this.normalizeIdentifier(identifier);

    if (!normalized) {
      return null;
    }

    return options.find((candidate) => {
      return candidate.identifier === normalized ||
        candidate.tenantId === normalized ||
        candidate.tenantKey === normalized ||
        candidate.label.toLowerCase() === normalized;
    }) ?? null;
  }

  private collectStrings(source: Record<string, unknown>, keys: string[]): string[] {
    const values = keys.flatMap((key) => this.extractStrings(source[key]));
    return Array.from(new Set(values.map((value) => this.normalizeIdentifier(value)).filter((value): value is string => Boolean(value))));
  }

  private extractStrings(value: unknown): string[] {
    if (typeof value === 'string') {
      return value ? [value] : [];
    }

    if (typeof value === 'number') {
      return [String(value)];
    }

    if (Array.isArray(value)) {
      return value.flatMap((entry) => this.extractStrings(entry));
    }

    if (this.isRecord(value)) {
      const nestedCandidate = this.firstString(value, ['id', 'tenant_id', 'tenantId', 'tenant_key', 'tenantKey', 'key', 'slug', 'org_id', 'orgId', 'name', 'label']);
      return nestedCandidate ? [nestedCandidate] : [];
    }

    return [];
  }

  private firstString(source: Record<string, unknown>, keys: string[]): string | null {
    for (const key of keys) {
      const value = source[key];
      const extracted = this.extractSingleString(value);
      if (extracted) {
        return extracted;
      }
    }

    return null;
  }

  private extractSingleString(value: unknown): string | null {
    if (typeof value === 'string') {
      const trimmed = value.trim();
      return trimmed.length > 0 ? trimmed : null;
    }

    if (typeof value === 'number' || typeof value === 'boolean') {
      return String(value);
    }

    return null;
  }

  private isPlatformScoped(source: Record<string, unknown>): boolean {
    const flags = [
      source['platform_scope'],
      source['platformScoped'],
      source['isPlatformScoped'],
      source['platform_scope_enabled']
    ];

    if (flags.some((flag) => this.isTruthy(flag))) {
      return true;
    }

    const roles = this.collectStrings(source, ['roles', 'role']);
    const permissions = this.collectStrings(source, ['permissions', 'permission']);
    const scopeMatches = ['platform_operator', 'platform:all_tenants', 'platform:all-tenants', 'tenant:select'].some((scope) => {
      return roles.includes(scope) || permissions.includes(scope);
    });

    return scopeMatches;
  }

  private isTruthy(value: unknown): boolean {
    if (typeof value === 'boolean') {
      return value;
    }

    if (typeof value === 'string') {
      const normalized = value.trim().toLowerCase();
      return ['true', '1', 'yes', 'y', 'platform', 'platform_operator', 'all_tenants'].includes(normalized);
    }

    return false;
  }

  private asRecord(value: unknown): Record<string, unknown> {
    return this.isRecord(value) ? value : {};
  }

  private isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === 'object' && value !== null && !Array.isArray(value);
  }

  private normalizeIdentifier(value: string | null | undefined): string {
    return value?.trim().toLowerCase() ?? '';
  }

  private readStoredSelection(): string | null {
    if (typeof sessionStorage === 'undefined') {
      return null;
    }

    return this.normalizeIdentifier(sessionStorage.getItem(TENANT_SELECTION_STORAGE_KEY));
  }

  private createLoadingState(): TenantWorkspaceState {
    return {
      authenticated: false,
      loading: true,
      platformScoped: false,
      currentTenant: null,
      availableTenants: [],
      canSwitchTenant: false,
      status: 'loading',
      reason: null
    };
  }

  private getApiBaseUrl(): string {
    return environment.baseApiEndpoint;
  }

  private getSubscriptionApiUrl(): string {
    return environment.subscriptionApiEndpoint;
  }

  private getSubscriptionExternalAdminApiUrl(): string {
    return environment.subscriptionExternalAdminApiEndpoint;
  }

  private getNotificationApiUrl(): string {
    return environment.notificationApiEndpoint;
  }

  private getAcquisitionApiUrl(): string {
    return environment.acquisitionApiEndpoint;
  }

  private getCadenceEngineUrl(): string {
    return environment.cadenceEngineEndpoint;
  }
}
