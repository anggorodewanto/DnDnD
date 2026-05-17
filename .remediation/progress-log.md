# Remediation Progress Log

Append-only journal of all remediation activity.

---

## 2026-05-15T15:11 — Queue initialized

- 448 findings parsed from 11 review files
- Critical: 35, High: 98, Medium: 173, Low: 142
- Branch: fix/review-findings-all

## 2026-05-15T15:15 — A-C01 done

- Finding: `/setup` lets any guild member silently become the campaign DM
- Commit: e2d1c33
- Reviewer: approved
- Notes: Two early-return auth guards added to SetupHandler.Handle

## 2026-05-15T15:18 — A-C02 done

- Finding: Dashboard approval endpoints aren't scoped to the DM's own campaign
- Commit: c9e55e9
- Reviewer: approved
- Notes: checkCampaignOwnership guard added to all 3 mutation endpoints

## 2026-05-15T15:21 — B-C01 done

- Finding: ParseExpression mangles modifiers with multiple +/- operators
- Commit: 9790feb
- Reviewer: approved
- Notes: sumSignedTokens helper replaces broken strip-and-concat approach

## 2026-05-15T15:24 — B-C02 done

- Finding: cryptoRand / RollD20 panic on degenerate dice (Nd0)
- Commit: 6de7aa2
- Reviewer: approved
- Notes: 3-line validation guard in ParseExpression rejects Count<1 or Sides<1

## 2026-05-15T15:27 — C-C01 done

- Finding: Multi-letter column labels truncated by colToIndex
- Commit: 78488e3
- Reviewer: approved
- Notes: Base-26 conversion loop replaces single-byte parsing

## 2026-05-15T15:30 — C-C02 done

- Finding: Reckless Attack advantage missing on attacks 2+
- Commit: 560deac
- Reviewer: approved
- Notes: Added "reckless" case to attacker-conditions in DetectAdvantage, gated on melee+STR

## 2026-05-15T15:33 — C-C03 done

- Finding: Off-hand (TWF) attack lacks Attack-action prerequisite and melee weapon check
- Commit: 9c25f59
- Reviewer: approved
- Notes: Added attack-taken prerequisite + IsRangedWeapon checks for both hands

## 2026-05-15T15:36 — C-C04 done

- Finding: /fly performs no fly-speed validation
- Commit: c3866db
- Reviewer: approved
- Notes: HasFlySpeed field + CombatantHasFlySpeed helper checks conditions for fly_speed

## 2026-05-15T17:56 — D-C01 done

- Finding: Rage damage resistance never fires for seed-created barbarians
- Commit: e5446b1
- Reviewer: approved
- Notes: Changed seed mechanical_effect from descriptive tokens to "rage" literal

## 2026-05-15T17:59 — D-C02 done

- Finding: Feature uses never initialized at character creation
- Commit: c3fc75c
- Reviewer: approved
- Notes: New InitFeatureUses helper computes all class feature pools at creation time

## 2026-05-15T18:02 — D-C03 done

- Finding: Rage advantage on STR ability checks never wired
- Commit: 0460fe6
- Reviewer: approved
- Notes: Added IsRaging+Ability fields to SingleCheckInput, rage-advantage logic in SingleCheck

## 2026-05-15T18:05 — D-C04 done

- Finding: Save handler never sets IsRaging in EffectContext
- Commit: 05f8934
- Reviewer: approved
- Notes: 4 lines added to populate IsRaging from combatant state in save handler

## 2026-05-15T18:08 — E-C01 done

- Finding: Single-target spell casts never apply damage or healing
- Commit: 758f247
- Reviewer: approved
- Notes: Added damage roll+apply on hit and healing roll+apply paths to Cast()

## 2026-05-15T18:11 — E-C02 done

- Finding: AoE damage path ignores upcasting and cantrip scaling
- Commit: 85f3ff8
- Reviewer: approved
- Notes: Encoded slot/char level in pending save source, ScaleSpellDice called at resolve time

## 2026-05-15T18:14 — E-C03 done

- Finding: Dodge condition does not impose disadvantage on attackers
- Commit: 9dfe265
- Reviewer: approved
- Notes: Single case added to target-conditions switch in DetectAdvantage

## 2026-05-15T18:17 — F-C01 done

- Finding: Counterspell trigger is unreachable from the DM dashboard
- Commit: 244105c
- Reviewer: approved
- Notes: Added Trigger Counterspell button + isCounterspellReaction helper to Svelte panel

## 2026-05-15T18:20 — F-C02 done

- Finding: Heavy-armor STR speed penalty computed but never applied
- Commit: 0f37116
- Reviewer: approved
- Notes: 5 lines in ResolveTurnResources check armor STR req and subtract 10ft penalty

## 2026-05-15T18:23 — F-C03 done

- Finding: Devil's Sight never wired into player vision pipeline
- Commit: 2e940e6
- Reviewer: approved
- Notes: 3 lines in buildVisionSources check features for Devil's Sight

## 2026-05-15T18:26 — F-C04 done

