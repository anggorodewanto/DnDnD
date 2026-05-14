# Batch 12: Leveling, DDB import, portal, manual creation (Phases 89–93b)

## Summary

All ten phases land with substantive implementations and tests. The
core mechanics (multiclass HP/spell-slot/prof-bonus recalculation,
ASI/feat select menus, DDB import + re-sync diff + pending approval
queue, portal OAuth + token-gated builder, character sheet view, DM
manual wizard with derived-stats preview) are present and aligned with
spec. Several gaps qualify as divergences rather than misses:

- **Spec calls for HP "max + average" but code uses "max + average"
  only for first class, subsequent classes use plain average (no +1)**
  — `internal/character/stats.go:30-42`. SRD also typically rounds
  ½ hit die up; `avg := die/2 + 1` is correct.
- **Ability score generation methods**: spec calls for point-buy /
  standard array / roll, with DM campaign setting controlling which
  are available. Portal builder only offers point-buy. DM dashboard
  accepts manual entry only.
- **Portal/dashboard char creation lack background mechanics**: only
  background-skill auto-merge is wired in portal; tool/language/
  feature grants from backgrounds are not applied.
- **DM dashboard manual creation drops multiclass JSON**: service
  forwards only `primaryClass`/`primarySubclass`, not the full
  `sub.Classes` slice (see Phase 93a finding).
- **Phase 89d DM-approval pending state is not durably persisted to
  the character card**: spec says card should show
  "⏳ ASI/Feat pending"; only the in-memory map (with optional store)
  is checked on the handler, no card-side flag.
- **DDB import only parses class features; non-SRD/homebrew flagging
  is structural-only** (no homebrew warnings, no spell-list-mismatch
  advisory in `Validate`).
- **Phase 90 re-sync pending storage is in-memory** (acknowledged as
  TODO in `service.go:60-70` — not strictly a spec gap but a
  durability concern flagged for follow-up).

## Per-phase findings

### Phase 89 — Character Leveling
- Status: **match** (with one HP-formula caveat noted above).
- Key files: `internal/levelup/{levelup.go,service.go,asi.go,feat.go,handler.go,notify.go,notifier_adapter.go,store_adapter.go,integration_test.go}`,
  `internal/character/{stats.go,spellslots.go}`,
  `dashboard/svelte/src/LevelUpPanel.svelte`.
- Findings:
  - `CalculateLevelUp` (`levelup.go:26`) wires class-entry editing →
    total level + HP + prof bonus + multiclass spell slots + pact
    slots + max attacks-per-action. HP formula uses first-class max +
    `die/2+1` for remaining levels (`stats.go:30-46`).
  - `IsASILevel` correctly hard-codes 4/8/12/16/19 plus Fighter
    6/14 and Rogue 10 (`levelup.go:72-91`).
  - `NeedsSubclassSelection` (`levelup.go:95-100`) flags
    subclass-required levels via class ref data.
  - Multiclass spell-slot table is the 21-row constant in
    `internal/character/spellslots.go:8` (matches spec lines
    2510-2511).
  - Notifications: `notify.go` builds public ("🎉 X reached Level N")
    + private detail messages; `notifier_adapter.go` routes to
    `#the-story` and `#your-turn` channels via campaign config.
  - Feat ASI bonuses applied via `applyFeatASI`
    (`service.go:368-397`); features array gets mechanical_effect
    payload stored as JSON.
  - DDB-imported characters are explicitly directed to re-import
    instead of leveling (handler banner + service comment).
  - **Minor divergence**: Multiclass HP for added classes does **not**
    grant the +1 (`(die/2)+1`) — actually it does in `stats.go:32`.
    Re-reading: `avg := die/2 + 1` so multiclass levels use the SRD
    average. ✓.
  - **Gap**: After-approval character card refresh and "⏳ ASI/Feat
    pending" indicator on the card are not implemented. Spec line
    2507 says the card should display the pending tag; current code
    posts to DM queue but doesn't mutate card state.

### Phase 89d — Discord Interactive Components for ASI/Feat
- Status: **match**.
- Key files: `internal/discord/asi_handler.go` (740 lines),
  `internal/discord/asi_handler_test.go`,
  `internal/discord/pending_asi_store.go` (durable store referenced
  via `ASIPendingStore`).
