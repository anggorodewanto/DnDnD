/**
 * Narration rendering library (Phase 100a).
 *
 * Mirrors internal/narration/markdown.go so the Svelte preview can render
 * without a round-trip to the server. The renderer passes Discord markdown
 * (bold/italic/blockquote) through untouched and extracts :::read-aloud
 * fenced blocks into embed objects.
 */

export const READ_ALOUD_COLOR = 0xd4af37;
const FENCE_OPEN = ':::read-aloud';
const FENCE_CLOSE = ':::';

/**
 * Render a narration source string into a DiscordMessage-shaped object:
 *   { body: string, embeds: [{ description, color }] }
 *
 * @param {string} src
 * @returns {{body: string, embeds: {description: string, color: number}[]}}
 */
export function renderDiscord(src) {
  const lines = String(src ?? '').split('\n');
  const body = [];
  const embeds = [];

  let i = 0;
  while (i < lines.length) {
    if (lines[i].trim() !== FENCE_OPEN) {
      body.push(lines[i]);
      i += 1;
      continue;
    }
    const { block, nextIndex, closed } = collectReadAloud(lines, i + 1);
    if (!closed) {
      body.push(lines[i]);
      i += 1;
      continue;
    }
    embeds.push({ description: block, color: READ_ALOUD_COLOR });
    i = nextIndex;
  }

  return {
    body: collapseBlankLines(body.join('\n')),
    embeds,
  };
}

function collectReadAloud(lines, start) {
  const block = [];
  for (let j = start; j < lines.length; j += 1) {
    if (lines[j].trim() === FENCE_CLOSE) {
      return { block: block.join('\n'), nextIndex: j + 1, closed: true };
    }
    block.push(lines[j]);
  }
  return { block: '', nextIndex: start, closed: false };
}

function collapseBlankLines(s) {
  const lines = s.split('\n');
  const out = [];
  let blank = 0;
  for (const line of lines) {
    if (line.trim() === '') {
      blank += 1;
      if (blank <= 1) {
        out.push('');
      }
      continue;
    }
    blank = 0;
    out.push(line);
  }
  return out.join('\n').replace(/^\n+|\n+$/g, '');
}

/**
 * Insert a read-aloud block template at the given caret position.
 * Returns the new string and the caret position inside the block so the
 * caller can restore the textarea selection.
 *
 * @param {string} src
 * @param {number} caret
 */
export function insertReadAloudBlock(src, caret) {
  const template = `${FENCE_OPEN}\n\n${FENCE_CLOSE}\n`;
  const before = src.slice(0, caret);
  const after = src.slice(caret);
  const prefix = before.length === 0 || before.endsWith('\n') ? '' : '\n';
  const insertion = `${prefix}${template}`;
  const newSrc = before + insertion + after;
  const newCaret = before.length + prefix.length + FENCE_OPEN.length + 1;
  return { text: newSrc, caret: newCaret };
}
