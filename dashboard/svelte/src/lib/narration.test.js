import { describe, it, expect } from 'vitest';
import { renderDiscord, insertReadAloudBlock, READ_ALOUD_COLOR } from './narration.js';

describe('renderDiscord', () => {
  it('passes plain text through unchanged', () => {
    const out = renderDiscord('Hello, adventurers!');
    expect(out.body).toBe('Hello, adventurers!');
    expect(out.embeds).toEqual([]);
  });

  it('passes bold/italic/blockquote markdown through', () => {
    const src = '**bold** and *italic*\n> quoted line';
    const out = renderDiscord(src);
    expect(out.body).toBe(src);
  });

  it('extracts read-aloud block into embed', () => {
    const src = 'Before.\n:::read-aloud\nThe stone door grinds open.\n:::\nAfter.';
    const out = renderDiscord(src);
    expect(out.body).toContain('Before.');
    expect(out.body).toContain('After.');
    expect(out.body).not.toContain(':::');
    expect(out.body).not.toContain('stone door');
    expect(out.embeds).toHaveLength(1);
    expect(out.embeds[0].description).toBe('The stone door grinds open.');
    expect(out.embeds[0].color).toBe(READ_ALOUD_COLOR);
  });

  it('handles multiple read-aloud blocks', () => {
    const src = ':::read-aloud\nFirst.\n:::\nmiddle\n:::read-aloud\nSecond.\n:::';
    const out = renderDiscord(src);
    expect(out.embeds).toHaveLength(2);
    expect(out.embeds[0].description).toBe('First.');
    expect(out.embeds[1].description).toBe('Second.');
    expect(out.body).toContain('middle');
  });

  it('treats unclosed fence as literal text', () => {
    const out = renderDiscord(':::read-aloud\nno closer');
    expect(out.embeds).toEqual([]);
    expect(out.body).toContain('no closer');
  });

  it('handles null/undefined input', () => {
    expect(renderDiscord(null).body).toBe('');
    expect(renderDiscord(undefined).body).toBe('');
  });
});

describe('insertReadAloudBlock', () => {
  it('inserts a template at the caret', () => {
    const out = insertReadAloudBlock('', 0);
    expect(out.text).toContain(':::read-aloud');
    expect(out.text).toContain(':::');
    // Caret should land on the empty line between the fences.
    expect(out.text[out.caret]).toBe('\n'); // next char is newline (blank line)
  });

  it('prepends a newline when caret is mid-line', () => {
    const src = 'hello';
    const out = insertReadAloudBlock(src, 5);
    expect(out.text.startsWith('hello\n:::read-aloud')).toBe(true);
  });

  it('does not prepend newline when caret is at start of line', () => {
    const src = 'hello\n';
    const out = insertReadAloudBlock(src, 6);
    expect(out.text).toBe('hello\n:::read-aloud\n\n:::\n');
  });
});
