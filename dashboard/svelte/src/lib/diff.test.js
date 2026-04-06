import { describe, it, expect } from 'vitest';
import { diffStates } from './diff.js';

describe('diffStates', () => {
  it('returns empty array when before and after are identical', () => {
    expect(diffStates({ hp: 10 }, { hp: 10 })).toEqual([]);
  });

  it('returns changed fields only', () => {
    const out = diffStates({ hp: 10, ac: 15 }, { hp: 5, ac: 15 });
    expect(out).toEqual([{ field: 'hp', before: 10, after: 5 }]);
  });

  it('detects added fields', () => {
    const out = diffStates({ hp: 10 }, { hp: 10, conditions: ['poisoned'] });
    expect(out).toEqual([{ field: 'conditions', before: undefined, after: ['poisoned'] }]);
  });

  it('detects removed fields', () => {
    const out = diffStates({ hp: 10, temp: 5 }, { hp: 10 });
    expect(out).toEqual([{ field: 'temp', before: 5, after: undefined }]);
  });

  it('handles nested objects via deep equality', () => {
    const before = { pos: { col: 'A', row: 1 } };
    const after = { pos: { col: 'B', row: 1 } };
    const out = diffStates(before, after);
    expect(out).toHaveLength(1);
    expect(out[0].field).toBe('pos');
  });

  it('treats nested equal objects as unchanged', () => {
    const before = { pos: { col: 'A', row: 1 } };
    const after = { pos: { col: 'A', row: 1 } };
    expect(diffStates(before, after)).toEqual([]);
  });

  it('returns empty when both are null/undefined', () => {
    expect(diffStates(null, null)).toEqual([]);
    expect(diffStates(undefined, undefined)).toEqual([]);
  });

  it('handles null before and object after', () => {
    const out = diffStates(null, { hp: 10 });
    expect(out).toEqual([{ field: 'hp', before: undefined, after: 10 }]);
  });

  it('handles object before and null after', () => {
    const out = diffStates({ hp: 10 }, null);
    expect(out).toEqual([{ field: 'hp', before: 10, after: undefined }]);
  });

  it('treats equal arrays as unchanged', () => {
    expect(diffStates({ tags: ['a', 'b'] }, { tags: ['a', 'b'] })).toEqual([]);
  });

  it('detects arrays of different length', () => {
    const out = diffStates({ tags: ['a'] }, { tags: ['a', 'b'] });
    expect(out).toHaveLength(1);
    expect(out[0].field).toBe('tags');
  });

  it('detects arrays with different elements at same length', () => {
    const out = diffStates({ tags: ['a', 'b'] }, { tags: ['a', 'c'] });
    expect(out).toHaveLength(1);
  });

  it('detects array vs object mismatch', () => {
    const out = diffStates({ v: [1, 2] }, { v: { 0: 1, 1: 2 } });
    expect(out).toHaveLength(1);
  });

  it('sorts fields for stable output', () => {
    const out = diffStates({ b: 1, a: 2 }, { b: 3, a: 4 });
    expect(out.map(d => d.field)).toEqual(['a', 'b']);
  });
});
