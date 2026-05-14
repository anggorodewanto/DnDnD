# Batch 11: Checks, rests, inventory, items (Phases 81–88c)

## Summary

The check, save, rest, inventory, loot, item-picker, shops, and magic-item
packages are all implemented as discrete services with Discord handlers
plumbed in `cmd/dndnd/main.go`. Coverage is broad and the phase-acceptance
tests exist. The most important cross-cutting gap is that magic-item
FeatureDefinitions (`magicitem.CollectItemFeatures`) are never appended in
the production attack / save / turn-start pipelines — only in tests — so
Phase 88a's "+1 weapon auto-applies to attack/damage", Cloak of Protection's
`modify_save`, and Boots of Speed's `modify_speed` do not actually fire in
combat. Two smaller divergences: (a) Paladin's Aura of Protection is not
mapped from class data into a FeatureDefinition, so /save tests pass synthetic
inputs but a real paladin gets no aura; (b) Long rest does not reduce
exhaustion (-1) — spec text is silent on this, so this is a 5e-RAW gap
rather than a strict spec divergence. F-9 (publisher in /attune) and the
parsing side of F-14 (`modify_speed` → `EffectModifySpeed`) are both correctly
wired.

## Per-phase findings

### Phase 81 — Skill & Ability Checks (`/check`)

- Status: Matches (with one wiring gap on passive checks)
- Key files: `internal/check/check.go`, `internal/check/format.go`,
  `internal/discord/check_handler.go`,
  `internal/dashboard/check_handler.go` (group), `internal/discord/check_handler.go:325` (contested)
- Findings:
  - `SingleCheck` covers ability + proficiency, expertise (double proficiency),
    Jack of All Trades, and exhaustion / condition auto-fail + adv/disadv via
    `combat.CheckAbilityCheckWithExhaustion`. (check.go:111-151)
  - F-15 target-context enforcement (Chebyshev adjacency + InCombat
    action-available ordering) is implemented and unit tested.
    (check.go:159-174)
  - `GroupCheck` (half-must-succeed, 5e RAW) and `ContestedCheck` exist and
    are wired (group via dashboard, contested via /check with a target ID
    falling back through `CheckOpponentResolver`).
  - `PassiveCheck` (10 + mod) exists at the service layer
    (check.go:233-251) but no production caller invokes it — the
    character-card render, Stealth-vs-passive-Perception detection, and
    /check itself never call it. Spec line 2580 says passive Perception
    should be "displayed on character card." Partial.
  - Stealth-armor-disadv + Medium Armor Master are correctly applied
    (check_handler.go:602+).

### Phase 82 — Saving Throws (`/save`)

- Status: Partial — service-layer modifiers work, but Paladin Aura of
  Protection and magic-item `modify_save` are not reachable from the live
  handler.
- Key files: `internal/save/save.go`, `internal/save/format.go`,
  `internal/discord/save_handler.go`
- Findings:
  - `Save` correctly composes ability + proficiency, exhaustion + condition
    effects (incl. auto-fail STR/DEX while paralyzed/stunned/unconscious/
    petrified, and dodge adv-on-DEX), and FeatureEffects via
    `combat.ProcessEffects`. Breakdown is rendered in
    `FormatSaveResult` (save/format.go).
  - `buildSaveFeatureEffects` (save_handler.go:269-282) builds
    FeatureDefinitions from the character's `classes` + `features` columns
    only. It does NOT append `magicitem.CollectItemFeatures(items,
    attunement)`, so a Cloak of Protection or Ring of Protection +1 will
    not surface its `modify_save` on /save. Divergent vs spec lines 2698
    (Cloak of Protection example) and the Phase 82 acceptance criterion
    "magic item bonuses" auto-included.
  - No FeatureDefinition is registered for Paladin **Aura of Protection** —
    grepping `seed_feats.go` / `feature_integration.go` shows no
    `aura_of_protection` mechanical effect. The /save tests construct an
    Aura effect directly into the service input
    (`save_test.go:182-206`), so unit tests pass but a real paladin
    character cannot get an aura via /save in production. Divergent.

### Phase 83a — Short & Long Rests (individual)

- Status: Matches
- Key files: `internal/rest/rest.go`, `internal/rest/format.go`,
  `internal/discord/rest_handler.go`
