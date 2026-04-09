import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  MESSAGE_PLAYER_ENDPOINT,
  validateMessagePlayerInput,
  sendPlayerMessage,
} from './messageplayer.js';

describe('MESSAGE_PLAYER_ENDPOINT', () => {
  it('points at the Phase 101 handler route', () => {
    expect(MESSAGE_PLAYER_ENDPOINT).toBe('/api/message-player/');
  });
});

describe('validateMessagePlayerInput', () => {
  const valid = {
    campaignId: '00000000-0000-0000-0000-000000000001',
    playerCharacterId: '00000000-0000-0000-0000-000000000002',
    authorUserId: 'discord-123',
    body: 'Hello adventurer.',
  };

  it('accepts a fully populated payload', () => {
    expect(validateMessagePlayerInput(valid)).toEqual({ ok: true });
  });

  it('rejects an empty body', () => {
    const res = validateMessagePlayerInput({ ...valid, body: '   ' });
    expect(res.ok).toBe(false);
    expect(res.error).toMatch(/body/i);
  });

  it('rejects a missing player character', () => {
    const res = validateMessagePlayerInput({ ...valid, playerCharacterId: '' });
    expect(res.ok).toBe(false);
    expect(res.error).toMatch(/player/i);
  });

  it('rejects a missing campaign id', () => {
    const res = validateMessagePlayerInput({ ...valid, campaignId: '' });
    expect(res.ok).toBe(false);
    expect(res.error).toMatch(/campaign/i);
  });

  it('rejects a missing author user id', () => {
    const res = validateMessagePlayerInput({ ...valid, authorUserId: '' });
    expect(res.ok).toBe(false);
    expect(res.error).toMatch(/author/i);
  });
});

describe('sendPlayerMessage', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('POSTs JSON to the message-player endpoint and returns parsed body', async () => {
    const created = { id: 'msg-1', body: 'Hi' };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(created),
    });

    const result = await sendPlayerMessage({
      campaignId: 'c1',
      playerCharacterId: 'p1',
      authorUserId: 'dm',
      body: 'Hi',
    });

    expect(result).toEqual(created);
    expect(fetch).toHaveBeenCalledTimes(1);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/message-player/');
    expect(options.method).toBe('POST');
    expect(options.headers['Content-Type']).toBe('application/json');
    const payload = JSON.parse(options.body);
    expect(payload).toEqual({
      campaign_id: 'c1',
      player_character_id: 'p1',
      author_user_id: 'dm',
      body: 'Hi',
    });
  });

  it('throws when the server responds non-ok', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: () => Promise.resolve('invalid body'),
    });

    await expect(
      sendPlayerMessage({
        campaignId: 'c1',
        playerCharacterId: 'p1',
        authorUserId: 'dm',
        body: '',
      }),
    ).rejects.toThrow(/invalid body/);
  });

  it('throws the default message when the server returns an empty body', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      text: () => Promise.resolve(''),
    });

    await expect(
      sendPlayerMessage({
        campaignId: 'c1',
        playerCharacterId: 'p1',
        authorUserId: 'dm',
        body: 'x',
      }),
    ).rejects.toThrow(/500/);
  });
});
