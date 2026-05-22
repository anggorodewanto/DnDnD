# Group H — Leveling + Import + Portal (Phases 89–92b)

Scope: Phases 89, 89d, 90, 91a, 91b, 91c, 92, 92b.
Codebase root: `/home/ab/projects/DnDnD`.
Spec ref: `docs/dnd-async-discord-spec.md` lines 2370–2516; `docs/phases.md` lines 517–557.

Findings sorted Critical → Low.

---

## [Critical] Single-class half-caster (Paladin/Ranger) gets the wrong slot count
- **Location:** /home/ab/projects/DnDnD/internal/character/spellslots.go:108
- **Spec/Phase ref:** spec §"Multiclass spell slots" line 2511 ("Single-class characters skip this entirely and use their class's own slot progression directly")
- **Problem:** `CalculateSpellSlots` always routes through `MulticastSpellSlots(casterLevel)` with `casterLevel = floor(classLevel/2)` for half-casters even for single-class characters. The multiclass table at caster level N is not the same as the half-caster's own table. Example: Paladin 3 → own table gives 3×1st-level slots; current code returns `MulticastSpellSlots(1) = {1:2}` → only 2 slots. Same shape error for Paladin 4 (own table 3 vs code 3 — coincidence), Paladin 5/6 (own 4+2 vs code's multiclass-3 = 4+2 OK), Paladin 7 (4+3+1 vs code 4+3 — missing 3rd-level slot). Single-class Rangers identical bug. Production Paladins/Rangers will be undercast across most levels.
- **Suggested fix:** Branch in `CalculateSpellSlots`: if `len(classes)==1 && progression=="half"`, look up a dedicated half-caster table (or compute as `MulticastSpellSlots(ceil(level/2))` per SRD half-caster column). Add tests covering Paladin 3, 5, 9, 13, 17, 19.

## [Critical] Feat prerequisites and "already-has-feat" exclusion not enforced anywhere in the live picker
- **Location:** /home/ab/projects/DnDnD/cmd/dndnd/discord_handlers.go:1155 (`asiFeatLister.ListEligibleFeats`)
- **Spec/Phase ref:** spec §"Feat path" lines 2486-2487 ("prerequisites checked automatically — e.g., Heavy Armor Master requires heavy armor proficiency, Ritual Caster requires INT or WIS 13+. … Feats the character already has are excluded.")
- **Problem:** The production FeatLister returns the first 25 feats alphabetically with **no prerequisite filtering** and **no exclusion of feats the character already has**. The `CheckFeatPrerequisites` helper exists in `internal/levelup/feat.go` but is never called in the player flow. The `asiServiceAdapter.ApproveASI` also doesn't run prereqs on approve. Players can pick any feat, even duplicates of existing ones or feats whose prereqs they don't meet.
- **Suggested fix:** Implement `ListEligibleFeats` to load the character's scores/proficiencies/spellcasting, run `CheckFeatPrerequisites` per feat, and exclude IDs already present in `Features` (Source=="feat"). Add an approval-time prereq re-check in `asiServiceAdapter.ApproveASI` (defense in depth, prevents race with concurrent score edits).

## [Critical] Level-up does not auto-add new class/subclass features
- **Location:** /home/ab/projects/DnDnD/internal/levelup/service.go:186 (`ApplyLevelUp`)
- **Spec/Phase ref:** spec §"Leveling workflow" lines 2453, 2471 ("⚔️ New feature: Extra Attack (2 attacks per action)")
- **Problem:** `ClassRefData.FeaturesByLevel` is loaded by the store adapter but never read in `ApplyLevelUp`. No code appends the class's level-N features (Extra Attack, Sneak Attack scaling, Channel Divinity, etc.) to `character.features` on level-up, so the Feature Effect System has no way to detect new mechanical effects. The private level-up notification likewise omits "New feature: …".
- **Suggested fix:** After computing `newClasses`, iterate `classRef.FeaturesByLevel` for the new level (and any skipped intermediate levels) and append to features (deduping by name+source). Surface the gained features in `LevelUpDetails.NewFeatures` and render them in `FormatPrivateLevelUpMessage`.

## [Critical] DDB import bypasses DM approval queue on first import
- **Location:** /home/ab/projects/DnDnD/internal/ddbimport/service.go:139
- **Spec/Phase ref:** spec §"D&D Beyond Import" line 2426 ("Character enters the DM approval queue; DM reviews and approves in the dashboard")
- **Problem:** On a fresh import (no existing DDB-URL row), `Service.Import` calls `CreateCharacter` **immediately**, mutating the DB before the DM has seen the preview. Re-syncs are correctly staged via `pending_ddb_imports`, but the first import inserts a live character row. Combined with `registration.Service.Import` adding a `pending` player_character row, the character record itself is committed before any review.
- **Suggested fix:** Mirror the re-sync path for first imports: stage the create-params in `pending_ddb_imports` keyed by a new id and only call `CreateCharacter` from `ApproveImport`. Or at minimum mark the character row inactive (`approval_status='pending_ddb'`) until DM clicks Approve.

