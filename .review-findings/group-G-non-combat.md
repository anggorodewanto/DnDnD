# Group G: Non-Combat (Phases 80-88c) — Correctness Review

Scope: `/recap`, `/check`, `/save`, `/rest` (short, long, party, interrupt), inventory + `/use` + `/give`, looting, item picker, shops, magic items (passive, active, attune, identify).

---

## [Critical] Passive-effect vocabulary in spec does not match the code parser
- **Location:** internal/magicitem/effects.go:112-160, internal/combat/feature_integration.go:585-603
- **Spec/Phase ref:** spec §Magic Items lines 2697-2701 (Phase 88a)
- **D&D rule:** Cloak of Protection grants +1 to AC and saves; Ring of Resistance grants resistance to a damage type.
- **Problem:** Spec explicitly demonstrates magic item passive effects as `{type: "modify_save", modifier: 1}` and `{type: "grant_resistance", damage_type: "fire"}`. The parser only recognizes `modify_saving_throw` and `resistance`. Any DM (or future SRD import) that hand-authors `passive_effects` JSON following the spec verbatim will silently produce no effect — the Cloak of Protection save bonus and Ring of Resistance resistance will not apply.
- **Suggested fix:** Either update the spec to use `modify_saving_throw` / `resistance`, or add `modify_save` and `grant_resistance` as accepted aliases in `convertPassiveEffect` and `convertMagicItemPassiveEntry`.

## [Critical] `/attune` does not require a short rest
- **Location:** internal/inventory/attunement.go:33-67, internal/discord/attune_handler.go:68-159
- **Spec/Phase ref:** spec §Magic Items lines 2710-2712 (Phase 88b)
- **D&D rule:** PHB p.138: attunement requires a creature to spend a short rest focused on the item.
- **Problem:** `Attune` validates inventory presence, requires_attunement, slot cap (3), and class restriction, but never checks that the caster is currently in / has just completed a short rest. The Discord handler can be called at any time, immediately granting bonuses. Spec line 2712 says "Requires a short rest (can be done during `/rest short` flow)".
- **Suggested fix:** Either accept `/attune` only inside the active short-rest button flow, or add a rest-context flag/parameter and reject the call when the campaign setting / encounter state shows no recent short rest.

## [Critical] `destroy_on_zero` roll happens at dawn, not when last charge is spent
- **Location:** internal/inventory/recharge.go:38-92, internal/inventory/active_ability.go:28-62
- **Spec/Phase ref:** spec §Magic Items line 2707; PHB Wand of Fireballs description
- **D&D rule:** RAW (Wand of Fireballs et al.): "If you expend the wand's last charge, roll a d20. On a 1, the wand crumbles into ashes and is destroyed."
- **Problem:** `UseCharges` decrements charges without any d20 destroy check at the moment the last charge is spent. The destroy roll is instead performed by `DawnRecharge` when it sees `Charges == 0`. That means (a) the player escapes the destroy check until the next dawn, and (b) the destroy check incorrectly fires every dawn the item is sitting at 0 charges (not only the dawn after the last charge was spent).
- **Suggested fix:** Move the d20 destroy roll into `UseCharges` when the deducted amount drains the wand to 0, and drop the check from `DawnRecharge` (it only rolls recharge dice).

## [Critical] Antitoxin "advantage vs poison" is not actually tracked
- **Location:** internal/inventory/service.go:135-140
- **Spec/Phase ref:** spec §Inventory Management lines 2647 (Phase 84)
- **D&D rule:** Antitoxin (PHB p.139): for 1 hour after drinking, the creature has advantage on saving throws against poison.
- **Problem:** `UseConsumable` consumes the antitoxin and posts a flavor message claiming advantage was granted, but no buff/condition is written to the character or combatant. The next poison save the player rolls is a plain d20.
- **Suggested fix:** Apply a timed condition (e.g. `condition_antitoxin` with `duration_remaining: 1h`) on the combatant/character that `CheckSaveWithExhaustion` (or save FES) consults to add advantage when `ability == "con"` and the incoming damage type / save tag is "poison".

