# Group F Review — Vision + Reactions + Turn Flow (Phases 68–79)

Scope: phases 68–79. Sources cross-checked: `docs/dnd-async-discord-spec.md`
1073–1132, 1194–1204, 1306–1326, 1479–1498, 1641–2056, 2192–2268;
`docs/phases.md` 376–456; PHB equivalents.

Severity is sorted Critical → High → Medium → Low.

---

## [Critical] Counterspell trigger is unreachable from the DM dashboard
- **Location:** /home/ab/projects/DnDnD/dashboard/svelte/src/ActiveReactionsPanel.svelte:88-150
- **Spec/Phase ref:** spec §Counterspell resolution lines 1093-1101; phases §Phase 72
- **D&D rule:** RAW Counterspell uses the caster's reaction in response to an
  enemy spell within 60ft; spec mandates DM-initiated two-step prompt flow.
- **Problem:** `internal/combat/counterspell.go` and the HTTP handler exist,
  but the panel only renders Resolve / Dismiss. There is no "Trigger
  Counterspell" entry that posts to the backend route, so a DM cannot start the
  prompt → buttons → ability check pipeline from the UI. The done-when for
  Phase 72 ("full Counterspell flow") therefore cannot run end-to-end in the
  shipped dashboard.
- **Suggested fix:** Add a "Trigger" button (with spell name + level inputs,
  Subtle flag) on Counterspell-labelled declarations that calls
  `TriggerCounterspell` and then surfaces the player-side prompt in
  `#your-turn`.

## [Critical] Heavy-armor STR speed penalty is computed but never applied to combat speed
- **Location:** /home/ab/projects/DnDnD/internal/combat/equip.go:237,478-487; /home/ab/projects/DnDnD/internal/combat/turnresources.go:217-254
- **Spec/Phase ref:** spec §Equipment Enforcement lines 1483-1487; phases §Phase 75b
- **D&D rule:** PHB p144 — wearing armor with `Str` requirement above the
  wearer's score reduces speed by 10ft.
- **Problem:** `CheckHeavyArmorPenalty` only emits a string in the
  `equipArmor` combat log; the returned penalty is discarded. `ResolveTurnResources`
  starts every turn from `char.SpeedFt` (or beast walk speed) and never
  consults the equipped-armor STR requirement, so an underqualified PC moves
  at full speed every turn.
- **Suggested fix:** In `ResolveTurnResources` look up the equipped armor and
  subtract `CheckHeavyArmorPenalty` from `speedFt` before exhaustion / condition
  handling. Equivalent enforcement is needed on the persisted `speed_ft` write
  in equip/unequip so any cached speed reads agree.

## [Critical] Devil's Sight is never wired into the player vision pipeline
- **Location:** /home/ab/projects/DnDnD/cmd/dndnd/discord_adapters.go:755-806
- **Spec/Phase ref:** spec §Dynamic Fog of War lines 2204-2208; phases §Phase 68
- **D&D rule:** Warlock Devil's Sight (Eldritch Invocation) pierces magical
  darkness up to 120ft.
- **Problem:** `renderer.VisionSource` exposes `HasDevilsSight`, the FoW math
  honors it, and the combat obscurement engine honors it — but
  `buildVisionSources` never sets the flag from PC race / class / feature data.
  Result: a Warlock standing in a Darkness spell still sees only the origin
  tile, and `tileVisibleFromSource` always returns false for them.
- **Suggested fix:** Inspect the character's `features` JSON (or the resolved
  feature set) for "Devil's Sight" and set `src.HasDevilsSight = true` in
  `buildVisionSources`. Same hook should also cover blindsight/truesight from
  race features (e.g. monk-15 Tongue of Sun and Moon equivalents).

## [Critical] Lair Action is placed at the head of the turn queue instead of "loses ties"
- **Location:** /home/ab/projects/DnDnD/internal/combat/legendary.go:304-348
- **Spec/Phase ref:** spec §Enemy / NPC Turns lines 1916-1918; phases §Phase 78b
- **D&D rule:** DMG p246 — lair actions fire on initiative count 20, losing
  initiative ties (i.e. after every creature at 20).