## [Critical] Levelup HTTP handler does not bound newLevel to 20
- **Location:** /home/ab/projects/DnDnD/internal/levelup/handler.go:106
- **Spec/Phase ref:** spec §"Internal Character Format" / 5e level cap; spec validator already enforces 1-20 elsewhere
- **Problem:** `HandleLevelUp` rejects `newLevel < 1` but accepts any positive int. A DM (or buggy client) can set Fighter to level 99 and the service will compute and commit nonsense HP/proficiency/spell-slots — `ProficiencyBonus` will silently return 0 for level > 20, and `MulticastSpellSlots` will return nil, wiping spell slots. Also no check that newLevel > current (decrement allowed without explicit confirmation).
- **Suggested fix:** Reject `newLevel < 1 || newLevel > 20` at the handler. Reject sum-of-class-levels > 20 in `ApplyLevelUp` (multiclass cap). Consider rejecting decrements unless a `force=true` flag is set.

## [High] Player-identity not validated on ASI button / select interactions
- **Location:** /home/ab/projects/DnDnD/internal/discord/asi_handler.go:354,391,647,731
- **Spec/Phase ref:** spec §"ASI path" line 2484; Phase 89d "Player choices"
- **Problem:** `HandleASIChoice`, `HandleASISelect`, `HandleASIFeatSelect`, `HandleASIFeatSubChoiceSelect` extract `discordUserID(interaction)` and store it as the player ID but never check that the interacting user is the character owner. Any guild member who can see the `#your-turn` message can press the buttons and submit choices for another player's character. The Discord message itself is ephemeral, but the custom-id flow doesn't enforce that.
- **Suggested fix:** Resolve `ASICharacterData.DiscordUserID` at the start of each handler and reject (`respondEphemeral` "this prompt is for <user>") if `interaction.Member.User.ID != charData.DiscordUserID`.

## [High] DM approve/deny buttons have no role check
- **Location:** /home/ab/projects/DnDnD/internal/discord/asi_handler.go:456 (`HandleDMApprove`), 524 (`HandleDMDeny`)
- **Spec/Phase ref:** spec §"DM approval" line 2497 ("The DM reviews and clicks [Approve]")
- **Problem:** Anyone who can see `#dm-queue` (which may include players in some campaign setups) can click Approve or Deny — no check that the interacting user has the DM role for the campaign. ASI/feat approval is a privileged stat mutation.
- **Suggested fix:** Look up the campaign for the guild and verify `interaction.Member.User.ID` matches the campaign's DM Discord ID (or has the DM role). Use the same gating helper that DM-queue notifications use elsewhere.

## [High] ASI ApproveASI silently rejects feat type instead of routing
- **Location:** /home/ab/projects/DnDnD/internal/levelup/asi.go:35 (`ApplyASI`)
- **Spec/Phase ref:** spec §"Feat path" line 2499 (feat application updates features + ASI bonus)
- **Problem:** `ApplyASI` returns an "unsupported ASI type" error when `choice.Type == ASIFeat`. The discord adapter at `cmd/dndnd/discord_handlers.go:1848` does route feats to `ApplyFeat` before reaching the service, but the service-level `ApproveASI` (which is also called via the HTTP handler `/api/levelup/asi/approve`) passes `levelup.ASIChoice{Type: "feat"}` straight into `ApplyASI`, which errors. The HTTP path is therefore broken for feat approvals.
- **Suggested fix:** In `Service.ApproveASI`, branch on `choice.Type == ASIFeat` and route to `ApplyFeat`, or have the HTTP handler accept a separate `/feat/approve` endpoint and reject feat-typed payloads on the asi endpoint.

## [High] DDB "off-list spell" detection only covers wizard with 16 spells
- **Location:** /home/ab/projects/DnDnD/internal/ddbimport/parser.go:382 (`classSpellLists`)
- **Spec/Phase ref:** spec §"Import validation" line 2434 ("Wizard spell list includes Cure Wounds (not on Wizard spell list)")
- **Problem:** The hard-coded `classSpellLists` map contains only `"wizard"` with 16 spells. Every other class (cleric, druid, bard, sorcerer, warlock, paladin, ranger) has no entry, so `isOffListClassSpell` returns false for any non-wizard import. Even for wizard, the list omits ~120+ wizard SRD spells, so the warning will misfire for legitimate wizard spells (e.g., Lightning Bolt, Counterspell) marking them as off-list/homebrew.
- **Suggested fix:** Drive `isOffListClassSpell` from the seeded `spells.classes` reference data (already in DB) rather than a hand-maintained map. Look up each spell by slug, check whether the character's class slug is in `spells.classes`.

