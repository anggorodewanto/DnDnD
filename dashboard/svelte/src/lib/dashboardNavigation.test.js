import { describe, expect, it } from 'vitest';
import {
  dashboardNavItems,
  dashboardViewTitle,
  isDashboardNavItemActive,
} from './dashboardNavigation.js';

describe('dashboardNavItems', () => {
  it('starts with the home destination that lands on Campaign Home', () => {
    expect(dashboardNavItems[0]).toMatchObject({
      id: 'home',
      label: 'Home',
      view: 'home',
      hash: '#home',
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
    const maps = dashboardNavItems.find((item) => item.id === 'dashboard');
    const encounters = dashboardNavItems.find((item) => item.id === 'encounters');
    const shops = dashboardNavItems.find((item) => item.id === 'shops');

    expect(isDashboardNavItemActive(maps, 'editor')).toBe(true);
    expect(isDashboardNavItemActive(encounters, 'encounter-editor')).toBe(true);
    expect(isDashboardNavItemActive(shops, 'shop-editor')).toBe(true);
  });

  it('exposes the formerly Go-rendered pages as Svelte panel entries', () => {
    const ids = dashboardNavItems.map((item) => item.id);
    expect(ids).toEqual(expect.arrayContaining(['home', 'errors', 'exploration', 'characters-new']));
  });
});

describe('dashboardViewTitle', () => {
  it('returns stable page titles for sidebar-routed views', () => {
    expect(dashboardViewTitle('home')).toBe('Campaign Home');
    expect(dashboardViewTitle('list')).toBe('Maps');
    expect(dashboardViewTitle('campaigns')).toBe('Campaigns');
    expect(dashboardViewTitle('approvals')).toBe('Character Approvals');
    expect(dashboardViewTitle('stat-block-library')).toBe('Stat Block Library');
    expect(dashboardViewTitle('errors')).toBe('Errors');
    expect(dashboardViewTitle('exploration')).toBe('Exploration');
    expect(dashboardViewTitle('characters-new')).toBe('Create Character');
  });

  it('falls back to the dashboard title for unknown views', () => {
    expect(dashboardViewTitle('unknown')).toBe('Dashboard');
  });
});