- Findings:
  - `ShortRest` spends hit dice (single- and multi-class via
    `HitDiceSpend[dieType]int`), recharges short-rest features, restores
    Pact Magic slots, and supports "study item during short rest" (Phase
    88c hook). (rest.go:116-207)
  - `LongRest` restores HP to max, all spell slots, all short/long
    features, half-total-level hit dice distributed proportionally across
    die types, resets death saves, and emits a prepared-caster reminder
    for Cleric/Druid/Paladin. (rest.go:257-352)
  - Long rest does NOT reduce exhaustion by 1 (5e PHB p.186 rule). Spec
    line 2587 onwards is silent on exhaustion in rest, so not strictly a
    spec divergence, but it is a 5e-RAW gap worth flagging.
  - Phase 88b dawn recharge for charged magic items is correctly integrated
    into LongRest (rest.go:336-349) via `inventory.DawnRecharge`. DestroyOnZero
    handled.

### Phase 83b — Party Rest & Interruption

- Status: Matches
- Key files: `internal/rest/party.go`, `internal/rest/party_handler.go`
- Findings:
  - `HandlePartyRest` partitions selected vs excluded characters, blocks
    rest during active combat, applies short/long to each rested character,
    and posts a single `FormatPartyRestSummary` to #roll-history.
    (party_handler.go:117-174)
  - `InterruptRest` returns `Benefits: "short"` only when restType=="long"
    AND `oneHourElapsed==true`, matching spec lines 2631-2636's 1-hour
    threshold rule. Per-character notification via
    `FormatInterruptNotification`. (party.go:17-22, party_handler.go:260+)
  - Short-rest party flow notifies each player to "use your hit dice
    buttons", but the per-player hit-dice button menu itself lives in the
    individual rest handler — slight UX seam.

### Phase 84 — Inventory Management

- Status: Matches (combat-time cost path correctly deferred per phase note)
- Key files: `internal/inventory/service.go`, `internal/inventory/equip.go`,
  `internal/inventory/api_handler.go`, `internal/discord/inventory_handler.go`,
  `internal/discord/use_handler.go`, `internal/discord/give_handler.go`
