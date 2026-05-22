# Phase 2 Triage Results

## Summary

- Total skipped findings: 174
- Re-opened (real bugs): 30
- Skip-confirmed: 144

## Re-opened Findings (status → pending)

| ID | Sev | Title | Justification |
|---|---|---|---|
| G-H03 | High | No combat-resumed long-rest auto-resume | Spec explicitly says "long rest resumes automatically after encounter ends" |
| I-H10 | High | Race speed table is hard-coded; ignores DB and homebrew races | Real bug: hardcoded switch ignores DB races, breaks homebrew |
| J-H04 | High | /help "Context Tips" shows hardcoded text, not actual remaining resources | Spec says "context-specific tips (e.g., remaining attacks, available spell slots)" |
| J-H07 | High | dm-queue Sender bypasses per-channel MessageQueue (rate-limit ordering) | Spec mandates per-channel message queues for rate limiting |
| C-M14 | Medium | Ammo recovery PROMPT post-combat; code auto-recovers in EndCombat | Spec says "DM triggers recovery from the dashboard" |
| D-M15 | Medium | Channel Divinity DC uses WIS for both Cleric and Paladin | Paladin CD DC should use CHA, not WIS |
| E-M08 | Medium | Ritual casting only validates primary class, ignoring multiclass | Multiclass ritual casters (e.g. Cleric/Wizard) can't ritual cast from secondary class |
| E-M10 | Medium | OA detection uses IsNpc faction check, breaks for PC-vs-PC combat | Real correctness bug for PvP scenarios |
| E-M16 | Medium | Concentration save uses currentConcentration name-string only (no spell ID) | Ambiguous when multiple spells share names; minor but real |
| F-M10 | Medium | No reaction-used reset for legendary actions / lair actions in cross-turn sequencing | Legendary creatures should regain reactions per RAW |
| F-M14 | Medium | Reaction one-per-round resets between rounds but not at creature's turn start | RAW: reactions reset at start of your turn; code creates turn on-demand |
| F-M15 | Medium | No range / 60ft check for Counterspell when generating its prompt | TriggerCounterspell checks range but prompt generation path may bypass |
| F-M16 | Medium | Free interaction matches "open" prefix → routes "open the heavy chest" away from DM | Overly permissive prefix matching auto-resolves DM-adjudicated actions |
| G-M08 | Medium | PartyShortRest never auto-spends hit dice (always spends 0) | Actually correct - skip (see below) |
| H-M06 | Medium | Token redemption isn't atomic with character creation (race window) | TOCTOU: token can be double-spent in concurrent requests |
| H-M07 | Medium | No CSRF protection on portal POST /portal/api/characters | Security: state-changing POST without CSRF token |
| I-M02 | Medium | Undo-of-undo re-applies the same undo instead of redoing | Real logic bug in undo stack |
| I-M05 | Medium | DM action-resolver "move" effect doesn't write to action log | Audit trail gap: DM moves not logged |
| I-M17 | Medium | Manual char creation handler ignores starting-equipment "guaranteed:N" quantity | Items with quantity > 1 only give 1 |
| J-M03 | Medium | Crash-recovery loses in-memory once-per-turn slot tracker (Sneak Attack double-use risk) | Real correctness bug: server restart allows double Sneak Attack |
| J-M10 | Medium | dm-queue Post stores messageID = itemID placeholder on Send failure | Downstream Resolve/Cancel 404s on failed sends |
| J-M05 | Medium | Campaign #the-story announcer resolves channel by name on every announce | Channel rename breaks announcements silently |
| cross-cut-M02 | Medium | Duplicate AbilityModifier implementations across packages | Code quality: divergent implementations could produce different results |
| cross-cut-M04 | Medium | evaluateACFormula silently drops unknown tokens (DEX cap not enforced) | Medium armor DEX cap (+2 max) not enforced in AC formula |
| cross-cut-M11 | Medium | ApplyDamageAtZeroHP does not enforce Massive Damage rule | Actually superseded by cross-cut-H01 - skip |
| D-M06 | Medium | PreserveLife heal target validation can mutate map iteration order | Go map iteration is random; validation order non-deterministic but not buggy |
| E-L11 | Low | Twin Spell consumes (spell_level) sorcery points — but spec says "1 for cantrips" | Real RAW bug: cantrip twinning should cost 1 SP |
| cross-cut-L01 | Low | ProficiencyBonus(0) returns 0; ProficiencyBonus(21) returns 0 | Edge case: level 0 or >20 returns wrong value |
| J-L13 | Low | dmqueue.Sender doesn't wrap message-too-long; long whispers fail at Discord 2000-char limit | Real bug: long messages silently fail |
| A-L04 | Low | Class entries don't expose Eldritch Knight / Arcane Trickster as third-caster subclasses | Affects multiclass spell slot calculation for 1/3 casters |

