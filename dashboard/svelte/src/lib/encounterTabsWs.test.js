import { describe, it, expect, vi, beforeEach } from 'vitest';
import { createEncounterTabsWs } from './encounterTabsWs.js';

/**
 * FakeWebSocket mirrors the one in wsClient.test.js so we can drive the
 * underlying connection lifecycle synchronously.
 */
class FakeWebSocket {
  static instances = [];
  constructor(url) {
    this.url = url;
    this.closed = false;
    this.onopen = null;
    this.onmessage = null;
    this.onclose = null;
    FakeWebSocket.instances.push(this);
  }
  close() {
    this.closed = true;
    if (this.onclose) this.onclose({ code: 1000 });
  }
  simulateOpen() { if (this.onopen) this.onopen(); }
  simulateMessage(data) {
    if (this.onmessage) this.onmessage({ data });
  }
}

beforeEach(() => { FakeWebSocket.instances = []; });

describe('createEncounterTabsWs', () => {
  it('opens one WebSocket per encounter ID with encounter_id query parameter', () => {
    const mgr = createEncounterTabsWs({
      url: 'ws://localhost/dashboard/ws',
      WebSocketCtor: FakeWebSocket,
      setTimeout: () => 0,
      clearTimeout: () => {},
    });

    mgr.setEncounters(['enc-a', 'enc-b']);

    expect(FakeWebSocket.instances).toHaveLength(2);
    expect(FakeWebSocket.instances[0].url).toContain('encounter_id=enc-a');
    expect(FakeWebSocket.instances[1].url).toContain('encounter_id=enc-b');

    mgr.close();
  });

  it('routes snapshots through mergeSnapshot into the encounter state store', () => {
    const mgr = createEncounterTabsWs({
      url: 'ws://localhost/dashboard/ws',
      WebSocketCtor: FakeWebSocket,
      setTimeout: () => 0,
      clearTimeout: () => {},
    });

    mgr.setEncounters(['enc-a']);
    mgr.updateState('enc-a', { hp_current: 10, name: 'Rooftop' });

    const ws = FakeWebSocket.instances[0];
    ws.simulateMessage(JSON.stringify({ hp_current: 5, round_number: 2 }));

    const state = mgr.getState('enc-a');
    expect(state.hp_current).toBe(5);
    expect(state.round_number).toBe(2);
    expect(state.name).toBe('Rooftop');
    // No dirty fields, so _pendingFromSnapshot must be empty.
    expect(state._pendingFromSnapshot).toEqual({});

    mgr.close();
  });

  it('preserves dirty DM edits and records them in _pendingFromSnapshot', () => {
    const mgr = createEncounterTabsWs({
      url: 'ws://localhost/dashboard/ws',
      WebSocketCtor: FakeWebSocket,
      setTimeout: () => 0,
      clearTimeout: () => {},
    });

    mgr.setEncounters(['enc-a']);
    mgr.updateState('enc-a', { hp_current: 7, round_number: 1 });
    mgr.markDirty('enc-a', 'hp_current');

    FakeWebSocket.instances[0].simulateMessage(
      JSON.stringify({ hp_current: 3, round_number: 2 }),
    );

    const state = mgr.getState('enc-a');
    // DM's dirty field wins locally.
    expect(state.hp_current).toBe(7);
    // Non-dirty field still updates.
    expect(state.round_number).toBe(2);
    // The pending indicator carries the "HP updated to 3 by player action"
    // value that Phase 103 introduced.
    expect(state._pendingFromSnapshot.hp_current).toBe(3);

    mgr.close();
  });

  it('closes sockets for encounters removed from the active list', () => {
    const mgr = createEncounterTabsWs({
      url: 'ws://localhost/dashboard/ws',
      WebSocketCtor: FakeWebSocket,
      setTimeout: () => 0,
      clearTimeout: () => {},
    });

    mgr.setEncounters(['enc-a', 'enc-b']);
    const wsA = FakeWebSocket.instances[0];
    const wsB = FakeWebSocket.instances[1];

    mgr.setEncounters(['enc-b']);
    expect(wsA.closed).toBe(true);
    expect(wsB.closed).toBe(false);

    mgr.close();
    expect(wsB.closed).toBe(true);
  });

  it('does not open duplicate sockets for encounters already connected', () => {
    const mgr = createEncounterTabsWs({
      url: 'ws://localhost/dashboard/ws',
      WebSocketCtor: FakeWebSocket,
      setTimeout: () => 0,
      clearTimeout: () => {},
    });

    mgr.setEncounters(['enc-a']);
    mgr.setEncounters(['enc-a', 'enc-b']);
    // Exactly one per unique encounter_id: enc-a, enc-b.
    expect(FakeWebSocket.instances).toHaveLength(2);
    mgr.close();
  });

  it('clearDirty removes a field from the dirty set', () => {
    const mgr = createEncounterTabsWs({
      url: 'ws://localhost/dashboard/ws',
      WebSocketCtor: FakeWebSocket,
      setTimeout: () => 0,
      clearTimeout: () => {},
    });

    mgr.setEncounters(['enc-a']);
    mgr.updateState('enc-a', { hp_current: 7 });
    mgr.markDirty('enc-a', 'hp_current');
    mgr.clearDirty('enc-a', 'hp_current');

    FakeWebSocket.instances[0].simulateMessage(
      JSON.stringify({ hp_current: 3 }),
    );

    expect(mgr.getState('enc-a').hp_current).toBe(3);
    mgr.close();
  });

  it('notifies subscribers whenever an encounter state changes', () => {
    const mgr = createEncounterTabsWs({
      url: 'ws://localhost/dashboard/ws',
      WebSocketCtor: FakeWebSocket,
      setTimeout: () => 0,
      clearTimeout: () => {},
    });

    mgr.setEncounters(['enc-a']);

    const events = [];
    const unsubscribe = mgr.subscribe((encID, state) => events.push([encID, state]));

    FakeWebSocket.instances[0].simulateMessage(
      JSON.stringify({ round_number: 9 }),
    );

    expect(events.length).toBeGreaterThanOrEqual(1);
    const [lastID, lastState] = events[events.length - 1];
    expect(lastID).toBe('enc-a');
    expect(lastState.round_number).toBe(9);

    unsubscribe();
    mgr.close();
  });
});