## [High] Builder service: token redeem races and isn't user-bound
- **Location:** /home/ab/projects/DnDnD/internal/portal/builder_service.go:219-238 (`CreateCharacter`)
- **Spec/Phase ref:** Phase 91a "one-time link generation … single-use token"
- **Problem:** (1) `RedeemToken` is called **after** `CreateCharacterRecord` succeeds, so a concurrent double-submit with the same token can produce two characters before either redemption wins. (2) The token's `discord_user_id` is never compared against the session `userID` at submit time — `ValidateToken`/`RedeemToken` only check existence, not ownership. A user who somehow obtains another player's token string can spend it. (3) A failed `RedeemToken` is logged but ignored, leaving an unburned token.
- **Suggested fix:** Validate the token first, compare `tok.DiscordUserID == userID`, then atomically mark-used (or use a transactional `RedeemTokenIfNotUsed` returning `(redeemed bool)`) before inserting the character. If redemption fails, rollback / refuse.

## [High] DDB import attunement-limit warning uses wrong signal
- **Location:** /home/ab/projects/DnDnD/internal/ddbimport/validator.go:102-112
- **Spec/Phase ref:** spec line 2435 ("⚠ 4 attuned items — exceeds default attunement limit of 3")
- **Problem:** The check counts items with `Equipped && RequiresAttunement`. Attunement in 5e is independent from equip state (a ring need not be "equipped"), and `RequiresAttunement` only says the item *can* be attuned, not that it *is*. The result is both false negatives (a legitimately attuned non-equipped item is missed) and false positives (an equipped attunement-capable but unattuned weapon counts).
- **Suggested fix:** Parse DDB's `attunedToCharacter` flag on each `ddbItem`/`ddbItemDef` and count only `attunedToCharacter == true`. Persist that flag into `character.AttunementSlots` on import.

## [High] Character sheet does not render conditions / active status effects
- **Location:** /home/ab/projects/DnDnD/internal/portal/character_sheet.go:20 (struct `CharacterSheetData`), /home/ab/projects/DnDnD/internal/portal/character_sheet_handler.go:152 (template)
- **Spec/Phase ref:** spec §"Character Sheet View" line 2396 ("read-only web page showing full character details … and all mechanical state")
- **Problem:** `CharacterSheetData` carries HP / spell slots / inventory but has no field for active conditions (poisoned, frightened, exhaustion level, concentration target). The Discord `#character-cards` and combat surface already track these; the portal sheet ignores them entirely.
- **Suggested fix:** Add `Conditions []character.Condition` (and `Concentration *ConcentrationTarget`, `ExhaustionLevel int`) to `CharacterSheetData`, hydrate from `character_data` JSONB, and render a "Conditions" section in the template (or a "None" empty state).

## [High] Starting equipment retains `any-martial` placeholder IDs in inventory
- **Location:** /home/ab/projects/DnDnD/internal/portal/starting_equipment.go:33 + /home/ab/projects/DnDnD/portal/svelte/src/App.svelte:201-234 + /home/ab/projects/DnDnD/internal/portal/builder_store_adapter.go:245
- **Spec/Phase ref:** Phase 91c "Equipment validated against class/background rules"
- **Problem:** Class packs include abstract groups like `"any-martial"`, `"any-simple-melee"`, `"any-martial:2"`. The Svelte UI splits the option string and pushes the raw token into `selectedEquipment`. `EquipmentToInventory` then creates an InventoryItem literally named `"any-martial"`. No second-level prompt lets the player pick which martial weapon they want. Inventory ends up with non-existent items.
- **Suggested fix:** When an option starts with `any-…`, present a follow-up selector populated from the SRD weapon list filtered by category. Resolve to a concrete item ID before storing. Also strip and apply the `:N` quantity suffix (currently dropped: a `"javelin:5"` becomes a single javelin).

## [High] Starting equipment ignores background packs
- **Location:** /home/ab/projects/DnDnD/internal/portal/starting_equipment.go (only class packs defined)
- **Spec/Phase ref:** spec §"Equipment" line 2390 ("choose from starting equipment options by class/background"); Phase 91c "class default equipment packs, background equipment"
- **Problem:** `StartingEquipmentPacks(class)` returns only the class pack. There is no `BackgroundEquipmentPack` table or API. Backgrounds in SRD grant items (e.g., Acolyte: holy symbol, prayer book, 5 sticks of incense; Soldier: insignia of rank, trophy, set of bone dice). Players who pick Acolyte get nothing background-specific.
- **Suggested fix:** Add a `backgroundStartingEquipment` map keyed by background ID and merge its `Guaranteed`/`Choices` into the builder's equipment step. Surface via `/portal/api/starting-equipment?background=...`.

