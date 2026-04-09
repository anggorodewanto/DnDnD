/**
 * Phase 103 — WebSocket state-sync client.
 *
 * Server-authoritative, push-only: the client connects to the dashboard
 * WebSocket, receives full encounter snapshots, and auto-reconnects with
 * exponential backoff (1s, 2s, 4s, 8s, 16s, 30s, capped) after any
 * unexpected disconnect. It does NOT maintain local game state — each
 * snapshot is treated as the complete view of the encounter.
 *
 * Callers get a handle with .close() to stop reconnection attempts.
 * All I/O dependencies (WebSocket, setTimeout) are injectable so the
 * module can be fully unit-tested under vitest without a browser.
 */

export const MAX_BACKOFF_MS = 30000;
export const BACKOFF_SEQUENCE_MS = [1000, 2000, 4000, 8000, 16000, 30000];

/**
 * @param {object} options
 * @param {string} options.url - Base WebSocket URL (ws:// or wss://).
 * @param {string} [options.encounterId] - Encounter to subscribe to. Appended as ?encounter_id=...
 * @param {(snapshot: any) => void} [options.onSnapshot] - Called with parsed JSON for each text frame.
 * @param {(status: 'connecting'|'open'|'reconnecting'|'closed') => void} [options.onStatus]
 * @param {typeof WebSocket} [options.WebSocketCtor] - DI hook for tests.
 * @param {typeof globalThis.setTimeout} [options.setTimeout]
 * @param {typeof globalThis.clearTimeout} [options.clearTimeout]
 * @returns {{ close: () => void }}
 */
export function createWsClient(options) {
  const {
    url,
    encounterId = '',
    onSnapshot,
    onStatus,
    WebSocketCtor = globalThis.WebSocket,
    setTimeout: setTimeoutFn = globalThis.setTimeout,
    clearTimeout: clearTimeoutFn = globalThis.clearTimeout,
  } = options || {};

  const fullUrl = buildUrl(url, encounterId);
  let backoffIndex = 0;
  let stopped = false;
  let currentWs = null;
  let reconnectTimerId = null;

  const emitStatus = (s) => {
    if (typeof onStatus === 'function') onStatus(s);
  };

  const scheduleReconnect = () => {
    if (stopped) return;
    const delay = BACKOFF_SEQUENCE_MS[Math.min(backoffIndex, BACKOFF_SEQUENCE_MS.length - 1)];
    backoffIndex += 1;
    emitStatus('reconnecting');
    reconnectTimerId = setTimeoutFn(() => {
      reconnectTimerId = null;
      connect();
    }, delay);
  };

  const connect = () => {
    if (stopped) return;
    emitStatus('connecting');
    const ws = new WebSocketCtor(fullUrl);
    currentWs = ws;

    ws.onopen = () => {
      backoffIndex = 0;
      emitStatus('open');
    };

    ws.onmessage = (event) => {
      if (typeof onSnapshot !== 'function') return;
      let parsed;
      try {
        parsed = JSON.parse(event.data);
      } catch (_err) {
        return;
      }
      onSnapshot(parsed);
    };

    ws.onclose = () => {
      if (stopped) return;
      scheduleReconnect();
    };

    // No onerror handler: the browser fires onclose after onerror anyway,
    // and onclose is what drives our reconnect logic.
  };

  connect();

  return {
    close() {
      stopped = true;
      if (reconnectTimerId !== null) {
        clearTimeoutFn(reconnectTimerId);
        reconnectTimerId = null;
      }
      if (currentWs) {
        try {
          currentWs.close();
        } catch (_err) {
          // ignore — close() may throw if the socket is already closed
        }
      }
      emitStatus('closed');
    },
  };
}

function buildUrl(base, encounterId) {
  if (!encounterId) return base;
  const sep = base.includes('?') ? '&' : '?';
  return `${base}${sep}encounter_id=${encodeURIComponent(encounterId)}`;
}
