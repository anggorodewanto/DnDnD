import { describe, expect, it } from 'vitest';
import {
  dashboardNavItems,
  dashboardViewTitle,
  isDashboardNavItemActive,
} from './dashboardNavigation.js';

describe('dashboardNavItems', () => {
  it('starts with a dashboard destination that returns to the main page', () => {
    expect(dashboardNavItems[0]).toMatchObject({
      id: 'dashboard',
      label: 'Dashboard',
      view: 'list',
      hash: '#maps',
    });
  });

  it('includes a campaigns destination near the top of the sidebar', () => {
    expect(dashboardNavItems[1]).toMatchObject({
      id: 'campaigns',
      label: 'Campaigns',
      view: 'campaigns',
      hash: '#campaigns',
    });
  });

  it('keeps editor sub-pages active under their parent sidebar destination', () => {
    const dashboard = dashboardNavItems.find((item) => item.id === 'dashboard');
    const encounters = dashboardNavItems.find((item) => item.id === 'encounters');
    const shops = dashboardNavItems.find((item) => item.id === 'shops');

    expect(isDashboardNavItemActive(dashboard, 'editor')).toBe(true);
    expect(isDashboardNavItemActive(encounters, 'encounter-editor')).toBe(true);
    expect(isDashboardNavItemActive(shops, 'shop-editor')).toBe(true);
  });
});

describe('dashboardViewTitle', () => {
  it('returns stable page titles for sidebar-routed views', () => {
    expect(dashboardViewTitle('list')).toBe('Dashboard');
    expect(dashboardViewTitle('campaigns')).toBe('Campaigns');
    expect(dashboardViewTitle('approvals')).toBe('Character Approvals');
    expect(dashboardViewTitle('stat-block-library')).toBe('Stat Block Library');
  });

  it('falls back to the dashboard title for unknown views', () => {
    expect(dashboardViewTitle('unknown')).toBe('Dashboard');
  });
});
