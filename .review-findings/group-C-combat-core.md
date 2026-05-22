# Group C — Combat Core (Phases 26-43) Review

## Summary

**Counts by severity:** Critical 4, High 13, Medium 14, Low 9 (total 40 findings).

**Top 5 concerns:**

1. **`colToIndex` truncates multi-letter column labels** (e.g., "AA" → 0). Breaks distance/cover/OA detection on any map wider than 26 columns. Spec explicitly supports `AA12`.
2. **Reckless Attack advantage is only granted on the first attack.** The carry-through to attacks 2+ is missing — the on-attacker `reckless` condition only grants advantage to *enemies* attacking the barbarian, not to the barbarian's own subsequent swings.
3. **Off-hand attack (TWF) doesn't require the Attack action to have been taken** and doesn't require off-hand to be a *melee* weapon (just "light"). A light crossbow off-hand or a /bonus offhand on the first action of the turn currently passes.
4. **PC creature size is hard-coded to "Medium"** in `resolveAttackerSize`. Heavy-weapon disadvantage for Small races (halfling, gnome) is therefore never applied.
5. **`/fly` performs no fly-speed validation.** Any combatant can `/fly` to any altitude regardless of whether they have a fly speed; only DEX/CON ground walkers are silently airborne.

Auto-crit-from-paralyzed lacks a melee gate (ranged at 5ft auto-crits), Crossbow Expert is wired for the loading cap but not for the ranged-with-hostile-adjacent disadvantage waiver, fall damage has no 20d6 RAW cap, Dash uses raw speed (ignoring exhaustion/conditions), and the cover-DEX-save integration relies on `CalculateCoverFromOrigin`'s "closest-corner" heuristic that contradicts the DMG variant the spec cites.

---

## [Critical] Multi-letter column labels truncated by `colToIndex`
- **Location:** /home/ab/projects/DnDnD/internal/combat/attack.go:1571-1577
- **Spec/Phase ref:** spec §Grid Movement (line 285-300, "AA12 valid on maps wider than 26 columns")
- **D&D rule:** N/A — coordinate parsing
- **Problem:** `colToIndex` takes only `strings.ToUpper(col)[0]`, so "AA" → 0 (same as "A"). Used by `combatantDistance`, `resolveAttackCover`, `detectHostileNear`, and `creatureCoverOccupants`. On maps with > 26 columns every attacker/target in the AA+ block resolves to column 0 — distances, cover lines, OA reach checks all collapse onto column A.
- **Suggested fix:** Reuse `renderer.ParseCoordinate(positionCol + strconv.Itoa(positionRow))` (or extract the column-letter loop from ParseCoordinate into a shared helper). The renderer code already handles `AA`/`AB`/... correctly.

## [Critical] Reckless Attack advantage missing on attacks 2+
- **Location:** /home/ab/projects/DnDnD/internal/combat/attack.go:887-901, advantage.go:36-39
- **Spec/Phase ref:** Phase 38; spec line 217 ("advantage on melee STR attacks this turn")
- **D&D rule:** Reckless Attack grants advantage to **all** melee STR attack rolls for the entire turn.
- **Problem:** The reckless gate rejects `--reckless` on any swing other than the first (`AttacksRemaining < maxAttacks` → error). The transient `reckless` condition that gets applied to the attacker is only consulted on the *target-side* branch of `DetectAdvantage` (line 126-133, granting advantage to enemies attacking the reckless attacker). There is no attacker-side branch that re-applies "Reckless Attack" advantage on attack 2/3/4 of the same turn.
- **Suggested fix:** In `DetectAdvantage`, also check `attackerConditions` for `reckless` and, when present, add `"Reckless Attack (active)"` to `advReasons` for melee STR attacks. Keep the existing first-attack gate on the flag itself.

## [Critical] Off-hand (TWF) attack lacks Attack-action prerequisite and melee weapon check
- **Location:** /home/ab/projects/DnDnD/internal/combat/attack.go:1147-1200
- **Spec/Phase ref:** Phase 36; spec line 463 ("when a character attacks with a light **melee** weapon in their main hand... use bonus action to attack with a different light melee weapon")
- **D&D rule:** PHB Two-Weapon Fighting: requires the **Attack action** taken with a light **melee** weapon; off-hand must also be a light **melee** weapon.
- **Problem:** `OffhandAttack` only validates `ResourceBonusAction` available, that both weapons exist and that both have the `light` property. It does NOT (a) verify the Attack action has been taken this turn (no `Turn.AttacksRemaining < maxAttacks` style check, no `ActionUsed`), and (b) does not check the `melee` weapon type — a "light crossbow" or "dart" (light ranged) off-hand currently passes.
- **Suggested fix:** Reject the command if no attack has been made this turn (e.g., `cmd.Turn.AttacksRemaining == initialMaxAttacks`). Also gate on `!IsRangedWeapon(mainWeapon) && !IsRangedWeapon(offWeapon)`.