## [High] Gold split silently discards remainder
- **Location:** internal/loot/service.go:289-329
- **Spec/Phase ref:** spec §Inventory Management line 2661 (Phase 85)
- **D&D rule:** N/A — but the spec contracts "Gold can be split evenly".
- **Problem:** `SplitGold` computes `share := pool.GoldTotal / int32(len(pcs))` then zeros the pool total. For 7 gp / 3 players, each receives 2 gp (6 total) and 1 gp evaporates. The remainder is neither carried back to the pool nor handed to any specific player.
- **Suggested fix:** Either leave `GoldTotal % len(pcs)` in the pool for the DM to dispense manually, or distribute the remainder one extra gp to the first N players.

## [High] Long-rest hit-dice restoration order is non-deterministic for multiclass
- **Location:** internal/rest/rest.go:409-441
- **Spec/Phase ref:** spec §Long Rest line 2609 (Phase 83a)
- **D&D rule:** PHB p.186: "[a long rest] regains spent Hit Dice, up to a number of dice equal to half of the character's total number of Hit Dice (minimum of one die). [...] the character's choice of which dice to spend."
- **Problem:** `LongRest` iterates over `maxHitDice` (a `map[string]int`) to allocate the half-level restoration budget. Map iteration order in Go is randomized, so a Fighter 3 / Wizard 2 with 0 d10 and 1 d6 remaining will sometimes get d6 restored first and sometimes d10 first. The player has no choice in the matter; there is no "preferred die type" input on `LongRestInput`.
- **Suggested fix:** Add a `HitDiceRestorePreference []string` field to `LongRestInput` (ordered die-type list); fall back to a stable order (e.g. largest die first) when unset.

## [High] No combat-resumed long-rest auto-resume
- **Location:** internal/rest/party.go:17-22, internal/rest/party_handler.go:269-308
- **Spec/Phase ref:** spec §Rest interruption line 2635 (Phase 83b)
- **D&D rule:** PHB p.186: a long rest is broken only by "1 hour of walking, fighting, casting spells, or similar adventuring activity"; sub-hour combat doesn't break it.
- **Problem:** Spec line 2635: "Combat lasting 1 hour or less does not break a long rest (5e RAW); the long rest resumes automatically after the encounter ends." `InterruptRest` only branches on `oneHourElapsed` and `restType`. There is no state tracking that a long rest was in progress, no auto-resume hook fired when an encounter completes within 1 hour.
- **Suggested fix:** Persist a `long_rest_started_at` timestamp on the character/campaign; when an encounter ends and the elapsed combat time is < 1 hour, re-fire the long rest application.

## [High] `/check medicine target:AR` does not validate target is dying and does not auto-stabilize
- **Location:** internal/discord/check_handler.go:286-320, internal/check/check.go:111-151
- **Spec/Phase ref:** spec §Death Saves line 2116 (Phase 81)
- **D&D rule:** Medicine (DC 10) stabilizes a creature at 0 HP.
- **Problem:** The check handler's TargetContext only enforces adjacency and action availability — it does not verify the target is at 0 HP (dying), and on a successful Medicine roll it does not flip the target's status to "stable" (zero death-save tallies, set `is_alive` etc.). Stabilization is instead routed through `/action stabilize`, but the spec example explicitly uses `/check medicine AR` as the entry path.
- **Suggested fix:** When `skill == "medicine"` and a target is supplied, branch into the stabilize path: require `target.HpCurrent == 0`, fail the roll path early otherwise, and on success (Total >= 10) persist the stabilization just like `ActionHandler.StabilizeTarget`.

## [High] Items auto-populated from defeated NPCs are not removed from NPC inventory
- **Location:** internal/loot/service.go:67-142
- **Spec/Phase ref:** spec §Inventory Management line 2654 (Phase 85)
- **Problem:** `CreateLootPool` reads each defeated NPC's gold + inventory into the pool but never zeros `char.Gold` or clears `char.Inventory` on the NPC's character row. If `CreateLootPool` is re-invoked (e.g. the DM deletes the pool and re-creates it) or if the same NPC ever participates in another encounter, its loot effectively duplicates.
- **Suggested fix:** Inside the pool-create transaction, also write `UpdateCharacterGold(0)` + `UpdateCharacterInventory([])` for each defeated NPC whose items moved into the pool, or persist a `looted: true` flag to short-circuit subsequent reads.