- Findings:
  - `BuildASIPromptComponents` constructs three buttons (+2 / +1+1 /
    Feat) with proper CustomID prefixes (`asi_handler.go:39-65`).
  - `BuildAbilitySelectMenu` (`:93`) renders ability options with
    "STR (16 -> 18)" labels, caps at 20, hides abilities already at
    20. MinValues/MaxValues correctly set for +2 vs +1+1.
  - `BuildDMApprovalComponents` (`:138`) renders ✅ Approve / ❌ Deny
    buttons; full handler flow `HandleASIChoice` → `HandleASISelect`
    → `HandleDMApprove`/`HandleDMDeny` is wired.
  - Feat path: `handleFeatChoice` (`:547`) calls injected
    `FeatLister`; falls back to "not yet available" stub if no lister
    wired. `buildFeatSelectMenu` caps at 25 options (Discord limit) —
    **pagination is a known follow-up** (line 583 comment).
  - F-89d durability: `ASIPendingStore` interface + `HydratePending`
    rehydrates from DB on startup so process restarts don't lose
    in-flight prompts (`:235-291`).
  - Player notification via DM channel on approval (`:702-712`).
  - **Gap**: Feats with sub-choices (Resilient → ability, Skilled →
    proficiencies) — spec line 2505 calls for a follow-up select
    menu. Current code stores `FeatID` and applies via
    `Service.ApplyFeat` but there is no second-step interactive
    prompt for feat-internal choices. Feats with `choose_ability`
    ASI bonus are handled in code (`asiBonus map[string]any` with
    `"choose_ability"`) but the interactive selection UI is missing.

### Phase 90 — D&D Beyond Import
- Status: **match** (with homebrew/advisory gaps).
- Key files: `internal/ddbimport/{client.go,parser.go,validator.go,diff.go,preview.go,service.go,urlparser.go}`.
- Findings:
  - `DDBClient.FetchCharacter` (`client.go:62`) hits
    `character-service.dndbeyond.com/character/v5/character/{id}`,
    handles 403 (private sharing) and 429 (rate limit) with
    exponential backoff up to maxDelay (1s→30s, 3 retries).
  - `ParseDDBJSON` (`parser.go:155`) maps name, race, multi-class
    entries with subclass, ability scores (base+bonus+override),
    HP, inventory + AC, currencies → gold, languages, proficiencies,
    features, spells from class/race/item/feat sources.
  - `Validate` (`validator.go:32`) enforces structural rules (name
    required, level 1-20, HP > 0, scores 1-30); emits advisory
    warnings for STR>20, multiclass ability minimums (per
    `multiclassPrereqs`), and attunement > 3.
  - `GenerateDiff` (`diff.go:9`) produces human-readable change list
    on re-sync.
  - `Service.Import` (`service.go:105`) creates row immediately on
    fresh import; on re-sync, stages an in-memory `pendingImport`
    keyed by UUID and returns `PendingImportID` for DM approval
    flow. `ApproveImport`/`DiscardImport` complete the loop.
  - Pending TTL = 24h matches portal token TTL (`:48`).
  - **Gap**: Homebrew detection is **not** implemented. Spec line
    2434 calls for "Wizard spell list includes Cure Wounds (not on
    Wizard spell list)" — current validator does not cross-check
    spells against the class spell list. Homebrew items/spells pass
    through as plain names without warning.
  - **Gap**: Pending re-syncs are in-memory only (acknowledged
    trade-off in `service.go:60-70`). Bot restart drops staged
    diffs; player must re-run `/import`.
  - **Minor**: DDB `speed` defaulted to 30 (`parser.go:215`) — no
    modifier resolution for races with non-30 speed.

### Phase 91a — Player Portal Auth & Scaffold
- Status: **match**.
- Key files: `internal/portal/{handler.go,routes.go,token_service.go,token_store.go,embed.go}`,
  `internal/auth/oauth2.go`, `cmd/dndnd/discord_adapters.go:28-46`,
  `internal/discord/registration_handler.go:226-300`.