## [High] DeriveSpeed ignores race
- **Location:** /home/ab/projects/DnDnD/internal/portal/builder_store_adapter.go:275
- **Spec/Phase ref:** spec §"Player Portal" line 2392 ("all derived stats … auto-calculated"); SRD races (Dwarf/Halfling/Gnome: 25 ft)
- **Problem:** `DeriveSpeed(_ string) int { return 30 }` always returns 30, even though the reference race row already carries `speed_ft`. Dwarf, Halfling, Gnome characters created via the portal will incorrectly start at 30 ft. The Svelte builder shows the correct race speed in the review step but the server overrides it.
- **Suggested fix:** Look up the race by ID in the BuilderStore (`GetRaceByID`) and use its `speed_ft`. Fall back to 30 only if the race is unknown.

## [High] DDB class names not normalised to internal IDs
- **Location:** /home/ab/projects/DnDnD/internal/ddbimport/parser.go:177; /home/ab/projects/DnDnD/internal/ddbimport/service.go:382 (`classHitDie`)
- **Spec/Phase ref:** Phase 90 "Parser converts DDB JSON into internal format"
- **Problem:** DDB returns capitalised class names (`"Fighter"`, `"Wizard"`). The parser stores them verbatim into `character.ClassEntry{Class: c.Definition.Name}` and the import-side `classHitDie` switches on Capitalized strings, while the rest of the system (refdata IDs, `levelup.IsASILevel`, `character.Slugify`) uses lowercase slugs (`"fighter"`). Result: imported characters can't level up via the dashboard (`asiLevels` keyed by lowercase, but extra-ASI lookup `extraASILevels["Fighter"]` misses) and the multiclass spell-slot lookup may mis-key spellcasting maps.
- **Suggested fix:** Lowercase / slugify DDB class names (`strings.ToLower` + replace spaces) before storing. Add a small mapping helper for known DDB→internal class IDs (e.g., DDB "Druid (Circle of the Moon)" → `"druid"`).

## [High] Plus-2 ASI silently truncates at cap (loses 1 point) without warning
- **Location:** /home/ab/projects/DnDnD/internal/levelup/asi.go:81 (`applyPlus2`)
- **Spec/Phase ref:** spec §"ASI path" line 2484 ("If a player tries to exceed 20, the bot rejects with '❌ STR is already 20 — choose a different ability.'")
- **Problem:** The spec wants the bot to **reject** when a single score would exceed 20; the code accepts the choice and silently caps at 20, so a STR-19 player picking +2 STR ends up with STR 20 and loses the second point with no notice. Same for plus1plus1 — partial application is allowed if either ability is at 19. Players have no chance to redirect the unused +1.
- **Suggested fix:** Reject the choice if `current + bonus > 20` (return error to handler so user picks again). `BuildAbilitySelectMenu` already filters abilities at 20, but allowing a score of 19 to "consume" only +1 of a +2 selection is the issue — either reject pre-emptively or split into a follow-up "+1 leftover" prompt.

## [High] /api/levelup/asi/approve endpoint has no character-owner / DM check
- **Location:** /home/ab/projects/DnDnD/internal/levelup/handler.go:129 (`HandleApproveASI`)
- **Spec/Phase ref:** spec §"DM approval" line 2497
- **Problem:** Routes are gated by `dmAuthMw` per `cmd/dndnd/main.go:1271`, but the handler still takes `CharacterID` from the JSON body without verifying the DM session is authorised for that character's campaign. A DM of campaign A can approve an ASI for a character in campaign B if they know the UUID.
- **Suggested fix:** Resolve campaign from the character row inside `ApproveASI`, and verify the authenticated DM's `discord_user_id` matches the campaign's DM. Or scope routes per `/campaign/{id}/`.

## [Medium] Level-up notification omits new features, spell slots, and class/old→new line
- **Location:** /home/ab/projects/DnDnD/internal/levelup/notify.go:14 (`FormatPrivateLevelUpMessage`)
- **Spec/Phase ref:** spec §"Level-up notification" lines 2467-2475 (full template with HP, prof bonus, new feature, ASI flag)
- **Problem:** Current output is `"Aria leveled up! (fighter 6)\nTotal Level: 6\nHP gained: +6\nProficiency Bonus: +3\n…"`. The spec template is `"🎉 Aria leveled up! Fighter 5 → Fighter 6\n  ❤️ HP: 38 → 44 (+6)\n  📈 Proficiency bonus: +3\n  ⚔️ New feature: Extra Attack (2 attacks per action)"`. Missing: old→new HP range, the leveled class's old level, the "New feature" line, the new spell slot lines, and any spell-known prompts.
- **Suggested fix:** Extend `LevelUpDetails` with `OldHPMax`, `OldClassLevel`, `NewFeatures []Feature`, `SpellSlotDelta`. Reformat the message to match the spec exactly, including emoji icons.

