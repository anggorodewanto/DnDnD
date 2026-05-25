import { describe, it, expect } from 'vitest';
import { decodeGID, tilesetForGID, tileSrcRect } from './tiledSprites.js';

// Tiled stores three flip flags in the high bits of a raw GID and the actual
// tile id in the low 29 bits. These constants mirror the Go renderer contract.
const FLIP_H = 0x80000000;
const FLIP_V = 0x40000000;
const FLIP_D = 0x20000000;

describe('decodeGID', () => {
  it('returns id with all flips false for a plain gid', () => {
    expect(decodeGID(5)).toEqual({ id: 5, flipH: false, flipV: false, flipD: false });
  });

  it('treats gid 0 as the empty tile', () => {
    expect(decodeGID(0)).toEqual({ id: 0, flipH: false, flipV: false, flipD: false });
  });

  it('masks the horizontal flip flag', () => {
    expect(decodeGID(FLIP_H | 7)).toEqual({ id: 7, flipH: true, flipV: false, flipD: false });
  });

  it('masks the vertical flip flag', () => {
    expect(decodeGID(FLIP_V | 7)).toEqual({ id: 7, flipH: false, flipV: true, flipD: false });
  });

  it('masks the diagonal flip flag', () => {
    expect(decodeGID(FLIP_D | 7)).toEqual({ id: 7, flipH: false, flipV: false, flipD: true });
  });

  it('masks all three flip flags at once', () => {
    expect(decodeGID(FLIP_H | FLIP_V | FLIP_D | 42)).toEqual({
      id: 42,
      flipH: true,
      flipV: true,
      flipD: true,
    });
  });

  it('handles a large low-29-bit id without sign issues', () => {
    // 0x1FFFFFFF is the largest representable tile id (all 29 low bits set).
    expect(decodeGID(FLIP_H | 0x1fffffff)).toEqual({
      id: 0x1fffffff,
      flipH: true,
      flipV: false,
      flipD: false,
    });
  });

  it('coerces non-finite/garbage input to the empty tile', () => {
    expect(decodeGID(undefined)).toEqual({ id: 0, flipH: false, flipV: false, flipD: false });
    expect(decodeGID(null)).toEqual({ id: 0, flipH: false, flipV: false, flipD: false });
    expect(decodeGID(NaN)).toEqual({ id: 0, flipH: false, flipV: false, flipD: false });
  });
});

describe('tilesetForGID', () => {
  const tsA = { firstgid: 1, image: '/api/assets/a' };
  const tsB = { firstgid: 17, image: '/api/assets/b' };
  const tsC = { firstgid: 49, image: '/api/assets/c' };
  const tilesets = [tsA, tsB, tsC];

  it('returns null for gid 0 (empty tile)', () => {
    expect(tilesetForGID(tilesets, 0)).toBeNull();
  });

  it('returns null when no tileset matches a gid below the first firstgid', () => {
    // firstgid starts at 1, so an id of 0 has no owning tileset; but also test
    // a gap-free set where nothing has firstgid <= id is impossible for id>=1.
    expect(tilesetForGID([{ firstgid: 10, image: 'x' }], 3)).toBeNull();
  });

  it('selects the tileset whose firstgid equals the id', () => {
    expect(tilesetForGID(tilesets, 17)).toBe(tsB);
  });

  it('selects the tileset with the greatest firstgid <= id', () => {
    expect(tilesetForGID(tilesets, 5)).toBe(tsA);
    expect(tilesetForGID(tilesets, 16)).toBe(tsA);
    expect(tilesetForGID(tilesets, 18)).toBe(tsB);
    expect(tilesetForGID(tilesets, 48)).toBe(tsB);
    expect(tilesetForGID(tilesets, 1000)).toBe(tsC);
  });

  it('works regardless of tileset ordering', () => {
    const shuffled = [tsC, tsA, tsB];
    expect(tilesetForGID(shuffled, 18)).toBe(tsB);
    expect(tilesetForGID(shuffled, 5)).toBe(tsA);
    expect(tilesetForGID(shuffled, 1000)).toBe(tsC);
  });

  it('returns null for empty or missing tileset lists', () => {
    expect(tilesetForGID([], 5)).toBeNull();
    expect(tilesetForGID(null, 5)).toBeNull();
    expect(tilesetForGID(undefined, 5)).toBeNull();
  });
});