- Findings:
  - Discord OAuth shared with main app: same `auth.OAuthService`
    instance wired via `portal.WithOAuth` in `cmd/dndnd/main.go:863`.
    Cookie session validated by `authMw`; portal does not provision a
    separate OAuth flow.
  - One-time token: `TokenService.CreateToken` mints crypto-random
    string (`token_service.go:48`); 24h TTL per
    `portalTokenCreateCharacterTTL`
    (`cmd/dndnd/discord_adapters.go:36`); `RedeemToken` enforces
    single-use; expired/used tokens raise `ErrTokenExpired`/`ErrTokenUsed`.
  - `Handler.ServeCreate` (`handler.go:80`) validates token and
    asserts `tok.DiscordUserID == userID` (scoped to authenticated
    player). Distinct error pages for expired/used/missing.
  - `/portal/auth/{login,callback,logout}` exposed when oauth opt
    set (`routes.go:51-55`).
  - Portal shell + assets served via embed.FS (`assets/`).

### Phase 91b — Player Portal Character Builder
- Status: **partial**.
- Key files: `internal/portal/{api_handler.go,builder_service.go,builder_store_adapter.go,refdata_adapter.go}`,
  `portal/svelte/src/App.svelte`,
  `portal/svelte/src/lib/{pointbuy.js,builder-options.js,backgrounds.js,api.js}`.
- Findings:
  - 7-step Svelte wizard: Basics → Class → Ability Scores → Skills →
    Equipment → Spells → Review (`App.svelte:13`). Form state held
    in `$state` runes preserves across step navigation.
  - Multiclass: `classEntries[]` with add/remove/update
    (`builder-options.js`); primary class drives spell list +
    starting equipment loading. Submission ships `classes` array →
    `BuilderStoreAdapter.resolveClassEntries` persists into
    `characters.classes` JSONB column.
  - Subrace + subclass dropdowns: `subraceOptions`,
    `subclassOptions`, `isSubclassEligible` (filters subclass picker
    by `subclass_level` threshold per class). F-10 enhancement.
  - Background skills merged via `mergeBackgroundSkills` (auto-add,
    user can deselect, re-merged at submit time as safety net).
  - Point-buy: 27-point budget, scores 8-15, cost table
    (`pointbuy.js`); `ValidatePointBuy` server-side mirror
    (`builder_service.go:243-258`).
  - Submission → DM approval queue: `BuilderService.CreateCharacter`
    sets `status="pending"`, calls `NotifyDMQueue`.
  - **Divergence — ability score generation methods**: only
    point-buy. Spec (lines 2388, 2408) calls for point-buy +
    standard array + roll, gated by DM campaign setting. No standard
    array picker, no roll method, no campaign setting controlling
    which is available.
  - **Divergence — derived stats on review**: `derivedHP`,
    `derivedAC`, `derivedSpeed` are computed in App.svelte
    (`:249-261`) but do NOT include proficiency bonus, save modifiers
    or skill modifiers in the review summary. Server-side
    `BuilderService.CreateCharacter` (`:162-164`) calls
    `DeriveHP`/`DeriveAC` and sets `profBonus =
    character.ProficiencyBonus(1)` — hard-codes level 1 even when
    multi-class entries exceed 1 level total. Should compute from
    `character.TotalLevel(classEntries)`.
  - **Gap — full SRD item picker**: spec mentions
    "select from full SRD item list" on equipment step — see Phase
    91c.

### Phase 91c — Player Portal Starting Equipment
- Status: **match**.
- Key files: `internal/portal/starting_equipment.go`,
  `portal/svelte/src/App.svelte:155-202`.
- Findings:
  - `StartingEquipmentPacks` returns class-specific pack with
    guaranteed items + choices (label + options list). All 12 SRD
    classes covered.
  - Choice options support `item-id:qty` and multi-item
    `"chain-mail:1,leather:1"` forms.
  - Manual add/remove via `manualEquipment[]` rooted in the full
    `listEquipment` SRD list.
  - **Gap**: No gold-method alternative ("buy with starting gold"
    per PHB) — only the predefined packs. Spec line 543 calls out
    "pre-set bundles vs custom" but doesn't explicitly require gold;
    spec section 2390 says "choose from starting equipment options
    by class/background, or select from full SRD item list" —
    current implementation provides both via packs + manualEquipment.
  - **Gap**: Background equipment is not auto-included (spec line
    2390 references "background equipment"). Equipment picker only
    consumes class starting-equipment.

