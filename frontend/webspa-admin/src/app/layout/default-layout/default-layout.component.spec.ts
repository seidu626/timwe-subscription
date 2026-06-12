import { filterNavItemsForWorkspace } from './default-layout.component';

describe('filterNavItemsForWorkspace', () => {
  const items = [
    { name: 'Dashboard', url: '/dashboard' },
    { name: 'Tenants', url: '/tenants', workspaceRequired: true },
    { name: 'Platform', url: '/platform', platformOnly: true }
  ];

  it('shows tenant management for ready tenant workspaces', () => {
    const filtered = filterNavItemsForWorkspace(items, {
      platformScoped: false,
      status: 'ready'
    } as any);

    expect(filtered.map((item) => item.name)).toEqual(['Dashboard', 'Tenants']);
  });

  it('hides tenant management until a workspace is ready', () => {
    const filtered = filterNavItemsForWorkspace(items, {
      platformScoped: false,
      status: 'selection-required'
    } as any);

    expect(filtered.map((item) => item.name)).toEqual(['Dashboard']);
  });

  it('keeps platform-only entries for platform-scoped workspaces', () => {
    const filtered = filterNavItemsForWorkspace(items, {
      platformScoped: true,
      status: 'ready'
    } as any);

    expect(filtered.map((item) => item.name)).toEqual(['Dashboard', 'Tenants', 'Platform']);
  });
});
