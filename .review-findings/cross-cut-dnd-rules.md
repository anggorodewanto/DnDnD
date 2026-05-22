# Cross-Cutting D&D 5e Rules Correctness Audit

Scope: read-only audit of math, formulas, and lookup tables across the
DnDnD Go backend, cross-checked against the PHB. Focused on duplication,
table accuracy, and arithmetic correctness — feature gaps that are simply
"not yet implemented" (XP-based leveling, carrying capacity, encumbrance
variant, multiclass spell-ability-per-class) are noted at the end rather
than as defects.

Findings are sorted Critical → High → Medium → Low.

---

## [Critical] Channel Divinity recharges on long rest, not short rest

- **Rule:** PHB p.59 (Cleric) / p.85 (Paladin) — "When you finish a
  **short or long rest**, you regain your expended use(s) of Channel
  Divinity."
- **Location:**
  `internal/combat/channel_divinity_integration_test.go:44`,
  `:379`, `:479` — every test seed for the `channel-divinity` feature
  marks `Recharge: "long"`. The rest service routes recharges by this
  field (`internal/rest/rest.go:225` short branch, `:401` long branch),
  so any character built with the existing test fixture or any
  caller that copies the same seed will only regain Channel Divinity on
  a long rest.
- **Expected:** `Recharge: "short"` so `Service.ShortRest` re-arms the
  pool at line 225 of `internal/rest/rest.go`.
- **Actual:** seed/test fixtures persist `"long"`, and there is no
  level-up / portal code that overrides the recharge cadence.
- **Problem:** Clerics and Paladins lose half their PHB Channel Divinity
  economy: in a typical dungeon day with two short rests and one long
  rest a Cleric 6+ would have 6 expected CD uses but only gets 2.
- **Suggested fix:** flip the recharge string to `"short"` in every
  fixture and any character-bootstrap code that initializes the
  `channel-divinity` feature use, and add a regression unit test in
  `internal/rest/rest_test.go` asserting CD is among
  `FeaturesRecharged` after `ShortRest`.

---

## [High] `routePhase43DeathSave` skips the drop-to-0 instant-death rule when overflow is exactly the limit but damage came from a hit at >0 HP and `adjusted` overshoots

- **Rule:** PHB p.197 Massive Damage — "If damage reduces you to 0 HP
  and there is damage remaining, you die instantly if the remaining
  damage equals or exceeds your hit point maximum."
- **Location:** `internal/combat/damage.go:336-346`,
  `internal/combat/deathsave.go:59-83`.
- **Expected:** overflow = damage that would have driven HP below 0
  (i.e. `-rawNewHP`). When overflow ≥ maxHP → instant death.
- **Actual:** the helper computes overflow only when `rawNewHP < 0`
  (line 339-341). If the hit takes the PC from > 0 directly to exactly
  0 (`rawNewHP == 0`), overflow is reported as 0 even when the actual
  damage was massive (e.g. 1 HP creature taking 200 damage that the
  store clamped to 0). `CheckInstantDeath(0, maxHP)` → false, so the PC
  enters the dying state instead of dying outright.
- **Problem:** edge case escapes Massive Damage in the common "killing
  blow" path. Easy to hit when a 1-HP NPC ally takes a crit greatsword
  swing (`> maxHP` damage clipped to 0).
- **Suggested fix:** compute `overflow := adjusted - int(target.HpCurrent)`
  whenever `target.HpCurrent > 0` (clamp to ≥ 0). That mirrors the PHB
  "remaining damage after HP drops to 0" definition without depending
  on the pre-clamp rawNewHP sign.

---

## [High] Multiclass spellcasting ability picks the highest score across classes

- **Rule:** PHB p.164 Multiclassing — "Each spell you know is associated
  with one of your classes, and you use the spellcasting ability of
  that class when casting the spell."