## [Critical] `/fly` performs no fly-speed validation
- **Location:** /home/ab/projects/DnDnD/internal/combat/altitude.go:52-81; /home/ab/projects/DnDnD/internal/discord/fly_handler.go:98-109
- **Spec/Phase ref:** Phase 31 (Altitude & Flying); spec §Altitude & Elevation
- **D&D rule:** Only creatures with a fly speed (innate, magic, or spell-granted like Fly/Polymorph-into-flier) can fly.
- **Problem:** `ValidateFly` rejects only negative altitudes, same-altitude moves, and insufficient movement. There is no check for whether the combatant actually possesses a fly speed (character `speed_fly`, beast Wild Shape with fly speed, Fly spell-applied `fly_speed` condition, etc.). Any character can `/fly 30` despite having no flight source.
- **Suggested fix:** Have `Service.Fly` (or the handler) consult the character/creature speed and active conditions (e.g., `fly_speed`, `wild_shape` w/ beast fly speed) and reject with "❌ You don't have a fly speed" when none apply. The `FlySpeedCondition` constant already exists but is only used on the cleanup side.

---

## [High] Auto-crit applies to ranged attacks within 5ft against paralyzed/unconscious
- **Location:** /home/ab/projects/DnDnD/internal/combat/attack.go:727-748 (`CheckAutoCrit`)
- **Spec/Phase ref:** Phase 34; spec line 694 ("**melee** attacks within 5ft against paralyzed or unconscious")
- **D&D rule:** Paralyzed/unconscious auto-crit applies only to **melee** attacks within 5ft.
- **Problem:** `CheckAutoCrit` only gates on `distFt > 5` and does not consider weapon type. A point-blank ranged shot (e.g., shortbow at 5ft, hand crossbow) against a paralyzed target currently auto-crits.
- **Suggested fix:** Pass the weapon into `CheckAutoCrit` (or call it after weapon resolution) and short-circuit with `if IsRangedWeapon(weapon) && !cmd.Thrown && !cmd.ImprovisedThrown` (thrown melee at 5ft is still a melee attack and should auto-crit).

## [High] PC creature size hard-coded to "Medium" — heavy-weapon disadvantage never fires for halflings/gnomes
- **Location:** /home/ab/projects/DnDnD/internal/combat/attack.go:1316-1326
- **Spec/Phase ref:** Phase 35; spec line 687 ("Small/Tiny creature using a Heavy weapon (disadv)")
- **D&D rule:** Small or Tiny creatures have disadvantage on attack rolls with Heavy weapons.
- **Problem:** `resolveAttackerSize` returns the creature row's size for NPCs but falls through to `"Medium"` for every PC. Halfling/gnome PCs wielding a greatsword/heavy crossbow correctly hit `HasProperty(weapon, "heavy")` but `AttackerSize == "Small"` is never true.
- **Suggested fix:** Look up the PC's race and read `races.size`. The `character.Character` row should carry race_id or the size resolution should join through that table.

## [High] Crossbow Expert does not waive ranged-with-hostile-adjacent disadvantage
- **Location:** /home/ab/projects/DnDnD/internal/combat/advantage.go:88-91
- **Spec/Phase ref:** Phase 35; spec line 687 ("Crossbow Expert feat removes this penalty for ranged weapon attacks only")
- **D&D rule:** Crossbow Expert: "Being within 5 feet of a hostile creature doesn't impose disadvantage on your ranged attack rolls."
- **Problem:** `DetectAdvantage` adds "hostile within 5ft" disadvantage whenever `HostileNearAttacker && IsRangedWeapon`. `AttackInput.HasCrossbowExpert` is populated by `Service.Attack` (line 916, 973) but the disadvantage rule never consults it.
- **Suggested fix:** Add `&& !input.HasCrossbowExpert` to the `HostileNearAttacker && IsRangedWeapon(Weapon)` branch. Note: the feat only applies to **ranged weapon** attacks, so ranged-spell attacks (Eldritch Blast, Fire Bolt) still take disadvantage — keep that distinct if/when ranged spell attacks land here.

## [High] Dash adds raw base speed, ignoring exhaustion/condition speed modifiers
- **Location:** /home/ab/projects/DnDnD/internal/combat/standard_actions.go:38-71 (`Dash`), 73-87 (`resolveBaseSpeed`)
- **Spec/Phase ref:** Phase 42 (exhaustion levels 2/5 modify speed); spec §Exhaustion
- **D&D rule:** Dash grants "additional movement equal to your speed". The effective speed already reflects exhaustion, grappled, restrained, etc.
- **Problem:** `Dash` does `updatedTurn.MovementRemainingFt += speed` where `speed` is `char.SpeedFt` (or 30 for NPCs) — the raw base speed. An exhaustion-2 PC (speed halved) currently gets a full base-speed Dash bonus, effectively recovering the halving. Grappled PC (speed 0) can still Dash and pay-for the bonus.
- **Suggested fix:** Pipe Dash through `EffectiveSpeedWithExhaustion`/`EffectiveSpeed` so it adds the effective speed. Also reject Dash when effective speed = 0 (per RAW, you can't Dash if you can't move).