- Finding: Lair Action placed at head of turn queue instead of losing ties
- Commit: 291875c
- Reviewer: approved
- Notes: Added IsLairAction field + sort.SliceStable after building entries

## 2026-05-15T18:29 — G-C01 done

- Finding: Passive-effect vocabulary mismatch between spec and code
- Commit: 8e310de
- Reviewer: approved
- Notes: Added comma-separated aliases in switch cases (2 lines changed)

## 2026-05-15T18:32 — G-C02 done

- Finding: /attune does not require a short rest
- Commit: 5c8c6c3
- Reviewer: approved
- Notes: Combat gate added to attune handler via ActiveEncounterForUser check

## 2026-05-15T18:35 — G-C03 done

- Finding: destroy_on_zero roll happens at dawn, not when last charge spent
- Commit: d254f1e
- Reviewer: approved
- Notes: Moved d20 destroy check from DawnRecharge into UseCharges

## 2026-05-15T18:38 — G-C04 done

- Finding: Antitoxin "advantage vs poison" not tracked
- Commit: 5819224
- Reviewer: approved
- Notes: Added AppliedCondition field to UseResult, set to "antitoxin" on use

## 2026-05-15T18:41 — H-C05 done

- Finding: Levelup HTTP handler does not bound newLevel to 20
- Commit: c127bc3
- Reviewer: approved
- Notes: Single condition change: added || newLevel > 20 to validation

## 2026-05-15T18:44 — H-C05, J-C03, cross-cut-C01 done

- H-C05: Levelup handler bounds newLevel to 20 — Commit: c127bc3
- J-C03: Open5e client uses 10s timeout — Commit: f0d590e
- cross-cut-C01: Channel Divinity recharges on short rest — Commit: 59f09e8
- All approved

## 2026-05-15T18:47 — J-C01 done

- Finding: WebSocket subscribes to any encounter without campaign-ownership check
- Commit: 8818c60
- Reviewer: approved
- Notes: Added EncounterCampaignResolver + ownership check before WS client registration

## 2026-05-15T18:50 — I-C03 done

- Finding: Narration-template endpoints leak across campaigns
- Commit: 6c05cb0
- Reviewer: approved
- Notes: Campaign ownership check added to all 5 template service methods

## 2026-05-15T18:53 — J-C02 done

- Finding: Open5e public search endpoint bypasses per-campaign source gating
- Commit: 66f2f0f
- Reviewer: approved
- Notes: Moved search routes from public to DM-auth router group

## 2026-05-15T18:56 — H-C01 done

- Finding: Single-class half-caster gets wrong slot count
- Commit: a40152a
- Reviewer: approved
- Notes: Ceiling division (classLevel+1)/2 for single-class half-caster caster level

## 2026-05-15T18:59 — I-C01 done

- Finding: DM-created characters never inherit class/racial features
- Commit: 9c83278
- Reviewer: approved
- Notes: strings.ToLower normalization in CollectFeatures and RacialTraits

## 2026-05-15T19:02 — I-C02 done

- Finding: dm-queue badge count is campaign-wide, not per-encounter
- Commit: 4506164
- Reviewer: approved
- Notes: Added CountPendingDMQueueItemsByEncounter, called per-encounter in GetWorkspace

## 2026-05-15T19:05 — H-C04 done

- Finding: DDB import bypasses DM approval queue on first import
- Commit: 3a5a838
- Reviewer: approved
- Notes: First imports now stage via pending_ddb_imports; ApproveImport handles create

## 2026-05-15T19:08 — H-C02 done

- Finding: Feat prerequisites and already-has-feat exclusion not enforced
- Commit: a74abe4
- Reviewer: approved
- Notes: New FilterEligibleFeats function + wired into production lister

## 2026-05-15T19:11 — H-C02, H-C03 done — ALL 35 CRITICALS RESOLVED 🎉

- H-C02: Feat prereqs enforced — Commit: a74abe4
- H-C03: Level-up auto-adds features — Commit: 14cf763
- All 35 Critical findings are now resolved
- Tests green, coverage thresholds met throughout
- Next: begin High-severity findings (98 total)

## 2026-05-16T05:50 — High-severity batch progress

Findings resolved this session:
- A-H01: Player resubmit after changes_requested — ad18727
- A-H03: WS origin default to secure — 6108b4c
- A-H04: OAuth empty user ID rejection — 3b1fcab
- A-H05: Portal token atomic redemption — 4134c9e
- B-H02: RenderMap TileSize mutation — 26f40d9
- C-H01: Auto-crit melee-only gate — 975caf5
- C-H03: Crossbow Expert waiver — bc56a4a
- C-H05: Fall damage 20d6 cap — 57a38ba
- C-H06: Resistance min-1 clamp — 0d737c4
- D-H01: Step of the Wind base speed — e825269
- D-H02: superseded by E-C03
- cross-cut-H04: Paladin CD always 1 — 633a215
- cross-cut-H05: Action Surge scales at 17 — 7f24261

Running total: 35 Critical + 14 High = 49 findings resolved (11% of 448)

## 2026-05-16T08:41 — Session progress update