- Findings:
  - `/inventory` formats by category, shows attunement (✨), charges,
    equipped+slot tags, gp total (service.go:212-300). Unidentified items
    show "Unidentified [type]" per spec line 2731.
  - `/use`: healing-potion / greater-healing-potion auto-resolve via dice
    roll; antitoxin handled with hint; everything else routes to DM-queue.
    (service.go:117-169). Matches spec lines 2644-2648.
  - `/give` validates inventory presence + (in combat) free-object
    interaction resource via the turn provider (give_handler.go:115-125,
    175-177). Adjacency is intentionally deferred per phases.md note on
    line 485. Per-spec line 2652 ("within 5ft") is therefore a phase-doc
    deferral, not a spec divergence.
  - Gold-only currency (spec line 2663 explicitly converts cp/sp/ep/pp →
    gp). No weight/encumbrance enforcement (spec line 2665 says "not
    enforced").

### Phase 85 — Looting System

- Status: Matches
- Key files: `internal/loot/service.go`, `internal/loot/api_handler.go`,
  `internal/discord/loot_handler.go`
- Findings:
  - `CreateLootPool` auto-populates from defeated NPC combatants (NPC +
    !IsAlive + CharacterID), summing gold and items.
    (service.go:67-142). Encounter must be `completed`.
  - `ListEligibleEncounters` filters by completed for the DM dropdown
    (service.go:157-179).
  - Single-claim enforced by `ClaimLootPoolItem` SQL update (only succeeds
    when `claimed_by IS NULL`); claim then writes item into the
    character's inventory via `AddItemQuantity` and persists. Error
    `ErrItemAlreadyClaimed` returned on duplicate.
  - `SplitGold` divides evenly across approved party members and zeroes
    pool gold; "Split Gold" button surface exists in API
    (`HandleSplitGold`).
  - `ClearPool` deletes unclaimed items and sets `status=closed`.
  - Items support optional Description (narrative) field on the
    `AddItemRequest`. Auto-populate sets it empty so DM can edit.

### Phase 86 — Item Picker (Dashboard Component)

- Status: Matches
- Key files: `internal/itempicker/handler.go`, `internal/itempicker/routes.go`
- Findings:
  - Search across weapons + armor + magic items with optional `category`
    and `q` query params; F-86 added `homebrew=true|false` filter and a
    `homebrew` bool on every result (handler.go:54-147).
  - `HandleCustomEntry` (POST .../items/custom) accepts freeform name +
    description + quantity + gold_gp / price_gp + type, returns a
    generated `custom-<uuid>` ID. Sets `Custom=true` and `Homebrew=true`.
  - `HandleCreatureInventories` returns gold + items per defeated NPC for
    the "creature inventory source" tab (spec line 2675).
  - The dashboard-side Svelte component isn't audited here but the API
    surface matches the spec bullet list (search, category filter,
    creature source, narrative description support via downstream
    consumers, custom entry, price override via the `PriceGP` field).

### Phase 87 — Shops & Merchants

- Status: Matches
- Key files: `internal/shops/service.go`, `internal/shops/handler.go`,
  `internal/shops/routes.go`
- Findings:
  - `CreateShop`/`UpdateShop`/`DeleteShop` persist named shop templates per
    campaign (service.go:52-104). Items are CRUD'd via `CreateShopItem`,
    `UpdateShopItem`, `DeleteShopItem` (with cascade delete on shop
    removal via `DeleteShopItemsByShop`).
  - `FormatShopAnnouncement` produces the "Post to #the-story" formatted
    list with name, price, description (service.go:131-155).
  - No buy/sell flow, haggle, or refresh cadence — spec line 2682-2684
    explicitly says "the DM manually transfers items and deducts gold,"
    so absence of those is by design.

### Phase 88a — Magic Items: Tracking, Bonuses & Passive Effects

- Status: Partial — pure-logic helpers correct, but production wiring into
  attack/save/turn-start pipelines is missing.
- Key files: `internal/magicitem/effects.go`,
  `internal/combat/feature_integration.go` (BuildFeatureDefinitions),
  `internal/combat/attack.go:1584`, `internal/combat/turnresources.go:262`,
  `internal/discord/save_handler.go:281`
- Findings:
  - `ItemFeatures` correctly converts equipped magic weapons/armor to
    `EffectModifyAttackRoll`/`EffectModifyDamageRoll`/`EffectModifyAC`.
    `CollectItemFeatures` walks inventory + attunement, prefers parsed
    `MagicProperties` JSON, falls back to `MagicBonus`. F-14:
    `convertPassiveEffect` handles `modify_speed` →
    `combat.EffectModifySpeed` with `TriggerOnTurnStart`. ✅
  - Critical divergence: `BuildFeatureDefinitions` accepts variadic
    `extraDefs` (combat/feature_integration.go:264) but every production
    caller passes ZERO extras:
    - `combat/attack.go:1584` — attack pipeline
    - `combat/turnresources.go:262` — turn-start speed
    - `discord/save_handler.go:281` — /save FES
    No production call to `magicitem.CollectItemFeatures(items,
    attunement)` exists outside of `magicitem/integration_test.go`. Result:
    a real +1 longsword adds nothing to attack or damage; a Cloak of
    Protection adds nothing to AC or saves; Boots of Speed do not change
    movement. The unit tests in `magicitem/integration_test.go` exercise
    the helper but no production path stitches the result into the combat
    code. This breaks the Phase 88a acceptance criterion ("Integration
    tests verify magic bonus application to attacks/damage/AC, passive
    effect registration").
  - `/inventory` display shows rarity + attunement correctly (Phase 88a's
    other deliverable). ✅

### Phase 88b — Magic Items: Active Abilities, Attunement & Identification

- Status: Matches
- Key files: `internal/inventory/active_ability.go`,
  `internal/inventory/attunement.go`, `internal/inventory/recharge.go`,
  `internal/discord/attune_handler.go`,
  `internal/discord/unattune_handler.go`
- Findings:
  - `UseCharges` validates item is magic, has charges, attunement met (when
    required), and amount ≤ current charges. (active_ability.go:28-62)
  - `Attune` enforces ≤3 slots, prevents double-attune, validates class
    restriction via `meetsClassRestriction`. Spec's 3-slot cap is
    enforced. (attunement.go:33-67)
  - `Unattune` removes a slot immediately (no rest cost), matching spec
    line 2713.
  - `DawnRecharge` rolls each item's recharge dice, caps at
    `MaxCharges`, and handles `DestroyOnZero`: when charges hit 0 and
    DestroyOnZero=true, rolls 1d20 and destroys on 1. (recharge.go)
  - F-9 publisher wired: `AttuneHandler.SetPublisher` accepts an
    `AttunePublisher`; `Handle` calls `h.publisher.PublishForCharacter`
    AFTER persisting the slot write
    (attune_handler.go:142-146). `main.go:1109` injects
    `magicItemSvc` (which uses the dashboard publisher + encounter
    lookup) into the handler. ✅

### Phase 88c — Magic Items: Identification Flow

- Status: Matches
- Key files: `internal/inventory/identification.go`,
  `internal/discord/cast_handler.go` (identify + detect-magic short-circuit)
- Findings:
  - `IdentifyItem` flips `identified=true` after validating the target is
    magic and currently unidentified.
  - `CastIdentify` validates `KnowsSpell`, requires a spell slot at
    `SlotLevel` unless `IsRitual`, then flips identified.
  - `StudyItemDuringRest` is invoked from `ShortRest` when
    `StudyItemID != ""` (rest.go:190-200).
  - `DetectMagicItems` returns the magic items in a list for the cast
    detect-magic surface. `/cast detect-magic` dispatch also scans nearby
    combatants via `NearbyScanner` and a 30-ft radius
    (cast_handler.go:121, 589-640).
  - `SetItemIdentified` lets the DM toggle `identified` from the dashboard
    (matches spec line 2731 "DM can set `identified: false`").

## Cross-cutting concerns

1. **Magic items never reach the combat / save / turn pipelines**
   (Phase 88a critical). `magicitem.CollectItemFeatures` is correct, but
   every production caller of `BuildFeatureDefinitions` passes zero extras.
   Fix: in `combat/attack.go`, `combat/turnresources.go`, and
   `discord/save_handler.go` build the FES definitions as
   `BuildFeatureDefinitions(classes, feats,
   magicitem.CollectItemFeatures(items, attunement))`. The character's
   inventory + attunement columns already flow through the same path that
   feeds `feats`.

2. **Paladin Aura of Protection isn't a class feature.** Spec line 2587-onward
   plus phase 82 require auto-include of Aura of Protection. Today the save
   tests inject a synthetic effect; the FES has no entry for it. Add an
   `aura_of_protection` mechanical effect (similar to existing rage /
   sneak_attack registrations in `BuildFeatureDefinitions`) keyed off
   Paladin level (CHA mod within 10ft, lvl 6+).

3. **PassiveCheck never invoked.** `check.PassiveCheck` is unit-tested but
   no character-card or DM-side flow calls it. Spec line 2580 expects
   passive Perception "displayed on character card." Hook this into the
   character-card render or into a passive-perception comparison when the
   DM hides creatures.

4. **Long rest does not reduce exhaustion (-1).** 5e RAW behavior, missing
   from `LongRest` in rest.go. Spec text is silent so technically not a
   spec violation, but the F-9 / phases-md style review item flagged it.

5. **Phase 84 /give adjacency intentionally deferred** per phases.md line
   485 — combat-time `/give` adjacency check + action cost are scheduled
   for a future combat-items phase; this is documented, not a divergence.

## Critical items

- **C-88a-MAGIC-ITEMS-NOT-WIRED**: `BuildFeatureDefinitions` consumers
  (combat/attack.go:1584, combat/turnresources.go:262,
  discord/save_handler.go:281) never receive
  `magicitem.CollectItemFeatures(items, attunement)`. A +1 longsword adds 0
  to attack/damage in real combat. This violates Phase 88a's "Done when"
  clause. Fix is a 3-line plumbing change per caller (parse inventory +
  attunement columns and pass as the third arg).

- **C-82-NO-AURA-OF-PROTECTION-FEATURE**: Paladin's Aura of Protection is
  not registered as a FeatureDefinition; only the synthetic test path
  exercises it. Add a `BuildFeatureDefinitions` branch for an
  `aura_of_protection` mechanical-effect ID (with a 10-ft
  `AllyWithin`-style condition) and seed it on the paladin class.