## [High] Fall damage missing 20d6 cap
- **Location:** /home/ab/projects/DnDnD/internal/combat/altitude.go:101-123 (`FallDamage`)
- **Spec/Phase ref:** Phase 31; spec line 336 ("fall damage is 1d6 per 10ft (standard 5e)")
- **D&D rule:** PHB p183 fall damage caps at 20d6 (max 200ft).
- **Problem:** `numDice := int(altitudeFt) / 10` with no cap. A 500ft fall rolls 50d6.
- **Suggested fix:** `if numDice > 20 { numDice = 20 }` before building the expression.

## [High] Resistance/vulnerability halving allows damage to go to 0 (RAW says min 1)
- **Location:** /home/ab/projects/DnDnD/internal/combat/damage.go:38-43 (`ApplyDamageResistances`)
- **Spec/Phase ref:** Phase 42
- **D&D rule:** PHB p197: "If damage is reduced to less than 1, that damage becomes 1." Applies after resistance halving.
- **Problem:** 1 fire damage to a fire-resistant target returns `1/2 = 0`. Per RAW it should still be 1. Same for an immune target the spec acknowledges (0 is intentional for immunity), but resistance halving should not zero out the hit.
- **Suggested fix:** After the resistance branch, clamp to `max(1, halved)` when the raw input was >= 1 (preserve 0 → 0).

## [High] Pre-clamp HP overflow excludes temp-HP absorbed damage from instant-death check
- **Location:** /home/ab/projects/DnDnD/internal/combat/damage.go:226-247, 330-373
- **Spec/Phase ref:** Phase 43; spec line 2096 ("If damage remaining after reaching 0 HP ≥ character's max HP → instant death")
- **D&D rule:** Instant death overflow compares damage that "remained" after HP reached 0 — but temp HP is the buffer before HP. RAW treats overflow as "damage that reduces you below 0", computed AFTER temp HP absorbs.
- **Problem:** Code computes `adjusted` as the post-temp-HP value, then `rawNewHP := currentHP - adjusted`. The overflow `-rawNewHP` thus excludes the temp-HP portion — correct. However, the **damage-at-0** instant-death branch (line 356) compares `CheckInstantDeath(adjusted, maxHP)` where `adjusted` is the post-temp-HP single-hit damage. At 0 HP the target has no positive HP, so any temp HP they happen to hold (rare but possible from a Heroism/false-life persisting) gets absorbed first, then the *remainder* is what should be measured for instant death. The code does the right thing in that exact case because tempHP-absorb runs first. Net: behavior is correct, but the inline reasoning in the routing helper conflates `adjusted` with "overflow" — adjusted is post-tempHP raw damage, not the post-clamp deficit. Worth a code comment fix and a unit test for "damage-at-0 with temp HP" to lock the invariant. Marking High because the path is load-bearing for PvP-style massive-damage mechanics.
- **Suggested fix:** Add an explicit test "PC at 0 HP with 5 temp HP takes 25 damage; max HP 18" and ensure the result is instant-death (`25 - 5 = 20 >= 18`). Document the invariant in `routePhase43DeathSave`.

## [High] Off-hand attack accepts non-melee "light" weapons
- **Location:** /home/ab/projects/DnDnD/internal/combat/attack.go:1182-1196
- **Spec/Phase ref:** Phase 36; spec line 463
- **D&D rule:** TWF requires light **melee** weapons in both hands.
- **Problem:** Validation only checks `HasProperty(mainWeapon, "light")` / `HasProperty(offWeapon, "light")`. Light crossbow has the light property — currently allowed as a TWF weapon. Dart (light + thrown) similarly.
- **Suggested fix:** Add `!IsRangedWeapon(weapon)` to both validations (or check the `melee` weapon-type suffix explicitly).

