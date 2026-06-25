# Issues Log — live play

Bugs, rough edges, and surprises found while running real games through the app.
One entry per issue. This is a **DM-side field journal**, distinct from the
AI-playtest harness's formal bug ledger — log freely here; promoting an issue to a
fixed + regression-tested item is a separate decision.

Status: `OPEN` · `WORKAROUND` · `FIXED` · `WONTFIX` · `INFO` (not a bug, just a note).

| # | Date | Area | Severity | Status | Summary |
| --- | --- | --- | --- | --- | --- |
| ISSUE-001 | 2026-06-24 | builder / spellcasting | major | FIXED | L3 warlock builder offers only cantrips — no leveled "spells known" selectable (Pact Magic ignored in max-spell-level derivation). |
| ISSUE-002 | 2026-06-24 | builder / persistence | major | FIXED | Full/half-caster `spell_slots` dropped at creation — `CreateCharacterRecord` never set it → portal-built wizard/cleric/etc. **could not cast leveled spells**. Fixed: persist standard slots in the canonical string-keyed `{current,max}` shape the `/cast` reader expects. |
| ISSUE-003 | 2026-06-24 | builder / spellcasting (frontend) | major | FIXED | Eldritch Knight (Fighter) & Arcane Trickster (Rogue) not recognized as casters by the frontend → **Spells step skipped entirely**. Fixed: subclass-aware `isSpellcaster`/budgets in JS + Go (INT third-casters from L3, EK/AT cantrip + spells-known + leveled tables); Spells step now shows with correct caps; server validation accepts the selections. |
| ISSUE-004 | 2026-06-24 | builder / AC | major | FIXED | Unarmored Defense never wired: builder never set `ac_formula`, so Barbarian (10+DEX+CON) & Monk (10+DEX+WIS) got **AC = 10+DEX**. Fixed: `unarmoredDefenseFormula` derives `"10 + DEX + CON"`/`"10 + DEX + WIS"` (the form `CalculateAC`/combat `RecalculateAC` parse, not the seed label) for unarmored barb/monk; fed into `DeriveStats` AC + persisted as `ac_formula`. Monk's UD voids shield bonus; armored falls back to armor AC. |
| ISSUE-005 | 2026-06-24 | builder / proficiency | minor→major | FIXED | Expertise (Rogue/Bard) never wired: combat reads an `"expertise"` proficiency key but the builder never collects it and `character.Proficiencies` has no Expertise field → wrong skill modifiers in play. **Fixed (TDD, `main` 6806bde):** added `Expertise []string` + `JackOfAllTrades` to `character.Proficiencies` (the JSONB `expertise` key `standard_actions.go:567` parses; `SkillModifier` `modifiers.go:25` doubles when a skill is in both expertise+proficient sets); builder collects N expert skills from proficient skills (Rogue L1=2, Bard L3=2) and persists them via `CreateCharacterRecord`; dashboard sheet + a latent levelup round-trip drop also closed. No schema change. Svelte bundle rebuilt. 452 vitest + cover-check green. (Out of scope: thieves'-tools expertise, ddbimport.) |
| ISSUE-006 | 2026-06-24 | builder / spellcasting | minor | FIXED | Level-1 Paladin/Ranger get a phantom L1 spell slot — `CalculateSpellSlots` half path uses `(level+1)/2` → 1 at L1 (half-casters get nothing until L2). Masked in the builder UI by an independent leveled-cap of 0, but wrong `spell_slots`/`max_spell_level` is stored and consumed elsewhere. **Fixed (TDD, `main` 558b2d4):** half-caster branch early-returns `nil` below level 2 (L1 Paladin/Ranger → no slots, max spell level 0); L2+ unchanged (L2 2×L1, L3 3×L1, L5 4×L1+2×L2). Downstream derive_stats / levelup verified. cover-check green. |
| ISSUE-007 | 2026-06-24 | builder / spellcasting (frontend) | unknown | OPEN (unconfirmed) | Multiclass spell *budget* in the UI (`cantripsKnown`/`leveledSpellCap`) computed from `classEntries[0]` only → secondary classes ignored, budget can be too low. Server `max_spell_level` is correct. Needs check on whether multiclass is exposed in the player builder. |
| ISSUE-008 | 2026-06-24 | builder / persistence | blocker | FIXED (adapter) | Portal submit 500s — `characters.languages` is `TEXT[] NOT NULL`, builder sends no languages, `pq.Array(nil)` → SQL NULL → constraint violation. Blocked **all** portal builds. Coerced nil→`[]` in `CreateCharacterRecord`. Underlying collection gap tracked as ISSUE-009. |
| ISSUE-009 | 2026-06-24 | builder / language selection | minor | OPEN | Builder collects **no concrete languages** — `backgrounds.js` carries only a *count* of bonus languages, never the strings. Characters persist with an empty language list instead of race base + background + chosen languages. Cosmetic today (languages unused in combat); fix = add a language-selection step + populate `submission.Languages`. |
| ISSUE-010 | 2026-06-24 | levelup / persistence | major | OPEN | Level-up persists `spell_slots` as `map[int]int` → `{"1":4}` (`levelup/levelup.go:14`, `service.go:243`), but the cast reader `ParseSpellSlots` (`combat/divine_smite.go:71`) unmarshals into `map[string]SlotInfo` (`{current,max}`). `{"1":4}` fails to unmarshal → `/cast` errors. Surfaced while fixing ISSUE-002. Needs a runtime confirm, but the shape mismatch is clear in code. Fix = level-up should write the `{current,max}` shape (or share the portal converter). |
| ISSUE-011 | 2026-06-25 | builder / equipment (frontend) | major | OPEN | Portal-built characters persist with **nothing equipped** — `equipped_main_hand`/`off_hand`/`armor` empty, all inventory items `equipped:false` — even when the player equips a weapon/armor in the builder. Breaks `/attack` (no weapon), armor AC, and the card "Equipped" row. Root cause is **frontend** (Go ingest + adapter persist `EquippedWeapon`/`WornArmor` fine): `CharacterBuilder.svelte` holds equip as two scalar dropdowns whose `$effect`s (`:444-452`) silently reset the pick to `''` when the chosen id/category doesn't match the filtered option lists (`:427-442`, id-resolution `:373-405`) — e.g. a starting-pack item id/category mismatch. Minor Go gap: `EquippedOffHand` (shield) never written to `CreateCharacterParams` (`builder_store_adapter.go:187-214`). Workaround: player runs `/equip` in Discord (items are in inventory). |
| ISSUE-012 | 2026-06-25 | character card / spellcasting | minor | OPEN | Discord character card + `/character` embed show **"Spell Slots: —" for warlocks** — they read only the `spell_slots` column and never fall back to `pact_magic_slots`. Card: `charactercard/format.go:79-82` + `service.go:223-226` (no pact field on `CardData`). `/character`: `discord/character_handler.go` `buildCharacterEmbed` has no spell-slot/pact row at all. Cosmetic-to-Discord only — combat `/cast` reads pact slots correctly (`combat/spellcasting.go:447-458`) and the web sheet renders a Pact Magic block (`portal/character_sheet_handler.go:396-401`). Fix = make the slot rows pact-aware. |

---

## Details

### ISSUE-001 — Warlock builder shows only cantrips (Pact Magic not derived)
- **Date:** 2026-06-24
- **Area:** portal character builder / spellcasting
- **Severity:** major — a warlock built via the web builder cannot pick any leveled
  spell, only cantrips. Renders the class' core mechanic unusable from the UI.
- **Status:** OPEN
- **Repro:** Build a single-class warlock (level ≥ 1, observed at level 3) in
  `/portal/create`. On the Spells step, cantrips (level 0) are selectable but all
  level 1–2 spells are unselectable/greyed.
- **Expected:** A level-3 warlock selects 2 cantrips **and** 4 known spells of
  level ≤ 2 (Pact Magic slot level at L3 = 2).
- **Actual:** Only cantrips selectable.
- **Root cause (verified):** Pact Magic is not folded into the builder's max
  spell level.
  - `character.CalculateSpellSlots` returns `nil` for a single-class warlock:
    the "half" branch is skipped (warlock is `"pact"`), then
    `CalculateCasterLevel` maps `"pact"` → 0 (`internal/character/spellslots.go:68`,
    `:129-145`).
  - The builder derives `MaxSpellLevel` solely from those (nil) slots →
    stays `0` (`internal/portal/derive_stats.go:97-103`).
  - Frontend: `levelsUpTo(0)` → `[]`, so `SpellPicker.isLevelSelectable` rejects
    every leveled spell while cantrips pass unconditionally
    (`portal/svelte/src/lib/spellcasting.js`, `.../spell-picker.js`).
  - `character.PactMagicSlotsForLevel` (`spellslots.go:112-124`) computes the
    correct pact slot level but is **never called** on this path.
- **Not a data bug:** warlock leveled spells are seeded — `SELECT level, count(*)
  FROM spells WHERE 'warlock' = ANY(classes) GROUP BY level;` → 9 at L1, 12 at L2,
  14 at L3, …
- **Fix idea:** Fold pact slot level into `MaxSpellLevel` for pact casters in
  `derive_stats.go` (consult `PactMagicSlotsForLevel`). Also verify the final
  character-create path actually persists `pact_magic_slots` so the built warlock
  can cast in play (separate from the UI gate). TDD + `make cover-check`.
- **Workaround:** finish the build cantrips-only and inject known spells +
  `pact_magic_slots` directly in the DB, or just fix it.
- **FIX (2026-06-24, TDD, on `main` working tree — not yet committed):** wired
  Pact Magic into the builder.
  - `internal/portal/derive_stats.go`: added `PactMagicSlots` to `DerivedStats` +
    a `pactMagicSlotsForClasses` helper; `DeriveStats` now raises `MaxSpellLevel`
    to the pact slot level (via `character.PactMagicSlotsForLevel`) for pact
    casters, combining with standard slots via max for multiclass.
  - `internal/portal/builder_store_adapter.go`: `CreateCharacterRecord` now
    persists `pact_magic_slots` for pact casters (non-warlocks unaffected).
  - Tests: 6 new red→green cases in `derive_stats_test.go` +
    `builder_store_adapter_test.go` (L3 warlock → MaxSpellLevel 2 + slots
    `{2,2,2}`; warlock/wizard multiclass → 3; non-casters nil; persistence).
  - `make cover-check` green (overall 90.63%, portal 88.61%). App rebuilt +
    restarted so the fix is live.

### ISSUE-002 — Standard-caster spell_slots may not persist at creation (UNCONFIRMED)
- **Date:** 2026-06-24
- **Area:** portal character builder / persistence
- **Severity:** unknown (potentially major if portal-built casters can't cast)
- **Status:** OPEN — **unconfirmed**, surfaced while fixing ISSUE-001.
- **Observation:** `BuilderStoreAdapter.CreateCharacterRecord`
  (`internal/portal/builder_store_adapter.go`) sets `PactMagicSlots` (after the
  ISSUE-001 fix) but never sets the generated `refdata.CreateCharacterParams.
  SpellSlots`, even though `DeriveStats` computes `SpellSlots` for full/half
  casters. Read paths appear to read the stored `spell_slots` column
  (`cmd/dndnd/dashboard_apis.go:324`).
- **To confirm:** build a wizard/cleric via the portal, approve, and check
  whether `/cast` / the sheet shows spell slots. If empty → real bug; fix by
  persisting `DeriveStats.SpellSlots` in the adapter (mirroring the pact fix). If
  slots appear → they're derived on read somewhere; close as INFO.

### ISSUE-004 — Unarmored Defense AC never wired (Barbarian/Monk) (FIXED)
- **Date:** 2026-06-24
- **Area:** portal character builder / AC derivation + persistence
- **Severity:** major — unarmored Barbarian/Monk got AC = 10 + DEX (missing
  CON/WIS), wrong at creation and at every combat AC recompute.
- **Status:** FIXED (TDD, `main`).
- **Root cause:** `DeriveStats` called `CalculateAC(..., "")` with an empty
  formula and `CreateCharacterRecord` never set `ac_formula`; combat
  `RecalculateAC` (`internal/combat/equip.go:387-419`) reads only `char.AcFormula`
  for unarmored defense. Only the Discord REST + DDB paths wrote it before.
- **Contract correction:** the live `ac_formula` value is the token form
  **`"10 + DEX + CON"` / `"10 + DEX + WIS"`** parsed by `evaluateACFormula`
  (`internal/character/stats.go:98`, mirrored in `equip.go:450`) — NOT the seed
  `mechanical_effect` label `ac_10_plus_dex_plus_con` (that label only drives
  feature definitions). A shield adds +2 unless the formula contains `WIS`
  (Monk UD voids it) — identical guard in `stats.go:70` and `equip.go:417`.
- **Fix:** `unarmoredDefenseFormula(classEntries, wornArmor, hasShield)` in
  `derive_stats.go` returns the CON form for an unarmored barbarian (shield ok),
  the WIS form for an unarmored, shieldless monk, else `""` (multiclass barb+monk
  prefers barbarian). `DeriveStats` feeds it to `CalculateAC`; `CreateCharacterRecord`
  persists it as `sql.NullString` (NULL for armored/non-UD). Tests in
  `derive_stats_test.go` + `builder_store_adapter_test.go` (barb 15, monk 15,
  barb+shield 17, armored barb → armor AC, fighter unchanged; persistence cases).
  `make cover-check` green (portal 89.30%). `DeriveAC` left untouched (no live
  callers).

### ISSUE-003 — EK/AT not recognized as casters in the builder (FIXED)
- **Date:** 2026-06-24
- **Area:** portal character builder (frontend gate + Go validation)
- **Severity:** major — an Eldritch Knight (Fighter) or Arcane Trickster (Rogue)
  built via the web builder got **no spell picker** (Spells step skipped). Worse
  than the warlock bug (warlock at least showed cantrips).
- **Status:** FIXED (TDD, `main`).
- **Root cause:** `CASTER_ABILITY` / `isSpellcaster` (`portal/svelte/src/lib/
  spellcasting.js`) keyed only on base class → `isCaster` false for fighter/rogue
  → `builder-steps.js` hid/skipped the Spells step. The Go spell-budget
  (`internal/portal/spellbudget.go`, used by `validateSpellCount`) likewise
  returned 0 for fighter/rogue, so even a shown picker would have been rejected on
  submit. Server `max_spell_level` (via `isThirdCasterSubclass` →
  `CalculateCasterLevel`) was already correct and untouched.
- **Fix:** made both sides subclass-aware. JS: `isThirdCaster(subclass, level)`
  (EK/AT slugs, level ≥ 3 = INT caster), `isSpellcaster`/
  `spellcastingAbilityForClass`/`cantripsKnown`/`leveledSpellCap` fall through to
  third-caster tables (EK 2→3 cantrips, AT 3→4, shared spells-known table);
  threaded subclass + level into `CharacterBuilder.svelte`. Go: mirrored
  `isThirdCaster` + third-caster tables in `spellbudget.go`; `spellCountCap`
  (`builder_service.go`) no longer bails for `SlotProgression=="none"` when EK/AT.
  Tests: Go `spellbudget_test.go` (EK/AT budgets + `validateSpellCount`), JS
  `spellcasting.test.js` (EK/AT casters, plain fighter/EK-L2 not). `npm test`
  441/441, `make cover-check` green (portal 89.12%). **Svelte bundle rebuilt**
  (`vite build`) since `internal/portal/assets/` is git-tracked.

### ISSUE-002 — Full/half-caster spell_slots dropped at creation (FIXED)
- **Date:** 2026-06-24
- **Area:** portal character builder / persistence
- **Severity:** major — portal-built wizard/cleric/sorcerer/druid/bard/paladin/
  ranger stored with `spell_slots = NULL`; `/cast` rejected them (no slots).
- **Status:** FIXED (TDD, `main`).
- **Root cause:** `DeriveStats` computes `SpellSlots` but the adapter
  `CreateCharacterRecord` (`internal/portal/builder_store_adapter.go`) only
  persisted `pact_magic_slots`, never standard `SpellSlots` → SQL NULL. Read paths
  (`/cast` → `parseIntKeyedSlots` → `ParseSpellSlots`) trust the stored column.
- **Fix:** added `spellSlotsForClasses` (`internal/portal/derive_stats.go`) that
  reuses `character.CalculateSpellSlots` and emits the canonical **string-keyed
  `{current,max}`** shape (fresh caster starts full, `current==max`); set
  `SpellSlots` in `CreateCharacterRecord` (NULL for non-casters). 3 red→green
  tests (Wizard L3, Paladin L2, Fighter L3 non-caster). `make cover-check` green
  (portal 89.05%, overall 90.66%). Verified the shape matches `ParseSpellSlots`
  (`combat/divine_smite.go:71`) + the dashboard `map[string]character.SlotInfo`
  reader, not level-up's incompatible `map[int]int` (→ ISSUE-010).

### ISSUE-008 — Portal submit 500s: languages NOT NULL violated (FIXED at write; collection gap OPEN)
- **Date:** 2026-06-24
- **Area:** portal character builder / persistence
- **Severity:** blocker — every portal "submit for DM approval" failed with HTTP 500.
- **Status:** FIXED (write-side) · underlying language-collection gap OPEN.
- **Repro:** Build any character in `/portal/create`, submit. Bot/app log:
  `ERROR creating character error="creating character: ERROR: null value in
  column "languages" of relation "characters" violates not-null constraint
  (SQLSTATE 23502)"`.
- **Root cause:** `db/migrations/20260310120006_create_characters.sql:28` →
  `languages TEXT[] NOT NULL`. Chain: submission `Languages []string`
  (`builder_service.go:48`, json `omitempty`) → `CreateCharacterParams.Languages`
  (`builder_service.go:510`) → adapter `Languages: p.Languages`
  (`builder_store_adapter.go:178`) → `pq.Array(arg.Languages)`
  (`refdata/characters.sql.go:105`). The Svelte builder **never collects concrete
  language strings** — `backgrounds.js` only carries a *count* of bonus languages
  — so the slice is always nil. `pq.GenericArray.Value()` returns SQL NULL for a
  nil slice → constraint violation. Guaranteed 500 for all portal builds; only
  surfaced now because this is the campaign's first portal-built character.
- **Fix (2026-06-24, TDD, `main` working tree, not committed):** in
  `CreateCharacterRecord` coerce `nil` → `[]string{}` before the insert
  (`pq.Array([]string{})` writes `'{}'`, non-null). 2 red→green tests in
  `builder_store_adapter_test.go` (nil → empty array; provided langs pass
  through). `make cover-check` green (portal 88.70%). App rebuilt + restarted.
- **Follow-up:** the builder collects no concrete languages — tracked separately
  as **ISSUE-009**.

### ISSUE-009 — Builder collects no concrete languages (only a count)
- **Date:** 2026-06-24
- **Area:** portal character builder / language selection
- **Severity:** minor — cosmetic today (languages aren't consumed in combat), but
  every portal-built character has an empty language list. Surfaced by ISSUE-008.
- **Status:** OPEN.
- **Detail:** `portal/svelte/src/lib/backgrounds.js` models bonus languages as an
  integer *count* (`languages: 2`, rendered via `formatLanguages`) and the builder
  never turns race base languages or that count into concrete strings.
  `CharacterSubmission.Languages` (`internal/portal/builder_service.go:48`,
  json `omitempty`) is therefore always empty, so `characters.languages` persists
  as `'{}'` (post ISSUE-008 fix; was a 500 before).
- **Fix idea:** add a language-selection step to the builder — auto-include the
  race's base languages (the `races.languages TEXT[]` seed column,
  `db/migrations/20260310120003:30`), let the player pick N background/class bonus
  languages, and populate `submission.Languages`. Persist them through the
  existing `CreateCharacterRecord` path (already wired). TDD + `make cover-check`.

<!-- Append a section per issue:

### ISSUE-001 — <short title>
- **Date:** YYYY-MM-DD
- **Area:** setup / auth / dashboard / register / combat / map / narration / …
- **Severity:** blocker / major / minor / cosmetic
- **Status:** OPEN
- **Repro:** exact steps (commands, clicks, IDs).
- **Expected:** what should happen.
- **Actual:** what happened (paste bot/log output verbatim).
- **Workaround:** if any.
- **Notes / fix idea:** code pointer if known.
-->
</content>