## [Medium] DDB diff is shallow — misses inventory, features, spells, proficiencies
- **Location:** /home/ab/projects/DnDnD/internal/ddbimport/diff.go:9 (`GenerateDiff`)
- **Spec/Phase ref:** spec §"Re-sync" line 2445 ("System diffs and shows DM what changed")
- **Problem:** `GenerateDiff` only compares name, race, level, classes, ability scores, HP, AC, speed, gold. The most common reasons to re-import — new spells learned, new features gained, equipment added — are invisible to the DM. A Fighter 5→6 re-sync shows only "Level: 5 → 6" with no indication that Extra Attack was added.
- **Suggested fix:** Add diffing for `Spells` (added/removed names), `Features` (added names), `Inventory` (added/removed by name), and `Proficiencies` (skills/saves delta). Truncate noisy diffs to ~10 items each.

## [Medium] DDB AC computation treats any ArmorClass<=3 as a shield
- **Location:** /home/ab/projects/DnDnD/internal/ddbimport/parser.go:285 (`computeAC`)
- **Spec/Phase ref:** spec §"D&D Beyond Import" line 2424 ("mapping … AC")
- **Problem:** Logic assumes "ArmorClass <= 3 == shield". This isn't safe: DDB sets `armorClass=2` on standard shields, but some homebrew/magic shields have +1 or +3 from properties. Some armor types could conceivably have `armorClass <= 3` (e.g., a homebrew armor entry). Also, base armor AC isn't combined with DEX modifier per armor type (light vs medium vs heavy) — `computeAC` just sums `baseAC + shieldBonus` without applying DEX caps.
- **Suggested fix:** Use the DDB item subtype field (`filterType`/`type == "Shield"`) for shield detection, and apply DEX bonus per armor type (light=+DEX uncapped, medium=+min(DEX,2), heavy=+0) as the local `internal/character.calculateArmorAC` already does.

## [Medium] DDB import doesn't validate ability scores were generated within 1-30 of submission's "stats" (no override sanity)
- **Location:** /home/ab/projects/DnDnD/internal/ddbimport/parser.go:228 (`parseAbilityScores`)
- **Spec/Phase ref:** spec line 2431 ("values within sane bounds (e.g., ability scores 1–30)")
- **Problem:** `parseAbilityScores` uses zero defaults for missing scores. If DDB returns an empty `stats` array (rare but possible for a partially-set-up character), the parsed character has STR=0, DEX=0… which then **passes** the validator only if no score is below 1 — wait, validator at validator.go:66 does reject `<1`. But the default-zero case still slips through the early `Name=="" || Level<1` checks since the validator runs AFTER buildCreateParams parses. Verify: validator.go:66 does check `< 1`. OK, but the case where DDB sets some but not all stats can leave one ability at 0 unintentionally.
- **Suggested fix:** Require all six DDB stat IDs (1-6) to be present and non-nil; otherwise return an error from `ParseDDBJSON`.

## [Medium] DDB import: features below `RequiredLevel` filtered, but subclass features filtered against parent class level (not subclass level)
- **Location:** /home/ab/projects/DnDnD/internal/ddbimport/parser.go:332-345 (`parseFeatures`)
- **Spec/Phase ref:** spec §"Manual Character Creation" line 2416 ("Subclass features are loaded from … features_by_level and merged …")
- **Problem:** Subclass features are filtered by `f.RequiredLevel > c.Level` where `c.Level` is the parent class level. That's actually OK in 5e (you only have a subclass at level 3 in most classes and class level == subclass level), but doesn't handle multi-class where the subclass entry might have been imported with its own level. More importantly, the subclass list is keyed on the same threshold rules. Minor.
- **Suggested fix:** Cross-check against `SubclassLevel` if DDB exposes it; otherwise add a comment that this is intentional.

## [Medium] Token redemption isn't atomic with character creation (race window)
- **Location:** /home/ab/projects/DnDnD/internal/portal/builder_service.go:237
- **Spec/Phase ref:** Phase 91a "single-use token"
- **Problem:** Two simultaneous form submits with the same token can both pass `ValidateToken` (which only checks Used and ExpiresAt), then both create a character, then race on `RedeemToken`. Result: one duplicate character per concurrent submit.
- **Suggested fix:** Use a transactional `RedeemTokenAndCreate` SQL flow, or use a `RedeemTokenIfNotUsed` returning `false` so the second caller bails before insertion.

