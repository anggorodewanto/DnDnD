// Equipment assembly for the character builder.
//
// Builds the flat equipment id list from the selected starting-equipment pack
// (guaranteed items + chosen options, which may batch comma-separated items
// each carrying a ":N" quantity) plus manually-added items. Manual items are
// deduped against the pack by bare item id (mirrors the original inline logic).
//
// preserveQuantities controls the suffix:
//   - false (default): emit bare ids ("crossbow-bolt") — feeds the
//     equipped-weapon/armor pickers and display, which match on plain ids.
//   - true: keep the ":N" suffix ("crossbow-bolt:20") so the backend seeds real
//     stack sizes (a full quiver, two handaxes) instead of a single item.
export function assembleEquipment({ pack, packChoices, manualEquipment, preserveQuantities = false } = {}) {
  const items = [];
  const emit = (entry) => (preserveQuantities ? entry : entry.split(':')[0]);

  if (pack) {
    if (pack.guaranteed) {
      for (const g of pack.guaranteed) items.push(emit(g));
    }
    if (pack.choices) {
      for (let i = 0; i < pack.choices.length; i++) {
        const chosen = packChoices?.[i];
        if (chosen !== undefined && pack.choices[i].options[chosen]) {
          for (const part of pack.choices[i].options[chosen].split(',')) items.push(emit(part));
        }
      }
    }
  }

  // Manual items: append only if their bare id isn't already present.
  const bareIds = new Set(items.map((e) => e.split(':')[0]));
  for (const id of manualEquipment ?? []) {
    const bare = id.split(':')[0];
    if (!bareIds.has(bare)) {
      bareIds.add(bare);
      items.push(emit(id));
    }
  }

  return items;
}