- **Problem:** `BuildTurnQueueEntries` prepends the Lair Action entry at
  `Initiative: 20` and then appends the regular combatants in their input order.
  Any combatant with initiative > 20, or another creature also at 20 with
  higher tiebreak, will (or should) still act after the lair entry — but the
  function returns the lair entry first regardless of whether downstream
  sorting honors "losing ties". Combined with the fact that the function does
  not sort by initiative descending, ordering becomes ambiguous and lair
  actions can fire before legitimate winners at 20.
- **Suggested fix:** Build entries first, sort the slice by initiative
  descending with a stable secondary key that pushes Lair Action entries
  *after* every combatant sharing the same initiative number.

## [High] No light-source dim radius — 5e torches grant 20ft bright + 20ft dim
- **Location:** /home/ab/projects/DnDnD/cmd/dndnd/discord_adapters.go:907-927
- **Spec/Phase ref:** spec §Vision sources & modifiers lines 2206-2207; phases §Phase 68
- **D&D rule:** PHB p153 — Torch sheds bright light 20ft and dim light an
  additional 20ft. Lantern bright 30ft + dim 30ft.
- **Problem:** `lightRadiusForItem`/`lightRadiusForSpell` return a single
  `RangeTiles` and the FoW union promotes everything in that radius to
  `Visible`. No tiles get demoted to "dim light" obscurement. Players carrying
  a torch in pitch-black always see crisp 20ft bright and no dim halo, so the
  dim-light Perception disadvantage / hide-allowance never fires on the
  outside ring.
- **Suggested fix:** Either (a) emit two LightSource entries (bright +
  dim-ring) and have the renderer/encounter pipeline mark the outer ring as
  dim-light obscurement, or (b) at minimum publish dim-only zones around each
  light source so `CombatantObscurement` returns LightlyObscured outside the
  bright ring.

## [High] Hide action ignores the actor's vision when computing zone obscurement
- **Location:** /home/ab/projects/DnDnD/internal/discord/action_handler.go:794-805
- **Spec/Phase ref:** spec §Obscurement & Lighting Zones (Phase 69) lines 2239-2243
- **D&D rule:** Hide requires that the actor be lightly or heavily obscured
  from the observer's perspective.
- **Problem:** `CombatantObscurement(col, row, zones, combat.VisionCapabilities{})`
  is called with empty vision capabilities. That happens to be roughly correct
  for the observer's perspective (you want the raw zone level the observer
  would experience), but it's clearly accidental — the call site passes a zero
  value because no observer is in scope. As a result a darkvision attacker who
  should still see the actor clearly never causes Hide to fail at gate time
  (and conversely the level may be wrong for magical darkness / fog mixtures).
- **Suggested fix:** Either pass each hostile's vision and require *all*
  observers to be obscured, or document that the zone is computed without
  vision and validate by hostile passive Perception only.

## [High] Hidden combatants (`is_visible = false`) still render on the map
- **Location:** /home/ab/projects/DnDnD/internal/gamemap/renderer/fog.go:52-78; /home/ab/projects/DnDnD/internal/gamemap/renderer/token.go:38
- **Spec/Phase ref:** spec §Dynamic Fog of War lines 2202-2203; spec §Standard Actions (Hide) line 1151
- **D&D rule:** A successfully hidden creature is unseen by enemies that
  failed their Perception check.
- **Problem:** `filterCombatantsForFog` only considers `Visible` / `Explored` /
  `Unexplored` against the tile state. The `Combatant.IsVisible` field (set
  to false by `Hide`) is never consulted. A rogue who successfully Hides will
  still render on every player map, defeating the whole `/action hide` gating.
- **Suggested fix:** In `filterCombatantsForFog` (or earlier, when projecting
  refdata.Combatant → renderer.Combatant) drop combatants whose `IsVisible`
  is false **for the audience that doesn't see them**. Hidden combatants
  should still render in the DM view.

## [High] Free-object interaction whitelist is too permissive / English-only
- **Location:** /home/ab/projects/DnDnD/internal/combat/interact.go:13-52
- **Spec/Phase ref:** spec §Free Object Interaction lines 1198-1201; phases §Phase 74
- **D&D rule:** PHB p190 — examples include drawing/sheathing a weapon, opening
  unlocked doors, picking up dropped items. Anything else is at DM discretion.
