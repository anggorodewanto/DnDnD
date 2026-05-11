# DnDnD Remediation — Task Ledger

Source: `.review-findings/SUMMARY.md` + chunk{1..8}_*.md. Status flow: pending → claimed → in_review → revisit → done.

## Critical (block end-to-end gameplay)

| id | severity | title | status | worker | rev |
|---|---|---|---|---|---|
| crit-01a | crit | Wire combat slash handlers (/attack /bonus /shove /interact /deathsave) | done | impl-C | 2 |
| crit-01b | crit | Wire spell slash handlers (/cast /CastAoE /prepare /action ready) | done | impl-C | 1 |
| crit-01c | crit | Wire workflow handlers (/undo /retire) + Set methods for already-built handlers (/help /equip /inventory /give /attune /unattune /character /asi); absorbs high-15 | done | impl-C | 1 |
| crit-02 | crit | /setup broken — pass real SetupHandler in main.go | done | impl-A1 | 1 |
| crit-03 | crit | Damage pipeline routes through ApplyDamageResistances + AbsorbTempHP | done | impl-A1 | 1 |
| crit-04 | crit | FES context populated in BuildAttackEffectContext (Features, IsRaging, advantage, ally, armor) | done | impl-A1 | 2 |
| crit-05 | crit | Turn lock + ownership enforced in /move /fly /distance | done | impl-A1 | 1 |
| crit-06 | crit | Portal token validator wired (replace nil + e2e-token placeholder) | done | impl-A2 | 1 |
| crit-07 | crit | PlayerNotifier wired in NewApprovalHandler (DM on approve/reject) | done | impl-A2 | 1 |

## High (visible regressions, clear spec drift)

| id | severity | title | status | worker | rev |
|---|---|---|---|---|---|
| high-08 | high | OnCharacterUpdated wired from mutation sites; populate Conditions/Concentration/Exhaustion in buildCardData | done | impl-H-par | 1 |
| high-09 | high | RollHistoryLogger production adapter — drop nils in discord_handlers.go | done | impl-H-main | 1 |
| high-10 | high | mapRegenerator field set on discordHandlerDeps; PostCombatMap posts PNG | done | impl-H-main | 1 |
| high-11 | high | Spell handler family wired (Cast/CastAoE/PrepareSpells/FontOfMagic/ReadyAction); ValidateSilenceZone + zone CRUD residue tracked in med-25 + med-26 | done | impl-C | 1 |
| high-12 | high | Magic item active abilities reachable: UseActiveAbility, DawnRecharge, CastIdentify, StudyItemDuringRest, DetectMagicItems | done | impl-H-par | 1 |
| high-13 | high | Loot dashboard, item picker, shops, party rest API handlers constructed in main.go | done | impl-H-main | 1 |
| high-14 | high | Phase 9b MessageQueue used in production sends (rate-limit retry) | done | impl-H-main | 1 |
| high-15 | high | /help wired via SetHelpHandler in production | done | impl-C | 1 |
| high-16 | high | DDB import re-sync gates UpdateCharacterFull on DM approval | done | impl-H-par | 1 |
| high-17 | high | OAuth/portal API surface — RegisterRoutes with WithAPI + WithCharacterSheet | done | impl-H-main | 1 |

## Medium