### Phase 92 — Player Portal Character Sheet View
- Status: **match**.
- Key files: `internal/portal/{character_sheet.go,character_sheet_handler.go,character_sheet_store.go}`,
  `internal/discord/character_handler.go`.
- Findings:
  - `CharacterSheetData` (`character_sheet.go:21-56`) exposes full
    sheet: scores + modifiers, all 18 skills, six saves, languages,
    features, spells, inventory, AC/HP/speed, proficiency bonus,
    spell slots, pact slots, hit dice remaining, attunement slots.
  - `ServeCharacterSheet` enforces ownership via
    `GetCharacterOwner`; `ErrNotOwner` returns 403.
  - `/character` Discord command produces ephemeral embed + portal
    link (`character_handler.go:72`).
  - Template + sort helpers via `sheetFuncMap`.

### Phase 92b — Spell List Storage & Display
- Status: **match**.
- Key files: `internal/portal/character_sheet_store.go:230-310`,
  `internal/portal/builder_store_adapter.go:97-110`,
  `internal/charactercard/service.go:316-355`,
  `internal/character/types.go:189-200`.
- Findings:
  - Portal builder writes selected spells to `character_data.spells`
    as `[]string` id list (`builder_store_adapter.go`).
  - `extractSpells` handles both shapes: DDB
    `[]character.DDBSpellEntry` (with name/level/source) and portal
    `[]string` (id only). `enrichSpells` joins against the `spells`
    refdata table to fill in school/casting time/range.
  - `prepared_spells` array drives the prepared indicator
    (`character_sheet_store.go:248-254`).
  - Character card embed surfaces "%d prepared / %d known"
    (`charactercard/format.go:82`).
  - **Minor**: No grouped-by-level rendering in code reviewed — UI
    sort happens via `SpellDisplayEntry.Level` field and template
    sorting; concrete grouping in the HTML template was not
    inspected here but the data model supports it.

### Phase 93a — DM Manual Creation (Basics→Ability Scores)
- Status: **partial**.
- Key files: `internal/dashboard/{charcreate.go,charcreate_handler.go,charcreate_service.go}`.
- Findings:
  - `DMCharacterSubmission` accepts full `Classes []ClassEntry`,
    `AbilityScores`, equipment, spells, languages, equipped weapon,
    worn armor (`charcreate.go:12-23`).
  - `DeriveDMStats` computes HP, AC, speed, total level, prof bonus,
    save proficiencies, all 6 saving-throw modifiers, all 18 skill
    modifiers, hit dice remaining, multiclass spell slots
    (`charcreate.go:91-156`). This is the strongest preview pipeline
    of the three creation paths.
  - Validation: total level capped at 20, ability scores 1-30,
    required fields enforced.
  - DM-created status = `"approved"` with `CreatedVia="dm_dashboard"`,
    DiscordUserID empty until player `/register`s
    (`charcreate_service.go:100-106`). Pre-approval matches spec.
  - **Bug — multiclass dropped**: `CharCreateService.CreateCharacter`
    sets only `primaryClass`/`primarySubclass` in
    `portal.CreateCharacterParams` (`charcreate_service.go:71-79`),
    but does **NOT** pass `Classes: sub.Classes`. The downstream
    `BuilderStoreAdapter.resolveClassEntries` falls back to a single
    `ClassEntry{Class, Subclass, Level:1}` because `p.Classes` is
    empty (`portal/builder_store_adapter.go:31-49`). A Fighter 5 /
    Rogue 3 DM-created character will persist as Fighter 1 with no
    Rogue entry.