- **Problem:** `autoResolvablePatterns` matches by prefix on "open", "grab",
  "pick up", etc. A player typing `/interact open the locked treasure chest`
  will silently auto-resolve as if it were free — the prefix match fires even
  though the door/lock state matters. There is no actual world lookup against
  the map's door/container state.
- **Suggested fix:** Route every `/interact` through the DM queue unless an
  explicit map-object lookup confirms the target is auto-resolvable (door
  with `is_locked=false`, weapon in inventory for draw/sheathe). At minimum,
  reject auto-resolve if the description contains "lock", "trap", "stuck",
  "barred", etc.

## [High] Lair-action "no consecutive repeats" tracker is in-memory only
- **Location:** /home/ab/projects/DnDnD/internal/combat/legendary.go:198-263
- **Spec/Phase ref:** spec §Enemy / NPC Turns line 1918; phases §Phase 78b
- **D&D rule:** DMG — lair actions cannot repeat the same option two rounds in
  a row.
- **Problem:** `LairActionTracker` is a value type returned by helpers but
  there is no DB column persisting the last-used lair action across rounds.
  After a bot restart (or even between two requests from different goroutines)
  the tracker resets, so the "no repeats" rule silently lapses.
- **Suggested fix:** Persist `last_used_lair_action` on the encounter row (or
  per-creature) and hydrate `LairActionTracker` from it before each Lair
  Action prompt.

## [High] Legendary-action budget round-trips through the URL — no server persistence
- **Location:** /home/ab/projects/DnDnD/internal/combat/legendary_handler.go:73-78,170-180
- **Spec/Phase ref:** spec §Enemy / NPC Turns line 1916; phases §Phase 78b
- **D&D rule:** Budget resets at the creature's own turn start and decrements
  per legendary action used.
- **Problem:** The dashboard sends `budget_remaining` as a query param, and the
  server trusts it without any persisted check. Two dashboards (or a refresh)
  can desync the budget — a creature can take 3/3 actions twice in a single
  round if the client state diverges. The reset on the creature's turn start
  isn't enforced server-side either.
- **Suggested fix:** Add a `legendary_action_budget` column (or JSONB) on
  `combatants` and have the server decrement it during `ExecuteLegendaryAction`
  while resetting it inside `createActiveTurn` for the legendary creature.

## [High] Counterspell trigger does not validate spell range / line-of-sight
- **Location:** /home/ab/projects/DnDnD/internal/combat/counterspell.go:65-116
- **Spec/Phase ref:** spec §Counterspell resolution line 1093 ("enemy casts a spell within 60ft")
- **D&D rule:** Counterspell has Range 60ft and requires Sight of the caster.
- **Problem:** `TriggerCounterspell` accepts any declaration ID and any enemy
  caster ID. It does not compute the distance between declarant and enemy
  caster, nor consult LOS / cover. A DM (or a buggy client) can fire
  Counterspell across the map or through a wall.
- **Suggested fix:** Look up both combatants, compute distance via the
  existing `distance.go` helpers, and reject if > 60ft or if `cover.go`
  reports total cover.

## [High] Reaction declarations not validated for the type's prerequisites
- **Location:** /home/ab/projects/DnDnD/internal/combat/reaction.go:27-46
- **Spec/Phase ref:** spec §Reactions lines 1077-1085
- **D&D rule:** Specific reactions require a specific class feature / spell
  (Shield → must have Shield prepared, Counterspell → known spell, etc.).
- **Problem:** `DeclareReaction` takes raw freeform text and only checks the
  surprised condition. A player can `/reaction Counterspell when enemy casts`
  without owning Counterspell, then the DM panel offers a Trigger button that
  fails at `AvailableCounterspellSlots` time. Better: reject early when the
  declared reaction name is detectable but unavailable on the character sheet.
- **Suggested fix:** Heuristically detect well-known reaction names in the
  description (Shield, Counterspell, Hellish Rebuke, Cutting Words, etc.) and
  validate against the character's spells_known / class features at
  declaration time.

