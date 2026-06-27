/**
 * SlotEditor.svelte — reusable spell/pact slot editor.
 *
 * The dashboard's vitest harness runs under the `node` environment with no DOM,
 * so the component can't be mounted. The pure value-shaping logic lives in the
 * component's `<script module>` block and is imported + exercised directly
 * here (mirroring the characterStatus.js pattern). The markup contract
 * (data-testids, empty state, Cancel wiring) is asserted against the .svelte
 * source text, matching the existing component-test convention.
 */
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import {
  clampCurrent,
  spellRowsFromProps,
  buildSlotPayload,
  formatSpellSummary,
  formatPactSummary,
} from './SlotEditor.svelte';

const src = readFileSync(
  fileURLToPath(new URL('./SlotEditor.svelte', import.meta.url)),
  'utf8',
);

describe('spellRowsFromProps', () => {
  it('derives numerically-sorted rows from a string-keyed slot map', () => {
    const rows = spellRowsFromProps({
      '2': { current: 0, max: 2 },
      '1': { current: 2, max: 4 },
      '10': { current: 1, max: 1 },
    });
    expect(rows).toEqual([
      { level: 1, current: 2, max: 4 },
      { level: 2, current: 0, max: 2 },
      { level: 10, current: 1, max: 1 },
    ]);
  });

  it('tolerates null / {} by returning an empty array', () => {
    expect(spellRowsFromProps(null)).toEqual([]);
    expect(spellRowsFromProps(undefined)).toEqual([]);
    expect(spellRowsFromProps({})).toEqual([]);
  });

  it('coerces missing current/max to 0', () => {
    expect(spellRowsFromProps({ '1': {} })).toEqual([{ level: 1, current: 0, max: 0 }]);
  });
});

describe('clampCurrent', () => {
  it('clamps current into the inclusive 0..max range', () => {
    expect(clampCurrent(5, 4)).toBe(4);
    expect(clampCurrent(-3, 4)).toBe(0);
    expect(clampCurrent(2, 4)).toBe(2);
    expect(clampCurrent(7, 0)).toBe(0);
  });
});

describe('buildSlotPayload', () => {
  it('builds a string-keyed spell-only payload and clamps current<=max', () => {
    const payload = buildSlotPayload(
      [
        { level: 1, current: 9, max: 4 },
        { level: 2, current: 0, max: 2 },
      ],
      null,
      '',
    );
    expect(payload).toEqual({
      spell_slots: {
        '1': { current: 4, max: 4 },
        '2': { current: 0, max: 2 },
      },
    });
    expect(payload).not.toHaveProperty('pact_magic_slots');
    expect(payload).not.toHaveProperty('reason');
  });

  it('builds a pact-only payload when there were no spell rows', () => {
    const payload = buildSlotPayload([], { slot_level: 2, current: 5, max: 2 }, 'rested');
    expect(payload).toEqual({
      pact_magic_slots: { slot_level: 2, current: 2, max: 2 },
      reason: 'rested',
    });
    expect(payload).not.toHaveProperty('spell_slots');
  });

  it('includes reason only when non-empty and omits both stores when empty', () => {
    expect(buildSlotPayload([], null, '   ')).toEqual({});
    expect(buildSlotPayload([{ level: 1, current: 1, max: 1 }], null, '  fix  ')).toEqual({
      spell_slots: { '1': { current: 1, max: 1 } },
      reason: 'fix',
    });
  });
});

describe('formatSpellSummary / formatPactSummary', () => {
  it('renders the compact slot summary', () => {
    expect(
      formatSpellSummary({ '1': { current: 2, max: 4 }, '2': { current: 0, max: 2 } }),
    ).toBe('Slots: L1 2/4 · L2 0/2');
    expect(formatSpellSummary(null)).toBe('');
    expect(formatSpellSummary({})).toBe('');
  });

  it('renders the compact pact summary', () => {
    expect(formatPactSummary({ slot_level: 2, current: 0, max: 2 })).toBe('Pact: L2 0/2');
    expect(formatPactSummary(null)).toBe('');
  });
});

describe('SlotEditor markup contract', () => {
  it('renders a row per spell level with current + max inputs', () => {
    expect(src).toContain('data-testid="slot-row-{row.level}"');
    expect(src).toContain('data-testid="slot-current-{row.level}"');
    expect(src).toContain('data-testid="slot-max-{row.level}"');
    expect(src).toContain('Level {row.level}');
    expect(src).toContain('{#each spellRows as row');
  });

  it('renders a pact row when pactSlots is present', () => {
    expect(src).toContain('data-testid="pact-row"');
    expect(src).toContain('Pact (Level {pactRow.slot_level})');
    expect(src).toContain('data-testid="pact-current"');
    expect(src).toContain('data-testid="pact-max"');
  });

  it('shows the empty state when neither store is present', () => {
    expect(src).toContain('data-testid="slot-editor-empty"');
    expect(src).toContain('No spell slots to edit.');
  });

  it('exposes a reason input, Save and Cancel buttons, and an error slot', () => {
    expect(src).toContain('data-testid="slot-reason"');
    expect(src).toContain('data-testid="slot-save"');
    expect(src).toContain('data-testid="slot-cancel"');
    expect(src).toContain('data-testid="slot-error"');
  });

  it('disables Save while busy and wires Save/Cancel to the callbacks', () => {
    const save = src.match(/data-testid="slot-save"[\s\S]*?<\/button>/);
    expect(save).not.toBeNull();
    expect(save[0]).toContain('onclick={handleSave}');
    expect(save[0]).toContain('disabled={busy');
    const cancel = src.match(/data-testid="slot-cancel"[\s\S]*?<\/button>/);
    expect(cancel).not.toBeNull();
    expect(cancel[0]).toContain('onCancel?.()');
    // handleSave delegates to onSave with the built payload.
    expect(src).toContain('onSave?.(buildSlotPayload(spellRows, pactRow, reason))');
  });
});
