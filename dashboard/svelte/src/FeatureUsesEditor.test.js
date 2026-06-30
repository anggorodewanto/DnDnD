/**
 * FeatureUsesEditor.svelte — reusable limited-use feature editor.
 *
 * The dashboard's vitest harness runs under the `node` environment with no DOM,
 * so the component can't be mounted. The pure value-shaping logic lives in the
 * component's `<script module>` block and is imported + exercised directly here
 * (mirroring SlotEditor.test.js). The markup contract (data-testids, empty
 * state, Cancel wiring) is asserted against the .svelte source text.
 */
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import {
  clampCurrent,
  featureRowsFromProps,
  buildFeatureUsesChanges,
  formatFeatureUsesSummary,
} from './FeatureUsesEditor.svelte';

const src = readFileSync(
  fileURLToPath(new URL('./FeatureUsesEditor.svelte', import.meta.url)),
  'utf8',
);

describe('clampCurrent', () => {
  it('clamps current into the inclusive 0..max range', () => {
    expect(clampCurrent(5, 3)).toBe(3);
    expect(clampCurrent(-3, 3)).toBe(0);
    expect(clampCurrent(2, 3)).toBe(2);
    expect(clampCurrent(7, 0)).toBe(0);
  });

  it('treats a negative max as unlimited (no upper clamp) but still floors at 0', () => {
    expect(clampCurrent(999, -1)).toBe(999);
    expect(clampCurrent(-5, -1)).toBe(0);
  });
});

describe('featureRowsFromProps', () => {
  it('derives alphabetically-sorted rows from a name-keyed map', () => {
    const rows = featureRowsFromProps({
      rage: { current: 1, max: 3, recharge: 'long' },
      bardic: { current: 2, max: 4, recharge: 'short' },
    });
    expect(rows).toEqual([
      { name: 'bardic', current: 2, max: 4, recharge: 'short' },
      { name: 'rage', current: 1, max: 3, recharge: 'long' },
    ]);
  });

  it('tolerates null / undefined / {} by returning an empty array', () => {
    expect(featureRowsFromProps(null)).toEqual([]);
    expect(featureRowsFromProps(undefined)).toEqual([]);
    expect(featureRowsFromProps({})).toEqual([]);
  });

  it('coerces missing current/max to 0 and missing recharge to empty', () => {
    expect(featureRowsFromProps({ rage: {} })).toEqual([
      { name: 'rage', current: 0, max: 0, recharge: '' },
    ]);
  });
});

describe('buildFeatureUsesChanges', () => {
  const seed = [
    { name: 'rage', current: 3 },
    { name: 'bardic', current: 2 },
  ];

  it('emits only the rows whose current changed from the seed', () => {
    const result = buildFeatureUsesChanges(
      [
        { name: 'rage', current: 1, max: 3 },
        { name: 'bardic', current: 2, max: 4 },
      ],
      seed,
      '',
    );
    expect(result).toEqual({ changes: [{ feature: 'rage', current: 1 }] });
    expect(result).not.toHaveProperty('reason');
  });

  it('clamps each emitted current into 0..max before diffing', () => {
    // Seed rage at 0 so the clamped 3 is a genuine change and we can observe it.
    const result = buildFeatureUsesChanges(
      [{ name: 'rage', current: 9, max: 3 }],
      [{ name: 'rage', current: 0 }],
      '',
    );
    expect(result.changes).toEqual([{ feature: 'rage', current: 3 }]);
  });

  it('returns an empty changes array when nothing changed', () => {
    const result = buildFeatureUsesChanges(
      [
        { name: 'rage', current: 3, max: 3 },
        { name: 'bardic', current: 2, max: 4 },
      ],
      seed,
      '   ',
    );
    expect(result).toEqual({ changes: [] });
  });

  it('includes a trimmed reason only when non-empty', () => {
    expect(
      buildFeatureUsesChanges([{ name: 'rage', current: 0, max: 3 }], seed, '  spent  '),
    ).toEqual({ changes: [{ feature: 'rage', current: 0 }], reason: 'spent' });
  });

  it('treats an unknown (unseeded) row as a change', () => {
    const result = buildFeatureUsesChanges(
      [{ name: 'newfeat', current: 1, max: 2 }],
      seed,
      '',
    );
    expect(result.changes).toEqual([{ feature: 'newfeat', current: 1 }]);
  });
});

describe('formatFeatureUsesSummary', () => {
  it('renders the compact feature-uses summary', () => {
    expect(
      formatFeatureUsesSummary({
        rage: { current: 1, max: 3 },
        bardic: { current: 2, max: 4 },
      }),
    ).toBe('Features: bardic 2/4 · rage 1/3');
  });

  it('renders an unlimited pool as ∞', () => {
    expect(formatFeatureUsesSummary({ rage: { current: 5, max: -1 } })).toBe(
      'Features: rage 5/∞',
    );
  });

  it('returns an empty string when there are no features', () => {
    expect(formatFeatureUsesSummary(null)).toBe('');
    expect(formatFeatureUsesSummary({})).toBe('');
  });
});

describe('FeatureUsesEditor markup contract', () => {
  it('renders a row per feature with a clamped current input', () => {
    expect(src).toContain('data-testid="feature-uses-row-{row.name}"');
    expect(src).toContain('data-testid="feature-uses-current-{row.name}"');
    expect(src).toContain('{#each rows as row');
    expect(src).toContain('onchange={() => clampRow(row)}');
  });

  it('shows the empty state when there are no features', () => {
    expect(src).toContain('data-testid="feature-uses-editor-empty"');
    expect(src).toContain('No feature uses to edit.');
  });

  it('exposes a reason input, Save and Cancel buttons, and an error slot', () => {
    expect(src).toContain('data-testid="feature-uses-reason"');
    expect(src).toContain('data-testid="feature-uses-save"');
    expect(src).toContain('data-testid="feature-uses-cancel"');
    expect(src).toContain('data-testid="feature-uses-error"');
  });

  it('disables Save while busy and wires Save/Cancel to the callbacks', () => {
    const save = src.match(/data-testid="feature-uses-save"[\s\S]*?<\/button>/);
    expect(save).not.toBeNull();
    expect(save[0]).toContain('onclick={handleSave}');
    expect(save[0]).toContain('disabled={busy');
    const cancel = src.match(/data-testid="feature-uses-cancel"[\s\S]*?<\/button>/);
    expect(cancel).not.toBeNull();
    expect(cancel[0]).toContain('onCancel?.()');
    // handleSave delegates to onSave with the built changes payload.
    expect(src).toContain('onSave?.(buildFeatureUsesChanges(rows, seedRows, reason))');
  });

  it('re-seeds editable state from props via a non-reactive lastSeed guard', () => {
    expect(src).toContain('if (featureUses === lastSeed) return;');
    expect(src).toContain('lastSeed = featureUses;');
    expect(src).toContain("reason = '';");
  });
});