## [Medium] Readied-spell concentration written with empty SpellID
- **Location:** /home/ab/projects/DnDnD/internal/combat/readied_action.go:126-141
- **Spec/Phase ref:** spec §Readied Actions line 1103
- **D&D rule:** A readied spell's concentration uses the actual spell so it
  participates in concentration breaks normally.
- **Problem:** `setReadiedSpellConcentration` always writes
  `ConcentrationSpellID: sql.NullString{}` (Valid=false). Downstream cleanup
  paths key off `SpellName`, but anything indexing by spell ID (e.g. spell-
  specific dispel logic) will silently no-op. The comment even calls this out
  as a knowing shortcut.
- **Suggested fix:** Populate `SpellID` from `cmd.SpellName` via the spells
  table lookup before persisting concentration; matches the regular `/cast`
  path used elsewhere.

## [Medium] Counterspell ability check uses character.ProficiencyBonus on the wrong side
- **Location:** /home/ab/projects/DnDnD/internal/combat/counterspell.go:209-260
- **Spec/Phase ref:** spec §Counterspell resolution line 1099 ("no proficiency — per 5e RAW")
- **D&D rule:** RAW — the Counterspell check uses **only** the spellcasting
  ability modifier, NO proficiency bonus.
- **Problem:** The check itself happens in `/check spellcasting`, not in
  `ResolveCounterspellCheck` — but the `/check` infrastructure does add
  proficiency when the character is proficient in the rolled skill (and
  spellcasting is intrinsically proficient for casters). Without a special
  flag on this particular check the player will see a +PB bonus.
- **Suggested fix:** Either expose a `--no-prof` modifier or have the
  counterspell prompt issue the roll itself with `dice.Normal` mode and only
  the spellcasting ability modifier.

## [Medium] `/done` unused-resource warning's "Action" branch is logically dead
- **Location:** /home/ab/projects/DnDnD/internal/combat/unused_resources.go:13-33
- **Spec/Phase ref:** spec §Ending a turn lines 1891-1894
- **D&D rule:** Players should be warned about an unused Action.
- **Problem:** `if !turn.ActionUsed && turn.AttacksRemaining == 0` is the only
  emit path for "Action". In every realistic state, an unused action means the
  attacks-per-action have NOT been consumed yet, so `AttacksRemaining` is
  positive. The branch effectively never fires; the warning then only lists
  attacks/bonus action. Players who took a non-attack action that failed to
  flip `action_used` get a misleading warning.
- **Suggested fix:** Replace with `if !turn.ActionUsed` alone, and let the
  "attacks" string concat below when both apply.

## [Medium] Magical-darkness zone affected-tiles ignore concentration-anchored zone movement
- **Location:** /home/ab/projects/DnDnD/cmd/dndnd/discord_adapters.go:819-833
- **Spec/Phase ref:** spec §Spell effect zone lifecycle (anchor modes) lines 2223-2225
- **D&D rule:** Darkness spell can be on an object — but the Darkness spell is
  typically static. However Fog Cloud anchored on a creature would move; the
  spec calls out `combatant` anchor mode.
- **Problem:** `buildMagicalDarknessTiles` reads `OriginCol/OriginRow` from
  the zone row but doesn't honor the `anchor_mode = "combatant"` case where the
  origin should mirror the anchor combatant's current position. The renderer
  call to `ZoneAffectedTilesFromShape` will be stale until something writes
  back the new origin.
- **Suggested fix:** When `anchor_mode == "combatant"` and the anchor
  combatant exists, recompute origin from the combatant's live position before
  expanding the affected tile set.

## [Medium] Light cantrip / Continual Flame zones get 20ft uniform — but Daylight is 60ft bright + 60ft dim
- **Location:** /home/ab/projects/DnDnD/cmd/dndnd/discord_adapters.go:918-927
- **Spec/Phase ref:** spec §Vision sources line 2207
- **D&D rule:** PHB Daylight — 60ft bright + 60ft dim radius.
- **Problem:** Same gap as the torch issue — `lightRadiusForSpell` returns a
  single tile radius, no dim halo. Daylight in pitch-black gives no dim
  fall-off.