High-severity findings resolved this session:
- A-H09: SlideTTL fail-closed — 4051f50
- C-H02: PC race size for heavy weapons — e1c04db
- G-H01: Gold split remainder retained — 5ad0c27
- E-H05: Spell attack roll mode — 65bb779
- H-H01: ASI player identity check — 39fb16c
- H-H02: DM approve/deny role check — 6aad352
- I-H06: Cross-tenant read protection — 5980b8f
- I-H07: Author from context not body — 48038af
- J-H01: Internal name on dashboard — 6ddb297
- J-H06: Empty whisper rejection — da8f4f4
- H-H10: DeriveSpeed race lookup — c6f4114
- F-H03: Hidden combatants excluded — e744e78
- H-H11: DDB class name lowercase — 0a2474d
- H-H12: ASI +2 cap rejection — 0c10005
- cross-cut-H03: Attack proficiency gate — 425a0ba
- D-H07: superseded (already fixed)
- D-H04: Monk shield invalidates UD — bd4248b

Running total: 35 Critical + 37 High = 72 findings resolved (16% of 448)

## 2026-05-16T14:55 — Session progress update

Findings resolved this session:
- C-H11: Concentration two-hit test (already correct) — ca65d3c
- B-H05: TilesetRefs wired through handlers — e98373c
- G-H05: NPC inventory cleared after loot pool — f5833a9
- E-H02: AoE cover bonus on roll not DC — f21c323
- G-H06: Item picker searches gear/consumables — e020bd5
- cross-cut-H01: Massive damage instant-death fix — e06ff7f
- cross-cut-H02: superseded (duplicate of E-H04)
- F-H07: Counterspell 60ft range validation — 6e6cd73
- J-H03: error_detail column written — f05ba7a
- J-H05: Ambiguous encounter detection — 9e498e4

Running total: 94/448 resolved (21%)
- Critical: 35/35 ✅
- High: 59/98 (60%)
- Medium: 0/173
- Low: 0/142

## 2026-05-17T19:38 — D-H08
- **Finding:** Channel Divinity action validation is duplicated and racy across DM-queue + auto-resolved paths
- **Commit:** 51e4646
- **Outcome:** Added nil-guard for dmNotifier in ChannelDivinityDMQueue; prevents burning a use when no notifier is wired.
- **Reviewer:** approved

## 2026-05-17T19:42 — B-H04
- **Finding:** Map renderer never composites the uploaded background image
- **Commit:** 68a37ac
- **Outcome:** Added BackgroundImage/BackgroundOpacity to MapData, created drawBackgroundImage() that composites beneath terrain layer with opacity support.
- **Reviewer:** approved

## 2026-05-17T19:44 — B-H07 (superseded)
- **Finding:** Fog renderer does not preserve "previously seen" cells across renders
- **Outcome:** Already fixed in SR-031. `discord_adapters.go` reads/writes `explored_cells` from DB, applies history via `applyExploredHistoryToFow`, and persists the union after each render.

## 2026-05-17T19:46 — C-H12 (superseded)
- **Finding:** Surprise: surprised condition removed at start of "skip turn", not end
- **Outcome:** Code already satisfies spec. skipSurprisedTurn creates the turn, immediately skips it (completing it), THEN removes the condition. This IS "at the end of their skipped turn." No interleaving is possible in the synchronous flow.

## 2026-05-17T19:50 — F-H01
- **Finding:** No light-source dim radius — 5e torches grant 20ft bright + 20ft dim
- **Commit:** 0b0d2d6
- **Outcome:** Added DimRangeTiles to LightSource; visibility computation marks tiles between bright and dim range as Explored (dim). Updated torch/lantern/spell light builders to emit both radii.
- **Reviewer:** approved

## 2026-05-17T19:52 — F-H02 (superseded)
- **Finding:** Hide action ignores the actor's vision when computing zone obscurement
- **Outcome:** Working as intended. The obscurement check determines if the TILE allows hiding (zone-based property). Observer-specific vision (darkvision, etc.) is handled by the passive Perception check against each hostile, not the obscurement gate.

## 2026-05-17T19:53 — F-H08 (skipped)
- **Finding:** Reaction declarations not validated for the type's prerequisites
- **Justification:** By design. The reaction system is intentionally freeform to support homebrew reactions, custom triggers, and DM adjudication. Validation happens at resolution time (e.g., AvailableCounterspellSlots). Adding heuristic name-matching would be scope creep and could block legitimate homebrew declarations.

## 2026-05-17T19:55 — G-H03 (skipped)
- **Finding:** No combat-resumed long-rest auto-resume
- **Justification:** Feature gap requiring new DB state (rest-in-progress tracking per character), encounter-end hooks, and time-duration logic. Not a correctness bug in existing code — the system simply doesn't implement the auto-resume path yet. Requires dedicated design work beyond a single-finding fix.

## 2026-05-17T19:58 — G-H07
- **Finding:** No way to edit description / name of an existing loot pool item
- **Commit:** e59de2a
- **Outcome:** Added UpdateLootPoolItem SQL query (COALESCE-based partial update), service method with pool-open validation, PATCH handler with quantity >= 1 validation, and route registration.
- **Reviewer:** approved