## [Medium] No CSRF protection on portal POST /portal/api/characters
- **Location:** /home/ab/projects/DnDnD/internal/portal/api_handler.go:193 (`SubmitCharacter`)
- **Spec/Phase ref:** Phase 91a "OAuth2 authentication … session validation"
- **Problem:** Session cookie is `SameSite=Lax` which protects most cross-origin POSTs from form-based CSRF, but not from JS-fetch from a same-site subdomain or XSS-controlled origin. There is no explicit CSRF token, no `Origin` / `Referer` check on the submit endpoint, and no double-submit cookie. The portal token in the JSON body is single-use but not bound to the session (see related finding).
- **Suggested fix:** Add an `Origin`/`Referer` check on all portal mutation endpoints rejecting any origin not in the trusted list, or issue a CSRF token in the create-page response and require it back in the submit JSON.

## [Medium] Multiclass spell-slot table check requires both classes to be casters; Eldritch Knight/Arcane Trickster ignored
- **Location:** /home/ab/projects/DnDnD/internal/character/spellslots.go:54 (`CalculateCasterLevel` switch on `SlotProgression`)
- **Spec/Phase ref:** spec line 2509 ("third casters (Eldritch Knight, Arcane Trickster) contribute class level × ⅓")
- **Problem:** The "third" branch exists in code but `SlotProgression == "third"` is keyed to the parent class (`fighter`/`rogue`), which is `"none"` in refdata. Third-caster progression is only available when the subclass is selected, and the current model has no per-subclass spellcasting override. So a Fighter 7 / Wizard 3 Eldritch Knight gets only the Wizard's contribution; the EK's caster level (floor(7/3)=2) is missed.
- **Suggested fix:** Extend `ClassSpellcasting` lookup to consult `subclass.spellcasting` when set, and have `CalculateCasterLevel` pick the higher progression (or sum class+subclass) per entry.

## [Medium] Char sheet template renders `{$level}` for spell slots from `map[string]SlotInfo`
- **Location:** /home/ab/projects/DnDnD/internal/portal/character_sheet_handler.go:374
- **Spec/Phase ref:** Phase 92 (sheet display)
- **Problem:** `SpellSlots` is `map[string]character.SlotInfo`; the template uses `range $level, $slot := .SpellSlots` and labels with `"Level {{$level}}"`. Iteration order of Go maps is unspecified, so spell slots render in random order each page load. Players will see "Level 3, Level 1, Level 2, …" non-deterministically.
- **Suggested fix:** Pre-sort spell slot levels in `enrichCharacterSheet` and pass an ordered slice (like `groupSpellsByLevel` does for spells).

## [Medium] DM denial reason is hard-coded; spec wants DM-supplied message
- **Location:** /home/ab/projects/DnDnD/internal/discord/asi_handler.go:543
- **Spec/Phase ref:** spec line 2503 ("If the DM denies (e.g., campaign doesn't allow certain feats), the player receives a message with the DM's reason")
- **Problem:** `HandleDMDeny` hard-codes `reason := "DM denied the choice. Please select again."` with no input from the DM. The button click is the entire interaction — no modal/follow-up to capture the reason.
- **Suggested fix:** Open a Discord modal on deny with a "Reason" text field; pass the entered string into `DenyASI`.

## [Medium] Builder service does not enforce racial ability cap (no score > 20 at creation)
- **Location:** /home/ab/projects/DnDnD/internal/portal/builder_service.go:444 (`ValidatePointBuy`) / 374 (`ValidateStandardArray`) / 388 (`ValidateRolledScores`)
- **Spec/Phase ref:** spec §"Ability scores" — point-buy raw 8-15, racial bonus, character creation cap 20
- **Problem:** Point-buy validation accepts scores up to 17 after racial bonus (+2 to base 15). That's correct. **But the racial-bonus assumption is `+2 max racial bonus`** — Half-Elf gets +2 CHA AND +1 to two others, so a Half-Elf with base 15 CHA + 2 = 17 CHA is OK, but base 14 STR + 1 = 15 STR (also OK). However, rolled mode allows scores 3-20 with no class-cap enforcement and no link back to racial bonuses — a roll of 18 + racial +2 = 20 is allowed but +3 would silently pass. Standard array similarly caps at 17 but doesn't verify that the high score actually came from racial bonus on a base array entry.
- **Suggested fix:** Track racial bonus separately from raw score, validate that raw score (without racial) is within method's legal range, and that final score is ≤20 at creation.