## [High] Item picker only searches weapons/armor/magic items
- **Location:** internal/itempicker/handler.go:57-156
- **Spec/Phase ref:** spec §Item Picker line 2674 (Phase 86)
- **Problem:** Spec calls out category filters for "weapons, armor, adventuring gear, potions, magic items, etc." `HandleSearch` only iterates `ListWeapons`, `ListArmor`, `ListMagicItems`. Adventuring gear (rope, torches, …) and potions/consumables are not searchable. The DM cannot use the picker to add a Healing Potion or a 50ft rope to a shop or loot pool without falling back to the "custom entry" freeform path, which loses SRD pricing/metadata.
- **Suggested fix:** Add ListAdventuringGear / ListConsumables (or whatever the underlying refdata exposes — `portal.ListEquipment` already aggregates these) and branch on `category == "gear"` / `category == "potions"`.

## [High] No way to edit description / name of an existing loot pool item
- **Location:** internal/loot/service.go (no Update), internal/loot/api_handler.go
- **Spec/Phase ref:** spec §Looting line 2654 (Phase 85)
- **Problem:** Spec says "Any item in the loot pool (including standard SRD items) supports an optional narrative description added by the DM (e.g., a Shortsword described as 'etched with dwarven runes…')". Auto-populated items always have `Description: ""` (service.go:126). The API exposes `AddItem` + `RemoveItem` but no Update endpoint, so the DM has to delete and re-add the item to attach a narrative description — which loses any single-claim history.
- **Suggested fix:** Add a `UpdateLootPoolItem` sqlc query + `PATCH .../loot/items/:itemID` endpoint that accepts name/description/quantity/price overrides.

## [High] Long rest does not propagate dawn recharge to party rest persistence
- **Location:** internal/rest/party_handler.go:180-216
- **Spec/Phase ref:** spec §Long Rest + §Magic Items recharge line 2707 (Phase 83b ↔ 88b)
- **Problem:** `applyPartyLongRest` builds `LongRestInput` without `Inventory` or `RechargeInfo`, so `LongRest`'s dawn-recharge branch (rest.go:467-480) never fires for party rests. Magic item charges therefore only recharge after individual `/rest long` runs through the Discord handler, never after a DM-initiated party long rest.
- **Suggested fix:** Extend `PartyCharacterInfo` to carry `Inventory` + `RechargeInfo`, pass them through, and persist `result.UpdatedInventory` to the character row.

## [High] Encounter-active check on rest can be bypassed for party rest if `HasActiveEncounter` returns false
- **Location:** internal/discord/rest_handler.go:159-164
- **Spec/Phase ref:** spec §Rest Constraints line 2630 (Phase 83a)
- **D&D rule:** "Rests cannot be initiated during active combat."
- **Problem:** Individual `/rest` calls `ActiveEncounterForUser` and rejects if the caller is a combatant. But the rest is still permitted for users not registered as combatants in an active encounter — even though the spec says rest is forbidden during active combat at the campaign level. A bystander could `/rest long` while their party is mid-fight.
- **Suggested fix:** Use `PartyEncounterChecker.HasActiveEncounter` (already used in party-rest path) in the individual handler too so any active encounter in the campaign blocks rests, not just ones the caller is a combatant in.

## [Medium] LongRest reports HPHealed even when no healing occurred
- **Location:** internal/rest/rest.go:377-384
- **Problem:** `result.HPHealed = input.HPMax - input.HPCurrent` is set unconditionally. If the character was already at full HP, HPHealed is 0 (fine), but in any case the value is descriptive, not load-bearing. Not a bug per se, but the `FormatLongRestResult` always shows "HP fully restored" even when no HP was actually missing.
- **Suggested fix:** Skip the "HP fully restored" line in `FormatLongRestResult` when `HPHealed == 0` for consistency with the short-rest formatter.

## [Medium] Gold split distributes to ALL approved players, not just encounter participants
- **Location:** internal/loot/service.go:301
- **Spec/Phase ref:** spec §Inventory Management line 2661 (Phase 85)
- **Problem:** `SplitGold` calls `ListPlayerCharactersByCampaignApproved(pool.CampaignID)`, which returns every approved PC in the campaign — including absent players who weren't in the encounter. Spec doesn't dictate either policy, but PHB convention is "split among the party that earned it".
- **Suggested fix:** Either pass an explicit recipient list from the dashboard (DM checks who shares) or scope to combatants from the originating encounter.

