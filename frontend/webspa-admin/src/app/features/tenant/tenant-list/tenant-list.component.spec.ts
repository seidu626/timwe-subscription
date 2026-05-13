import { of } from 'rxjs';

import { TenantListComponent } from './tenant-list.component';

describe('TenantListComponent', () => {
  function createComponent() {
    const tenantService = {
      list: jasmine.createSpy().and.returnValue(of({
        tenants: [],
        total_count: 0,
        page: 1,
        page_size: 20
      })),
      update: jasmine.createSpy().and.returnValue(of({
        id: 'tenant-1',
        tenant_key: 'nrg',
        name: 'NRG Prime',
        status: 'ACTIVE',
        default_country: 'GH',
        metadata: {},
        created_at: '2026-05-10T00:00:00Z',
        updated_at: '2026-05-10T00:00:00Z',
        audit_log_id: 'audit-1'
      })),
      create: jasmine.createSpy().and.returnValue(of({
        id: 'tenant-2',
        tenant_key: 'newco',
        name: 'NewCo',
        status: 'ACTIVE',
        default_country: 'GH',
        metadata: {},
        created_at: '2026-05-13T00:00:00Z',
        updated_at: '2026-05-13T00:00:00Z',
        audit_log_id: 'audit-2'
      }))
    };
    const snackBar = {
      open: jasmine.createSpy()
    };

    const component = new TenantListComponent(tenantService as any, snackBar as any);
    return { component, tenantService, snackBar };
  }

  it('loads tenant catalog rows with current paging and filters', () => {
    const { component, tenantService } = createComponent();
    component.filters = { q: 'nrg', status: 'ACTIVE' };
    component.page = 2;
    component.pageSize = 10;

    component.loadTenants();

    expect(tenantService.list).toHaveBeenCalledWith({
      page: 2,
      page_size: 10,
      q: 'nrg',
      status: 'ACTIVE'
    });
  });

  it('sends normalized tenant updates with JSON metadata', () => {
    const { component, tenantService } = createComponent();
    component.editTenant({
      id: 'tenant-1',
      tenant_key: 'nrg',
      name: 'NRG',
      status: 'ACTIVE',
      default_country: 'GH',
      metadata: { kind: 'canonical-default' },
      created_at: '2026-05-10T00:00:00Z',
      updated_at: '2026-05-10T00:00:00Z'
    });
    component.form.name = ' NRG Prime ';
    component.form.default_country = 'gh';
    component.metadataText = '{"tier":"gold"}';

    component.saveTenant();

    expect(tenantService.update).toHaveBeenCalledWith('tenant-1', {
      name: 'NRG Prime',
      status: 'ACTIVE',
      default_country: 'GH',
      metadata: { tier: 'gold' }
    });
    expect(tenantService.create).not.toHaveBeenCalled();
  });

  it('sends normalized tenant creates with JSON metadata', () => {
    const { component, tenantService } = createComponent();
    component.form = {
      tenant_key: ' NewCo ',
      name: ' NewCo ',
      status: 'ACTIVE',
      default_country: 'gh'
    };
    component.metadataText = '{"owner":"ops"}';

    component.saveTenant();

    expect(tenantService.create).toHaveBeenCalledWith({
      tenant_key: 'newco',
      name: 'NewCo',
      status: 'ACTIVE',
      default_country: 'GH',
      metadata: { owner: 'ops' }
    });
    expect(tenantService.update).not.toHaveBeenCalled();
  });

  it('does not update when metadata is not a JSON object', () => {
    const { component, tenantService, snackBar } = createComponent();
    component.editingTenantId = 'tenant-1';
    component.form = { tenant_key: 'nrg', name: 'NRG', status: 'ACTIVE', default_country: 'GH' };
    component.metadataText = '[]';

    component.saveTenant();

    expect(tenantService.update).not.toHaveBeenCalled();
    expect(tenantService.create).not.toHaveBeenCalled();
    expect(snackBar.open).toHaveBeenCalledWith('Metadata must be a JSON object', 'Close', { duration: 4000 });
  });
});