## [Medium] HP recalculation on level-up assumes the standard "average+1" formula but spec also offers rolling option (not implemented)
- **Location:** /home/ab/projects/DnDnD/internal/character/stats.go:30 (avg formula)
- **Spec/Phase ref:** spec §"Leveling workflow" line 2453 ("HP (`hp_max` adds the new class's hit die + CON mod)")
- **Problem:** Code always uses fixed average + 1 per level. SRD allows rolling for HP each level; spec doesn't strictly require rolling, but doesn't forbid it either. More importantly, the level-up code uses `CalculateHP` which **re-derives total HP from scratch** rather than adding only the delta — this works if the formula is purely deterministic, but it overwrites any DM-manual HP adjustments the character may have had.
- **Suggested fix:** Either commit to fixed-HP only (document) or add a `roll` vs `average` mode at campaign settings. Make the HP add additive: `newHP = oldHPMax + dieAvg + conMod` rather than a full recompute that could erase manual edits.

## [Medium] Pending ASI choices: storePendingChoice writes to DB asynchronously in background goroutine pattern but uses `context.Background()`
- **Location:** /home/ab/projects/DnDnD/internal/discord/asi_handler.go:314,328
- **Spec/Phase ref:** Phase 89d "durable store for pending ASI/Feat choices"
- **Problem:** `storePendingChoice` and `removePendingChoice` call the store with `context.Background()` instead of the interaction's context. If the bot is shutting down or the DB is slow, these calls can't be cancelled and may leak. Also, save errors are only logged — the in-memory copy is still authoritative, so a saved choice + crashed-before-DB-flush survives in memory, but a restart loses it silently.
- **Suggested fix:** Plumb a request-scoped context (with a shutdown-aware parent) through the handlers. Bubble Save errors back to the player ("⚠ your choice may not survive a restart").

## [Medium] Portal Svelte builder doesn't expose subclass selection at the right level
- **Location:** /home/ab/projects/DnDnD/portal/svelte/src/App.svelte (subclass section); /home/ab/projects/DnDnD/portal/svelte/src/lib/builder-options.js
- **Spec/Phase ref:** spec §"Class" line 2387 ("subclass (if applicable at level)") and §"Manual Character Creation" line 2407 ("subclass (if available at current level)")
- **Problem:** Builder uses `isSubclassPickerVisible(row)` to gate subclass selection, but doesn't enforce the per-class threshold: Cleric chooses at 1, Wizard at 2, most others at 3. If `isSubclassEligible` is solely level-based, a Cleric L1 may or may not be prompted (depending on implementation in `builder-options.js`). Worth confirming. Also, characters created at L1 wizard *must not* pick a subclass (Wizard subclass starts at L2), but the picker may be presented based on class metadata alone.
- **Suggested fix:** Honour `class.subclass_level` from refdata: only show the picker when `row.level >= class.subclass_level`. Pre-fill the level field with 1 by default.

## [Medium] Portal builder calculates HP only for level-1 single-class case; ignores multiclass at submit
- **Location:** /home/ab/projects/DnDnD/portal/svelte/src/App.svelte:281-286 (`derivedHP`)
- **Spec/Phase ref:** spec §"Review" line 2392 ("HP, AC, proficiency bonus, saving throws auto-calculated")
- **Problem:** The Svelte review step's `derivedHP` is `hd + conMod` using only `selectedClassData.hit_die` and the L1 formula. The form supports multi-class entries (`classEntries`) and any level, but the review preview ignores both. A player building a Fighter 5 / Wizard 3 sees an HP preview of `10 + conMod` which is misleading. Backend computes correctly, but UX is broken.
- **Suggested fix:** Mirror `character.CalculateHP` semantics in JS using `classEntries` + `finalScores`.

## [Medium] DDB import: TempHP can become negative when HPMax overridden lower than removed HP
- **Location:** /home/ab/projects/DnDnD/internal/ddbimport/parser.go:191-198
- **Spec/Phase ref:** general HP invariant (current ≤ max, ≥ 0)
- **Problem:** When `OverrideHitPoints` is set lower than `BaseHitPoints + BonusHitPoints - RemovedHitPoints`, `HPCurrent` is clamped to 0 but `HPMax` (`*OverrideHitPoints`) can be less than `HPCurrent` post-clamp. The validator requires `HPMax > 0` but doesn't enforce `HPCurrent ≤ HPMax`.
- **Suggested fix:** After computing both, `if HPCurrent > HPMax { HPCurrent = HPMax }`. Add this invariant to the validator.

## [Low] DDB import discards spell `Source` for non-class spells; warning text wrong
- **Location:** /home/ab/projects/DnDnD/internal/ddbimport/validator.go:114-125
- **Spec/Phase ref:** spec line 2434
- **Problem:** Off-list warning loops `pc.Spells`, finds `spell.OffList`, then calls `primaryClassName(pc.Classes)` which uses `pc.Classes[0].Class` — but the spell may have been gained from a race/item/feat source. Warning text reads "<primary class> spell list includes <spell>", which is misleading when the spell was attached via Magic Initiate.
- **Suggested fix:** Skip warnings for spells where `spell.Source != "class"`. The parser already populates `Source` for race/item/feat.

