import { describe, it, expect } from 'vitest';
import { mergeSnapshot } from './optimisticMerge.js';

describe('mergeSnapshot', () => {
  it('returns the snapshot when there are no dirty fields', () => {
    const current = { hp_current: 10, name: 'Grog' };
    const snap = { hp_current: 3, name: 'Grog', ac: 15 };
    const merged = mergeSnapshot(current, snap, new Set());
    expect(merged).toEqual({ hp_current: 3, name: 'Grog', ac: 15, _pendingFromSnapshot: {} });
  });

  it('preserves a dirty field and records the pending snapshot value', () => {
    const current = { hp_current: 10, name: 'Grog' };
    const snap = { hp_current: 3, name: 'Grog' };
    const merged = mergeSnapshot(current, snap, new Set(['hp_current']));
    expect(merged.hp_current).toBe(10); // DM draft preserved
    expect(merged.name).toBe('Grog');
    expect(merged._pendingFromSnapshot).toEqual({ hp_current: 3 });
  });

  it('does not record pending when the dirty field matches the snapshot', () => {
    const current = { hp_current: 3 };
    const snap = { hp_current: 3 };
    const merged = mergeSnapshot(current, snap, new Set(['hp_current']));
    expect(merged.hp_current).toBe(3);
    expect(merged._pendingFromSnapshot).toEqual({});
  });

  it('supports Array dirtyFields input as well as Set', () => {
    const current = { hp_current: 10 };
    const snap = { hp_current: 3 };
    const merged = mergeSnapshot(current, snap, ['hp_current']);
    expect(merged.hp_current).toBe(10);
    expect(merged._pendingFromSnapshot).toEqual({ hp_current: 3 });
  });

  it('handles null/undefined currentState by treating it as empty', () => {
    const snap = { hp_current: 3, name: 'X' };
    const merged = mergeSnapshot(null, snap, new Set());
    expect(merged.hp_current).toBe(3);
    expect(merged.name).toBe('X');
  });

  it('handles null/undefined dirtyFields by treating it as empty', () => {
    const merged = mergeSnapshot({ a: 1 }, { a: 2, b: 3 }, null);
    expect(merged).toEqual({ a: 2, b: 3, _pendingFromSnapshot: {} });
  });

  it('drops existing _pendingFromSnapshot entries whose dirty flag has cleared', () => {
    const current = { hp: 10, _pendingFromSnapshot: { hp: 3, ac: 12 } };
    const snap = { hp: 5, ac: 14 };
    // hp is no longer dirty, so it should take the snapshot value and clear the pending
    const merged = mergeSnapshot(current, snap, new Set([]));
    expect(merged.hp).toBe(5);
    expect(merged.ac).toBe(14);
    expect(merged._pendingFromSnapshot).toEqual({});
  });

  it('returns a new object (does not mutate the input currentState)', () => {
    const current = { hp: 10 };
    const snap = { hp: 3 };
    const merged = mergeSnapshot(current, snap, new Set(['hp']));
    expect(current).toEqual({ hp: 10 }); // unchanged
    expect(merged).not.toBe(current);
  });

  it('merges fields that are only present on the snapshot', () => {
    const current = { hp: 10 };
    const snap = { hp: 10, ac: 15, name: 'Grog' };
    const merged = mergeSnapshot(current, snap, new Set());
    expect(merged.ac).toBe(15);
    expect(merged.name).toBe('Grog');
  });

  it('treats deeply-equal nested objects as unchanged (no pending)', () => {
    const current = { stats: { hp: 10, ac: 15 } };
    const snap = { stats: { hp: 10, ac: 15 } };
    const merged = mergeSnapshot(current, snap, new Set(['stats']));
    expect(merged._pendingFromSnapshot).toEqual({});
  });

  it('records pending when nested object differs', () => {
    const current = { stats: { hp: 10, ac: 15 } };
    const snap = { stats: { hp: 3, ac: 15 } };
    const merged = mergeSnapshot(current, snap, new Set(['stats']));
    expect(merged.stats).toEqual({ hp: 10, ac: 15 });
    expect(merged._pendingFromSnapshot).toEqual({ stats: { hp: 3, ac: 15 } });
  });

  it('records pending when nested object has a different number of keys', () => {
    const current = { stats: { hp: 10 } };
    const snap = { stats: { hp: 10, ac: 15 } };
    const merged = mergeSnapshot(current, snap, new Set(['stats']));
    expect(merged._pendingFromSnapshot.stats).toEqual({ hp: 10, ac: 15 });
  });

  it('records pending when draft is null but snapshot is object', () => {
    const current = { stats: null };
    const snap = { stats: { hp: 5 } };
    const merged = mergeSnapshot(current, snap, new Set(['stats']));
    expect(merged.stats).toBe(null);
    expect(merged._pendingFromSnapshot.stats).toEqual({ hp: 5 });
  });

  it('records pending when draft is object but snapshot is number', () => {
    const current = { x: { a: 1 } };
    const snap = { x: 42 };
    const merged = mergeSnapshot(current, snap, new Set(['x']));
    expect(merged.x).toEqual({ a: 1 });
    expect(merged._pendingFromSnapshot.x).toBe(42);
  });

  it('_pendingFromSnapshot key on snapshot is ignored', () => {
    const current = { hp: 10 };
    const snap = { hp: 10, _pendingFromSnapshot: { hp: 'garbage' } };
    const merged = mergeSnapshot(current, snap, new Set());
    expect(merged._pendingFromSnapshot).toEqual({});
  });

  it('keeps current-only fields that are not on the snapshot', () => {
    const current = { hp: 10, localDraft: 'unsaved' };
    const snap = { hp: 10 };
    const merged = mergeSnapshot(current, snap, new Set(['localDraft']));
    expect(merged.localDraft).toBe('unsaved');
  });
});
