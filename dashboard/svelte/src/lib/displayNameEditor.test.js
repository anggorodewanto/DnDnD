import { describe, it, expect, vi } from 'vitest';
import { normalizeDisplayName, commitDisplayName } from './displayNameEditor.js';

describe('normalizeDisplayName', () => {
  it('trims surrounding whitespace', () => {
    expect(normalizeDisplayName('  Forest Chase  ')).toBe('Forest Chase');
  });

  it('returns empty string for null or undefined', () => {
    expect(normalizeDisplayName(null)).toBe('');
    expect(normalizeDisplayName(undefined)).toBe('');
  });

  it('returns empty string when input is only whitespace (clears override)', () => {
    expect(normalizeDisplayName('   ')).toBe('');
  });

  it('leaves non-whitespace content unchanged', () => {
    expect(normalizeDisplayName('Forest Chase')).toBe('Forest Chase');
  });
});

describe('commitDisplayName', () => {
  it('calls onCommit with the normalized next value when it differs from current', async () => {
    const onCommit = vi.fn().mockResolvedValue({ display_name: 'Forest Chase' });
    const result = await commitDisplayName({ current: '', next: '  Forest Chase  ', onCommit });
    expect(onCommit).toHaveBeenCalledWith('Forest Chase');
    expect(result).toEqual({ status: 'saved', value: 'Forest Chase' });
  });

  it('skips onCommit entirely when the normalized value matches current', async () => {
    const onCommit = vi.fn();
    const result = await commitDisplayName({ current: 'Same', next: '  Same  ', onCommit });
    expect(onCommit).not.toHaveBeenCalled();
    expect(result).toEqual({ status: 'unchanged', value: 'Same' });
  });

  it('passes empty string to onCommit to clear the override', async () => {
    const onCommit = vi.fn().mockResolvedValue({ display_name: null });
    const result = await commitDisplayName({ current: 'Old', next: '', onCommit });
    expect(onCommit).toHaveBeenCalledWith('');
    expect(result).toEqual({ status: 'saved', value: '' });
  });

  it('treats null current the same as empty string', async () => {
    const onCommit = vi.fn();
    const result = await commitDisplayName({ current: null, next: '', onCommit });
    expect(onCommit).not.toHaveBeenCalled();
    expect(result).toEqual({ status: 'unchanged', value: '' });
  });

  it('returns an error result when onCommit throws', async () => {
    const onCommit = vi.fn().mockRejectedValue(new Error('network boom'));
    const result = await commitDisplayName({ current: '', next: 'X', onCommit });
    expect(result.status).toBe('error');
    expect(result.error).toBe('network boom');
    expect(result.value).toBe('X');
  });

  it('returns unchanged with empty onCommit when no callback is provided and values match', async () => {
    const result = await commitDisplayName({ current: 'X', next: 'X' });
    expect(result).toEqual({ status: 'unchanged', value: 'X' });
  });

  it('returns saved without invoking anything when no callback is provided and values differ', async () => {
    const result = await commitDisplayName({ current: '', next: 'X' });
    expect(result).toEqual({ status: 'saved', value: 'X' });
  });
});