## [Low] LevelUpDetails.NewSpellSlots not surfaced in API response
- **Location:** /home/ab/projects/DnDnD/internal/levelup/handler.go:25 (`LevelUpResponse`)
- **Spec/Phase ref:** Phase 89 "auto-recalculate … spell slots (multiclass table)"
- **Problem:** Service computes new spell slots and stores them, but `LevelUpResponse` doesn't include them, so the dashboard UI can't show "new slots gained". Same for `NewPactSlots`.
- **Suggested fix:** Add `NewSpellSlots map[int]int` and `NewPactSlots *PactMagicSlots` to the response struct.

## [Low] Pending DDB import map cache not consulted before durable lookup
- **Location:** /home/ab/projects/DnDnD/internal/ddbimport/service.go:222 (`loadPendingImport`)
- **Spec/Phase ref:** Phase 90 "Re-sync … DM review"
- **Problem:** Logic is fine — in-memory checked first, then DB. But after DB load the entry is NOT cached back into the in-memory map, so every subsequent operation (in the rare case of multiple lookups within TTL window) hits the DB.
- **Suggested fix:** After successful DB load, write back into the in-memory map.

## [Low] DiscordUserID race-case in HandleASIFeatSelect: lookupFeatName re-lists all feats
- **Location:** /home/ab/projects/DnDnD/internal/discord/asi_handler.go:668-679,792
- **Spec/Phase ref:** Phase 89d
- **Problem:** Every feat-select interaction calls `featLister.ListEligibleFeats` again to resolve a single name (and once more in `lookupFeatName` for sub-choice). For DB-backed implementations this is O(N) per interaction.
- **Suggested fix:** Add a `GetFeat(id)` to the lister interface or cache feat names by ID in the handler.

## [Low] Portal landing page does not list user's existing characters
- **Location:** /home/ab/projects/DnDnD/internal/portal/handler.go:89 (`ServeLanding`)
- **Spec/Phase ref:** spec §"Player Portal" line 2382 ("character creation, viewing, and management")
- **Problem:** Landing only shows "Use the links from Discord to create or view your characters." There is no list of the user's owned characters with links to their sheets, despite the portal claiming "viewing and management" responsibility.
- **Suggested fix:** Query `player_characters WHERE discord_user_id = $1` on the landing page and render a list with sheet links per character.

## [Low] ASI prompt button on re-trigger doesn't deduplicate or expire
- **Location:** /home/ab/projects/DnDnD/internal/discord/asi_handler.go:307 (`storePendingChoice`)
- **Spec/Phase ref:** spec §"Pending state" line 2507 ("the player can re-trigger the prompt")
- **Problem:** Each re-trigger overwrites the previous pending choice with the same characterID key — that's fine. But the OLD `#dm-queue` message still has Approve/Deny buttons. A DM could click an outdated message and approve the previous (replaced) choice; the handler would error since the map no longer has the matching pending. The error is silently logged and the DM sees no feedback.
- **Suggested fix:** When re-triggering, edit/delete the prior `#dm-queue` message (track its ID in `PendingASIChoice`). Or at minimum, show a clearer "this choice has been superseded" message on the DM click.

## [Low] `applyFeatProficiencyChoices` saves proficiencies even when caller didn't change them
- **Location:** /home/ab/projects/DnDnD/internal/levelup/service.go:461-491
- **Spec/Phase ref:** Phase 89 "Feat effects applied"
- **Problem:** For `resilient` with `Choices.Ability==""` or `skilled` with empty Skills slice, the function still calls `UpdateProficiencies` with the unchanged JSON — generating a needless DB write and an extra `OnCharacterUpdated` card refresh. Minor perf hit.
- **Suggested fix:** Bail early if no changes were applied.

---

## Phase status summary

- **Phase 89: Major issues** (single-class half-caster slot bug, no feature auto-grant, level cap unbounded, notification incomplete).
- **Phase 89d: Major issues** (no player-identity check on buttons, no DM role check, partial-apply +2 cap behaviour).
- **Phase 90: Major issues** (fresh import bypasses DM approval, off-list detection covers only 16 wizard spells, attunement-count signal wrong, class-name not normalised).
- **Phase 91a:** OK on auth/state/cookies; **issues** in token redemption ordering and lack of CSRF/Origin checks on submit.
- **Phase 91b: Issues** in feat prereq enforcement (n/a here, but ability cap & race speed in this scope), HP preview mismatch.
- **Phase 91c: Major issues** — placeholder `any-martial` IDs stored verbatim in inventory; background packs missing.
- **Phase 92: Issues** — conditions/concentration not rendered; spell-slot map order non-deterministic.
- **Phase 92b:** OK overall (storage, prepared indicator, joining to refdata, /character summary). Minor — fallback name=ID when ref lookup misses; that's acceptable.