## [Medium] Long rest does not zero death-save tallies when both are zero
- **Location:** internal/rest/rest.go:444-446
- **Spec/Phase ref:** spec line 2610: "Death save tallies reset to 0/0"
- **Problem:** `result.DeathSavesReset = true` only when prior tallies were nonzero. If the character had 0/0 going in, the result reports `DeathSavesReset = false`, but the spec says rest always resets to 0/0. More importantly, the result struct does not carry the updated counts at all — callers persist or pass through their own values. This is a contract gap: a caller that ignored the result would leave tallies untouched.
- **Suggested fix:** Always emit `DeathSaveSuccessesAfter: 0`, `DeathSaveFailuresAfter: 0` on the result and have callers persist them unconditionally.

## [Medium] Item Picker custom-entry endpoint accepts negative gold/quantity silently
- **Location:** internal/itempicker/handler.go:208-239
- **Spec/Phase ref:** spec §Item Picker line 2677 (Phase 86)
- **Problem:** `HandleCustomEntry` only normalises `qty <= 0` to 1, not `GoldGP` or `PriceGP`. A negative `gold_gp` or `price_gp` is accepted and propagated, which can lead to a shop with negative price or a loot drop that subtracts gold when picked up.
- **Suggested fix:** Reject `gold_gp < 0` and `price_gp < 0` with 400, or clamp to 0.

## [Medium] CastIdentify accepts ritual without 10-minute delay enforcement
- **Location:** internal/inventory/identification.go:81-110
- **Spec/Phase ref:** spec §Identifying magic items line 2731 (Phase 88c)
- **D&D rule:** Identify (PHB p.252): "minute" cast; ritual variant adds 10 minutes.
- **Problem:** The ritual branch skips the spell-slot consumption entirely but performs the identification synchronously. There is no narrative-time gate or delay marker, which is fine if the spec is "DM enforces narrative time", but the function silently lets a player ritual-cast Identify during a combat round at zero cost.
- **Suggested fix:** When `IsRitual` is true, require an `OutOfCombat` precondition (no active encounter for the caster) — matching the `/rest` combat-block convention.

## [Medium] CastIdentify silently allows identifying items that aren't magic
- **Location:** internal/inventory/identification.go:24-44
- **Problem:** `identifyUnidentifiedItem` validates `IsMagic` correctly, but the early returns on "already identified" / "not magic" produce errors with raw item names. Acceptable, but the wrapping `CastIdentify` doesn't check whether the character actually has Identify on their spell list / known spells — only `KnowsSpell` (a caller-provided bool). A future caller that forgets to populate `KnowsSpell` could let any character cast it.
- **Suggested fix:** Defensive: surface `KnowsSpell == false` as a typed error and add a Discord-handler-side check that verifies the spell appears in the character's `prepared_spells` / `known_spells` JSONB.

## [Medium] Combat recap truncation cuts mid-line and may produce orphan round headers
- **Location:** internal/combat/recap.go:71-78, 93-116
- **Spec/Phase ref:** spec §Combat Recap line 2065 (Phase 80)
- **Problem:** `TruncateRecap` slices the string at `maxLen - len(suffix)` which can leave the message ending inside a UTF-8 multibyte sequence (player/creature names containing emoji or non-ASCII) or in the middle of a round header. The recap will render with a broken last entry.
- **Suggested fix:** Truncate on the last newline before the cutoff, and use `utf8.RuneStart` (or `strings.LastIndex(msg[:cutoff], "\n")`) to align on a safe boundary.

## [Medium] PartyShortRest never auto-spends hit dice (always spends 0)
- **Location:** internal/rest/party_handler.go:218-260
- **Spec/Phase ref:** spec §DM-Initiated Party Rest lines 2614-2624 (Phase 83b)
- **Problem:** Spec line 2621: "For **short rests**, each included player is prompted individually in `#your-turn` to spend hit dice (same button menu as the player-initiated flow). Players who don't respond within 10 minutes default to spending 0." The current handler calls `applyCharShortRest` with `HitDiceSpend: {}` and only sends a "use your hit dice buttons" notification — there is no actual Discord button interaction posted, no per-player ephemeral hit-dice prompt, and no 10-minute timeout machinery.
- **Suggested fix:** Reuse `RestHandler.handleShortRest`'s `BuildHitDiceButtons` flow per included player, with a 10-minute auto-skip timer.