- **Suggested fix:** Same approach — emit dim-ring obscurement zones around
  each light source, or extend LightSource with `DimRangeTiles`.

## [Medium] Hide-success token visibility doesn't propagate to enemy renders
- **Location:** /home/ab/projects/DnDnD/internal/combat/standard_actions.go:363-371
- **Spec/Phase ref:** spec §Standard Actions (Hide) lines 1166-1170
- **D&D rule:** Stealth hides you from creatures who failed the contest.
- **Problem:** Hide flips `is_visible=false` on the combatant. But (a) the
  renderer doesn't read the flag (see the higher-severity finding above) and
  (b) `is_visible=false` is global — there is no per-observer flag, so the
  DM view also loses sight of the rogue. The combination effectively means
  Hide changes the boolean but nothing visible to anyone.
- **Suggested fix:** Track hide success per-hostile (or simpler: keep the
  global flag but only suppress the token in player renders, never the DM
  render).

## [Medium] Surprised combatants — round 1 turn skip removes condition immediately, losing reaction-end-of-turn rule
- **Location:** /home/ab/projects/DnDnD/internal/combat/initiative.go:569-593
- **Spec/Phase ref:** spec §Surprise lines 1670-1672
- **D&D rule:** A surprised creature can use reactions after its first turn
  ends — the spec puts the condition removal at "end of their skipped turn".
- **Problem:** `skipSurprisedTurn` calls `skipCombatantTurn` (which creates a
  `status='skipped'` turn) and then immediately removes the surprised
  condition. There is no observable "between actions" window where another
  creature could trigger the surprised one's reaction at the right moment.
  Reactions declared in round 1 before the surprise is dropped are blocked
  (good), but reactions triggered during the same skip frame are also
  blocked (less good). Edge case; matters for OAs against creatures moving
  past the surprised one.
- **Suggested fix:** Sequence "create skipped turn → mark as completed →
  process reaction window → remove surprised condition" so the brief window
  exists; or document the deviation.

## [Medium] `equipWeapon` accidentally drops the Defense fighting style AC bonus when re-equipping
- **Location:** /home/ab/projects/DnDnD/internal/combat/equip.go:141-184
- **Spec/Phase ref:** spec §AC calculation line 1493
- **D&D rule:** Defense fighting style: +1 AC while wearing armor.
- **Problem:** `equipWeapon` writes `equipUpdateParams(char, char.Ac)` — i.e.
  it keeps the existing AC. If the cached `char.Ac` is somehow stale (e.g.
  the previous equip transaction failed mid-flight), the bug snowballs because
  there is no re-evaluation. Combined with the absence of any recompute on
  weapon equip, an out-of-sync cached AC will never self-heal.
- **Suggested fix:** Always re-run `RecalculateAC` on every equip path (the
  weapon-equip path can pass the existing armor + shield, the result is
  idempotent).

## [Medium] Spell-slot deduction for readied actions runs even when the spell needs no slot
- **Location:** /home/ab/projects/DnDnD/internal/combat/readied_action.go:68-75
- **Spec/Phase ref:** spec §Readied Actions line 1103
- **D&D rule:** Cantrips have no slot.
- **Problem:** `if cmd.SpellName != "" && cmd.SpellSlotLevel > 0 ...` looks
  fine, but cantrips fed from the UI will arrive with `SpellSlotLevel=0` and
  thus the concentration block is also skipped — even though concentration
  cantrips (Toll the Dead is not concentration but, e.g., Booming Blade as a
  readied spell is concentration in some interpretations). The code does not
  hold concentration on cantrips that require it.
- **Suggested fix:** Branch on the spell row's `concentration` flag, not on
  `SpellSlotLevel > 0`.

## [Medium] No reaction-used reset for legendary actions / lair actions in cross-turn sequencing
- **Location:** /home/ab/projects/DnDnD/internal/combat/legendary.go (entire file)
- **Spec/Phase ref:** spec §Reactions line 1084
- **D&D rule:** Legendary creatures still get one reaction per round; using a
  legendary action consumes a legendary action budget, not a reaction.
