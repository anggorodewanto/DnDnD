# Batch Dispatch Log

format: `<timestamp> | batch_name | task_ids | notes`

2026-05-10T?? | critical-A1 | crit-02, crit-03, crit-04, crit-05 | parallel implementers, disjoint file scopes (main.go+router; combat damage callers; combat/attack.go; discord/move|fly|distance handlers) | committed 79a6edf
2026-05-10T?? | critical-A2 | crit-06, crit-07 | single combined implementer (both nil-fixes in cmd/dndnd/main.go ~lines 580-740) | committed 49ab856
2026-05-10T?? | critical-C-1 | crit-01a | sequential single implementer — combat handler family (attack, bonus, shove, interact, deathsave). /undo /prepare /retire /character split into crit-01c since they touch different services. Spell handlers in crit-01b. | committed 23112a8
2026-05-10T?? | critical-C-2 | crit-01b | sequential single implementer — spell handler family (/cast, /cast (AoE), /prepare-spells, /action ready). Font-of-magic already done in crit-01a /bonus dispatch. | committed e671444
2026-05-10T?? | critical-C-3 | crit-01c | sequential single implementer — wires 8 already-built handlers + new /undo /retire. Absorbs high-15. | committed 456cb33
2026-05-10T?? | high-main | high-09, high-10, high-13, high-14, high-17 | sequential single implementer — all main.go wiring (RollHistoryLogger, MapRegenerator, loot/itempicker/shops/party-rest mounts, MessageQueue, portal WithAPI+WithCharacterSheet)
2026-05-10T?? | high-par | high-08, high-12, high-16 | three parallel implementers — OnCharacterUpdated fan-out (charactercard); magic-item active abilities (inventory + cast handler + rest handler); DDB re-sync DM approval gate (ddbimport)