## [Medium] Loot pool created from "completed" encounter — combatants gold lost if encounter status mismatch
- **Location:** internal/loot/service.go:67-118
- **Problem:** Only NPCs flagged `IsAlive == false` contribute loot. If an encounter is marked `completed` but contains creatures whose `IsAlive` is still true (e.g. fled, surrendered) — they're skipped silently. Spec line 2654 says "all items and gold from **defeated creatures**", which is fine — but the encounter-completion event flow doesn't appear to clearly distinguish "killed", "fled", "surrendered". A surrendered NPC's gold is unreachable to the DM through this auto-populate path.
- **Suggested fix:** Allow the DM to explicitly mark which combatants count as defeated (a `defeated` boolean or a `loot_eligible` flag) so surrender/flee scenarios don't silently lose their inventory contribution.

## [Medium] Loot pool item ItemID null when claimed from custom entries breaks downstream `/use`
- **Location:** internal/loot/service.go:243-256
- **Problem:** `ClaimItem` reads `claimed.ItemID.String` (sql.NullString → empty string when null) and constructs an `InventoryItem` with `ItemID: ""`. Players who later `/use` or `/equip` reference items by ID — an empty ID means `findItemIndex` matches the FIRST item with empty ID, which could be a different custom drop.
- **Suggested fix:** Generate a stable synthetic ID (e.g. `custom-<uuid>`) for items missing an SRD `item_id` before persisting into the character inventory.

## [Medium] `Equip` blocks re-equipping the same item but silently allows two main-hand items via `OffHand=false`
- **Location:** internal/inventory/equip.go:27-62
- **Problem:** `Equip` only checks `if item.Equipped { error }`. It doesn't check whether ANOTHER item is already in the requested slot. So a player can `/equip longsword` then `/equip greataxe`, ending up with both items flagged Equipped/main_hand in the inventory JSON. Combat resolution probably picks the "first match" — non-deterministic.
- **Suggested fix:** Before flipping `updated[idx].Equipped = true`, walk the slice and unequip any other item already in the target slot.

## [Medium] LongRest doesn't recharge `recharge: "dawn"` features distinct from `"long"`
- **Location:** internal/rest/rest.go:401-407
- **Problem:** `LongRest` resets features whose `Recharge` is "short" or "long". Wand-of-fireballs / many magic items use `"dawn"` (spec line 2705 `recharge: "dawn"`). Dawn-recharge magic items go through the `DawnRecharge` path correctly, but any class/race feature declared with `recharge: "dawn"` (or `"daily"`) is never reset by long rest.
- **Suggested fix:** Treat `"dawn"` and `"daily"` as additional recharge keywords during long rest, or document explicitly that "dawn" is reserved for magic items only.

## [Medium] LongRest never zeroes the input.PactMagicSlots when it does mutate them
- **Location:** internal/rest/rest.go:392-398
- **Problem:** `LongRest` mutates `input.PactMagicSlots.Current` via pointer but does not surface the new value on the result (LongRestResult only has `PactSlotsRestored bool`). Callers that read from the input pointer get the update; callers using only the result struct lose the new count. Inconsistent with `ShortRestResult.PactSlotsCurrent`.
- **Suggested fix:** Add `PactSlotsCurrent int` to `LongRestResult` and set it from `input.PactMagicSlots.Current`.

## [Medium] `/rest` doesn't enforce one-long-rest-per-24h even narratively (no warning to DM)
- **Location:** internal/discord/rest_handler.go (no 24h check)
- **Spec/Phase ref:** spec line 2629 (Phase 83a)
- **Problem:** Spec says "system does not enforce calendar" — but a defensive warning to the DM via `#dm-queue` when a player rests twice in <24 in-game hours would help. Currently nothing is logged/warned; spec acknowledges this as a deliberate design, so this is informational only.
- **Suggested fix:** Optional — pass a `last_long_rest_at` field on the character and surface a warning hint when `/rest long` fires within <24h of the last one.