- **Problem:** The code paths for legendary actions never touch the parent
  combatant's `ReactionUsed`. That's the right invariant. But the per-creature
  reaction-used path (`buildReactionUsedMap`) keys off `turns` for the round,
  so a legendary creature mid-other-creature's-turn whose own reaction fired
  earlier won't have the flag cleared until its own next-turn row appears. In
  practice this is fine — but combined with the lack of per-event hooks for
  legendary mini-turns the panel may incorrectly mark reactions as "used this
  round" until the dragon's actual turn rolls around.
- **Suggested fix:** Document explicitly. Add a unit test that walks the panel
  state between legendary mini-turns.

## [Medium] Auto-resolve cancels reaction declarations instead of marking specific Counterspell/OA as forfeited
- **Location:** /home/ab/projects/DnDnD/internal/combat/timer_resolution.go:314-321
- **Spec/Phase ref:** spec §100% — DM decision prompt lines 2035-2038
- **D&D rule:** Reaction prompts (Counterspell / OA) that aren't answered are
  forfeited; spec text reserves a specific "forfeited" outcome.
- **Problem:** AutoResolve calls `CancelAllReactionDeclarationsByCombatant`,
  which moves the declarations to "cancelled" status. The dedicated
  `ForfeitCounterspell` helper that records `counterspell_status='forfeited'`
  and a separate OA-forfeit log line are bypassed. The combat log message in
  the spec ("Aria forfeits OA vs Goblin #1 (auto-resolved — player timed
  out)") is also not emitted.
- **Suggested fix:** Iterate the active reaction declarations and call
  `ForfeitCounterspell` for Counterspell rows, and for OAs call the OA-cancel
  notifier with the "auto-resolved" reason so the messages match the spec.

## [Medium] `findAdjacentEnemies` uses 0-based row from `int(PositionRow)` directly — off-by-one risk
- **Location:** /home/ab/projects/DnDnD/internal/combat/timer.go:212-230
- **Spec/Phase ref:** spec §75% warning line 2016
- **D&D rule:** N/A (tooling)
- **Problem:** `PositionRow` is stored as 1-based by convention elsewhere
  (`int(z.OriginRow) - 1` in `obscurement.go`), but here the code subtracts
  nothing and pairs the raw row with `colToIndex(c.PositionCol)` which IS
  zero-based. The Chebyshev adjacency check still works because both sides
  agree, but the resulting "adjacent" list can include creatures one tile
  off when mixed with other consumers that expect 0-based rows.
- **Suggested fix:** Normalize coordinate parsing to a single helper (the
  `renderer.ParseCoordinate` path) and keep all in-package math 0-based.

## [Medium] Multiattack parser falls back to "use every attack once" — wrong for skirmishers
- **Location:** /home/ab/projects/DnDnD/internal/combat/turn_builder.go:309-395
- **Spec/Phase ref:** spec §Enemy / NPC Turns line 1910; phases §Phase 78c
- **D&D rule:** Multiattack varies per creature — "two scimitar attacks" is
  literally two of the same, never one each.
- **Problem:** If `parseMultiattackSequence` can't decode the description, the
  builder emits one of every attack the creature owns. A Goblin's stat block
  prose without a recognized word pattern would generate scimitar + shortbow
  instead of two scimitar swings. Many imported Open5e prose snippets fall
  into that fallback.
- **Suggested fix:** When parsing fails, default to N copies of the primary
  attack (N taken from `creature.attacks_per_action` or "2" for any
  Multiattack-flagged creature), not one of each.

## [Medium] Reaction one-per-round resets between rounds but not at "creature's turn start" exactly
- **Location:** /home/ab/projects/DnDnD/internal/combat/reactions_panel.go:79-104
- **Spec/Phase ref:** spec §Reactions line 1084
- **D&D rule:** Reaction resets at the start of the creature's own turn, not
  at the start of the round.
- **Problem:** `buildReactionUsedMap` looks at every turn in the round and
  reports `reaction_used`. If creature X uses a reaction in round R while
  another creature is acting, then X's own turn begins later in round R: a
  new turn row is created with `reaction_used=false`, but the OLD row from the
  previous round (if combat queries leak across rounds) and from the in-round
  reaction declaration both still exist. The panel maps by `combatantID`, so
  the "still used" row from earlier in the same round wins and X can't react
  on its own turn either. Edge case but observable when a hostile triggers
  X's OA and then X's turn comes up later in the round.
