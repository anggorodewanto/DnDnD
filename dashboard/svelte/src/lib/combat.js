/**
 * Combat logic utilities for the Combat Manager.
 */

/**
 * Standard 5e conditions.
 */
export const STANDARD_CONDITIONS = [
  'Blinded', 'Charmed', 'Deafened', 'Frightened', 'Grappled',
  'Incapacitated', 'Invisible', 'Paralyzed', 'Petrified', 'Poisoned',
  'Prone', 'Restrained', 'Stunned', 'Unconscious',
];

/**
 * Apply damage to a combatant, absorbing temp HP first.
 * Returns new { hp_current, temp_hp, is_alive }.
 */
export function applyDamage(combatant, amount) {
  if (amount <= 0) {
    return {
      hp_current: combatant.hp_current,
      temp_hp: combatant.temp_hp,
      is_alive: combatant.hp_current > 0,
    };
  }

  let remaining = amount;
  let tempHp = combatant.temp_hp || 0;
  let hpCurrent = combatant.hp_current;

  // Absorb from temp HP first
  if (tempHp > 0) {
    const absorbed = Math.min(tempHp, remaining);
    tempHp -= absorbed;
    remaining -= absorbed;
  }

  // Then reduce current HP
  hpCurrent = Math.max(0, hpCurrent - remaining);

  return {
    hp_current: hpCurrent,
    temp_hp: tempHp,
    is_alive: hpCurrent > 0,
  };
}

/**
 * Apply healing to a combatant, capped at hp_max.
 * Returns new { hp_current, is_alive }.
 */
export function applyHealing(combatant, amount) {
  if (amount <= 0) {
    return {
      hp_current: combatant.hp_current,
      is_alive: combatant.hp_current > 0,
    };
  }

  const hpCurrent = Math.min(combatant.hp_max, combatant.hp_current + amount);
  return {
    hp_current: hpCurrent,
    is_alive: hpCurrent > 0,
  };
}

/**
 * Returns a health tier string based on HP percentage.
 * Used for token color coding.
 */
export function healthTier(hpCurrent, hpMax) {
  if (hpMax <= 0) return 'dead';
  if (hpCurrent <= 0) return 'dead';

  const pct = hpCurrent / hpMax;
  if (pct > 0.75) return 'healthy';
  if (pct > 0.5) return 'wounded';
  if (pct > 0.25) return 'bloodied';
  return 'critical';
}

/**
 * Add a condition to a conditions array (no duplicates).
 * Returns new array.
 */
export function addCondition(conditions, condition) {
  if (conditions.includes(condition)) return conditions;
  return [...conditions, condition];
}

/**
 * Remove a condition from a conditions array.
 * Returns new array.
 */
export function removeCondition(conditions, condition) {
  return conditions.filter(c => c !== condition);
}

/**
 * Returns the opacity for a combatant token on the DM map.
 * Invisible combatants are rendered with reduced opacity.
 */
export function tokenOpacity(combatant) {
  if (combatant.is_visible === false) return 0.4;
  return 1.0;
}

/**
 * Calculate grid distance in feet using Chebyshev distance.
 * Diagonals cost 5ft same as cardinal moves per spec.
 * @param {number} col1 - Start column (0-based).
 * @param {number} row1 - Start row (0-based).
 * @param {number} col2 - End column (0-based).
 * @param {number} row2 - End row (0-based).
 * @returns {number} Distance in feet.
 */
export function gridDistance(col1, row1, col2, row2) {
  const dc = Math.abs(col2 - col1);
  const dr = Math.abs(row2 - row1);
  return Math.max(dc, dr) * 5;
}

/**
 * Convert a column letter (A-Z, AA, etc.) to a 0-based index.
 */
export function colToIndex(col) {
  if (!col) return 0;
  let result = 0;
  for (let i = 0; i < col.length; i++) {
    result = result * 26 + (col.charCodeAt(i) - 64);
  }
  return result - 1;
}

/**
 * Convert a 0-based column index to a column letter (A-Z, AA, etc.).
 * Reverse of colToIndex.
 */
export function indexToCol(idx) {
  let result = '';
  let n = idx + 1;
  while (n > 0) {
    n--;
    result = String.fromCharCode(65 + (n % 26)) + result;
    n = Math.floor(n / 26);
  }
  return result;
}

/**
 * Return all tiles within Chebyshev distance (range in tiles) of a center tile,
 * excluding the center itself. Clips to map bounds.
 * @param {number} centerCol - 0-based column.
 * @param {number} centerRow - 0-based row.
 * @param {number} rangeTiles - Range in tiles.
 * @param {number} mapWidth - Map width in tiles.
 * @param {number} mapHeight - Map height in tiles.
 * @returns {{ col: number, row: number }[]}
 */
export function tilesInRange(centerCol, centerRow, rangeTiles, mapWidth, mapHeight) {
  if (rangeTiles <= 0) return [];
  const tiles = [];
  const minCol = Math.max(0, centerCol - rangeTiles);
  const maxCol = Math.min(mapWidth - 1, centerCol + rangeTiles);
  const minRow = Math.max(0, centerRow - rangeTiles);
  const maxRow = Math.min(mapHeight - 1, centerRow + rangeTiles);
  for (let r = minRow; r <= maxRow; r++) {
    for (let c = minCol; c <= maxCol; c++) {
      if (c === centerCol && r === centerRow) continue;
      tiles.push({ col: c, row: r });
    }
  }
  return tiles;
}

/**
 * Check if a wall blocks movement between two adjacent tiles.
 * @param {number} col1 - Start column (0-based).
 * @param {number} row1 - Start row (0-based).
 * @param {number} col2 - End column (0-based).
 * @param {number} row2 - End row (0-based).
 * @param {object[]} walls - Wall objects from Tiled JSON ({ x, y, width, height }).
 * @param {number} tileSize - Tile size in pixels.
 * @returns {boolean}
 */
export function isWallBetween(col1, row1, col2, row2, walls, tileSize) {
  for (const wall of walls) {
    if (wall.width > 0 && wall.height === 0) {
      // Horizontal wall: blocks vertical movement
      const wallRow = wall.y / tileSize;
      const wallColStart = wall.x / tileSize;
      const wallColEnd = (wall.x + wall.width) / tileSize;
      if (col1 === col2) {
        const minRow = Math.min(row1, row2);
        if (wallRow === minRow + 1 && col1 >= wallColStart && col1 < wallColEnd) {
          return true;
        }
      }
    } else if (wall.height > 0 && wall.width === 0) {
      // Vertical wall: blocks horizontal movement
      const wallCol = wall.x / tileSize;
      const wallRowStart = wall.y / tileSize;
      const wallRowEnd = (wall.y + wall.height) / tileSize;
      if (row1 === row2) {
        const minCol = Math.min(col1, col2);
        if (wallCol === minCol + 1 && row1 >= wallRowStart && row1 < wallRowEnd) {
          return true;
        }
      }
    }
  }
  return false;
}