## [Medium] ShortRest hit-die roll: minimum healing of 0 vs spec's "minimum 1 per HD" framing
- **Location:** internal/rest/rest.go:200-204
- **Spec/Phase ref:** Review task asks "CON mod min 1 per HD"; PHB p.186 says minimum 0.
- **D&D rule:** RAW (PHB p.186): "minimum of 0". House rule "minimum 1 per HD" is common but not RAW.
- **Problem:** Code clamps to `healing >= 0`, matching RAW. If the reviewer's "min 1 per HD" line is meant to be normative for DnDnD, the code is non-conforming. If it's a paraphrase of RAW, the code is correct.
- **Suggested fix:** Confirm the intended policy. If "min 1 per HD" is house-ruled in spec, change `if healing < 0` to `if healing < 1` in rest.go:200.

## [Medium] Item picker custom entry returns `Homebrew: true` but doesn't persist anywhere
- **Location:** internal/itempicker/handler.go:227-238
- **Problem:** `HandleCustomEntry` mints a `custom-<uuid>` ID and returns a CustomEntryResponse, but nothing is stored server-side. If the same item is later referenced from a shop or loot pool via that ID, no row exists to look it up — the picker treats it as ephemeral.
- **Suggested fix:** Either persist into a `homebrew_items` table (campaign-scoped) or document the ephemeral nature so downstream consumers know to copy the description+price into their own row.

## [Low] Save handler combines roll modes via two `CombineRollModes` calls — order-sensitive
- **Location:** internal/save/save.go:90-92
- **Problem:** `finalMode = CombineRollModes(CombineRollModes(input.RollMode, condMode), featureMode)`. `CombineRollModes` cancels advantage/disadvantage symmetrically (standard 5e), so order is mathematically irrelevant. But if a future effect adds a "force advantage" or "force disadvantage" override, the nesting could matter. Defensive nit.
- **Suggested fix:** Add a variadic `CombineRollModes(modes ...RollMode)` or document the precedence rules.

## [Low] Group-check success threshold rounds in 5e's favor only for even counts
- **Location:** internal/check/check.go:282-306
- **Spec/Phase ref:** spec line 2583 (Phase 81)
- **D&D rule:** PHB p.175: "If at least half the group succeeds, the whole group succeeds." For an odd-sized group (5 players), 3 must succeed; the code uses `result.Passed*2 >= len(participants)`, i.e. for 5 players passed*2 >= 5, so passed >= 2.5, so passed >= 3. Correct.
- **Problem:** None — the test passes. Listed only because the integer arithmetic is non-obvious.
- **Suggested fix:** N/A.

## [Low] Contested check is "status quo" on tie — initiator's choice not surfaced
- **Location:** internal/check/check.go:332-352
- **Spec/Phase ref:** spec §Contested checks line 2585
- **D&D rule:** PHB p.174: "In the case of a tie, the situation remains the same as it was before the contest." 5e RAW is "no winner, no change". The code returns `Tie: true` and no winner. Display says "Result: Tie (status quo maintained)".
- **Problem:** Correct, just confirming.

## [Low] Equip slot warning text references `/attune cloak-of-protection` even for items already attuned to a different name
- **Location:** internal/inventory/equip.go:51-54
- **Problem:** Warning interpolates `item.Name`, not `item.ItemID`, so `/attune Cloak of Protection` (with the displayed name including spaces) is printed. Players typing literal `/attune Cloak of Protection` will fail (the slash command expects an item-id token).
- **Suggested fix:** Use `item.ItemID` in the warning string and add `(ID: %s)` for clarity.

## [Low] FormatLootAnnouncement doesn't include item descriptions
- **Location:** internal/loot/service.go:357-380
- **Spec/Phase ref:** spec line 2657 sample format
- **Problem:** Spec shows the Discord loot message as a flat name list ("Shortsword ×2, 15 gp, Healing Potion ×1, Mysterious Key") — implementation matches. But narrative descriptions added by the DM via `AddItem` are not surfaced in the announcement at all; players only see them after running `/loot` (if even then).
- **Suggested fix:** Optionally include description as a parenthetical or italicized suffix per item in the announcement.