| id | severity | title | status | worker | rev |
|---|---|---|---|---|---|
| med-18 | med | Phase 25 initiative tracker auto-post + auto-update in #initiative-tracker | done | impl-MB1 | 1 |
| med-19 | med | Phase 26b end-combat: concentration end + (ammo deferred) + timer cancellation | done | impl-MB1 | 1 |
| med-20 | med | Phase 26a first-combatant ping on StartCombat | done | impl-MB1 | 1 |
| med-21 | med | Phase 30 /move look up creature size + max speed (drop Medium/30ft hardcodes) | done | impl-MB1 | 1 |
| med-22 | med | Phase 37 thrown weapon removed from hand | done | impl-orch | 1 |
| med-23 | med | Phase 38 Reckless first-attack-only enforcement | done | impl-orch | 1 |
| med-24 | med | Phase 55 OAs invoked from /move; PC reach weapons supported | done | impl-MB3 | 1 |
| med-25 | med | Phase 61 silence zone — Cast pre-validates ValidateSilenceZone | done | impl-MB1 | 1 |
| med-26 | med | Phase 67 Cast invokes zone creation; ZoneDefinition.AnchorMode added | done | impl-MB2 | 1 |
| med-27 | med | Phase 68 FoW: explored history wired (two-range light + real shadowcasting deferred) | partial | impl-MB3 | 1 |
| med-28 | med | Phase 71 readied spell deducts slot + sets concentration | done | impl-MB1 | 1 |
| med-29 | med | Phase 72 counterspell — Subtle bypass + ErrSubtleSpellNotCounterspellable (prompt UI deferred) | partial | impl-MB2 | 1 |
| med-30 | med | Phase 66b metamagic — interactive Empowered/Careful/Heightened prompts | deferred | - | 0 |
| med-31 | med | Phase 75b stealth_disadv honored by /check stealth; heavy-armor speed penalty applied | done | impl-MB2 | 1 |
| med-32 | med | Phase 81 /check target option used; group/contested/passive checks wired | done | impl-MB1 | 1 |
| med-33 | med | Phase 82 FeatureEffects populated in save_handler (Aura of Protection, Bless, magic-item, dodge-on-DEX) | done | impl-MB1 | 1 |
| med-34 | med | Phase 83a rest gated on DM approval | done | impl-MB1 | 1 |
| med-35 | med | Phase 84 /use and /give combat costs + delete dead PotionBonusAction field | done | impl-MB3 | 2 |
| med-36 | med | Phase 89 ASI/feat select-menu implemented (drop "not yet available" stub) | done | impl-MB2 | 1 |
| med-37 | med | Phase 99/101 Homebrew + Character Overview Svelte UIs | done | impl-MB2 | 1 |
| med-38 | med | Phase 104b publisher fan-out: chunk recommendation stale — verdict not-applicable (rest is functional, magicitem has no Service struct; existing publisher hooks cover the dashboard event needs) | not-applicable | impl-MB3 | 2 |
| med-39 | med | Phase 21a App.svelte campaign UUID — derive from session, drop placeholder | done | impl-MB1 | 1 |
| med-40 | med | Phase 15 Campaign Home counts live (DMQueueCount, PendingApprovals) | done | impl-MB1 | 1 |
| med-41 | med | Phase 11 production code path calls Service.CreateCampaign (or /setup auto-creates) | done | impl-MB1 | 1 |
| med-42 | med | Phase 20 ASSET_DATA_DIR documented or defaulted to /data/assets on fly | done | impl-orch | 1 |
| med-43 | med | Class features (Wild Shape spellblock + auto-revert, Rage end-of-turn, Bardic sweep done; Stunning Strike + Smite + Uncanny Dodge prompts deferred) | partial | impl-MB2 | 1 |

## Low / cosmetic

| id | severity | title | status | worker | rev |
|---|---|---|---|---|---|
| low-44 | low | Phase 5 LogSpellValidationWarnings invoked at startup | done | impl-orch | 1 |
| low-45 | low | Phase 3 condition seeder ships 16 vs spec 15 (surprised) — reconcile | done | impl-orch | 1 |
| low-46 | low | Phase 6 spot-check uses "+1 Longsword" not generic weapon-plus-1 | done | impl-orch | 1 |
| low-47 | low | TurnTimer.Stop wraps in sync.Once to prevent double-stop panic | done | impl-orch | 1 |
| low-48 | low | FES effect types EffectAura/EffectDMResolution/EffectReplaceRoll/EffectGrantProficiency consumed or removed | done | impl-orch | 1 |

## Phase 121.4 transcripts (last)

| id | severity | title | status | worker | rev |
|---|---|---|---|---|---|
| pt-49 | pt | Phase 121.4 record real transcripts after playtest-quickstart green | pending | - | 0 |