- **Location:** `internal/combat/spellcasting.go:1544-1557`
  (`resolveSpellcastingAbilityScore`), called from
  `internal/combat/spellcasting.go:567`.
- **Expected:** lookup the spell's source class (e.g. wizard for fire
  bolt, cleric for cure wounds) and use that class's
  `SpellcastingAbilityForClass` score.
- **Actual:** iterates every class, computes its ability score, and
  returns `max`. A Wiz1/Cle1 with INT 16 / WIS 18 casts *fire bolt*
  using WIS 18 (incorrect — should be INT 16).
- **Problem:** every saving-throw DC, spell attack roll, and damage
  modifier becomes wrong for multiclass casters, and the bug silently
  inflates power: optimizers will pick the higher of the two abilities
  every time.
- **Suggested fix:** plumb the spell's `Classes` slice (already on the
  refdata spell row) into the resolver, intersect with the caster's
  classes, and use the first match. Fall back to the existing "max"
  only when the spell isn't on any of the caster's class lists (e.g.
  feat-granted spells).

---

## [High] Attack roll always adds proficiency bonus regardless of weapon proficiency

- **Rule:** PHB p.194 Attack Rolls — "You add your proficiency bonus
  to your attack roll when you attack using a weapon with which you
  have proficiency."
- **Location:** `internal/combat/attack.go:103-106` (`AttackModifier`).
- **Expected:** add `profBonus` only when the character is proficient
  with the weapon (proficiencies are seeded on classes in
  `internal/refdata/seed_classes.go`).
- **Actual:** `AttackModifier` returns `ability + profBonus`
  unconditionally; no caller in `internal/combat/attack.go` consults
  `Proficiencies.Weapons` or the class weapon list before invoking it.
- **Problem:** a wizard wielding a longsword still gets +PB on the
  swing, breaking the PHB's principal class differentiator on weapons.