## [Low] `/check` group-check participant modifier accepts raw int — no validation
- **Location:** internal/check/check.go:254-306
- **Problem:** `GroupParticipant.Modifier` is whatever the caller supplies. No bounds check (-20…+20 is the practical range for level 1-20). A malformed call can produce nonsensical totals.
- **Suggested fix:** Optional defensive clamp.

## [Low] `Attune` error string mixes emoji + format verb prefix
- **Location:** internal/inventory/attunement.go:35
- **Problem:** Error is `fmt.Errorf("❌ You already have %d attuned items. Use `/unattune [item]` to free a slot.", maxAttunementSlots)`. Mixing emoji and the slash-command hint inside an error message means callers that `log.Printf("%v", err)` produce log lines with emojis. Cosmetic.
- **Suggested fix:** Surface the user-facing string in the handler, return a plain `errors.New("max attunement slots reached")` from the package.

## [Low] Inventory `IsPotion` only knows about healing-potion / greater-healing-potion
- **Location:** internal/inventory/service.go:108-114
- **Problem:** Used to decide bonus-action vs action cost for potions, but Antitoxin is also a potion in the SRD and would still incur an action even when `potion_bonus_action: true`. Spec line 2650 implies the setting applies to "drinking a potion", not just healing potions.
- **Suggested fix:** Either widen `IsPotion` to recognize the consumable type tag, or rename it to `IsHealingPotion`.

## [Low] SplitGold zeros pool even if no players found (defensive check exists but bug-prone)
- **Location:** internal/loot/service.go:301-329
- **Problem:** If `len(pcs) == 0` the function returns early with an error before zeroing gold (correct). If `len(pcs) > 0` but every `UpdateCharacterGold` call fails, the gold is still zeroed at the end. The order is: distribute → zero. If distribution partially fails (one UpdateCharacterGold errors mid-loop), the function returns early without zeroing — but the players who already got their share keep it. Partial distribution + partial pool retention is acceptable for now but should be transactional.
- **Suggested fix:** Wrap in a single sqlc transaction.

## [Low] Shop announcement doesn't show stock counts when present
- **Location:** internal/shops/service.go:131-155
- **Problem:** `FormatShopAnnouncement` shows "Name — Ngp _desc_" but does not include quantity (`item.Quantity`). For finite-stock shops this is a gap.
- **Suggested fix:** Optionally render `Quantity ×N` when `item.Quantity > 1`.

## [Low] Combat recap doesn't tag "[Round X, Turn Y]" inside lines
- **Location:** internal/combat/recap.go:93-116
- **Problem:** Spec line 2065 says "Entries are grouped by round and turn for readability". Code groups by round only via `── Round N ──` headers; within a round, all entries are listed sequentially without a turn divider. The sample output in the spec (lines 2070-2079) does flow turn-by-turn naturally because each log line names the actor, so this is acceptable but the explicit "turn" grouping is absent.
- **Suggested fix:** Optionally insert a blank line between turn changes (detect by adjacent log lines with different combatant_id).

---

## Phase status
- Phase 80 (Combat Recap): OK — minor formatting / UTF-8 truncation issue
- Phase 81 (/check): OK — Medicine stabilize gap, otherwise correct
- Phase 82 (/save): OK
- Phase 83a (Short & Long Rest individual): OK — HD restoration order + dawn-vs-long recharge nits
- Phase 83b (Party Rest & Interruption): OK — auto-resume after sub-hour combat missing; short-rest hit-dice prompt loop incomplete
- Phase 84 (Inventory): OK — antitoxin advantage tracking unwired
- Phase 85 (Looting): OK — gold-split remainder discarded; NPC inventory not cleared
- Phase 86 (Item Picker): OK — only weapons/armor/magic items searched
- Phase 87 (Shops & Merchants): OK — by-design "DM transfers manually"
- Phase 88a (Magic Items passive): OK — spec vocabulary drift (`modify_save` vs `modify_saving_throw`)
- Phase 88b (Magic Items active + attune): OK — `/attune` doesn't require short rest; destroy_on_zero timing wrong
- Phase 88c (Magic Items identification): OK
