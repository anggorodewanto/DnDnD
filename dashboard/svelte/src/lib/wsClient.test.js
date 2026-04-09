import { describe, it, expect, vi, beforeEach } from 'vitest';
import { createWsClient, BACKOFF_SEQUENCE_MS, MAX_BACKOFF_MS } from './wsClient.js';

/**
 * FakeWebSocket lets tests drive the underlying connection lifecycle
 * synchronously. Instances are recorded so the test can assert on
 * construction args and simulate open/message/close events.
 */
class FakeWebSocket {
  static instances = [];
  constructor(url) {
    this.url = url;
    this.sent = [];
    this.closed = false;
    this.onopen = null;
    this.onmessage = null;
    this.onclose = null;
    this.onerror = null;
    FakeWebSocket.instances.push(this);
  }
  send(m) { this.sent.push(m); }
  close() {
    this.closed = true;
    if (this.onclose) this.onclose({ code: 1000 });
  }
  simulateOpen() { if (this.onopen) this.onopen(); }
  simulateMessage(data) { if (this.onmessage) this.onmessage({ data }); }
  simulateClose(code = 1006) {
    this.closed = true;
    if (this.onclose) this.onclose({ code });
  }
}

/** Fake setTimeout that records scheduled callbacks and lets us flush them. */
function makeFakeTimer() {
  const scheduled = [];
  let nextId = 1;
  const setTimeout = (fn, ms) => {
    const id = nextId++;
    scheduled.push({ id, fn, ms });
    return id;
  };
  const clearTimeout = (id) => {
    const idx = scheduled.findIndex((s) => s.id === id);
    if (idx >= 0) scheduled.splice(idx, 1);
  };
  const flushNext = () => {
    const next = scheduled.shift();
    if (!next) return null;
    next.fn();
    return next.ms;
  };
  return { setTimeout, clearTimeout, flushNext, scheduled };
}

beforeEach(() => {
  FakeWebSocket.instances = [];
});

describe('BACKOFF_SEQUENCE_MS', () => {
  it('follows 1s,2s,4s,8s,16s,30s… capped at MAX_BACKOFF_MS', () => {
    expect(BACKOFF_SEQUENCE_MS).toEqual([1000, 2000, 4000, 8000, 16000, 30000]);
    expect(MAX_BACKOFF_MS).toBe(30000);
  });
});