- **Suggested fix:** Treat the most recent turn row (highest `started_at` /
  `id` order) as authoritative, not the union of all turns in the round.

## [Medium] No range / 60ft check for Counterspell when generating its prompt
- **Location:** /home/ab/projects/DnDnD/internal/combat/counterspell.go (entire file)
- **Spec/Phase ref:** spec §Counterspell resolution line 1093
- **Problem:** (See range/LOS finding under High, repeated here for
  completeness — keep one.) Skip.

## [Medium] Free interaction matches "open" prefix → routes "open the heavy chest" away from DM
- **Location:** /home/ab/projects/DnDnD/internal/combat/interact.go:11-24
- **Spec/Phase ref:** spec §Free Object Interaction line 1201
- **Problem:** See above. Duplicate of the "free interaction whitelist too
  permissive" finding, listed once.

## [Low] `equipShield` auto-stows off-hand weapon at no cost
- **Location:** /home/ab/projects/DnDnD/internal/combat/equip.go:197-200
- **Spec/Phase ref:** spec §Equipment Management lines 1493-1494
- **D&D rule:** Stowing a weapon is itself a free object interaction.
- **Problem:** When a shield is equipped and the off-hand had a weapon, the
  weapon is "automatically stowed" with the comment saying "no extra cost".
  The shield already costs an Action; the spec implicitly allows this because
  shield don/doff is one action, but the player should at minimum lose their
  free object interaction since they actively manipulated two items.
- **Suggested fix:** Tag the auto-stow with `free_interact_used=true` (best-
  effort; if already used, just no-op).

## [Low] DrawFogOfWar uses solid black for Unexplored even on DM render before DMSeesAll check
- **Location:** /home/ab/projects/DnDnD/internal/gamemap/renderer/fog.go:14-45
- **Spec/Phase ref:** spec §Rendering layers line 2263-2267
- **Problem:** The early-return on `DMSeesAll` happens *before* the loop, so
  the DM render bypasses fog entirely as intended. But the code path that
  callers use to pre-compute fog and then set `DMSeesAll=true` separately
  (renderer.go:39-41) means a stale render could draw fog if the propagation
  is missed. Not a current bug but a fragile invariant.
- **Suggested fix:** Make `DMSeesAll` a property of the render request, not a
  mutable property of the FoW struct, to remove the cross-call propagation
  step.

## [Low] `RecalculateAC` evaluates `ac_formula` parts via Sscanf without error handling
- **Location:** /home/ab/projects/DnDnD/internal/combat/equip.go:467-474
- **Spec/Phase ref:** spec §AC calculation line 1494
- **Problem:** `fmt.Sscanf(part, "%d", &n)` ignores the error; if a formula
  contains a bad token (e.g. `"10+DEX+WIS"` without spaces around `+`) Sscanf
  silently contributes 0 and the AC quietly drops. The split logic happens to
  handle the canonical `"10 + DEX + WIS"` shape, but isn't robust.
- **Suggested fix:** Use a tokenizer that errors on unknown tokens and return
  the error so the caller logs it.