- **Suggested fix:** add a `proficient bool` parameter (or look up the
  character's weapon proficiencies inside `AttackModifier`) and gate
  the proficiency add on it. Seed data already records class weapon
  proficiencies; a thin helper `HasWeaponProficiency(char, weapon)`
  centralizes the lookup.

---

## [High] Paladin Channel Divinity max uses scale to 2 at level 15

- **Rule:** PHB p.85 Paladin class table — Paladin never gains a second
  Channel Divinity use; subclasses grant more *options*, not more
  *uses*. Only the Cleric's CD scales (1 at L2, 2 at L6, 3 at L18).
- **Location:** `internal/combat/channel_divinity.go:31-38`
  (`ChannelDivinityMaxUses` paladin branch), test fixed at
  `channel_divinity_test.go:49-50` (`{15, 2}`, `{20, 2}`).
- **Expected:** Paladin returns `1` at every level ≥ 3.
- **Actual:** Paladin returns `2` at level ≥ 15.
- **Problem:** doubles Paladin's CD budget at high tiers, allowing
  Sacred Weapon + Turn the Unholy (or any two of Devotion/Vengeance/
  Conquest's options) back-to-back — RAW Paladin must choose one per
  rest.
- **Suggested fix:** drop the `level >= 15 → 2` branch (return 1 for
  level ≥ 3, 0 otherwise) and update
  `TestChannelDivinityMaxUses_Paladin` accordingly. If the scaling is
  intentional homebrew, document it in `docs/` and flag explicitly so
  it isn't mistaken for PHB-RAW.

---

## [High] Action Surge max uses never scales to 2 at fighter level 17

- **Rule:** PHB p.72 Fighter class table — Action Surge has 2 uses at
  Fighter 17+.
- **Location:** every `action-surge` feature seed asserts `Max: 1`
  (e.g. `internal/rest/rest_test.go:84`, `:212`, `:272`;
  `internal/discord/rest_handler_test.go:79`, `:396`, `:556`, `:644`;
  `internal/combat/action_surge_test.go:22`). No level-up code raises
  `Max` to 2 — there is no equivalent of
  `combat.ChannelDivinityMaxUses` for Action Surge anywhere in
  `internal/levelup/` or `internal/combat/action_surge.go`.
- **Expected:** `Max = 1` at Fighter 2-16, `Max = 2` at Fighter 17-20.
- **Actual:** stays at 1 forever; high-tier fighters effectively lose
  one of their signature class features.
- **Suggested fix:** add `ActionSurgeMaxUses(fighterLevel int) int`
  (`1` at L2-16, `2` at L17+) in `internal/combat/action_surge.go`,
  and have the level-up service (`internal/levelup/service.go`) bump
  `featureUses["action-surge"].Max` when a Fighter crosses level 17.

---

## [Medium] `CalculateHP` only awards the level-1 max die to `classes[0]`

- **Rule:** PHB p.15 / p.164 Multiclassing HP — only the *first* class
  taken at character creation grants the max hit die at level 1; every
  subsequent multiclass dip uses the average/rolled value.
- **Location:** `internal/character/stats.go:21-47`.
- **Expected:** treat the original starting class (per character
  creation history) as the "first" class regardless of slice order.
- **Actual:** uses `classes[0]` from the in-memory slice. Any code path
  that rebuilds the class slice in a different order (multiclass dip
  prepended, level-up flow that sorts by name, etc.) silently shifts
  the max-HP-at-L1 bonus to the wrong class — a Wiz1→Fig1 character
  who has the slice reordered to `[Fighter, Wizard]` jumps from
  `6 + 5(=avg5) = 11` to `10 + 4(=avg) = 14` HP.
- **Suggested fix:** persist an explicit `IsStartingClass` flag on
  ClassEntry (or always pin the starting class to index 0 at write
  time) and key the max-die-at-L1 off that flag instead of slice
  position. Add a regression test that flips the order of
  `[]ClassEntry` and asserts identical HP.

---

## [Medium] Duplicate `AbilityModifier` implementations across packages

- **Rule:** PHB p.13 — modifier = floor((score − 10) / 2). Single
  formula, must be consistent for negative odd diffs (Go truncates
  toward zero).
- **Location:**
  - `internal/character/stats.go:124-130` (`character.AbilityModifier`)
  - `internal/combat/initiative.go:24-30` (`combat.AbilityModifier`)
- **Expected:** one canonical implementation reused everywhere.
- **Actual:** two near-identical implementations live in different
  packages. Both currently produce the same value, but the
  duplication makes it easy for a future tweak (e.g. honoring an
  ability-score cap of 30) to land in only one place. The combat
  package's `AbilityModifier` is used by spell save DC, spell attack
  mod, initiative, channel divinity, sacred weapon, etc., while the
  character version powers the character card and level-up.
- **Suggested fix:** delete `combat.AbilityModifier` and have combat
  call `character.AbilityModifier` (the import already exists in
  several files in combat/). Add a `golangci-lint` rule or a unit
  test that fails if both exist.

---

## [Medium] `combatant.AbilityScores` JSON keys are not normalized — `Get` only handles two cases per ability

- **Rule:** N/A (defensive correctness rather than PHB).
- **Location:** `internal/character/types.go:20-36`
  (`AbilityScores.Get`), `internal/combat/initiative.go:44-61`
  (`AbilityScores.ScoreByName`).
- **Expected:** treat the ability key case-insensitively so that
  formulas like `"10 + DEX + WIS"` and `"10 + dex + wis"` and mixed
  case all hit.
- **Actual:** `character.AbilityScores.Get` matches only `"str"` /
  `"STR"`. Mixed case (`"Str"`, `"sTR"`) falls through to `return 0`,
  silently subtracting the ability from AC/Save formulas. The combat
  variant uses `strings.ToLower` (correct), so the two structs diverge
  in case-handling behavior.
- **Suggested fix:** wrap with `strings.ToLower` before the switch in
  `character.AbilityScores.Get`. Add a unit test for the mixed-case
  formula path.

---

## [Medium] `evaluateACFormula` silently drops unknown tokens (DEX/DEX cap of medium armor not enforced)

- **Rule:** PHB p.144 — medium armor adds DEX mod, max +2. Heavy armor
  ignores DEX.
- **Location:** `internal/character/stats.go:91-115`
  (`evaluateACFormula`).
- **Expected:** unarmored-defense formulas (`"10 + DEX + CON"`,
  `"10 + DEX + WIS"`, `"13 + DEX"`) and only those should be valid
  inputs; anything else should error or fall back, not silently add
  zero.
- **Actual:** unknown tokens (e.g. a typo `"10 + DEXT"`) call
  `strconv.Atoi("DEXT")` → 0 with the error discarded; the formula
  evaluates to `10` rather than surfacing the typo. Most ACFormulas
  are seeded so unlikely to hit production, but the silent fallback
  hides data errors.
- **Suggested fix:** track parse success and surface an error to the
  caller (or log + return base 10) when a token is neither ability
  abbreviation nor numeric.

---

## [Medium] Pact magic slot table: levels 11-20 cap at slot level 5 — correct, but `Max` and `Current` are both updated even when the level didn't change

- **Rule:** PHB p.107 Warlock — pact slot level scales: L1=1st, L3=2nd,
  L5=3rd, L7=4th, L9-10=5th. L11+: 3 slots at 5th. L17-20: 4 slots at
  5th.
- **Location:** `internal/character/spellslots.go:67-103`. Verified
  against PHB table — all 20 rows match.
- **Expected:** matches PHB. ✓
- **Actual:** matches PHB. ✓
- **No action.** Recording here so future audits can skip
  re-verification.

---

## [Medium] `classHitDie` in rest service hard-codes class IDs; mis-named or homebrew classes fall through to d8

- **Rule:** PHB p.45-105 — each class has a fixed hit die.
- **Location:** `internal/rest/rest.go:486-499`.
- **Expected:** the function should mirror `character.HitDieValue` and
  the class refdata; missing classes should return an error or empty
  string, not silently default to d8.
- **Actual:** unknown class strings yield `"d8"` (line 498), so a
  homebrew class (or a typo like `"Barbaian"`) silently gets the bard
  die. There is also no entry for artificer (would default to d8,
  which is RAW-correct for artificer but coincidental).
- **Suggested fix:** look up the hit die through the same path the
  character card uses (`internal/refdata/seed_classes.go` exposes
  `hit_die` per class). Return `("", false)` for unknown classes so
  callers surface the data integrity problem.

---

## [Medium] Divine Smite undead/fiend bonus on crit doubles to +2d8 — RAW reading is ambiguous

- **Rule:** PHB p.85 — "The damage increases by 1d8 if the target is
  an undead or a fiend, to a maximum of 5d8." Errata clarified that
  the +1d8 is *in addition* to the base 5d8 cap (so smite on undead
  can roll 6d8 at slot 4+).
- **Location:** `internal/combat/divine_smite.go:59-68`
  (`SmiteDamageFormula`). Crit doubles total count after the undead
  add, so an undead crit at slot 4 yields `(5+1)*2 = 12d8`.
- **Expected:** acceptable per most published readings — the +1d8 for
  undead/fiend is a separate damage die and doubles on crit (5e crit
  doubles all attack-roll damage dice).
- **Actual:** matches that interpretation.
- **Problem:** none mechanical, but the implementation is also
  consistent with the JC tweet ruling. Flag here so future audits
  don't "fix" it back to single-die.
- **Suggested fix:** add a comment in
  `internal/combat/divine_smite.go:59` linking to the SAC ruling so
  the rationale survives refactors.

---

## [Medium] `SneakAttack` extra dice list never validated "once per turn"

- **Rule:** PHB p.96 — Sneak Attack triggers once per **turn** (not
  once per round, not once per attack), and the rogue must hit with a
  finesse or ranged weapon, with advantage on the roll, or with an
  ally within 5 ft of the target.
- **Location:** `internal/combat/feature_integration.go:86-106`
  (`SneakAttackFeature`).
- **Expected:** `OncePerTurn: true` flag is set and the FES engine
  must consume the slot at strike time.
- **Actual:** the flag is set; the rearm path lives in
  `internal/combat/initiative.go:683` (`clearUsedEffectsForCombatant`
  at turn-start). RAW is "your turn"; the implementation re-arms on
  the rogue's own turn-start, which matches RAW. ✓
- **Suggested fix:** No code change. Note here so reviewers don't
  flag this on a future pass.

---

## [Medium] Initiative tiebreak does not use DEX score (just DEX modifier)

- **Rule:** PHB p.189 — no formal tiebreak; many tables use "highest
  DEX score breaks ties" (DMG variant).
- **Location:** `internal/combat/initiative.go:166-177`
  (`SortByInitiative`).
- **Expected:** roll DESC, DEX **score** DESC, name ASC (most common
  table rule); alternatively, prompt the DM.
- **Actual:** roll DESC, DEX **modifier** DESC, name ASC. Two PCs with
  DEX 14 and DEX 15 both get +2 mod, so the alphabetic name decides
  even though the DEX-15 PC should win.
- **Suggested fix:** swap to comparing raw DEX scores when modifiers
  are tied, or document the tiebreak in `docs/playtest-checklist.md`
  so DMs know to override.

---

## [Medium] Pact magic slot recovery on short rest restores slot count but not slot level

- **Rule:** PHB p.107 — short rest "regains expended slot expenditure".
  Slot level should already match the warlock's current level.
- **Location:** `internal/rest/rest.go:235-241`.
- **Expected:** `Current = Max`. ✓
- **Actual:** matches. But `SlotLevel` is not refreshed against the
  warlock's current level — if a character levels up but their stored
  `PactMagicSlots.SlotLevel` lags behind (because the level-up path
  forgot to recompute), the short rest preserves the stale level.
- **Suggested fix:** in
  `internal/levelup/levelup.go:CalculateLevelUp` (around line 45-52),
  always rebuild PactMagicSlots from `PactMagicSlotsForLevel`. Verify
  callers persist the rebuilt struct.

---

## [Medium] `ApplyDamageAtZeroHP` does not itself enforce the Massive Damage rule

- **Rule:** PHB p.197 — "If the damage equals or exceeds your hit
  point maximum, you suffer instant death." Applies even when already
  at 0 HP and taking damage.
- **Location:** `internal/combat/deathsave.go:157-192`
  (`ApplyDamageAtZeroHP`).
- **Expected:** the helper itself should check massive damage and
  return `TokenDead` when damage ≥ maxHP.
- **Actual:** the helper unconditionally adds 1 or 2 failures. The
  enclosing `routePhase43DeathSave`
  (`internal/combat/damage.go:355-358`) does check Massive Damage
  *before* calling the helper, so the production path is correct.
  However, the helper's API is fragile — a future caller that
  doesn't wrap the call in the same guard will silently lose the
  instant-death case.
- **Suggested fix:** push the Massive Damage check into
  `ApplyDamageAtZeroHP` itself (accept `damage int, maxHP int`) and
  drop the redundant guard at the call site. The current crit→2
  failures branch should stay.

---

## [Low] `ProficiencyBonus(0)` returns 0; `ProficiencyBonus(21)` also returns 0

- **Rule:** PHB p.15 — proficiency bonus is defined only for levels
  1-20.
- **Location:** `internal/character/stats.go:134-139`.
- **Expected:** out-of-range returns 0 (matches code).
- **Actual:** ✓ — but the function silently returns 0 instead of
  erroring on invalid input.
- **Suggested fix:** non-critical. Callers should validate level
  before calling. Consider a build-time assertion in tests.

---

## [Low] `RollDeathSave` returns no `DeathSaves` on nat-20 (relies on caller resetting)

- **Rule:** PHB p.197 — "On a 20, you regain 1 hit point" (implicitly
  resets failures/successes since you're no longer dying).
- **Location:** `internal/combat/deathsave.go:88-98` returns an
  outcome without `DeathSaves`. Handler at
  `internal/discord/deathsave_handler.go:117-122` writes an empty
  `DeathSaves{}` on heal-to-1 path. ✓ Production behavior correct.
- **Expected:** zero tally persisted on heal-from-0.
- **Actual:** ✓ when the handler path is used. The API contract is
  implicit (caller knows to clear).
- **Suggested fix:** explicitly set `DeathSaves: DeathSaves{}` on the
  nat-20 branch so the outcome is self-describing and any future
  caller doesn't have to remember the "reset on heal" rule.

---

## [Low] Twinned Spell cost for cantrips is 1 SP but no AOE / single-target restriction is enforced

- **Rule:** PHB Sorcerer Metamagic — Twinned Spell can only target a
  spell that targets exactly one creature and is not an AOE.
- **Location:** `internal/combat/sorcery.go:38-50`
  (`SorceryPointCost`).
- **Expected:** Twinned Spell validation should reject spells with
  `area_of_effect` populated or with `targets > 1`.
- **Actual:** only the SP cost is computed; no spell shape check.
  Combat will accept Twinned Fireball.
- **Suggested fix:** in `ValidateMetamagic`, require the spell row's
  `AreaOfEffect.Valid == false` when "twinned" is selected, and
  block twinned-on-self spells.

---

## [Low] Diagonal-move cost uses PHB default (5 ft per diagonal)

- **Rule:** PHB p.192 — default is 5 ft per square (Chebyshev). DMG
  variant uses 5/10/5.
- **Location:** `internal/pathfinding/pathfinding.go:151-172` —
  heuristic & step cost both 5 ft on diagonals.
- **Expected:** PHB default (5 ft). ✓
- **Actual:** matches PHB default. ✓
- **No action.** Recording here so future audits skip.

---

## [Low] Bardic Inspiration die scaling — confirmed correct

- **Rule:** PHB p.53 Bard table — d6 at 1, d8 at 5, d10 at 10, d12 at
  15.
- **Location:** `internal/combat/bardic_inspiration.go:14-29`.
- **Expected:** matches PHB. ✓
- **Actual:** matches PHB. ✓
- **No action.**

---

## [Low] Wild Shape CR cap (standard + Circle of the Moon) — confirmed correct

- **Rule:** PHB p.66 / p.69. Standard druid: 1/4 at 2, 1/2 at 4, 1 at
  8. Moon druid: 1 at 2, level/3 (round down) at 6+.
- **Location:** `internal/combat/wildshape.go:56-73`.
- **Expected:** matches PHB. ✓
- **Actual:** matches PHB. ✓
- **No action.**

---

## [Low] Cover bonuses match PHB

- **Rule:** PHB p.196 — half cover +2 AC and DEX saves; three-quarters
  +5; full = no LoS.
- **Location:** `internal/combat/cover.go:31-47`.
- **Expected:** ✓
- **Actual:** ✓
- **No action.**

---

## [Low] Critical hit dice doubling correctly excludes the static modifier

- **Rule:** PHB p.196 — double the dice, add modifiers once.
- **Location:** `internal/dice/roller.go:98-124` (`RollDamage`) —
  doubles `expr.Groups[i].Count`, leaves `expr.Modifier` untouched.
  `internal/combat/attack.go:628-637` (`rollFESExtraDice`) also passes
  `critical=true` for sneak attack / smite extra dice (RAW: those dice
  double on crit).
- **Expected:** ✓
- **Actual:** ✓
- **No action.**

---

## [Low] Concentration save DC = max(10, floor(damage/2))

- **Rule:** PHB p.203.
- **Location:** `internal/combat/concentration.go:16-24`.
- **Expected:** ✓
- **Actual:** ✓
- **No action.**

---

## [Low] Cantrip dice multiplier (×2 at L5, ×3 at L11, ×4 at L17)

- **Rule:** PHB p.200.
- **Location:** `internal/combat/spellcasting.go:1258-1271`.
- **Expected:** ✓
- **Actual:** ✓
- **No action.**

---

## [Low] Class save proficiencies match PHB

- **Rule:** PHB class tables. Barb STR/CON; Bard DEX/CHA; Cleric
  WIS/CHA; Druid INT/WIS; Fighter STR/CON; Monk STR/DEX; Paladin
  WIS/CHA; Ranger STR/DEX; Rogue DEX/INT; Sorc CON/CHA; Warlock
  WIS/CHA; Wizard INT/WIS.
- **Location:** `internal/refdata/seed_classes.go` — every class's
  `SaveProficiencies` is checked.
- **Expected:** ✓
- **Actual:** ✓ for all 12 PHB classes.
- **No action.**

---

## [Low] 18-skill list and ability mapping

- **Rule:** PHB p.174.
- **Location:** `internal/character/types.go:203-222`
  (`SkillAbilityMap`).
- **Expected:** ✓ — 18 entries, all mapped to the correct ability.
- **Actual:** ✓
- **No action.**

---

## [Low] Rage damage bonus and uses/day match PHB

- **Rule:** PHB p.48.
- **Location:** `internal/combat/rage.go:13-43`.
- **Expected:** ✓ — +2/+3/+4 at L1/9/16; 2/3/4/5/6/unlimited uses by
  level.
- **Actual:** ✓
- **No action.**

---

## [Low] Monk martial-arts die scaling matches PHB

- **Rule:** PHB p.78.
- **Location:** `internal/combat/monk.go:504-523`.
- **Expected:** ✓ — d4/d6/d8/d10 at L1/5/11/17.
- **Actual:** ✓
- **No action.**

---

## [Low] Sneak attack dice count `(rogueLevel+1)/2` rounds up correctly

- **Rule:** PHB p.96.
- **Location:** `internal/combat/feature_integration.go:466-471`.
- **Expected:** L1→1, L2→1, L3→2, L4→2, …, L19→10, L20→10.
- **Actual:** ✓
- **No action.**

---

## [Low] Sorcery point slot creation costs match PHB

- **Rule:** PHB p.101 — 2/3/5/6/7 SP per L1-5 slot.
- **Location:** `internal/combat/sorcery.go:29-35`
  (`slotCreationCosts`).
- **Expected:** ✓
- **Actual:** ✓
- **No action.**

---

## [Low] Armor seed data matches PHB

- **Rule:** PHB p.145 armor table.
- **Location:** `internal/refdata/seeder.go:135-152`.
- **Expected:** ✓ — base AC, DEX bonus, DEX cap, STR requirements,
  weights, stealth-disadvantage flag all match.
- **Actual:** ✓
- **No action.**

---

## [Low] Unarmored Defense formulas evaluated at runtime via `evaluateACFormula`

- **Rule:** PHB. Barb: 10 + DEX + CON. Monk: 10 + DEX + WIS. Mage
  Armor: 13 + DEX. Sorcerer (draconic): 13 + DEX.
- **Location:** seeded as strings in feature `mechanical_effect`
  (`internal/refdata/seed_classes.go:24`, `:194`, `:353`) and
  evaluated by `character.evaluateACFormula`
  (`internal/character/stats.go:91`).
- **Expected:** ✓
- **Actual:** ✓ — when `armor == nil` the formula is taken;
  `max(baseAC, formulaAC)` is enforced (line 60-62 of
  `character/stats.go`) so unarmored defense never *reduces* AC.
- **No action.**

---

## [Low] Exhaustion level effects align with PHB

- **Rule:** PHB p.291. L1 disadv on checks, L2 speed ÷ 2, L3 disadv on
  attacks & saves, L4 max HP ÷ 2, L5 speed 0, L6 death.
- **Location:** `internal/combat/damage.go:75-112`.
- **Expected:** ✓
- **Actual:** ✓
- **No action.**

---

## [Low] Spell save DC and spell attack bonus formulas

- **Rule:** PHB p.205 — save DC = 8 + PB + ability mod; attack bonus
  = PB + ability mod.
- **Location:** `internal/combat/channel_divinity.go:85-89`
  (`SpellSaveDC`), `internal/combat/spellcasting.go:96-100`
  (`SpellAttackModifier`).
- **Expected:** ✓
- **Actual:** ✓
- **No action.**

---

## [Low] Multiclass caster level uses `level/2` for half / `level/3` for third casters

- **Rule:** PHB p.165 Multiclass Spellcaster table — half caster
  contribution = level ÷ 2 (round down), third = level ÷ 3 (round
  down).
- **Location:** `internal/character/spellslots.go:47-64`.
- **Expected:** ✓ — RAW says paladin/ranger contribute `floor(L/2)`
  (so a single-class L1 paladin gets no slots, an L1 paladin / L1
  cleric multiclass treats only the cleric level).
- **Actual:** ✓
- **No action.** Note: artificer is RAW `ceil(level/2)` (round up
  for multiclass), but no artificer class is seeded so the rule
  doesn't apply yet.

---

## [Low] ASI levels include the Fighter (6, 14) and Rogue (10) extras

- **Rule:** PHB. Standard ASI levels: 4, 8, 12, 16, 19. Fighter +6
  and +14. Rogue +10.
- **Location:** `internal/levelup/levelup.go:72-79`.
- **Expected:** ✓
- **Actual:** ✓
- **No action.**

---

# Feature gaps (not flagged as defects)

These PHB rules from the audit list are not implemented anywhere in the
backend. Whether they should be is a product question (the project is
"Discord-based async D&D") so they are recorded for completeness rather
than counted as findings.

- **XP thresholds for leveling** (300/900/2700/…). No XP system in
  any package; level-ups appear to be DM-awarded.
- **Carrying capacity (STR × 15)**, push/drag/lift (STR × 30), and the
  optional encumbrance variant (STR × 5 / × 10).
- **Per-spell class-source spellcasting ability** for multiclass
  characters (see High-severity finding above for the same root
  cause).
- **Twinned Spell single-target validation** (see Low-severity
  finding).
- **CR-based monster proficiency bonus table.** Stat blocks are
  authored individually so there is no derivation function — but no
  evidence one is needed.
- **Round-based long rest duration enforcement (8 h, 1 h short).** The
  rest service is called explicitly by the player and trusted; no
  in-game time gate.

---

# Summary

- **Critical:** 1 — Channel Divinity recharge fixture/seed always
  marks "long".
- **High:** 4 — drop-to-0 Massive Damage edge case; multiclass spell
  ability picks max; attack roll always adds PB; Paladin CD scales to
  2 at L15; Action Surge never scales to 2 at L17 (counted together
  as "Action Surge no level scaling" — one finding).
- **Medium:** 9 — duplicate AbilityModifier, multiclass-first-class HP
  ordering, AC formula silent token drop, mixed-case ability key
  handling, classHitDie fallback to d8, smite undead crit
  interpretation note, sneak attack rearm note (informational), init
  tiebreak DEX mod vs score, pact slot level refresh, massive damage
  inside `ApplyDamageAtZeroHP`.
- **Low:** ~15 confirmations (no action) plus a handful of doc /
  comment improvements.

Top five concerns are listed first in the file (Channel Divinity
recharge, drop-to-0 Massive Damage, multiclass spell ability, attack
proficiency bonus, Paladin/Action Surge max-use scaling).
