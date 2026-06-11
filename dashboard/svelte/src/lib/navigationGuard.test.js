import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  UNSAVED_WARNING,
  registerDirtyCheck,
  hasUnsavedChanges,
  confirmDiscard,
  beforeUnloadHandler,
} from './navigationGuard.js';

// The guard keeps a single module-level slot. Reset it between cases so a
// lingering registration can't leak across tests.
afterEach(() => {
  registerDirtyCheck(() => false)();
});

describe('navigationGuard', () => {
  describe('hasUnsavedChanges', () => {
    it('is false when no editor has registered', () => {
      expect(hasUnsavedChanges()).toBe(false);
    });

    it('reflects the registered editor dirty getter', () => {
      let dirty = false;
      registerDirtyCheck(() => dirty);
      expect(hasUnsavedChanges()).toBe(false);
      dirty = true;
      expect(hasUnsavedChanges()).toBe(true);
    });

    it('is false again once the editor unregisters', () => {
      const unregister = registerDirtyCheck(() => true);
      expect(hasUnsavedChanges()).toBe(true);
      unregister();
      expect(hasUnsavedChanges()).toBe(false);
    });
  });

  describe('registerDirtyCheck race handling', () => {
    it('keeps the latest registration when the outgoing editor tears down after the incoming one mounts', () => {
      const unregisterA = registerDirtyCheck(() => false); // outgoing editor
      const unregisterB = registerDirtyCheck(() => true); // incoming editor
      // Svelte may destroy the outgoing block *after* mounting the new one;
      // its late unregister must not blank out the live editor.
      unregisterA();
      expect(hasUnsavedChanges()).toBe(true);
      unregisterB();
      expect(hasUnsavedChanges()).toBe(false);
    });
  });

  describe('confirmDiscard', () => {
    it('proceeds without prompting when clean', () => {
      const confirmFn = vi.fn(() => false);
      registerDirtyCheck(() => false);
      expect(confirmDiscard(confirmFn)).toBe(true);
      expect(confirmFn).not.toHaveBeenCalled();
    });

    it('prompts and proceeds when dirty and the user confirms', () => {
      const confirmFn = vi.fn(() => true);
      registerDirtyCheck(() => true);
      expect(confirmDiscard(confirmFn)).toBe(true);
      expect(confirmFn).toHaveBeenCalledWith(UNSAVED_WARNING);
    });

    it('blocks navigation when dirty and the user cancels', () => {
      const confirmFn = vi.fn(() => false);
      registerDirtyCheck(() => true);
      expect(confirmDiscard(confirmFn)).toBe(false);
    });
  });

  describe('beforeUnloadHandler', () => {
    it('does nothing when clean', () => {
      registerDirtyCheck(() => false);
      const event = { preventDefault: vi.fn(), returnValue: null };
      expect(beforeUnloadHandler(event)).toBeUndefined();
      expect(event.preventDefault).not.toHaveBeenCalled();
      expect(event.returnValue).toBeNull();
    });

    it('blocks unload and sets returnValue when dirty', () => {
      registerDirtyCheck(() => true);
      const event = { preventDefault: vi.fn(), returnValue: null };
      const result = beforeUnloadHandler(event);
      expect(event.preventDefault).toHaveBeenCalled();
      expect(event.returnValue).toBe(UNSAVED_WARNING);
      expect(result).toBe(UNSAVED_WARNING);
    });
  });
});