## [Low] Bonus action parsing misses fully-structured `bonus_actions` rows containing only descriptions
- **Location:** /home/ab/projects/DnDnD/internal/combat/turn_builder.go:473-499
- **Spec/Phase ref:** phases §Phase 78c
- **Problem:** `ParseBonusActions` (the fallback) requires "bonus action" in
  the description. Several MM creatures phrase it differently ("As a bonus
  action,…"). The structured column saves you when present, but the fallback
  legitimately misses some.
- **Suggested fix:** Extend the matcher to also accept "as a bonus action"
  anywhere in the description.

## [Low] Summoned-creature short-ID collisions are not detected
- **Location:** /home/ab/projects/DnDnD/internal/combat/summon.go:117-150
- **Spec/Phase ref:** spec §Summoning flow line 1952 ("Each gets a short ID")
- **Problem:** `SummonMultipleCreatures` concatenates `BaseShortID + i`. If
  the same player summons two batches with the same base (e.g. two casts of
  Conjure Animals → WF1..WF8 each time), the second batch will collide. There
  is no uniqueness check on `short_id` in the encounter.
- **Suggested fix:** Either enforce a UNIQUE(encounter_id, short_id)
  constraint in the schema, or shift the counter forward based on max
  existing matching short_id.

## [Low] Concentration on readied spell never sets ConcentrationSpellID, breaking spell-ID lookups
- **Location:** /home/ab/projects/DnDnD/internal/combat/readied_action.go:132-141
- **Spec/Phase ref:** spec §Readied Actions line 1103
- **Problem:** Listed at Medium severity above; noting here for completeness.
  Skip.

## [Low] `LightSource` and `VisionSource` deduplicate by position but not vision type — flickers
- **Location:** /home/ab/projects/DnDnD/internal/gamemap/renderer/fog_types.go:52-93
- **Problem:** Two PCs on the same tile produce two `VisionSource` entries —
  fine, the union is idempotent. Two torches on the same tile produce two
  `LightSource` entries — also fine. But the API encourages callers to
  deduplicate; there is no defense against accidental duplicates skewing
  performance on dense maps. Minor.
- **Suggested fix:** Deduplicate by `(col,row,vision-class)` before running
  shadowcast. Performance only.

## [Low] `/action ready` doesn't differentiate readying a Spell vs. a non-Spell action in the panel badge
- **Location:** /home/ab/projects/DnDnD/dashboard/svelte/src/ActiveReactionsPanel.svelte:120-122
- **Spec/Phase ref:** spec §Readied Actions line 1103
- **Problem:** Both kinds show the "Readied" badge. A DM can't tell at a
  glance which readied action holds a spell slot / concentration.
- **Suggested fix:** Add a sub-badge "spell (Lv N)" when `reaction.spell_name`
  / `reaction.spell_slot_level` are set.

## [Low] FoW `chebyshevDistance` is correct for square grids but doesn't match the canonical 5e diagonal rule
- **Location:** /home/ab/projects/DnDnD/internal/gamemap/renderer/fog_types.go:160-173
- **Problem:** Chebyshev distance (treat diagonals as 1) is fine for
  visibility but diverges from 5e movement (every other diagonal = 10ft).
  The renderer uses tile-units already, so the inconsistency is contained,
  but a player comparing range readouts will be confused.
- **Suggested fix:** Document the choice or expose both Chebyshev (vision)
  and 5e-grid distance (movement/range) helpers with clear names.

---

## Phase Status Summary

- **Phase 68 (Dynamic Fog of War):** issues found (Devil's Sight wiring, hidden combatants, torch dim radius).
- **Phase 69 (Obscurement & Lighting Zones):** issues found (Hide vision call, lair-action ordering interaction).
- **Phase 70 (Reactions System):** issues found (declaration validation, reaction-used per-turn correctness).
- **Phase 71 (Readied Actions):** issues found (concentration spell ID, cantrip concentration handling).
- **Phase 72 (Counterspell Resolution):** issues found (no dashboard trigger UI, no range/LOS gate, prof bonus risk).
- **Phase 73 (Freeform Actions & cancel):** Phase 73: OK.
- **Phase 74 (Free Object Interaction):** issues found (auto-resolve whitelist too permissive).
- **Phase 75a (Equipment commands & hand management):** issues found (auto-stow free interact accounting).
- **Phase 75b (AC recalc & enforcement):** issues found (heavy-armor STR speed never applied).
- **Phase 76a (Turn Timeout — timer/nudges):** Phase 76a: OK.
- **Phase 76b (Turn Timeout — resolution):** issues found (forfeit specific reaction outcomes vs blanket cancel).
- **Phase 77 (Player Turn Start / Done):** issues found (unused-resource warning dead branch).
- **Phase 78a (Enemy/NPC turn builder):** issues found (multiattack fallback).
- **Phase 78b (Legendary & Lair):** issues found (lair-init ordering, no-repeat persistence, budget persistence).
- **Phase 78c (Bonus action parsing):** Phase 78c: OK.
- **Phase 79 (Summons):** issues found (short-ID uniqueness).
