import { describe, it, expect, vi } from 'vitest';
import { DM_QUEUE_LIST_ENDPOINT, iconForKind, fetchDMQueueList } from './dmqueue.js';

describe('DM_QUEUE_LIST_ENDPOINT', () => {
  it('points at the dashboard queue list route', () => {
    expect(DM_QUEUE_LIST_ENDPOINT).toBe('/dashboard/queue/');
  });
});

describe('iconForKind', () => {
  it('maps every known kind to a non-default glyph', () => {
    const known = [
      'freeform_action',
      'reaction_declaration',
      'rest_request',
      'skill_check_narration',
      'consumable',
      'enemy_turn_ready',
      'narrative_teleport',
      'player_whisper',
      'undo_request',
      'retire_request',
    ];
    for (const kind of known) {
      expect(iconForKind(kind), `kind=${kind}`).not.toBe('??');
    }
  });

  it('falls through to "??" for unknown kinds', () => {
    expect(iconForKind('mystery')).toBe('??');
    expect(iconForKind('')).toBe('??');
  });
});

describe('fetchDMQueueList', () => {
  it('GETs the list endpoint with same-origin credentials and returns the JSON body', async () => {
    const body = [
      { id: 'abc', kind: 'player_whisper', kind_label: 'Player Whisper', player_name: 'Aria', summary: 'x', status: 'pending', resolve_path: '/dashboard/queue/abc' },
    ];
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => body,
    });
    const got = await fetchDMQueueList(fetchImpl);
    expect(fetchImpl).toHaveBeenCalledWith('/dashboard/queue/', { credentials: 'same-origin' });
    expect(got).toEqual(body);
  });

  it('throws with the response text on non-OK', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: false,
      status: 403,
      text: async () => 'forbidden: DM only',
    });
    await expect(fetchDMQueueList(fetchImpl)).rejects.toThrow(/forbidden: DM only/);
  });

  it('falls back to a generic error when the body is empty', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      text: async () => '',
    });
    await expect(fetchDMQueueList(fetchImpl)).rejects.toThrow(/Request failed: 500/);
  });
});