### Phase 93b — DM Manual Creation (Equipment/Spells/Features)
- Status: **match**.
- Key files: `internal/dashboard/{charcreate.go (CollectFeatures, lookups),charcreate_handler.go (preview/spells endpoints),charcreate_service.go}`.
- Findings:
  - Equipment: full SRD list via
    `HandleListRefEquipment` + starting-equipment packs via
    `HandleListRefStartingEquipment`. Equipment IDs persisted as
    JSONB (`portal.EquipmentToInventoryWithEquipped`).
  - Spells: `HandleListRefSpells` filters by class then trims by
    max spell level computed from level (`filterSpellsByMaxLevel`).
  - Features: `CollectFeatures` walks racial traits + per-class
    level-indexed features + subclass features
    (`charcreate.go:160-194`), surfaced via
    `WithFeatureProvider` injection at service construction.
  - Preview endpoint (`HandlePreview`) renders derived stats +
    feature list without persisting — DM can iterate.
  - 7-step HTML wizard (Basics → Classes → Ability Scores →
    Equipment → Spells → Features → Review) in
    `charCreatePageTemplate` (`charcreate_handler.go:324+`).
  - **Gap**: Class/subclass/feat mechanical interactions (Sneak
    Attack damage, Rage bonus, Extra Attack) are auto-applied
    elsewhere via the Feature Effect System (Phase 44), but the
    creation wizard does not display these as auto-applied effects;
    only feature names+descriptions render. Acceptable since the
    runtime resolves mechanical effects on combat lookup.

## Cross-cutting concerns

- **DDB durability**: `Service.pending` map is in-memory; restart
  loses pending re-syncs. Same pattern existed in
  `ASIHandler.pending` but was fixed by F-89d with `ASIPendingStore`.
  Apply the same fix to DDB re-syncs (a `pending_imports` table).
- **Ability score generation**: no standard array, no roll method,
  no DM campaign setting gating methods. Spec calls for this in both
  the portal builder (2388) and dashboard manual flow (2408).
- **Background mechanics**: only skills are auto-added in portal.
  Spec implicitly expects tool proficiencies, languages, and
  background feature (e.g. Folk Hero's "Rustic Hospitality") to be
  applied. None of these flow through.
- **DM-created multiclass persistence bug** (Phase 93a): high
  impact because DM-created characters bypass the approval queue
  and player /register-links into a structurally wrong character.
- **F-16 Svelte migration**: `LevelUpPanel.svelte` migrated the
  level-up widget; legacy server-rendered `/dashboard/levelup` page
  retained as fallback per template banner. Both target the same
  `POST /api/levelup` endpoint.
- **Pending-card UI**: spec calls for "⏳ ASI/Feat pending" tag on
  the character card during the approval window. Not implemented;
  card refresh is implicit via `publishForCharacter` only after
  approval lands.
- **Feat sub-choices**: feats requiring follow-up selection
  (Resilient, Skilled, Elemental Adept) lack the second-stage
  interactive prompt described in spec line 2505. The data
  structures support `ASIBonus.choose_ability`, but no UI surfaces
  the choice.
- **DDB homebrew/spell-list advisory warnings**: current validator
  catches STR>20, multiclass prereqs, attunement>3, but misses the
  "spell on non-class list" and homebrew-tag advisories.

## Critical items

1. **Phase 93a bug — multiclass entries dropped on DM creation**
   (`internal/dashboard/charcreate_service.go:71-79` does not set
   `CreateCharacterParams.Classes`). DM-created multiclass
   characters persist as single-class level 1. High impact — silent
   data loss for a flagship feature.
2. **Phase 91b proficiency-bonus hard-coded to level 1** in
   `BuilderService.CreateCharacter` (`builder_service.go:164`).
   Portal-submitted multiclass / level>1 characters get the wrong
   prof bonus until DM corrects. (The dashboard adapter only uses
   level=1 anyway because the spec is "starter at level 1" for
   portal builder per common DnD on-ramp — confirm intent.)
3. **Ability score generation methods missing** across both portal
   and dashboard. Point-buy only, no standard array, no roll
   method, no campaign setting. Affects Phase 91b and 93a.
4. **Phase 90 advisory warnings incomplete** — spec line 2434
   "Wizard spell list includes Cure Wounds" warning is not
   implemented; homebrew detection missing.
5. **Phase 89 / 89d pending-card UI absent** — "⏳ ASI/Feat
   pending" tag on `#character-cards` not rendered while choice
   awaits DM approval.
6. **Phase 89d feat sub-choice UX missing** — Resilient / Skilled /
   Elemental Adept require a follow-up select menu before posting
   to `#dm-queue`. Not implemented.
7. **DDB re-sync pending entries are in-memory only** — restart
   drops staged updates (acknowledged trade-off; consider mirroring
   the F-89d durable store pattern).
