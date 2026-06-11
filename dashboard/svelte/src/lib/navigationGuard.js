/**
 * Unsaved-changes navigation guard.
 *
 * The map editor and encounter builder track a `dirty` flag but live inside the
 * dashboard shell (App.svelte), which previously discarded their state on
 * sidebar navigation, the Back buttons, and tab close without any prompt. Each
 * editor registers a `() => boolean` dirty getter while mounted; the shell
 * consults it before in-app navigation and from a single `beforeunload`
 * handler, so unsaved work is never thrown away silently.
 *
 * Kept DOM-light so it unit-tests under the repo's node test environment.
 */

export const UNSAVED_WARNING =
  'You have unsaved changes that will be lost. Leave without saving?';

// Only one editor is mounted at a time, so a single slot suffices.
let dirtyCheck = null;

/**
 * Register the active editor's dirty getter. Returns an unregister function
 * suitable as a Svelte `$effect` cleanup. Last registration wins; the
 * unregister only clears the slot if it still owns it, so a late teardown from
 * an outgoing editor can't blank out the editor that just mounted.
 *
 * @param {() => boolean} fn
 * @returns {() => void}
 */
export function registerDirtyCheck(fn) {
  dirtyCheck = fn;
  return () => {
    if (dirtyCheck === fn) dirtyCheck = null;
  };
}

/** @returns {boolean} whether the active editor reports unsaved changes. */
export function hasUnsavedChanges() {
  return typeof dirtyCheck === 'function' && dirtyCheck() === true;
}

/**
 * Decide whether navigation may proceed: true when there are no unsaved
 * changes, otherwise the result of the caller-supplied confirm prompt.
 *
 * @param {(message: string) => boolean} confirmFn
 * @returns {boolean}
 */
export function confirmDiscard(confirmFn) {
  if (!hasUnsavedChanges()) return true;
  return confirmFn(UNSAVED_WARNING) === true;
}

/**
 * `beforeunload` handler: triggers the browser's native leave-confirmation when
 * the active editor is dirty, and is a no-op otherwise.
 *
 * @param {BeforeUnloadEvent} event
 */
export function beforeUnloadHandler(event) {
  if (!hasUnsavedChanges()) return undefined;
  event.preventDefault();
  // Legacy browsers require returnValue to be set to show the prompt.
  event.returnValue = UNSAVED_WARNING;
  return UNSAVED_WARNING;
}
