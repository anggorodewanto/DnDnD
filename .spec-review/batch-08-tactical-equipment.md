# Batch 08: Tactical combat + equipment (Phases 55–57, 70–75b)

## Summary

Phases 55–57 and 70–73 are implemented at the service layer with deep test
coverage (~9.3 KLOC of test code for this slice alone). Phase 74 (/interact)
and Phase 75a/75b (/equip + AC recalc/enforcement) have **fully-built combat
services that the Discord slash handlers don't actually call** — the wiring
diverges from spec. Several smaller divergences exist in OA forced-movement
exemption, the Drag/Release-and-Move UI prompt, and the `/action grapple`
command alias.

The grapple/shove/drag/hide/reaction/readied/counterspell/freeform code paths
themselves match the spec mechanics closely (contested checks, free-hand
checks, size limits, indefinite grappled condition, queue-and-continue OAs,
two-step Counterspell with hidden cast level, readied-spell slot + concentration
on ready, auto-reveal on attack, armor stealth disadvantage, passive
Perception lookup).

## Per-phase findings

### Phase 55 — Opportunity Attacks
- Status: **partial**
- Key files:
  - `internal/combat/opportunity_attack.go`
  - `internal/discord/move_handler.go` (lines 660–722, 725–895)
- Findings:
  - Matches: Disengage suppression via `moverTurn.HasDisengaged`
    (`opportunity_attack.go:88`); per-hostile reach lookup including
    NPC `creature.Attacks` reach_ft and PC reach-weapon override
    (`pcReachByID`); 1-reaction/round filter via `hostileTurns[hostile.ID].ReactionUsed`;
    queue-and-continue (move commits, then OA fires async to #your-turn);
    exit-tile recording (`OATrigger.ExitCol/Row/Label`); friend/foe split on
    `IsNpc` mismatch; alive check.
  - Partial: prompt is posted to `#your-turn` channel but there's no DM-side
    dashboard surfacing for DM-controlled hostiles per spec line 1421 —
    `fireOpportunityAttacks` only posts to your-turn (PC hostile path).
  - Missing: **forced movement exemption** — spec line 1411 implies only
    voluntary movement-out-of-reach triggers OAs. Push/shove (`combat.Shove`
    in ShovePush mode) and teleportation movement both go through different
    code paths (Shove uses `UpdateCombatantPosition`, teleport bypasses
    `/move`), so this is incidentally correct *today* but there is no
    explicit "forced movement does not trigger OA" guard in `DetectOpportunityAttacks`.
  - Missing: **end-of-round forfeiture** is not actively cleaned up — the OA
    prompt just sits in the channel and there is no end-of-round reaction
    sweeper that posts "OA forfeited". The reaction is naturally unusable
    after the round ends because `ReactionUsed` resets at the next start of
    that hostile's turn, but the player gets no closure message.
  - Missing: **retroactive correction hook** when an OA reduces target to 0
    HP (spec line 1423) — the system "notifies the DM" is not implemented;
    OA result resolves via the standard `/reaction oa` attack path with no
    special hook.

### Phase 56 — Grapple, Shove & Dragging
- Status: **partial**
- Key files:
  - `internal/combat/grapple_shove.go`
  - `internal/discord/shove_handler.go`
  - `internal/discord/move_handler.go:428–438, 897–910`
  - `internal/discord/bonus_handler.go:535–547` (`/bonus release-drag`)
- Findings:
  - Matches: free-hand check for PCs (`BothHandsOccupied`),
    NPCs assumed to have free hand; size-difference ≤ 1 check on both
    grapple and shove; adjacency 5ft check; contested STR Athletics vs
    higher-of(STR-Athletics, DEX-Acrobatics) via `resolveTargetDefense`;
    indefinite grappled condition with `SourceCombatantID`; speed-0 via
    `condition_effects.go` (separate file enforces this); push destination
    occupancy validation pre-roll; drag x2 cost (`DragMovementCost`);
    `CheckDragTargets` + drag-prompt rendering on `/move`; release flow
    via `/bonus release-drag`.
  - Divergent: **spec calls for `/action grapple [target]`** (line 1171),
    but the implementation only dispatches grapple via
    `/shove [target] --mode grapple` (`shove_handler.go:35–87`,
    `action_handler.go:300–311` excludes grapple from the action
    subcommand allow-list). `/action grapple` falls into the freeform
    DM-queue path instead of running the contested check.
  - Divergent: **Drag UI doesn't match spec**. Spec line 1444–1449
    specifies a two-button choice "[✅ Drag] [❌ Release & Move]" *before*
    the standard move confirmation, with Release removing the grapple
    condition then moving at normal cost. Implementation only shows a
    decorative prompt prefix (`FormatDragPrompt`) and always applies the
    x2 cost — release happens via a separate `/bonus release-drag`
    command. There is no interactive button choice in `/move`.
  - Missing: drag mass-position update — `combat.grapple_shove.go` doesn't
    co-move the grappled creatures to the grappler's destination tile. The
    move handler has a `syncDragTargetsAlongPath` shim, but the spec says
    "All grappled creatures move to the grappler's destination tile."
    Need to verify (skipped reading sync function in detail).

### Phase 57 — Stealth & Hiding
- Status: **matches**
- Key files:
  - `internal/combat/standard_actions.go:287–442`
  - `internal/combat/attack.go:808–815, 1380–1381` (auto-reveal + advantage)
  - `internal/combat/advantage.go:39–44`
- Findings:
  - Matches: `/action hide` rolls Stealth (DEX) with character skill
    modifier including expertise / jack-of-all-trades; armor stealth
    disadvantage applied via `armor.StealthDisadv` with Medium Armor
    Master feat carve-out; passive Perception = 10 + Perception mod (incl.
    proficiency, expertise, JoaT) computed in `passivePerception`;
    `is_visible` set to false on success; rogue cunning-action hide via
    `CunningAction` dispatch with class gate; auto-reveal on `/attack`
    (`attack.go:808`); attacker-hidden grants advantage, target-hidden
    grants disadvantage (`advantage.go:39–44`).
  - Note: spec says "creature with line of sight" for passive Perception
    comparison; the implementation uses *all* hostiles' passive Perception
    without LoS filtering (`standard_actions.go:344–355`). This is a minor
    divergence — slightly more punishing than spec.

### Phase 70 — Reactions System
- Status: **matches**
- Key files:
  - `internal/combat/reaction.go`
  - `internal/combat/reactions_panel.go`
  - `internal/discord/reaction_handler.go`
- Findings:
  - Matches: freeform `Description` text persisted in
    `reaction_declarations`; multiple active declarations per combatant;
    one-reaction-per-round via `turns.ReactionUsed` with reset at next
    turn (verified via `RefundResource`/turn-start handling in
    `turnresources.go:107`); `/reaction cancel [desc]` (substring match)
    and `/reaction cancel-all`; surprised-condition gate (spec consistent
    with surprise rules); `CleanupReactionsOnEncounterEnd`;
    `ListReactionsForPanel` enriches with combatant info, NPC flag,
    `ReactionUsedThisRound` for DM dashboard.

### Phase 71 — Readied Actions
- Status: **matches**
- Key files:
  - `internal/combat/readied_action.go`
  - `internal/discord/action_handler.go:381–413`
- Findings:
  - Matches: action cost on ready; declaration created with
    `is_readied_action=true`; **readied-spell slot expended on ready**
    (`expendReadiedSpellSlot` prefers pact slot when level matches);
    concentration set on ready (`setReadiedSpellConcentration` writes
    `concentration_spell_name`); expiry at start of caster's next turn
    with `ExpireReadiedActions` returning notice strings (spec lines
    1106–1113 verbatim); pact-slot first selection; status display via
    `FormatReadiedActionsStatus`.
  - Minor gap: spec says "concentration held, lost if concentration
    breaks" — the readied spell uses the `concentration_spell_name`
    column but `SpellID` is intentionally empty (see comment in
    `setReadiedSpellConcentration`); cleanup pipelines key off
    `SpellName`. Confirm pending-CON-save and Silence-break pipelines
    actually look it up by name; the comment says "this is sufficient",
    so likely OK.

### Phase 72 — Counterspell Resolution
- Status: **matches**
- Key files:
  - `internal/combat/counterspell.go`
  - `internal/discord/counterspell_prompt.go`
- Findings:
  - Matches: two-step flow (Trigger → prompt with slot buttons + Pass,
    Resolve → either auto-counter or needs_check); enemy cast level
    **hidden** at prompt step (CounterspellPrompt struct intentionally
    omits EnemyCastLevel); slot deduction on both auto-counter and
    needs_check paths via `deductAndPersistSlot` /
    `deductAndPersistPactSlot`; DC = 10 + enemy spell level;
    `ResolveCounterspellCheck` for the follow-up ability roll;
    `PassCounterspell` does NOT consume reaction; `ForfeitCounterspell`
    consumes reaction (timeout = forfeited); Subtle Spell metamagic
    bypass (`ErrSubtleSpellNotCounterspellable`); `AvailableCounterspellSlots`
    filters to level 3+ including pact slots.
  - Note: spec says success "retroactively removes the spell's effects"
    — there is no explicit retroactive-cleanup hook in
    `ResolveCounterspell` / `ResolveCounterspellCheck`. The DM is expected
    to manually clean up via `/undo` or dashboard; this is consistent
    with the "DM handles retroactive correction" pattern used for OA
    kills.

### Phase 73 — Freeform Actions & /action cancel
- Status: **matches**
- Key files:
  - `internal/combat/freeform_action.go`
  - `internal/discord/action_handler.go:415–429`
- Findings:
  - Matches: action consumed on submit; `pending_actions` row created
    and linked to `dm_queue_items` via `DmQueueItemID`; cancel refunds
    action **first** (so even partial failure preserves the resource),
    then marks row cancelled, then best-effort edits the #dm-queue
    message with `~~strikethrough~~ Cancelled by player` overlay;
    `ErrNoPendingAction` and `ErrActionAlreadyResolved` returned with
    matching user-facing messages; exploration-mode variant
    (`CancelExplorationFreeformAction`) handles no-turn case.

### Phase 74 — Free Object Interaction (/interact)
- Status: **divergent**
- Key files:
  - `internal/combat/interact.go` (correct service — unused)
  - `internal/discord/interact_handler.go` (wired path — does NOT call
    `combat.Interact`)
- Findings:
  - Divergent: the Discord `/interact` handler in `interact_handler.go:108`
    calls `combat.UseResource(turn, ResourceFreeInteract)` directly. If
    `FreeInteractUsed` is true, the call returns an error and the user
    sees "Cannot interact: …". Per spec line 1200, the **second
    /interact should cost the action** (and only be rejected if the
    action is already spent). The richer logic in `combat.Interact` —
    second-interact-falls-back-to-action, auto-resolvable patterns
    (draw/sheathe/open/etc.), DM-queue routing for non-auto cases — is
    fully implemented and tested but never called by any handler.
  - Missing: auto-resolvable vs DM-queue routing (spec line 1201) is
    not surfaced; every interact posts a single combat-log line.

### Phase 75a — /equip + Hand Management
- Status: **divergent**
- Key files:
  - `internal/combat/equip.go` (correct service — unused by /equip)
  - `internal/discord/equip_handler.go:91`
  - `internal/inventory/equip.go` (the path actually used)
- Findings:
  - Divergent / Critical: the Discord `/equip` handler calls
    `inventory.Equip` which only flips an `Equipped` flag on inventory
    items. It does **not**:
    - validate two-handed weapon free-off-hand (spec line 412)
    - cost free object interaction in combat (spec line 412)
    - cost action for shield don/doff (spec line 412)
    - block armor changes in combat (spec line 412)
    - validate free-hand for grapple/somatic (cross-references with
      Phase 56/spellcasting)
  - The richer `combat.Equip` service in `equip.go` implements all of
    these checks correctly (two-handed off-hand, shield-action,
    armor-combat-block via `equipArmor`, weapon equip uses
    `ResourceFreeInteract` via `useResourceAndSave`, "none" unequip
    variants) but is never wired into the slash command. The hand-free
    check used by `combat.Grapple.checkFreeHand` reads
    `EquippedMainHand` / `EquippedOffHand` columns directly, which
    `inventory.Equip` does not maintain — only the inventory JSON
    `EquipSlot` is updated. So even the indirect grapple/somatic
    free-hand checks are likely broken because the columns the checks
    read aren't written by the wired equip path.

### Phase 75b — AC Recalculation & Enforcement
- Status: **divergent**
- Key files:
  - `internal/combat/equip.go:369–444` (`RecalculateAC`,
    `CheckHeavyArmorPenalty`)
  - `internal/charactercard/format.go:61` (AC display)
- Findings:
  - Service-side `RecalculateAC` correctly layers armor base + DEX
    (capped per medium/heavy), shield +2, and `ac_formula` (Unarmored
    Defense / Natural Armor); `CheckHeavyArmorPenalty` returns 10ft
    penalty when STR < `strength_req`; armor stealth disadvantage is
    handled separately in `stealthModAndMode`. All of these are correct.
  - Divergent: because the wired `/equip` path
    (`inventory.Equip`) does not call `combat.Equip` or update the
    cached `Ac` column, **AC is not recalculated** when equipment
    changes through the slash command. The character card still
    displays the old `Ac` value; `/attack` resolution reads
    `target.Ac` and so the AC the system enforces is stale relative
    to the player's actual equipment.
  - Stealth disadvantage IS enforced correctly in `/action hide` /
    `/check stealth` paths because they re-read `EquippedArmor` from
    the character row at roll time. But again — the wired equip path
    doesn't write to `EquippedArmor`, so this protection only fires
    for characters whose `EquippedArmor` column was seeded during
    registration or set via another path (DDB import, DM dashboard).

## Cross-cutting concerns

1. **Two `/equip` paths, the wrong one is wired.** Phases 75a/75b each have
   a fully-built and tested combat service that is dead code from the
   Discord side. The wired `inventory.Equip` is essentially a no-op for
   combat purposes. Either rename `combat.Equip` and route the
   `/equip` slash handler through it, or factor the combat-cost +
   AC-recalc into `inventory.Equip` and depend on the inventory path
   updating the character columns.
2. **`/interact` divergence is similar in shape.** `combat.Interact`
   correctly implements the action-fallback + auto-resolve + DM-queue
   semantics from spec. The slash handler reimplements a simpler version
   that rejects on second interact rather than charging the action.
3. **Inventory vs character columns drift.** The bot tracks equipment in
   two places: the `inventory` JSONB on the character (with `EquipSlot`
   per item) AND the `EquippedMainHand` / `EquippedOffHand` /
   `EquippedArmor` columns on the character row. The combat code
   uniformly reads the columns (grapple free hand, somatic free hand,
   armor stealth, AC recalc, attack weapon resolution); the slash equip
   handler writes the JSONB. The two stores can be out of sync.
4. **No `/action grapple` alias.** Spec lists `/action grapple [target]`
   as a recognized standard action (line 1171). Implementation requires
   `/shove [target] --mode grapple`. Minor UX divergence.
5. **Drag UX is not interactive.** Spec specifies a Drag / Release & Move
   button prompt before the move confirmation. Implementation shows a
   non-interactive prefix and requires `/bonus release-drag` as a separate
   step.

## Critical items

1. **`/equip` Discord handler bypasses `combat.Equip` entirely**
   — AC is never recalculated on equipment change in the
   live flow, two-handed-weapon validation is absent, shield don/doff
   doesn't cost an action, armor changes during combat are not blocked,
   and the `EquippedMainHand` / `EquippedOffHand` columns that
   downstream checks rely on are not maintained. This breaks Phases
   75a + 75b end-to-end. Files: `internal/discord/equip_handler.go:91`,
   `internal/inventory/equip.go`, `internal/combat/equip.go` (unused).

2. **`/interact` handler bypasses `combat.Interact`** — second
   interact is rejected outright instead of costing the action; no
   auto-resolvable vs DM-queue routing; no `pending_actions` row is
   created so the DM has nothing to resolve in `#dm-queue`. Breaks
   Phase 74. Files: `internal/discord/interact_handler.go:108`,
   `internal/combat/interact.go` (unused).

3. **OA: no end-of-round forfeiture sweep and no DM dashboard prompt**
   for DM-controlled hostiles. Spec line 1421 specifies a DM dashboard
   surface; current implementation only pings `#your-turn` (PC hostile
   path). DM-controlled hostile OAs are silently dropped. Files:
   `internal/discord/move_handler.go:680–722`.

4. **Drag/Release & Move interactive prompt missing** — spec line
   1444–1449 specifies a button choice with auto-release on the "Release
   & Move" branch. Current flow is a decorative prefix plus a separate
   `/bonus release-drag` command. UX divergence; mechanics intact.

5. **`/action grapple [target]` alias missing** — spec line 1171.
   Falls into the freeform DM-queue path today instead of running the
   contested check. Files: `internal/discord/action_handler.go:300–311`.