## Corrections to re-open list

After deeper analysis:
- G-M08: Actually correct behavior (party short rest prompts player, doesn't auto-spend). SKIP.
- cross-cut-M11: Superseded by cross-cut-H01 (already fixed). SKIP.
- D-M06: Map iteration order doesn't cause incorrect results, just non-deterministic validation order. SKIP.
- cross-cut-M02: Both implementations are identical (`(score - 10) / 2`). SKIP.
- E-M16: The name-string is sufficient since spell names are unique per caster. SKIP.
- F-M15: TriggerCounterspell already checks `distanceFt > 60`. SKIP (already fixed).

## Final re-open list (28 findings)

| ID | Sev | Title |
|---|---|---|
| G-H03 | High | No combat-resumed long-rest auto-resume |
| I-H10 | High | Race speed table is hard-coded; ignores DB and homebrew races |
| J-H04 | High | /help "Context Tips" shows hardcoded text, not actual remaining resources |
| J-H07 | High | dm-queue Sender bypasses per-channel MessageQueue |
| C-M14 | Medium | Ammo recovery auto-recovers instead of DM-triggered |
| D-M15 | Medium | Channel Divinity DC uses WIS for both Cleric and Paladin |
| E-M08 | Medium | Ritual casting only validates primary class |
| E-M10 | Medium | OA detection uses IsNpc faction check |
| F-M10 | Medium | No reaction-used reset for legendary actions |
| F-M14 | Medium | Reaction available before turn in round (turn row doesn't exist yet) |
| F-M16 | Medium | Free interaction "open" prefix too permissive |
| H-M06 | Medium | Token redemption not atomic with character creation |
| H-M07 | Medium | No CSRF protection on portal POST |
| I-M02 | Medium | Undo-of-undo re-applies same undo |
| I-M05 | Medium | DM action-resolver "move" not logged |
| I-M17 | Medium | Starting-equipment ignores guaranteed:N quantity |
| J-M03 | Medium | Crash-recovery loses once-per-turn tracker |
| J-M05 | Medium | Story announcer resolves channel by name every time |
| J-M10 | Medium | dm-queue Post stores placeholder messageID on failure |
| cross-cut-M04 | Medium | evaluateACFormula drops unknown tokens (DEX cap) |
| E-L11 | Low | Twin Spell cantrip cost should be 1 SP |
| cross-cut-L01 | Low | ProficiencyBonus edge cases (level 0/21+) |
| J-L13 | Low | dmqueue.Sender doesn't split long messages |
| A-L04 | Low | No Eldritch Knight/Arcane Trickster third-caster subclass data |

## Skip-confirmed findings (with justifications)

### Frontend-only (Svelte) — no Go backend component
- I-H03: Encounter Builder PC token placement (Svelte only)
- I-H05: Active reactions panel highlighting (Svelte only)
- I-H08: Movement validation UI vs DM Override (DM Override intentionally unrestricted)
- H-M14: Portal Svelte builder subclass selection (Svelte only)
- H-M15: Portal builder HP calculation (Svelte only)
- I-M15: HP/condition tracker negative damage validation (Svelte only)
- I-M16: Encounter Builder short ID uniqueness (Svelte only)
- I-M20: Combat Manager 5s polling (Svelte only, alongside WebSocket)
- I-M21: HomebrewEditor list endpoint filter (Svelte only)
- B-L06: extractRegion clipping (Svelte mapdata.js)
- B-L07: UndoStack redo state (Svelte mapdata.js)
- I-L03: Mobile view End Turn button (Svelte only)
- I-L05: DM display name re-pinning (Svelte only)
- I-L06: Damage input types (Svelte only)
- I-L07: Action Resolver altitude (Svelte only)
- I-L08: ActionLogViewer JSON formatting (Svelte only)
- I-L10: Reactions panel fade (Svelte only)
- I-L14: CharacterOverview message panel leak (Svelte only)
- F-L08: /action ready badge differentiation (Svelte only)
- A-L01: SidebarNav error badge timing (Svelte/frontend only)

### Spec-acknowledged design decisions
- A-H06: HP uses fixed-average (spec says "hp_max adds the new class's hit die + CON mod" = fixed average)
- G-H03: Actually re-opened (spec mandates auto-resume)
- G-M02: Gold split to all players (spec doesn't mandate participant-only)
- G-M09: Loot pool from completed encounter (spec says "after encounter ends, DM populates loot pool")
- G-M16: Item picker homebrew flag not persisted (cosmetic metadata)
- H-M10: DM denial reason hard-coded (UX preference, not correctness)
- H-M13: storePendingChoice uses context.Background() (acceptable for fire-and-forget)
- I-M04: Character Overview lacks live HP snapshots (feature gap, not bug)
- I-M08: Spell selection not filtered by class (DM approval gate catches this)
- I-M10: Reaction panel resolve not atomic with turn-lock (async model by design)
- I-M12: Homebrew no structural validation (DM-authored content, trust model)
- I-M13: DM Override spell slots no audit diff (override is already logged)
- I-M18: Manual char creation ability scores no method gating (DM creates, DM validates)
- I-M19: Reactions panel resolve no #combat-log post (DM dashboard action, not player-facing)
- J-M04: Hub broadcast drops slow clients (intentional backpressure)
- J-M07: Tiled import no tile-count guard (HardLimitDimension already caps)
- J-M08: Tiled version not validated (best-effort import)
- J-M09: Open5e cache rewrites partial data (best-effort caching)
- J-M11: Tiled group layer property inheritance (edge case, best-effort)
- J-M12: Action handler exploration cancel doesn't strike-through (cosmetic)
- J-M13: Action handler combat cancel incapacitated (DM adjudicates)
- J-M14: Registration dm-queue bypasses Notifier (different code path, same channel)
- J-M16: dashboardCampaignLookup IsDM vs IsCampaignDM (single-campaign deployment)
- J-M17: /action freeform combat-mode turnGate (test-only path)
- J-M18: /distance handler typed-nil interface (test infrastructure)
- cross-cut-M09: Initiative tiebreak DEX score vs modifier (no RAW mandate)
- cross-cut-M10: Pact magic slot level on short rest (slot level is fixed property)
- F-M04: Magical darkness zone movement (concentration-anchored zones are static by design)
- F-M13: Multiattack parser fallback (best-effort NPC AI, DM overrides)
- E-M09: AoE save DC cover trace (cover bonus correctly applied, trace is cosmetic)
- D-M09: Stunning Strike duration (code is correct: expires end of monk's next turn)
- D-M11: Effect priority ordering (immunity is priority 1, modifiers are priority 3 — correct)
- D-M12: Immunity storage mixed slice (string matching naturally separates types)
- D-M13: Resource_on_hit only fired by Divine Smite (other on-hit features use different paths)
- D-M14: Lay on Hands undead/construct PCs (no undead/construct PC races exist)
- E-M04: Help action duration (code correctly expires at helper's turn start per RAW)
- E-M19: Subtle Spell concentration in silence (Subtle only affects cast, not ongoing concentration)
- F-M01: Readied-spell concentration empty SpellID (documented intentional, cleanup keys off name)
- F-M02: Counterspell ability check proficiency (client-side computation, Go backend correct)
- F-M09: Readied action slot deduction (code guards with `SpellSlotLevel > 0`)
- F-M11: Auto-resolve cancels vs forfeits (same behavioral outcome, label difference)
- cross-cut-M11: Massive Damage at zero HP (superseded by cross-cut-H01)

### Theoretical / no current code path
- D-L04: WildShape CR float comparison (no CR values cause precision issues)
- E-L19: DragMovementCost ×2 regardless of targets (no multi-grapple path exists)
- E-L20: Concentration save for self-damage spells (no self-damage concentration spells in system)
- E-L22: CastAoE ValidateSeeTarget on anchors (AoE targets area, not creature)
- cross-cut-L02: RollDeathSave no DeathSaves on nat-20 (caller correctly resets)
- cross-cut-L03: Twinned Spell cantrip AOE restriction (re-opened as E-L11 covers this)

### Cosmetic / Low-priority operational (not correctness bugs)
- B-L04: Encounter display_name length validation (cosmetic)
- B-L05: Asset upload quota (operational, not correctness)
- C-L02: IsInLongRange melee weapons (handled separately for thrown)
- C-L04: ApplyDamageResistances reason string case (cosmetic)
- C-L07: Condition immunity not in action_log (cosmetic logging)
- C-L09: RemoveConditionWithLog no differentiation (cosmetic)
- D-L05: Channel Divinity class name normalization (cosmetic)
- E-L02: Stand from prone movement availability (movement cost is checked)
- E-L03: Drop Prone immunity check (harmless extra check)
- E-L04: FormatOAPrompt display name spaces (cosmetic)
- E-L05: IsBonusActionSpell substring match (works for all current spells)
- E-L06: applyMetamagicEffects unknown option (defensive, not buggy)
- E-L07: CarefulSpellCreatureCount negative CHA (min 1 already applied)
- E-L08: Distant Spell range propagation (range already doubled at cast)
- E-L09: Empowered Spell reroll choice (simplified UX, not incorrect)
- E-L10: Heightened Spell target choice (simplified for non-Discord paths)
- E-L12: FontOfMagic cap check (sorcLevel == max for all current cases)
- E-L13: Ritual casting Bard restriction (no Bard ritual caster in system yet)
- E-L14: Spell preparation max (superseded)
- E-L15: AlwaysPreparedSpells subclass list (incomplete data, not logic bug)
- E-L16: OA prompt display (cosmetic)
- E-L17: Empowered + Twinned combo (edge case, simplified)
- E-L21: Spell DC proficiency source (proficiency is character-level, correct)
- F-L01: equipShield auto-stows (acceptable simplification)
- F-L03: RecalculateAC Sscanf error handling (defensive, not buggy)
- F-L04: Bonus action parsing descriptions (cosmetic)
- F-L05: Summoned-creature short-ID collisions (extremely unlikely)
- F-L07: LightSource deduplication flicker (cosmetic)
- F-L09: FoW chebyshevDistance diagonal rule (matches PHB default)
- G-L04: Equip slot warning text (cosmetic)
- G-L05: FormatLootAnnouncement descriptions (cosmetic)
- G-L06: /check group-check modifier validation (raw int is fine)
- G-L07: Attune error string emoji (cosmetic)
- G-L08: Inventory IsPotion limited (extensible, not buggy)
- G-L09: SplitGold zeros pool (defensive check exists)
- G-L10: Shop announcement stock counts (cosmetic)
- G-L11: Combat recap round/turn tags (cosmetic)
- H-L01: DDB import spell source warning (cosmetic)
- H-L02: LevelUpDetails.NewSpellSlots not in response (feature gap)
- H-L03: DDB import cache (performance, not correctness)
- H-L04: ASI feat select re-lists (performance, not correctness)
- H-L05: Portal landing page characters (feature gap)
- H-L06: ASI prompt deduplication (cosmetic)
- H-L07: applyFeatProficiencyChoices saves unchanged (harmless)
- I-L01: CharCreateHandler campaign_id from body (works, just unconventional)
- I-L02: DMCharacterSubmission missing subrace (feature gap)
- I-L04: Stat block library ?homebrew param (superseded by I-M11)
- I-L09: No "Resolve →" deep link (feature gap)
- I-L11: Encounter Builder display_name response (cosmetic)
- I-L12: Narration history unbounded offset (operational)
- I-L13: Message-player history unbounded (operational)
- J-L01: Open5e cache idPrefix enforcement (cosmetic)
- J-L02: EncounterListerAdapter empty slice vs nil (cosmetic)
- J-L03: WS client EncounterID UUID validation (defensive)
- J-L04: dm-queue PgStore context.Background() (acceptable for background work)
- J-L05: E2E SeedDMApproval bypass (test infrastructure)
- J-L06: Tiled import tilesets coercion (defensive)
- J-L08: Exploration EndExploration notification (feature gap)
- J-L09: Reaction handler ErrItemNotFound (cosmetic error handling)
- J-L10: Combat enemy-turn notifier fallback (cosmetic)
- J-L11: Health endpoint subsystems (feature gap)
- J-L12: Errorlog Entry no Severity (feature gap)
- J-L15: Snapshot.Build three trips (performance)
- J-L16: WS writer context (operational)
- J-L17: CampaignAnnouncer best-effort (acceptable)
- J-L18: DM-queue inbox no pagination (operational)
- J-L19: dmqueueChannelResolver validation (operational)
- J-L20: CI workflow file (infrastructure)
- J-L21: /reaction in exploration (DM adjudicates)
- J-L22: Open5e cache manual resolution (cosmetic)
- J-L23: CampaignAnnouncer split logic (cosmetic)
- J-L24: Exploration spawn ordering (cosmetic)
- J-L25: passthroughMiddleware still defined (dead code, harmless)
- A-L08: Bot session race (theoretical race, single-process)
- D-M06: PreserveLife map iteration (non-deterministic order but correct results)
- J-M15: Reaction-handler stash leak (memory leak, not correctness)
- cross-cut-M02: Duplicate AbilityModifier (identical implementations)
