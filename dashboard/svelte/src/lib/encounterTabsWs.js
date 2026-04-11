/**
 * Phase 105 — Per-tab WebSocket state-sync manager for the tabbed Combat
 * Workspace. Each active encounter gets its own wsClient subscription
 * (`?encounter_id=<uuid>`) and a merged view-model state that preserves DM
 * form edits via Phase 103's mergeSnapshot.
 *
 * The manager is a plain JS object with no Svelte dependency so it can be
 * unit-tested in vitest. CombatManager.svelte wires it into the UI via
 * setEncounters / subscribe / markDirty / clearDirty.
 */

import { createWsClient } from './wsClient.js';
import { mergeSnapshot } from './optimisticMerge.js';

/**
 * @param {object} options
 * @param {string} options.url - Base WebSocket URL (ws:// or wss://).
 * @param {typeof WebSocket} [options.WebSocketCtor]
 * @param {typeof globalThis.setTimeout} [options.setTimeout]
 * @param {typeof globalThis.clearTimeout} [options.clearTimeout]
 */
export function createEncounterTabsWs(options) {
  const { url, WebSocketCtor, setTimeout: setTimeoutFn, clearTimeout: clearTimeoutFn } = options || {};

  /** @type {Map<string, {client: {close: () => void}, state: object, dirty: Set<string>}>} */
  const tabs = new Map();
  /** @type {Set<(encID: string, state: object) => void>} */
  const subscribers = new Set();

  const notify = (encID) => {
    const tab = tabs.get(encID);
    if (!tab) return;
    for (const fn of subscribers) fn(encID, tab.state);
  };

  const openTab = (encID) => {
    if (tabs.has(encID)) return;
    const tab = { client: null, state: {}, dirty: new Set() };

    tab.client = createWsClient({
      url,
      encounterId: encID,
      onSnapshot: (snapshot) => {
        tab.state = mergeSnapshot(tab.state, snapshot, tab.dirty);
        notify(encID);
      },
      WebSocketCtor,
      setTimeout: setTimeoutFn,
      clearTimeout: clearTimeoutFn,
    });

    tabs.set(encID, tab);
  };

  const closeTab = (encID) => {
    const tab = tabs.get(encID);
    if (!tab) return;
    tab.client.close();
    tabs.delete(encID);
  };

  return {
    /**
     * Sync the list of open tabs to exactly the given encounter IDs. Opens
     * new wsClients for new IDs, closes any removed ones, and leaves
     * existing connections untouched.
     * @param {string[]} encounterIDs
     */
    setEncounters(encounterIDs) {
      const wanted = new Set(encounterIDs);
      for (const existing of [...tabs.keys()]) {
        if (!wanted.has(existing)) closeTab(existing);
      }
      for (const id of encounterIDs) {
        openTab(id);
      }
    },

    /**
     * Manually merge new fields into the view-model state for an
     * encounter (used when initial data arrives via HTTP before the
     * first WebSocket snapshot).
     */
    updateState(encID, patch) {
      const tab = tabs.get(encID);
      if (!tab) return;
      tab.state = { ...tab.state, ...patch };
      if (!tab.state._pendingFromSnapshot) {
        tab.state._pendingFromSnapshot = {};
      }
      notify(encID);
    },

    getState(encID) {
      const tab = tabs.get(encID);
      return tab ? tab.state : null;
    },

    /**
     * Mark a form field as "dirty" (DM is actively editing). While the
     * field is dirty, snapshots will NOT overwrite its current value;
     * the incoming value is parked in `_pendingFromSnapshot.<field>`.
     */
    markDirty(encID, field) {
      const tab = tabs.get(encID);
      if (!tab) return;
      tab.dirty.add(field);
    },

    clearDirty(encID, field) {
      const tab = tabs.get(encID);
      if (!tab) return;
      tab.dirty.delete(field);
    },

    /**
     * Subscribe to state changes. Callback is invoked with
     * (encounterID, mergedState) whenever a snapshot or local update
     * occurs. Returns an unsubscribe function.
     */
    subscribe(fn) {
      subscribers.add(fn);
      return () => subscribers.delete(fn);
    },

    /** Close all sockets and release resources. */
    close() {
      for (const encID of [...tabs.keys()]) closeTab(encID);
      subscribers.clear();
    },
  };
}