describe('createWsClient', () => {
  it('opens a WebSocket and calls onStatus("connecting") then "open"', () => {
    const statuses = [];
    const timer = makeFakeTimer();
    createWsClient({
      url: 'ws://x/ws',
      encounterId: 'enc-1',
      onSnapshot: () => {},
      onStatus: (s) => statuses.push(s),
      WebSocketCtor: FakeWebSocket,
      setTimeout: timer.setTimeout,
      clearTimeout: timer.clearTimeout,
    });

    expect(statuses).toEqual(['connecting']);
    expect(FakeWebSocket.instances).toHaveLength(1);
    expect(FakeWebSocket.instances[0].url).toBe('ws://x/ws?encounter_id=enc-1');

    FakeWebSocket.instances[0].simulateOpen();
    expect(statuses).toEqual(['connecting', 'open']);
  });

  it('dispatches parsed JSON messages to onSnapshot', () => {
    const received = [];
    const timer = makeFakeTimer();
    createWsClient({
      url: 'ws://x/ws',
      encounterId: 'e',
      onSnapshot: (s) => received.push(s),
      WebSocketCtor: FakeWebSocket,
      setTimeout: timer.setTimeout,
      clearTimeout: timer.clearTimeout,
    });
    FakeWebSocket.instances[0].simulateOpen();
    FakeWebSocket.instances[0].simulateMessage('{"type":"encounter_snapshot","encounter_id":"e"}');
    expect(received).toEqual([{ type: 'encounter_snapshot', encounter_id: 'e' }]);
  });

  it('ignores malformed JSON without throwing', () => {
    const received = [];
    const timer = makeFakeTimer();
    createWsClient({
      url: 'ws://x/ws',
      encounterId: 'e',
      onSnapshot: (s) => received.push(s),
      WebSocketCtor: FakeWebSocket,
      setTimeout: timer.setTimeout,
      clearTimeout: timer.clearTimeout,
    });
    FakeWebSocket.instances[0].simulateOpen();
    expect(() => FakeWebSocket.instances[0].simulateMessage('not json')).not.toThrow();
    expect(received).toEqual([]);
  });

  it('reconnects with exponential backoff after unexpected close', () => {
    const statuses = [];
    const timer = makeFakeTimer();
    createWsClient({
      url: 'ws://x/ws',
      encounterId: 'e',
      onSnapshot: () => {},
      onStatus: (s) => statuses.push(s),
      WebSocketCtor: FakeWebSocket,
      setTimeout: timer.setTimeout,
      clearTimeout: timer.clearTimeout,
    });

    // First connection closes before opening → schedule reconnect @1s
    FakeWebSocket.instances[0].simulateClose(1006);
    expect(statuses[statuses.length - 1]).toBe('reconnecting');
    expect(timer.scheduled[0].ms).toBe(1000);

    // Flush reconnect; new ws, close again → next backoff = 2s
    timer.flushNext();
    expect(FakeWebSocket.instances).toHaveLength(2);
    FakeWebSocket.instances[1].simulateClose(1006);
    expect(timer.scheduled[0].ms).toBe(2000);

    timer.flushNext();
    FakeWebSocket.instances[2].simulateClose(1006);
    expect(timer.scheduled[0].ms).toBe(4000);

    timer.flushNext();
    FakeWebSocket.instances[3].simulateClose(1006);
    expect(timer.scheduled[0].ms).toBe(8000);

    timer.flushNext();
    FakeWebSocket.instances[4].simulateClose(1006);
    expect(timer.scheduled[0].ms).toBe(16000);

    timer.flushNext();
    FakeWebSocket.instances[5].simulateClose(1006);
    expect(timer.scheduled[0].ms).toBe(30000);

    // Cap: further closes stay at 30s
    timer.flushNext();
    FakeWebSocket.instances[6].simulateClose(1006);
    expect(timer.scheduled[0].ms).toBe(30000);
  });

  it('resets backoff to 1s after a successful open', () => {
    const timer = makeFakeTimer();
    createWsClient({
      url: 'ws://x/ws',
      encounterId: 'e',
      onSnapshot: () => {},
      WebSocketCtor: FakeWebSocket,
      setTimeout: timer.setTimeout,
      clearTimeout: timer.clearTimeout,
    });

    FakeWebSocket.instances[0].simulateClose(1006);
    expect(timer.scheduled[0].ms).toBe(1000);
    timer.flushNext();

    FakeWebSocket.instances[1].simulateClose(1006);
    expect(timer.scheduled[0].ms).toBe(2000);
    timer.flushNext();

    // This one opens successfully, then closes — backoff should reset to 1s
    FakeWebSocket.instances[2].simulateOpen();
    FakeWebSocket.instances[2].simulateClose(1006);
    expect(timer.scheduled[0].ms).toBe(1000);
  });

  it('.close() stops reconnect attempts', () => {
    const statuses = [];
    const timer = makeFakeTimer();
    const client = createWsClient({
      url: 'ws://x/ws',
      encounterId: 'e',
      onSnapshot: () => {},
      onStatus: (s) => statuses.push(s),
      WebSocketCtor: FakeWebSocket,
      setTimeout: timer.setTimeout,
      clearTimeout: timer.clearTimeout,
    });

    FakeWebSocket.instances[0].simulateOpen();
    client.close();
    expect(statuses[statuses.length - 1]).toBe('closed');
    // Simulating a close after client.close() must NOT schedule a reconnect
    FakeWebSocket.instances[0].simulateClose(1000);
    expect(timer.scheduled).toHaveLength(0);
  });

  it('URL without query params appends encounter_id correctly', () => {
    const timer = makeFakeTimer();
    createWsClient({
      url: 'ws://x/ws?foo=1',
      encounterId: 'enc-42',
      onSnapshot: () => {},
      WebSocketCtor: FakeWebSocket,
      setTimeout: timer.setTimeout,
      clearTimeout: timer.clearTimeout,
    });
    expect(FakeWebSocket.instances[0].url).toBe('ws://x/ws?foo=1&encounter_id=enc-42');
  });

  it('omits encounter_id param when encounterId is empty', () => {
    const timer = makeFakeTimer();
    createWsClient({
      url: 'ws://x/ws',
      encounterId: '',
      onSnapshot: () => {},
      WebSocketCtor: FakeWebSocket,
      setTimeout: timer.setTimeout,
      clearTimeout: timer.clearTimeout,
    });
    expect(FakeWebSocket.instances[0].url).toBe('ws://x/ws');
  });

  it('.close() while a reconnect is pending clears the timer', () => {
    const timer = makeFakeTimer();
    const client = createWsClient({
      url: 'ws://x/ws',
      encounterId: 'e',
      onSnapshot: () => {},
      WebSocketCtor: FakeWebSocket,
      setTimeout: timer.setTimeout,
      clearTimeout: timer.clearTimeout,
    });
    FakeWebSocket.instances[0].simulateClose(1006);
    expect(timer.scheduled).toHaveLength(1);
    client.close();
    expect(timer.scheduled).toHaveLength(0);
  });

  it('.close() tolerates WebSocket.close() throwing', () => {
    class ThrowingWs extends FakeWebSocket {
      close() {
        throw new Error('boom');
      }
    }
    const timer = makeFakeTimer();
    const client = createWsClient({
      url: 'ws://x/ws',
      encounterId: 'e',
      onSnapshot: () => {},
      WebSocketCtor: ThrowingWs,
      setTimeout: timer.setTimeout,
      clearTimeout: timer.clearTimeout,
    });
    expect(() => client.close()).not.toThrow();
  });

  it('onerror handler does not throw', () => {
    const timer = makeFakeTimer();
    createWsClient({
      url: 'ws://x/ws',
      encounterId: 'e',
      onSnapshot: () => {},
      WebSocketCtor: FakeWebSocket,
      setTimeout: timer.setTimeout,
      clearTimeout: timer.clearTimeout,
    });
    expect(() => {
      if (FakeWebSocket.instances[0].onerror) {
        FakeWebSocket.instances[0].onerror(new Error('x'));
      }
    }).not.toThrow();
  });

  it('tolerates missing onStatus / onSnapshot callbacks', () => {
    const timer = makeFakeTimer();
    expect(() =>
      createWsClient({
        url: 'ws://x/ws',
        encounterId: 'e',
        WebSocketCtor: FakeWebSocket,
        setTimeout: timer.setTimeout,
        clearTimeout: timer.clearTimeout,
      }),
    ).not.toThrow();
    expect(() => {
      FakeWebSocket.instances[0].simulateOpen();
      FakeWebSocket.instances[0].simulateMessage('{"ok":true}');
      FakeWebSocket.instances[0].simulateClose(1006);
    }).not.toThrow();
  });
});