describe('tileSrcRect', () => {
  it('computes the source rect for the first tile of a simple tileset', () => {
    const ts = {
      firstgid: 1,
      columns: 4,
      tilewidth: 16,
      tileheight: 16,
      margin: 0,
      spacing: 0,
    };
    // id 1 => local index 0 => column 0, row 0.
    expect(tileSrcRect(ts, 1)).toEqual({ sx: 0, sy: 0, sw: 16, sh: 16 });
  });

  it('advances across columns then wraps to the next row', () => {
    const ts = { firstgid: 1, columns: 4, tilewidth: 16, tileheight: 16, margin: 0, spacing: 0 };
    // id 4 => local index 3 => column 3, row 0.
    expect(tileSrcRect(ts, 4)).toEqual({ sx: 48, sy: 0, sw: 16, sh: 16 });
    // id 5 => local index 4 => column 0, row 1.
    expect(tileSrcRect(ts, 5)).toEqual({ sx: 0, sy: 16, sw: 16, sh: 16 });
    // id 6 => local index 5 => column 1, row 1.
    expect(tileSrcRect(ts, 6)).toEqual({ sx: 16, sy: 16, sw: 16, sh: 16 });
  });

  it('respects a non-1 firstgid', () => {
    const ts = { firstgid: 17, columns: 4, tilewidth: 16, tileheight: 16, margin: 0, spacing: 0 };
    // id 17 => local index 0 => column 0, row 0.
    expect(tileSrcRect(ts, 17)).toEqual({ sx: 0, sy: 0, sw: 16, sh: 16 });
    // id 22 => local index 5 => column 1, row 1.
    expect(tileSrcRect(ts, 22)).toEqual({ sx: 16, sy: 16, sw: 16, sh: 16 });
  });

  it('accounts for margin and spacing', () => {
    const ts = { firstgid: 1, columns: 3, tilewidth: 32, tileheight: 32, margin: 2, spacing: 4 };
    // local index 0 => col 0, row 0: margin only.
    expect(tileSrcRect(ts, 1)).toEqual({ sx: 2, sy: 2, sw: 32, sh: 32 });
    // local index 1 => col 1, row 0: margin + 1*(tile+spacing).
    expect(tileSrcRect(ts, 2)).toEqual({ sx: 2 + 36, sy: 2, sw: 32, sh: 32 });
    // local index 3 => col 0, row 1.
    expect(tileSrcRect(ts, 4)).toEqual({ sx: 2, sy: 2 + 36, sw: 32, sh: 32 });
    // local index 4 => col 1, row 1.
    expect(tileSrcRect(ts, 5)).toEqual({ sx: 2 + 36, sy: 2 + 36, sw: 32, sh: 32 });
  });

  it('supports non-square tiles', () => {
    const ts = { firstgid: 1, columns: 2, tilewidth: 24, tileheight: 48, margin: 0, spacing: 0 };
    // local index 3 => col 1, row 1.
    expect(tileSrcRect(ts, 4)).toEqual({ sx: 24, sy: 48, sw: 24, sh: 48 });
  });

  it('falls back to a single column when columns is missing or zero', () => {
    const ts = { firstgid: 1, columns: 0, tilewidth: 16, tileheight: 16, margin: 0, spacing: 0 };
    // With columns coerced to 1, each id stacks vertically.
    expect(tileSrcRect(ts, 1)).toEqual({ sx: 0, sy: 0, sw: 16, sh: 16 });
    expect(tileSrcRect(ts, 3)).toEqual({ sx: 0, sy: 32, sw: 16, sh: 16 });
  });
});
