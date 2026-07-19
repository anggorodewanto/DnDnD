import { describe, it, expect, vi } from 'vitest';
import {
  DM_QUEUE_LIST_ENDPOINT,
  iconForKind,
  fetchDMQueueList,
  buildItemEndpoint,
  buildResolveEndpoint,
  buildReplyEndpoint,
  buildNarrateEndpoint,
  fetchDMQueueItem,
  resolveItem,
  replyItem,
  narrateItem,
} from './dmqueue.js';

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
      'initiative_staged',
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

describe('endpoint builders', () => {
  it('builds the per-item GET URL', () => {
    expect(buildItemEndpoint('abc')).toBe('/dashboard/queue/abc');
  });
  it('builds the resolve URL', () => {
    expect(buildResolveEndpoint('abc')).toBe('/dashboard/queue/abc/resolve');
  });
  it('builds the reply URL', () => {
    expect(buildReplyEndpoint('abc')).toBe('/dashboard/queue/abc/reply');
  });
  it('builds the narrate URL', () => {
    expect(buildNarrateEndpoint('abc')).toBe('/dashboard/queue/abc/narrate');
  });
  it('URI-encodes the item id', () => {
    expect(buildResolveEndpoint('a/b')).toBe('/dashboard/queue/a%2Fb/resolve');
  });
});

describe('fetchDMQueueItem', () => {
  it('GETs the per-item endpoint with same-origin credentials and returns JSON', async () => {
    const body = {
      id: 'abc',
      kind: 'player_whisper',
      kind_label: 'Player Whisper',
      player_name: 'Aria',
      summary: 'x',
      status: 'pending',
      outcome: '',
      is_whisper: true,
      is_skill_check_narration: false,
    };
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => body,
    });
    const got = await fetchDMQueueItem('abc', fetchImpl);
    expect(fetchImpl).toHaveBeenCalledWith('/dashboard/queue/abc', { credentials: 'same-origin' });
    expect(got).toEqual(body);
  });

  it('throws with the response text on non-OK', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
      text: async () => 'not found',
    });
    await expect(fetchDMQueueItem('missing', fetchImpl)).rejects.toThrow(/not found/);
  });

  it('falls back to a generic error when body is empty', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      text: async () => '',
    });
    await expect(fetchDMQueueItem('x', fetchImpl)).rejects.toThrow(/Request failed: 500/);
  });
});

describe('resolveItem', () => {
  it('POSTs JSON outcome to the resolve endpoint', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) });
    await resolveItem('abc', 'table flipped', fetchImpl);
    expect(fetchImpl).toHaveBeenCalledTimes(1);
    const [url, opts] = fetchImpl.mock.calls[0];
    expect(url).toBe('/dashboard/queue/abc/resolve');
    expect(opts.method).toBe('POST');
    expect(opts.headers['Content-Type']).toBe('application/json');
    expect(opts.credentials).toBe('same-origin');
    expect(JSON.parse(opts.body)).toEqual({ outcome: 'table flipped' });
  });

  it('throws with the response text on non-OK', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      text: async () => 'boom',
    });
    await expect(resolveItem('abc', 'x', fetchImpl)).rejects.toThrow(/boom/);
  });
});

describe('replyItem', () => {
  it('POSTs JSON reply to the reply endpoint', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) });
    await replyItem('w1', 'You succeed.', fetchImpl);
    const [url, opts] = fetchImpl.mock.calls[0];
    expect(url).toBe('/dashboard/queue/w1/reply');
    expect(opts.method).toBe('POST');
    expect(opts.headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(opts.body)).toEqual({ reply: 'You succeed.' });
  });

  it('throws with the response text on non-OK', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: async () => 'not a whisper item',
    });
    await expect(replyItem('w1', 'x', fetchImpl)).rejects.toThrow(/not a whisper item/);
  });
});

describe('narrateItem', () => {
  it('POSTs JSON narration to the narrate endpoint', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) });
    await narrateItem('sc1', 'You spot the trap.', fetchImpl);
    const [url, opts] = fetchImpl.mock.calls[0];
    expect(url).toBe('/dashboard/queue/sc1/narrate');
    expect(opts.method).toBe('POST');
    expect(opts.headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(opts.body)).toEqual({ narration: 'You spot the trap.' });
  });

  it('throws with the response text on non-OK', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      text: async () => 'boom',
    });
    await expect(narrateItem('sc1', 'x', fetchImpl)).rejects.toThrow(/boom/);
  });
});