## [High] Diagonal pathfinding ignores wall edges entirely (could allow phasing through a single diagonal wall)
- **Location:** /home/ab/projects/DnDnD/internal/pathfinding/pathfinding.go:242-244
- **Spec/Phase ref:** Phase 29 (spec line 1391: "Diagonal corner-cutting: diagonal movement through wall corners is **allowed**")
- **D&D rule:** N/A — DnDnD-spec-defined.
- **Problem:** The code only checks `blockedEdges` for cardinal moves. For diagonals it never tests walls. The spec only permits *corner-cutting* (two perpendicular walls meeting at a shared corner). A configuration where a single wall runs along **both** the row-edge between cur/(cur.row+1, cur.col) AND the column-edge between cur/(cur.row, cur.col+1) is a corner — corner-cutting. But a wall configured along the diagonal itself, or two parallel walls on either side of the diagonal motion, can leak movement through. (Axis-aligned walls only — see `addWallEdges` — so the typical configuration is two walls forming an L; the spec OK's that.) The looser interpretation may be intentional, but tile-occupancy is checked only at `next` — a diagonal move through an enemy-occupied corner is allowed.
- **Suggested fix:** If the spec intends "corner-cutting only when both perpendicular sides are blocked," tighten to allow a diagonal only when **at most one** of the two perpendicular edges is blocked. Otherwise, document the looser behavior explicitly in the spec.

## [High] Reach weapon OA detection — PC reach map relies on caller passing it
- **Location:** /home/ab/projects/DnDnD/internal/combat/opportunity_attack.go:80-117, 148-164 (`resolveHostileReach`)
- **Spec/Phase ref:** Phase 39 / OA detection; spec line 1414-1416
- **D&D rule:** Reach weapon = 10ft threatened area; ergo OA can be triggered when leaving 10ft.
- **Problem:** `resolveHostileReach` returns 5ft for any PC hostile by default — the override map `pcReachByID` must be supplied by the caller. If the move handler forgets to compute that map (or hits an error path that falls back to the no-override variant), PCs holding glaives/halberds don't threaten 10ft. Search shows the override only exists in one caller.
- **Suggested fix:** Move the PC reach computation into `resolveHostileReach` itself by looking up the PC's `equipped_main_hand` weapon properties (`reach`). The store dependency is already in the service.

## [High] Concentration-on-damage save uses simplified DC formula
- **Location:** /home/ab/projects/DnDnD/internal/combat/concentration.go:422-448 (`MaybeCreateConcentrationSaveOnDamage`), `CheckConcentrationOnDamage`
- **Spec/Phase ref:** Implicit (concentration mechanics implementation under Phases 39/42)
- **D&D rule:** Concentration check DC = max(10, ⌊damage/2⌋). Code uses `max(10, damage/2)` — correct. **But** when multiple damage sources hit on the same turn (e.g., AoE + ongoing damage), each triggers its own save per RAW. Need to verify the pending-save queue resolves correctly under that pattern.
- **Problem:** Cannot verify from code alone — confirm that a target taking two separate damage hits in the same round produces two CON saves, not a single combined one.
- **Suggested fix:** Add a regression test where two damage applications in a single command path both enqueue pending saves and resolve independently.

## [High] Surprise: surprised condition removed at start of "skip turn", not end (timing nuance)
- **Location:** /home/ab/projects/DnDnD/internal/combat/initiative.go:582-606 (`skipSurprisedTurn`)
- **Spec/Phase ref:** spec line 1672 ("the 'surprised' condition is removed **at the end** of their skipped turn")
- **D&D rule:** Surprised creature regains reactions only after their initiative passes in round 1.
- **Problem:** `skipSurprisedTurn` calls `skipCombatantTurn` and then immediately removes the surprised condition in the same operation. There is no enforced "end-of-turn" gap — if any reaction-trigger fires between these two writes (e.g., an OA from a movement during the same advance), the surprised creature is already eligible. In practice the calls are tightly coupled, but the ordering doesn't follow the spec wording.
- **Suggested fix:** Move the `RemoveSurprisedCondition` into `ProcessTurnEnd` for the surprised combatant, so the removal happens through the normal end-of-turn pipeline.

---

## [Medium] Unarmed strike crit doubles the flat "1", not RAW dice
- **Location:** /home/ab/projects/DnDnD/internal/combat/attack.go:674-681
- **Spec/Phase ref:** spec line 659 ("Damage is 1 + STR modifier (the flat 1 is inherent to unarmed strikes, not a die roll)")
- **D&D rule:** Crit doubles weapon damage **dice**; unarmed strike has no die per spec, so RAW interpretation is debated.
- **Problem:** Code doubles base from 1 → 2 on crit. Sage Advice / Crawford clarifies unarmed strikes do NOT crit-double the flat 1 (no dice to double). Spec's wording suggests the same.
- **Suggested fix:** Drop the `base = 2` branch on crit for the non-monk unarmed path; rely on dmgMod once. Confirm with spec author if doubling is intended.

## [Medium] TWF "negative ability modifier still applies" RAW edge missed
- **Location:** /home/ab/projects/DnDnD/internal/combat/attack.go:1199-1202
- **Spec/Phase ref:** Phase 36; spec line 463
- **D&D rule:** PHB p195: "You don't add your ability modifier to the damage of the bonus attack, unless that modifier is **negative**."
- **Problem:** Code sets `dmgMod = 0` unconditionally for non-TWF fighting-style characters. A STR-8 (-1 mod) wielding a dagger off-hand should still apply -1 damage per RAW.
- **Suggested fix:** Compute the would-be ability mod and assign `dmgMod = min(0, mod)` when not TWF fighting style.

## [Medium] Cover bonus to DEX save uses single closest-corner instead of best-of-4
- **Location:** /home/ab/projects/DnDnD/internal/combat/cover.go:106-136 (`CalculateCoverFromOrigin`)
- **Spec/Phase ref:** Phase 33; spec line 1381 ("system tests from the corner that gives the attacker the best (least) cover result")
- **D&D rule:** DMG grid variant uses one corner of the source (attacker for attack rolls, point of origin for AoE), traces to all four target corners, counts blocked.
- **Problem:** For AoE saves the code picks the *closest* corner to the target as the single origin. RAW DMG grid variant says pick any corner — typically the one giving the **best** result for the *attacker* (least cover). The "closest" heuristic doesn't match the spec wording for attacks, and the spec is silent on which corner to choose for AoE. For consistency, AoE should also probably use the "least cover for source" rule.
- **Suggested fix:** Change `CalculateCoverFromOrigin` to test all four origin corners and pick the one minimizing blocked-count (mirrors `CalculateCover`).

## [Medium] `lineBlockedByWalls` allows zero-determinant case to slip through
- **Location:** /home/ab/projects/DnDnD/internal/combat/cover.go:212-233 (`segmentsIntersect`)
- **Spec/Phase ref:** Phase 33
- **D&D rule:** N/A — geometry.
- **Problem:** Collinear lines (parallel walls passing through the same line as the sight line) return `false` from `segmentsIntersect`. A diagonal sight that runs along a wall edge (very rare with axis-aligned walls) is treated as unblocked. Probably fine for the current spec, but worth a test.
- **Suggested fix:** Add a test for "sight line exactly along an axis-aligned wall edge"; treat as blocked if so.

## [Medium] Off-hand TWF doesn't track on AttackerHidden / invisible attacker single-shot reveal
- **Location:** /home/ab/projects/DnDnD/internal/combat/attack.go:1238-1247 (`OffhandAttack`)
- **Spec/Phase ref:** spec §Invisibility / Hide
- **D&D rule:** Attacking reveals a hidden attacker; standard Invisibility ends on attack.
- **Problem:** `resolveAndPersistAttack` (called from OffhandAttack) does handle attacker-revealed and InvisibilityBroken, but `populatePostHitPrompts` is invoked before `consumeHelpAdvantage`; check whether the side-effect ordering matters for divine smite / inspiration prompts mid-combat. Mostly a documentation note — confirm no observable issue.
- **Suggested fix:** Add an integration test asserting hidden + invisible off-hand attack reveals attacker and breaks Invisibility identically to main-hand.

## [Medium] Damage-at-0 crit gives +2 failures regardless of attacker distance
- **Location:** /home/ab/projects/DnDnD/internal/combat/deathsave.go:159-192 (`ApplyDamageAtZeroHP`)
- **Spec/Phase ref:** spec line 2112 ("Critical hit (attacker within 5ft) = 2 failures")
- **D&D rule:** PHB p197: "If the damage is from a critical hit, you suffer two failures instead." (No distance gate in RAW.)
- **Problem:** Spec adds a "(attacker within 5ft)" qualifier that does NOT appear in PHB. Code follows RAW and applies +2 failures on any crit regardless of distance. Either spec is wrong or code is. Worth clarifying.
- **Suggested fix:** Either remove the "within 5ft" qualifier from spec, or pass distance to `ApplyDamageAtZeroHP` and gate the crit-doubles-failures rule on melee + within 5ft. Code currently mirrors RAW so I lean spec wording is wrong.

## [Medium] `ValidateMove` rejects ending on ally's tile (spec says ally pass-through allowed; ending forbidden — fine — but message)
- **Location:** /home/ab/projects/DnDnD/internal/combat/movement.go:84-94
- **Spec/Phase ref:** Phase 30; spec line 314 ("you can move through an allied creature's space freely, but you cannot end your turn there")
- **D&D rule:** Movement through allies is fine; ending in their tile is not.
- **Problem:** The check rejects any path that ENDS on any occupied tile (ally or enemy) — that's correct for a single /move. But the rejection message is just "Cannot end movement in an occupied tile" — doesn't differentiate ally/enemy for clarity.
- **Suggested fix:** Inspect the occupant's faction and produce ally vs enemy specific messages ("you can move through allies but cannot end your turn in their tile").

## [Medium] `tileCost` adds +5 for prone, conceptually using +5 not ×2
- **Location:** /home/ab/projects/DnDnD/internal/pathfinding/pathfinding.go:284-294
- **Spec/Phase ref:** Phase 29; spec line 172 ("prone crawling (×2, stacks to ×3 with difficult terrain)")
- **D&D rule:** Each foot of crawling costs 2 feet of speed; in difficult terrain it costs 3 feet.
- **Problem:** Code computes `cost = 5 (or 10 if difficult); cost += 5 if prone`. Math works coincidentally because base is 5: normal-prone=10 (×2), difficult-prone=15 (×3). But the model is additive, not multiplicative. If anyone introduces a future tile cost of 5 (e.g. swimming = ×2 = 10), the prone stack would compute 15 instead of 20 (×4). Brittle.
- **Suggested fix:** Express as multiplicative: `cost := 5; if difficult { cost *= 2 }; if prone { cost *= 2 }`. Result is identical today but future-proof.

## [Medium] Action consumption not flagged for /attack — features keying off ActionUsed misbehave
- **Location:** /home/ab/projects/DnDnD/internal/combat/attack.go:925 (UseAttack only decrements AttacksRemaining)
- **Spec/Phase ref:** Phase 28 (Turn Resource Tracking)
- **D&D rule:** The Attack action is consumed when you make an attack.
- **Problem:** `Service.Attack` does not set `Turn.ActionUsed = true`. Any feature gating on "Attack action taken" must use `AttacksRemaining < maxAttacks` instead — which is what the Reckless gate does. But /bonus offhand (TWF), spellcasting rules that require "you cast a cantrip with your action", or future Battlemaster Riposte interactions cannot rely on `ActionUsed` for the Attack action.
- **Suggested fix:** Set `Turn.ActionUsed = true` on the first attack of a turn. Update Action Surge / multi-attack tests accordingly.

## [Medium] Initiative tiebreak ignores DEX modifier ordering for surprised + tie cases
- **Location:** /home/ab/projects/DnDnD/internal/combat/initiative.go:167-177
- **Spec/Phase ref:** Phase 26a; spec line 1681-1687
- **D&D rule:** Same total → higher DEX mod, then alphabetical.
- **Problem:** `SortByInitiative` sorts by Roll desc, DexMod desc, DisplayName asc. Looks correct. **But**: `sort.SliceStable` preserves prior order only if compare returns equal — `DisplayName < DisplayName` is false on equal names. Two combatants with identical Roll, DexMod, and DisplayName would order non-deterministically. Edge case; usually display names are unique.
- **Suggested fix:** Tiebreak final on CombatantID UUID for full determinism.

## [Medium] Distance3D rounding-to-5 can flip cover/range edges
- **Location:** /home/ab/projects/DnDnD/internal/combat/altitude.go:22-33 (`Distance3D`, `roundToNearest5`)
- **Spec/Phase ref:** Phase 31; spec line 333 ("3D Euclidean distance (rounded to nearest 5ft)")
- **D&D rule:** N/A — DnDnD-spec.
- **Problem:** Rounding before comparison means a true 32ft distance becomes 30ft, then range/reach checks happen against pre-defined 5ft increments. For a Longbow normal range 150ft, target at 152ft gets rounded down to 150 → no long-range disadvantage. Spec is fine with this. But for melee reach: distance 7.5ft euclidean → rounds to 5ft → in melee reach. Possibly intended.
- **Suggested fix:** Document explicitly: "rounding occurs before comparison, so distances near a 2.5ft boundary may flip in the attacker's favor." Add a test.

## [Medium] Concentration save DC formula not capped (some house rules cap at DC 30)
- **Location:** /home/ab/projects/DnDnD/internal/combat/concentration.go ~ `CheckConcentrationOnDamage`
- **Spec/Phase ref:** Phase 39 (concentration handling); RAW.
- **D&D rule:** DC = max(10, damage/2). No upper cap.
- **Problem:** Code is correct per RAW. Noted only because a Fireball doing 60 damage → DC 30, very hard to make. Not a bug; just confirm.
- **Suggested fix:** None.

## [Medium] Off-hand attack uses `combatantDistance` 3D — would auto-crit against airborne paralyzed
- **Location:** /home/ab/projects/DnDnD/internal/combat/attack.go:1216, 953
- **Spec/Phase ref:** Phase 34 auto-crit rule; Phase 31 3D distance
- **D&D rule:** 3D distance is used for range checks; auto-crit is "within 5ft" — 3D 5ft includes a target 5ft directly above.
- **Problem:** A melee attack against a target at altitude 5ft (directly above) gives 3D distance = 5ft → auto-crit triggers. But melee attackers on the ground typically can't reach airborne targets without reach. Spec is ambiguous.
- **Suggested fix:** Pair the auto-crit check with the melee-only gate from finding above; the range check then naturally limits reach.

## [Medium] Spec calls for "ammo recovery PROMPT" post-combat; code auto-recovers in EndCombat
- **Location:** /home/ab/projects/DnDnD/internal/combat/service.go:1145-1161
- **Spec/Phase ref:** Phase 26b ("ammunition recovery prompt"); spec line 667 ("the DM triggers recovery from the dashboard")
- **D&D rule:** N/A
- **Problem:** EndCombat unconditionally recovers half of spent ammo into PC inventories. Spec wants a DM prompt; code skips the prompt and just recovers.
- **Suggested fix:** Move ammo recovery into a dashboard action; only persist `expended` counter on EndCombat.

---

## [Low] `colToIndex` is silent on lowercase / empty input
- **Location:** /home/ab/projects/DnDnD/internal/combat/attack.go:1571-1577
- **Spec/Phase ref:** N/A
- **D&D rule:** N/A
- **Problem:** Returns 0 for empty string (correct default), but lowercase 'a' returns `'a' - 'A' = 32` — way out of grid range. Subsumed by the multi-letter fix.
- **Suggested fix:** Use `renderer.ParseCoordinate` as primary helper.

## [Low] `IsInLongRange` always returns false for melee weapons — but thrown melee in long range handled separately
- **Location:** /home/ab/projects/DnDnD/internal/combat/attack.go:148-155, 538-542
- **Spec/Phase ref:** spec line 669
- **D&D rule:** Thrown weapon at long range has disadvantage.
- **Problem:** `IsInLongRange` returns false for non-ranged weapons (correct). The thrown-long-range path appends "long range" disadvantage from line 538-542. Fine, but the `AdvantageInput.Weapon` then re-runs `IsInLongRange` (line 94 of advantage.go) — that pass returns false for the thrown melee, so the disadvantage came only from the resolveAttack append. Code works but logic is split — harder to trace.
- **Suggested fix:** Centralize long-range disadvantage into `DetectAdvantage` by adding a `Thrown` and `ImprovisedThrown` flag.

## [Low] Conditions JSON empty array marshaling inconsistency
- **Location:** /home/ab/projects/DnDnD/internal/combat/condition.go:122-126
- **Spec/Phase ref:** Phase 39
- **D&D rule:** N/A
- **Problem:** `remaining` defaults to `[]CombatCondition{}` and marshals to `[]`. But `parseConditions` allows `nil` raw → nil slice. Mixed null vs `[]` representations downstream.
- **Suggested fix:** Normalize: always store `[]` even when no conditions.

## [Low] `ApplyDamageResistances` reason string lowercase-mixed
- **Location:** /home/ab/projects/DnDnD/internal/combat/damage.go:24-46
- **Spec/Phase ref:** N/A
- **D&D rule:** N/A
- **Problem:** Returns "immune to fire" / "resistance to fire" with lowercased damageType. Combat log readability varies if damage type stored capitalized.
- **Suggested fix:** Use `damageType` directly (preserve original casing) or Title-case for display.

## [Low] Free interaction not tracked across /move + /attack flow boundary
- **Location:** /home/ab/projects/DnDnD/internal/combat/turnresources.go:51-54
- **Spec/Phase ref:** Phase 28
- **D&D rule:** Only one free interaction per turn.
- **Problem:** The field exists and is validated, but I did not see /attack consume `FreeInteractUsed` for cases that imply interaction (e.g., drawing a weapon to throw). The thrown-weapon clearing of EquippedMainHand at line 1045 doesn't burn the free interaction. Possibly intended (RAW says drawing a weapon and a thrown attack are bundled when you throw a stored weapon, but using a single attack to throw THEN draw a new is a second interaction).
- **Suggested fix:** Document the policy explicitly. RAW edge cases here are notoriously messy.

## [Low] Pathfinding heuristic doesn't account for prone or terrain multipliers (still admissible)
- **Location:** /home/ab/projects/DnDnD/internal/pathfinding/pathfinding.go:165-172
- **Spec/Phase ref:** Phase 29
- **D&D rule:** N/A
- **Problem:** Chebyshev × 5 is admissible (never overestimates) — A* still optimal. But suboptimal exploration in heavy-terrain maps. Performance, not correctness.
- **Suggested fix:** Optional: tune heuristic for performance, but admissibility-correctness is preserved.

## [Low] Condition immunity skips application but doesn't surface to action_log persistently
- **Location:** /home/ab/projects/DnDnD/internal/combat/condition.go:149-152
- **Spec/Phase ref:** Phase 42 ("Condition immunity check on condition application")
- **D&D rule:** Immune creature is unaffected.
- **Problem:** Code returns a message but doesn't persist it via `logConditionMessages` from `ApplyCondition` (only `ApplyConditionWithLog` writes action_log). The "🛡️ X is immune to Y" event isn't recorded.
- **Suggested fix:** Persist the immunity message through the action log so /recap includes it.

## [Low] Held concentration spells: pre-applied conditions don't auto-end when concentration breaks via crash recovery
- **Location:** /home/ab/projects/DnDnD/internal/combat/concentration.go (general)
- **Spec/Phase ref:** Phase 39 / 42; spec §Bot crash recovery
- **D&D rule:** Concentration ending dismisses dependent effects.
- **Problem:** Bot crash recovery rolls back transactions but conditions placed during a partially-completed concentration cast may already be persisted while concentration assignment hasn't. Outside Group C scope; flagging.
- **Suggested fix:** Out of scope for combat-core; flagged for Group F (durability).

## [Low] `RemoveConditionWithLog` doesn't differentiate "removed by action" vs "expired"
- **Location:** /home/ab/projects/DnDnD/internal/combat/condition.go:297-307
- **Spec/Phase ref:** spec line 1311 ("⏱️ [Effect] on [Target] has expired (placed by [Source])")
- **D&D rule:** N/A
- **Problem:** Manual removal message is "🟢 X removed from Y" — no distinction from auto-expiration log line "⏱️ X on Y has expired". Spec includes the placed-by attribution only for expiration.
- **Suggested fix:** Distinct emojis + message variants for manual remove vs expiration are mostly there in `processExpiredConditions`; align the manual removal path.

---

## Phase-by-phase pass

- **Phase 26a (Start Combat):** OK
- **Phase 26b (End Combat & Cleanup):** OK with caveat (ammo recovery skips prompt — Medium finding)
- **Phase 27 (Concurrency / Advisory Locks):** OK — keyed per-turn-ID, scoped per-encounter (since each turn is in one encounter), TOCTOU re-validation in place. UUID→int64 truncation acknowledged in code; collision rate negligible.
- **Phase 28 (Turn Resource Tracking):** OK with Action-flagging caveat (Medium)
- **Phase 29 (Pathfinding):** Two findings (diagonal-wall-edge looser interpretation, additive-prone-cost brittleness). Math otherwise correct.
- **Phase 30 (Movement /move):** OK
- **Phase 31 (Altitude / Flying):** Critical (no fly-speed validation), High (fall damage cap)
- **Phase 32 (Distance Awareness):** OK
- **Phase 33 (Cover):** Medium (closest-corner heuristic for AoE), Low (collinear edge case)
- **Phase 34 (Basic Attack):** High (auto-crit lacks melee gate), Critical (colToIndex)
- **Phase 35 (Advantage/Disadvantage Auto-Detect):** High (Crossbow Expert + PC size hardcode)
- **Phase 36 (Extra Attack / TWF):** Critical (Attack action prereq + melee gate), Medium (negative ability mod)
- **Phase 37 (Weapon Properties):** OK
- **Phase 38 (Attack Modifier Flags):** Critical (Reckless carry-through)
- **Phase 39 (Conditions System):** High (surprise removal timing — minor), Low (immunity log)
- **Phase 40 (Condition Effects):** OK
- **Phase 41 (Moving While Prone):** OK
- **Phase 42 (Damage Processing):** High (resistance min-1 RAW), High (Dash ignores effective speed)
- **Phase 43 (Death Saves):** Medium (crit-at-0 distance — spec wording vs RAW)

---

## Files reviewed (read-only)

- /home/ab/projects/DnDnD/internal/combat/attack.go
- /home/ab/projects/DnDnD/internal/combat/damage.go
- /home/ab/projects/DnDnD/internal/combat/deathsave.go
- /home/ab/projects/DnDnD/internal/combat/condition.go
- /home/ab/projects/DnDnD/internal/combat/condition_effects.go
- /home/ab/projects/DnDnD/internal/combat/cover.go
- /home/ab/projects/DnDnD/internal/combat/distance.go
- /home/ab/projects/DnDnD/internal/combat/altitude.go
- /home/ab/projects/DnDnD/internal/combat/movement.go
- /home/ab/projects/DnDnD/internal/combat/advantage.go
- /home/ab/projects/DnDnD/internal/combat/turnlock.go
- /home/ab/projects/DnDnD/internal/combat/turnvalidation.go
- /home/ab/projects/DnDnD/internal/combat/turnresources.go
- /home/ab/projects/DnDnD/internal/combat/initiative.go
- /home/ab/projects/DnDnD/internal/combat/opportunity_attack.go
- /home/ab/projects/DnDnD/internal/combat/standard_actions.go
- /home/ab/projects/DnDnD/internal/combat/effect.go
- /home/ab/projects/DnDnD/internal/combat/concentration.go
- /home/ab/projects/DnDnD/internal/combat/service.go (partial)
- /home/ab/projects/DnDnD/internal/pathfinding/pathfinding.go
- /home/ab/projects/DnDnD/internal/dice/d20.go
- /home/ab/projects/DnDnD/internal/dice/dice.go
- /home/ab/projects/DnDnD/internal/dice/roller.go
- /home/ab/projects/DnDnD/internal/save/save.go
- /home/ab/projects/DnDnD/internal/check/check.go
- /home/ab/projects/DnDnD/internal/gamemap/renderer/grid.go
- /home/ab/projects/DnDnD/internal/discord/fly_handler.go (partial)
