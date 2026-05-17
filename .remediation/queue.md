# Remediation Queue

Total findings: 448
Critical: 35
High: 98
Medium: 173
Low: 142

## Queue

| # | ID | Severity | Status | Title | Location |
|---|---|---|---|---|---|
| 1 | A-C01 | Critical | done | `/setup` lets any guild member silently become the campaign DM | cmd/dndnd/discord_adapters.go:135-163 |
| 2 | A-C02 | Critical | done | Dashboard approval endpoints aren't scoped to the DM's own campaign | internal/dashboard/approval_handler.go:230-338 (Approve/Reject/RequestChanges) |
| 3 | B-C01 | Critical | done | `ParseExpression` mangles modifiers with multiple `+`/`-` operators | `/home/ab/projects/DnDnD/internal/dice/dice.go:46-58` |
| 4 | B-C02 | Critical | done | `cryptoRand` / `RollD20` panic on degenerate dice (`Nd0`) | `/home/ab/projects/DnDnD/internal/dice/roller.go:48-54`, |
| 5 | C-C01 | Critical | done | Multi-letter column labels truncated by `colToIndex` | /home/ab/projects/DnDnD/internal/combat/attack.go:1571-1577 |
| 6 | C-C02 | Critical | done | Reckless Attack advantage missing on attacks 2+ | /home/ab/projects/DnDnD/internal/combat/attack.go:887-901, advantage.go:36-39 |
| 7 | C-C03 | Critical | done | Off-hand (TWF) attack lacks Attack-action prerequisite and melee weapon check | /home/ab/projects/DnDnD/internal/combat/attack.go:1147-1200 |
| 8 | C-C04 | Critical | done | `/fly` performs no fly-speed validation | /home/ab/projects/DnDnD/internal/combat/altitude.go:52-81; /home/ab/projects/DnD... |
| 9 | D-C01 | Critical | done | Rage damage resistance never fires for seed-created barbarians | /home/ab/projects/DnDnD/internal/combat/feature_integration.go:347 and /home/ab/... |
| 10 | D-C02 | Critical | done | Feature uses never initialized at character creation | /home/ab/projects/DnDnD/internal/portal/builder_store_adapter.go:125 (CreateChar... |
| 11 | D-C03 | Critical | done | Rage advantage on STR ability checks never wired | /home/ab/projects/DnDnD/internal/check/check.go (no FES integration) and /home/a... |
| 12 | D-C04 | Critical | done | Save handler never sets IsRaging in EffectContext | /home/ab/projects/DnDnD/internal/discord/save_handler.go:199 |
| 13 | E-C01 | Critical | done | Single-target spell casts never apply damage or healing | `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:584-598`, `/home/ab/pro... |
| 14 | E-C02 | Critical | done | AoE damage path ignores upcasting and cantrip scaling | `/home/ab/projects/DnDnD/internal/combat/aoe.go:851` (`ResolveAoEPendingSaves` r... |
| 15 | E-C03 | Critical | done | Dodge condition does not impose disadvantage on attackers | `/home/ab/projects/DnDnD/internal/combat/advantage.go:104-134` (no `dodge` case ... |
| 16 | F-C01 | Critical | done | Counterspell trigger is unreachable from the DM dashboard | /home/ab/projects/DnDnD/dashboard/svelte/src/ActiveReactionsPanel.svelte:88-150 |
| 17 | F-C02 | Critical | done | Heavy-armor STR speed penalty is computed but never applied to combat speed | /home/ab/projects/DnDnD/internal/combat/equip.go:237,478-487; /home/ab/projects/... |
| 18 | F-C03 | Critical | done | Devil's Sight is never wired into the player vision pipeline | /home/ab/projects/DnDnD/cmd/dndnd/discord_adapters.go:755-806 |
| 19 | F-C04 | Critical | done | Lair Action is placed at the head of the turn queue instead of "loses ties" | /home/ab/projects/DnDnD/internal/combat/legendary.go:304-348 |
| 20 | G-C01 | Critical | done | Passive-effect vocabulary in spec does not match the code parser | internal/magicitem/effects.go:112-160, internal/combat/feature_integration.go:58... |
| 21 | G-C02 | Critical | done | `/attune` does not require a short rest | internal/inventory/attunement.go:33-67, internal/discord/attune_handler.go:68-15... |
| 22 | G-C03 | Critical | done | `destroy_on_zero` roll happens at dawn, not when last charge is spent | internal/inventory/recharge.go:38-92, internal/inventory/active_ability.go:28-62 |
| 23 | G-C04 | Critical | done | Antitoxin "advantage vs poison" is not actually tracked | internal/inventory/service.go:135-140 |
| 24 | H-C01 | Critical | done | Single-class half-caster (Paladin/Ranger) gets the wrong slot count | /home/ab/projects/DnDnD/internal/character/spellslots.go:108 |
| 25 | H-C02 | Critical | done | Feat prerequisites and "already-has-feat" exclusion not enforced anywhere in the live picker | /home/ab/projects/DnDnD/cmd/dndnd/discord_handlers.go:1155 (`asiFeatLister.ListE... |
| 26 | H-C03 | Critical | done | Level-up does not auto-add new class/subclass features | /home/ab/projects/DnDnD/internal/levelup/service.go:186 (`ApplyLevelUp`) |
| 27 | H-C04 | Critical | done | DDB import bypasses DM approval queue on first import | /home/ab/projects/DnDnD/internal/ddbimport/service.go:139 |
| 28 | H-C05 | Critical | done | Levelup HTTP handler does not bound newLevel to 20 | /home/ab/projects/DnDnD/internal/levelup/handler.go:106 |
| 29 | I-C01 | Critical | done | DM-created characters never inherit class or racial features | /home/ab/projects/DnDnD/internal/dashboard/feature_provider.go:38, 49, 63-65 ; /... |
| 30 | I-C02 | Critical | done | Pending #dm-queue badge count is campaign-wide, not per-encounter | /home/ab/projects/DnDnD/internal/combat/workspace_handler.go:173-180, 271 ; quer... |
| 31 | I-C03 | Critical | done | Narration-template Get/Update/Delete/Duplicate/Apply leak across campaigns | /home/ab/projects/DnDnD/internal/narration/template_handler.go:110-193 ; /home/a... |
| 32 | J-C01 | Critical | done | WebSocket subscribes to any encounter without campaign-ownership check | /home/ab/projects/DnDnD/internal/dashboard/ws.go:135 |
| 33 | J-C02 | Critical | done | Open5e public search endpoint bypasses per-campaign source gating | /home/ab/projects/DnDnD/internal/open5e/handler.go:37 (`RegisterPublicRoutes`); ... |
| 34 | J-C03 | Critical | done | Open5e HTTP client has no timeout — upstream stall can hang any /search request | /home/ab/projects/DnDnD/internal/open5e/client.go:43 |
| 35 | cross-cut-C01 | Critical | done | Channel Divinity recharges on long rest, not short rest | `internal/combat/channel_divinity_integration_test.go:44`, |
| 36 | A-H01 | High | done | Player can never resubmit after `changes_requested` (broken status flow) | internal/registration/service.go:46-56 + internal/dashboard/approval_store.go:30... |
| 37 | A-H02 | High | skipped | OAuth access/refresh tokens stored in plaintext | internal/auth/session_store.go:50-62 + db/migrations/20260310120001_create_sessi... |
| 38 | A-H03 | High | done | WebSocket origin verification defaults to `InsecureSkipVerify: true` | internal/dashboard/handler.go:117-170 (default `wsInsecureSkipVerify: true`), in... |
| 39 | A-H04 | High | done | OAuth callback handler treats any 4xx error from Discord as a generic 403 | internal/auth/oauth2.go:150-156, 178-182 |
| 40 | A-H05 | High | done | Portal token redemption has a TOCTOU race | internal/portal/token_service.go:82-90 + internal/portal/token_store.go:81-88 |
| 41 | A-H06 | High | skipped | HP calculation always uses fixed-average; no rolled-HP path | internal/character/stats.go:21-47 |
| 42 | A-H07 | High | done | Welcome DM sent to every joining member even when no campaign exists | internal/discord/bot.go:119-131 + internal/discord/welcome.go:6-19 |
| 43 | A-H08 | High | done | Fuzzy match suggestion message renders incorrectly when multiple matches | internal/discord/registration_handler.go:97-100 |
| 44 | A-H09 | High | done | Sessions middleware re-issues cookie even when slide TTL fails silently | internal/auth/middleware.go:62-77 |
| 45 | B-H01 | High | done | Map size limits not enforced when rendering, only at create-time | `/home/ab/projects/DnDnD/internal/gamemap/renderer/renderer.go:12-16`; |
| 46 | B-H02 | High | done | `RenderMap` mutates caller-supplied `MapData.TileSize` | `/home/ab/projects/DnDnD/internal/gamemap/renderer/renderer.go:13-16` |
| 47 | B-H03 | High | done | Asset upload accepts arbitrary MIME types (XSS / file-type abuse risk) | `/home/ab/projects/DnDnD/internal/asset/handler.go:36-83`, |
| 48 | B-H04 | High | done | Map renderer never composites the uploaded background image | `/home/ab/projects/DnDnD/internal/gamemap/renderer/renderer.go` |
| 49 | B-H05 | High | done | `TilesetRefs` request field silently dropped by HTTP handler | `/home/ab/projects/DnDnD/internal/gamemap/handler.go:65-82, |
| 50 | B-H06 | High | done | DM-view fog-of-war ignores `MapData.DMSeesAll` when caller pre-computed fog *without* setting the flag on `FogOfWar` | `/home/ab/projects/DnDnD/internal/gamemap/renderer/renderer.go:33-47`, |
| 51 | B-H07 | High | superseded | Fog renderer does not preserve "previously seen" cells across renders | `/home/ab/projects/DnDnD/internal/gamemap/renderer/fog_types.go:68-93` |
| 52 | C-H01 | High | done | Auto-crit applies to ranged attacks within 5ft against paralyzed/unconscious | /home/ab/projects/DnDnD/internal/combat/attack.go:727-748 (`CheckAutoCrit`) |
| 53 | C-H02 | High | done | PC creature size hard-coded to "Medium" — heavy-weapon disadvantage never fires for halflings/gnomes | /home/ab/projects/DnDnD/internal/combat/attack.go:1316-1326 |
| 54 | C-H03 | High | done | Crossbow Expert does not waive ranged-with-hostile-adjacent disadvantage | /home/ab/projects/DnDnD/internal/combat/advantage.go:88-91 |
| 55 | C-H04 | High | done | Dash adds raw base speed, ignoring exhaustion/condition speed modifiers | /home/ab/projects/DnDnD/internal/combat/standard_actions.go:38-71 (`Dash`), 73-8... |
| 56 | C-H05 | High | done | Fall damage missing 20d6 cap | /home/ab/projects/DnDnD/internal/combat/altitude.go:101-123 (`FallDamage`) |
| 57 | C-H06 | High | done | Resistance/vulnerability halving allows damage to go to 0 (RAW says min 1) | /home/ab/projects/DnDnD/internal/combat/damage.go:38-43 (`ApplyDamageResistances... |
| 58 | C-H07 | High | done | Pre-clamp HP overflow excludes temp-HP absorbed damage from instant-death check | /home/ab/projects/DnDnD/internal/combat/damage.go:226-247, 330-373 |
| 59 | C-H08 | High | superseded | Off-hand attack accepts non-melee "light" weapons | /home/ab/projects/DnDnD/internal/combat/attack.go:1182-1196 |
| 60 | C-H09 | High | done | Diagonal pathfinding ignores wall edges entirely (could allow phasing through a single diagonal wall) | /home/ab/projects/DnDnD/internal/pathfinding/pathfinding.go:242-244 |
| 61 | C-H10 | High | done | Reach weapon OA detection — PC reach map relies on caller passing it | /home/ab/projects/DnDnD/internal/combat/opportunity_attack.go:80-117, 148-164 (`... |
| 62 | C-H11 | High | done | Concentration-on-damage save uses simplified DC formula | /home/ab/projects/DnDnD/internal/combat/concentration.go:422-448 (`MaybeCreateCo... |
| 63 | C-H12 | High | superseded | Surprise: surprised condition removed at start of "skip turn", not end (timing nuance) | /home/ab/projects/DnDnD/internal/combat/initiative.go:582-606 (`skipSurprisedTur... |
| 64 | D-H01 | High | done | Step of the Wind dash adds remaining movement, not base speed | /home/ab/projects/DnDnD/internal/combat/monk.go:444 |
| 65 | D-H02 | High | superseded | Dodge condition grants no defensive disadvantage to attackers | /home/ab/projects/DnDnD/internal/combat/advantage.go:104 (switch on `c.Condition... |
| 66 | D-H03 | High | done | Auto-ability selection for finesse weapons silently disables rage damage | /home/ab/projects/DnDnD/internal/combat/attack.go:1583 (`attackAbilityUsed`) |
| 67 | D-H04 | High | done | Monk Unarmored Defense not invalidated by shield | /home/ab/projects/DnDnD/internal/combat/equip.go:416 and /home/ab/projects/DnDnD... |
| 68 | D-H05 | High | done | Monk Unarmored Movement not gated on "no shield" | /home/ab/projects/DnDnD/internal/combat/monk.go:487 (`UnarmoredMovementFeature`)... |
| 69 | D-H06 | High | done | Wild Shape on-revert does not restore the druid's speed snapshot | /home/ab/projects/DnDnD/internal/combat/wildshape.go:181 (`RevertWildShape`) |
| 70 | D-H07 | High | superseded | Wild Shape activation does not block druid spellcasting | /home/ab/projects/DnDnD/internal/combat/spellcasting.go:381 and /home/ab/project... |
| 71 | D-H08 | High | done | Channel Divinity action validation is duplicated and racy across DM-queue + auto-resolved paths | /home/ab/projects/DnDnD/internal/combat/channel_divinity.go:160, :366, :446, :52... |
| 72 | E-H01 | High | done | Help action grants advantage only on attacks, not on ability checks | `/home/ab/projects/DnDnD/internal/combat/standard_actions.go:254-261`; `advantag... |
| 73 | E-H02 | High | done | AoE pending save DC subtraction loses cover information | `/home/ab/projects/DnDnD/internal/combat/aoe.go:592` (`Dc: int32(ps.DC - ps.Cove... |
| 74 | E-H03 | High | done | Pact-magic upcast respects pact level but silently ignores `--slot` requests | `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:446-457` |
| 75 | E-H04 | High | done | Multiclass spellcasting ability picks highest score, not class-of-spell | `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:1542-1557` (`resolveSpe... |
| 76 | E-H05 | High | done | Spell attack rolls never apply advantage/disadvantage | `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:638` (`roller.RollD20(a... |
| 77 | E-H06 | High | superseded | Concentration check DC always fires "max(10, dmg/2)" but DC=10 isn't max with damage 19 | `/home/ab/projects/DnDnD/internal/combat/concentration.go:18-24` |
| 78 | E-H07 | High | superseded | AoE damage applies `int(float64(baseDamage)*0.5)` truncates instead of rounding | `/home/ab/projects/DnDnD/internal/combat/aoe.go:1024` |
| 79 | F-H01 | High | done | No light-source dim radius — 5e torches grant 20ft bright + 20ft dim | /home/ab/projects/DnDnD/cmd/dndnd/discord_adapters.go:907-927 |
| 80 | F-H02 | High | superseded | Hide action ignores the actor's vision when computing zone obscurement | /home/ab/projects/DnDnD/internal/discord/action_handler.go:794-805 |
| 81 | F-H03 | High | done | Hidden combatants (`is_visible = false`) still render on the map | /home/ab/projects/DnDnD/internal/gamemap/renderer/fog.go:52-78; /home/ab/project... |
| 82 | F-H04 | High | done | Free-object interaction whitelist is too permissive / English-only | /home/ab/projects/DnDnD/internal/combat/interact.go:13-52 |
| 83 | F-H05 | High | done | Lair-action "no consecutive repeats" tracker is in-memory only | /home/ab/projects/DnDnD/internal/combat/legendary.go:198-263 |
| 84 | F-H06 | High | done | Legendary-action budget round-trips through the URL — no server persistence | /home/ab/projects/DnDnD/internal/combat/legendary_handler.go:73-78,170-180 |
| 85 | F-H07 | High | done | Counterspell trigger does not validate spell range / line-of-sight | /home/ab/projects/DnDnD/internal/combat/counterspell.go:65-116 |
| 86 | F-H08 | High | skipped | Reaction declarations not validated for the type's prerequisites | /home/ab/projects/DnDnD/internal/combat/reaction.go:27-46 |
| 87 | G-H01 | High | done | Gold split silently discards remainder | internal/loot/service.go:289-329 |
| 88 | G-H02 | High | done | Long-rest hit-dice restoration order is non-deterministic for multiclass | internal/rest/rest.go:409-441 |
| 89 | G-H03 | High | skipped | No combat-resumed long-rest auto-resume | internal/rest/party.go:17-22, internal/rest/party_handler.go:269-308 |
| 90 | G-H04 | High | done | `/check medicine target:AR` does not validate target is dying and does not auto-stabilize | internal/discord/check_handler.go:286-320, internal/check/check.go:111-151 |
| 91 | G-H05 | High | done | Items auto-populated from defeated NPCs are not removed from NPC inventory | internal/loot/service.go:67-142 |
| 92 | G-H06 | High | done | Item picker only searches weapons/armor/magic items | internal/itempicker/handler.go:57-156 |
| 93 | G-H07 | High | done | No way to edit description / name of an existing loot pool item | internal/loot/service.go (no Update), internal/loot/api_handler.go |
| 94 | G-H08 | High | done | Long rest does not propagate dawn recharge to party rest persistence | internal/rest/party_handler.go:180-216 |
| 95 | G-H09 | High | done | Encounter-active check on rest can be bypassed for party rest if `HasActiveEncounter` returns false | internal/discord/rest_handler.go:159-164 |
| 96 | H-H01 | High | done | Player-identity not validated on ASI button / select interactions | /home/ab/projects/DnDnD/internal/discord/asi_handler.go:354,391,647,731 |
| 97 | H-H02 | High | done | DM approve/deny buttons have no role check | /home/ab/projects/DnDnD/internal/discord/asi_handler.go:456 (`HandleDMApprove`),... |
| 98 | H-H03 | High | done | ASI ApproveASI silently rejects feat type instead of routing | /home/ab/projects/DnDnD/internal/levelup/asi.go:35 (`ApplyASI`) |
| 99 | H-H04 | High | done | DDB "off-list spell" detection only covers wizard with 16 spells | /home/ab/projects/DnDnD/internal/ddbimport/parser.go:382 (`classSpellLists`) |
| 100 | H-H05 | High | done | Builder service: token redeem races and isn't user-bound | /home/ab/projects/DnDnD/internal/portal/builder_service.go:219-238 (`CreateChara... |
| 101 | H-H06 | High | done | DDB import attunement-limit warning uses wrong signal | /home/ab/projects/DnDnD/internal/ddbimport/validator.go:102-112 |
| 102 | H-H07 | High | done | Character sheet does not render conditions / active status effects | /home/ab/projects/DnDnD/internal/portal/character_sheet.go:20 (struct `Character... |
| 103 | H-H08 | High | done | Starting equipment retains `any-martial` placeholder IDs in inventory | /home/ab/projects/DnDnD/internal/portal/starting_equipment.go:33 + /home/ab/proj... |
| 104 | H-H09 | High | done | Starting equipment ignores background packs | /home/ab/projects/DnDnD/internal/portal/starting_equipment.go (only class packs ... |
| 105 | H-H10 | High | done | DeriveSpeed ignores race | /home/ab/projects/DnDnD/internal/portal/builder_store_adapter.go:275 |
| 106 | H-H11 | High | done | DDB class names not normalised to internal IDs | /home/ab/projects/DnDnD/internal/ddbimport/parser.go:177; /home/ab/projects/DnDn... |
| 107 | H-H12 | High | done | Plus-2 ASI silently truncates at cap (loses 1 point) without warning | /home/ab/projects/DnDnD/internal/levelup/asi.go:81 (`applyPlus2`) |
| 108 | H-H13 | High | done | /api/levelup/asi/approve endpoint has no character-owner / DM check | /home/ab/projects/DnDnD/internal/levelup/handler.go:129 (`HandleApproveASI`) |
| 109 | I-H01 | High | done | Dashboard DM-created chars miss background skill proficiencies | /home/ab/projects/DnDnD/internal/dashboard/charcreate.go:221-253, 117 ; compare ... |
| 110 | I-H02 | High | done | DM character form doesn't pass campaign_id to spell/equipment refdata | /home/ab/projects/DnDnD/internal/dashboard/charcreate_handler.go:30-34, 281-318,... |
| 111 | I-H03 | High | skipped | Encounter Builder doesn't place PC tokens at combat start | /home/ab/projects/DnDnD/dashboard/svelte/src/EncounterBuilder.svelte:368-389 |
| 112 | I-H04 | High | done | Action Resolver `move` effect bypasses turn lock, walls, and concentration hooks | /home/ab/projects/DnDnD/internal/combat/dm_dashboard_handler.go:215-313, 400-421 |
| 113 | I-H05 | High | skipped | Active reactions panel highlights every active reaction on enemy turns | /home/ab/projects/DnDnD/dashboard/svelte/src/ActiveReactionsPanel.svelte:65-68 |
| 114 | I-H06 | High | done | Cross-tenant reads on character overview / narration history / message history | /home/ab/projects/DnDnD/internal/characteroverview/handler.go:35-47 ; /home/ab/p... |
| 115 | I-H07 | High | done | Narration & message-player handlers trust author_user_id from request body | /home/ab/projects/DnDnD/internal/narration/handler.go:49-91 ; /home/ab/projects/... |
| 116 | I-H08 | High | skipped | Movement-validation rules differ between drag-and-drop UI and DM Override | /home/ab/projects/DnDnD/dashboard/svelte/src/CombatManager.svelte:691-717 ; /hom... |
| 117 | I-H09 | High | superseded | Manual character creation skips ability-score method validation | /home/ab/projects/DnDnD/internal/dashboard/charcreate.go:72-77 |
| 118 | I-H10 | High | skipped | Race speed table is hard-coded; ignores DB and homebrew races | /home/ab/projects/DnDnD/internal/dashboard/charcreate.go:204-217 |
| 119 | I-H11 | High | done | DM character creation handler is not protected by DM auth | /home/ab/projects/DnDnD/internal/dashboard/charcreate_handler.go:83-103, 112-138... |
| 120 | J-H01 | High | done | Saved/Active encounter Campaign Home cards show player-facing display_name, not the spoilery internal name | /home/ab/projects/DnDnD/cmd/dndnd/main.go:243-246 and 261-265 (`encounterListerA... |
| 121 | J-H02 | High | done | Reaction-declaration → dm-queue itemID mapping is in-memory only; lost on restart breaks /reaction cancel | /home/ab/projects/DnDnD/internal/discord/reaction_handler.go:51-86 (`itemIDs map... |
| 122 | J-H03 | High | done | DM dashboard error panel cannot render stack trace / structured detail — error_detail column never written | /home/ab/projects/DnDnD/internal/errorlog/recorder.go:18-29 (`Entry`); /home/ab/... |
| 123 | J-H04 | High | skipped | /help "Context Tips" shows hardcoded text, not actual remaining resources | /home/ab/projects/DnDnD/internal/discord/help_handler.go:80-96 |
| 124 | J-H05 | High | done | One character can be in two active encounters (no DB constraint; LIMIT 1 in query masks the bug) | /home/ab/projects/DnDnD/db/queries/encounters.sql:46-51 (`GetActiveEncounterIDBy... |
| 125 | J-H06 | High | done | /whisper accepts empty message and spams a dm-queue item | /home/ab/projects/DnDnD/internal/discord/whisper_handler.go:61-80 |
| 126 | J-H07 | High | skipped | dm-queue Sender bypasses the per-channel MessageQueue (rate-limit ordering) | /home/ab/projects/DnDnD/internal/dmqueue/sender.go:19-41 |
| 127 | J-H08 | High | done | WS reader/writer can race on slow-client drop | /home/ab/projects/DnDnD/internal/dashboard/ws.go:77-101 (`Hub.Run` broadcast blo... |
| 128 | J-H09 | High | done | Encounter snapshot publisher does NOT trigger on /move position writes | /home/ab/projects/DnDnD/internal/discord/move_handler.go:686-735; combat.Service... |
| 129 | cross-cut-H01 | High | done | `routePhase43DeathSave` skips the drop-to-0 instant-death rule when overflow is exactly the limit but damage came from a hit at >0 HP and `adjusted` overshoots | `internal/combat/damage.go:336-346`, |
| 130 | cross-cut-H02 | High | superseded | Multiclass spellcasting ability picks the highest score across classes | `internal/combat/spellcasting.go:1544-1557` |
| 131 | cross-cut-H03 | High | done | Attack roll always adds proficiency bonus regardless of weapon proficiency | `internal/combat/attack.go:103-106` (`AttackModifier`). |
| 132 | cross-cut-H04 | High | done | Paladin Channel Divinity max uses scale to 2 at level 15 | `internal/combat/channel_divinity.go:31-38` |
| 133 | cross-cut-H05 | High | done | Action Surge max uses never scales to 2 at fighter level 17 | every `action-surge` feature seed asserts `Max: 1` |
| 134 | A-M01 | Medium | done | `MessageQueue.Stop` doesn't preempt long backoff sleeps | internal/discord/queue.go:90-134 |
| 135 | A-M02 | Medium | done | `SplitMessage` splits on bytes, can produce invalid UTF-8 mid-codepoint | internal/discord/message.go:67-122 |
| 136 | A-M03 | Medium | done | Fuzzy match Levenshtein operates on bytes, not runes | internal/registration/fuzzy.go:10-40 |
| 137 | A-M04 | Medium | done | `ShortID` operates on bytes, may produce invalid UTF-8 for non-ASCII names | internal/charactercard/shortid.go:21-29 |
| 138 | A-M05 | Medium | superseded | WebSocket hub channels are unbuffered and synchronous | internal/dashboard/ws.go:31-103 |
| 139 | A-M06 | Medium | done | Approval POST endpoints accept POST without CSRF protection | internal/dashboard/approval_handler.go:50-58, 230-338 |
| 140 | A-M07 | Medium | done | Expertise grants double-prof bonus even when not proficient | internal/character/modifiers.go:18-35 |
| 141 | A-M08 | Medium | superseded | `error_log.error_detail` column written by no Go code | internal/errorlog/pgstore.go:79-83 + db/migrations/20260427120001_create_error_l... |
| 142 | A-M09 | Medium | done | Spell-slots map sorted lexicographically — slot "10" would precede "2" | internal/charactercard/format.go:152-170 |
| 143 | A-M10 | Medium | done | `CreatePlaceholder` inserts `ac = 0` for new characters | internal/registration/service.go:122-141 |
| 144 | A-M11 | Medium | done | Welcome DM message hardcodes channel names that may not exist | internal/discord/welcome.go:6-19 |
| 145 | A-M12 | Medium | superseded | `/setup` handler runs no authorization check beyond Discord's default-perms hint | internal/discord/setup.go:217-249 |
| 146 | A-M13 | Medium | pending | HP recompute on multiclassing assumes secondary classes never reach level 1 with max die | internal/character/stats.go:30-42 |
| 147 | A-M14 | Medium | pending | `setup` channel creation has no rollback on partial failure | internal/discord/setup.go:128-182 |
| 148 | B-M01 | Medium | done | Initiative DEX-tie alphabetical sort is byte-wise, not D&D-aware | `/home/ab/projects/DnDnD/internal/combat/initiative.go:166-177` |
| 149 | B-M02 | Medium | done | PNG renderer renders zero-cost / oversized canvas for invalid `MapData` | `/home/ab/projects/DnDnD/internal/gamemap/renderer/renderer.go:18-30`. |
| 150 | B-M03 | Medium | pending | Encounter map_id is nullable in the schema but Phase 22/23/26 assume it | `/home/ab/projects/DnDnD/db/migrations/20260312120001_create_encounter_templates... |
| 151 | B-M04 | Medium | pending | Encounter template `Duplicate` does not generate fresh `short_id`s | `/home/ab/projects/DnDnD/internal/encounter/service.go:115-139`. |
| 152 | B-M05 | Medium | pending | No validation that `position_col` / `position_row` are inside the map bounds | `/home/ab/projects/DnDnD/internal/combat/service.go:863-913` |
| 153 | B-M06 | Medium | pending | DB does not enforce the map-dimension hard limit | `/home/ab/projects/DnDnD/db/migrations/20260310120009_create_maps.sql:6-7`. |
| 154 | B-M07 | Medium | pending | Tiled import accepts a `width=0, height=0` map if it's not `infinite` | `/home/ab/projects/DnDnD/internal/gamemap/import.go:88-103`. |
| 155 | B-M08 | Medium | pending | Stacked-token offset can place tokens outside their grid cell | `/home/ab/projects/DnDnD/internal/gamemap/renderer/token.go:28-36`. |
| 156 | B-M09 | Medium | pending | `/api/assets/upload` response sets headers after potentially writing body | `/home/ab/projects/DnDnD/internal/asset/handler.go:103-111` |
| 157 | B-M10 | Medium | pending | Local asset storage path-traversal: filename is discarded, but campaign UUID isn't validated | `/home/ab/projects/DnDnD/internal/asset/local_store.go:32-46`. |
| 158 | B-M11 | Medium | pending | Map `UpdateMap` allows shrinking width/height without re-clipping `tiled_json` | `/home/ab/projects/DnDnD/internal/gamemap/service.go:124-151`. |
| 159 | B-M12 | Medium | pending | `RenderQueue` never times-out or drops requests on render failure | `/home/ab/projects/DnDnD/internal/gamemap/renderer/queue.go:76-93`. |
| 160 | C-M01 | Medium | pending | Unarmed strike crit doubles the flat "1", not RAW dice | /home/ab/projects/DnDnD/internal/combat/attack.go:674-681 |
| 161 | C-M02 | Medium | pending | TWF "negative ability modifier still applies" RAW edge missed | /home/ab/projects/DnDnD/internal/combat/attack.go:1199-1202 |
| 162 | C-M03 | Medium | pending | Cover bonus to DEX save uses single closest-corner instead of best-of-4 | /home/ab/projects/DnDnD/internal/combat/cover.go:106-136 (`CalculateCoverFromOri... |
| 163 | C-M04 | Medium | pending | `lineBlockedByWalls` allows zero-determinant case to slip through | /home/ab/projects/DnDnD/internal/combat/cover.go:212-233 (`segmentsIntersect`) |
| 164 | C-M05 | Medium | pending | Off-hand TWF doesn't track on AttackerHidden / invisible attacker single-shot reveal | /home/ab/projects/DnDnD/internal/combat/attack.go:1238-1247 (`OffhandAttack`) |
| 165 | C-M06 | Medium | pending | Damage-at-0 crit gives +2 failures regardless of attacker distance | /home/ab/projects/DnDnD/internal/combat/deathsave.go:159-192 (`ApplyDamageAtZero... |
| 166 | C-M07 | Medium | done | `ValidateMove` rejects ending on ally's tile (spec says ally pass-through allowed; ending forbidden — fine — but message) | /home/ab/projects/DnDnD/internal/combat/movement.go:84-94 |
| 167 | C-M08 | Medium | pending | `tileCost` adds +5 for prone, conceptually using +5 not ×2 | /home/ab/projects/DnDnD/internal/pathfinding/pathfinding.go:284-294 |
| 168 | C-M09 | Medium | pending | Action consumption not flagged for /attack — features keying off ActionUsed misbehave | /home/ab/projects/DnDnD/internal/combat/attack.go:925 (UseAttack only decrements... |
| 169 | C-M10 | Medium | pending | Initiative tiebreak ignores DEX modifier ordering for surprised + tie cases | /home/ab/projects/DnDnD/internal/combat/initiative.go:167-177 |
| 170 | C-M11 | Medium | pending | Distance3D rounding-to-5 can flip cover/range edges | /home/ab/projects/DnDnD/internal/combat/altitude.go:22-33 (`Distance3D`, `roundT... |
| 171 | C-M12 | Medium | pending | Concentration save DC formula not capped (some house rules cap at DC 30) | /home/ab/projects/DnDnD/internal/combat/concentration.go ~ `CheckConcentrationOn... |
| 172 | C-M13 | Medium | pending | Off-hand attack uses `combatantDistance` 3D — would auto-crit against airborne paralyzed | /home/ab/projects/DnDnD/internal/combat/attack.go:1216, 953 |
| 173 | C-M14 | Medium | pending | Spec calls for "ammo recovery PROMPT" post-combat; code auto-recovers in EndCombat | /home/ab/projects/DnDnD/internal/combat/service.go:1145-1161 |
| 174 | D-M01 | Medium | pending | Divine Smite crit bonus computed twice when target is undead and crit | /home/ab/projects/DnDnD/internal/combat/divine_smite.go:59 (`SmiteDamageFormula`... |
| 175 | D-M02 | Medium | pending | Divine Smite eligibility doesn't enforce "weapon attack" | /home/ab/projects/DnDnD/internal/combat/divine_smite.go:52 (`IsSmiteEligible`) a... |
| 176 | D-M03 | Medium | pending | Action Surge resets `AttacksRemaining` from current character data instead of remembering the action's attack count | /home/ab/projects/DnDnD/internal/combat/action_surge.go:58 |
| 177 | D-M04 | Medium | pending | Bardic Inspiration self-grant rejected even when out of combat | /home/ab/projects/DnDnD/internal/combat/bardic_inspiration.go:151 |
| 178 | D-M05 | Medium | pending | Bardic Inspiration: no 60ft range validation | /home/ab/projects/DnDnD/internal/combat/bardic_inspiration.go (no distance check... |
| 179 | D-M06 | Medium | pending | PreserveLife heal target validation can mutate map iteration order under errors | /home/ab/projects/DnDnD/internal/combat/channel_divinity.go:625 |
| 180 | D-M07 | Medium | pending | Turn Undead does not differentiate "can see or hear" requirement | /home/ab/projects/DnDnD/internal/combat/channel_divinity.go:213 |
| 181 | D-M08 | Medium | pending | Wild Shape concentration retention not implemented | /home/ab/projects/DnDnD/internal/combat/wildshape.go:333 (`ActivateWildShape`) |
| 182 | D-M09 | Medium | pending | Stunning Strike duration uses `"end_of_turn"` with `DurationRounds: 1` | /home/ab/projects/DnDnD/internal/combat/monk.go:398 |
| 183 | D-M10 | Medium | pending | Rage rounds counter underflows below 0 | /home/ab/projects/DnDnD/internal/combat/rage.go:158 (`DecrementRageRound`) |
| 184 | D-M11 | Medium | pending | Resolution priority places `EffectModifyAttackRoll` and `EffectModifySave` at the same priority as immunities-after — but advantage cancellation is a later step | /home/ab/projects/DnDnD/internal/combat/effect.go:134 (`EffectPriority`) |
| 185 | D-M12 | Medium | pending | EffectGrantImmunity stores condition immunities in the same slice as damage immunities | /home/ab/projects/DnDnD/internal/combat/effect.go:322 |
| 186 | D-M13 | Medium | pending | Resource_on_hit prompt only fired by Divine Smite, not by feature declarations | /home/ab/projects/DnDnD/internal/combat/class_feature_prompt.go and /home/ab/pro... |
| 187 | D-M14 | Medium | pending | Lay on Hands self-targeting can heal undead/construct PCs without rejection | /home/ab/projects/DnDnD/internal/combat/lay_on_hands.go:58 |
| 188 | D-M15 | Medium | pending | Channel Divinity DC uses WIS for both Cleric and Paladin | /home/ab/projects/DnDnD/internal/combat/channel_divinity.go:186 |
| 189 | E-M01 | Medium | pending | Twinned Spell does not enforce "single creature target" beyond AoE/self check | `/home/ab/projects/DnDnD/internal/combat/metamagic.go:108-116` |
| 190 | E-M02 | Medium | pending | Pact slot deduction does not refuse when `effectiveSlotLevel == 0` and damage path requires upcast | `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:444-457` |
| 191 | E-M03 | Medium | pending | Hide: success comparison ties go to perceiver, but spec says "meets or exceeds" | `/home/ab/projects/DnDnD/internal/combat/standard_actions.go:361` |
| 192 | E-M04 | Medium | pending | Help action duration tied to ally's turn, not helper's turn | `/home/ab/projects/DnDnD/internal/combat/standard_actions.go:254-261` (`SourceCo... |
| 193 | E-M05 | Medium | pending | Material component check treats `material_cost_gp = 0` as costly when `Valid = true` | `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:470` (`if spell.Materia... |
| 194 | E-M06 | Medium | pending | Stand from prone uses integer half (12 for speed 25) — matches Sage Advice but no rounding-direction test | `/home/ab/projects/DnDnD/internal/combat/condition_effects.go:254-257` |
| 195 | E-M07 | Medium | done | Spell range validation accepts unrecognized `range_type` values silently | `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:77-94` (`ValidateSpellR... |
| 196 | E-M08 | Medium | pending | Ritual casting only validates primary class, ignoring multiclass | `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:432-438` |
| 197 | E-M09 | Medium | pending | AoE save DC pre-subtracts cover bonus but stores no roll-side trace for full cover exclusion | `/home/ab/projects/DnDnD/internal/combat/aoe.go:545-562` |
| 198 | E-M10 | Medium | pending | OA detection uses `IsNpc` faction check, breaks for PC-vs-PC combat | `/home/ab/projects/DnDnD/internal/combat/opportunity_attack.go:106-108` |
| 199 | E-M11 | Medium | pending | Hide's auto-reveal-on-attack does not strip prior hide condition records | `/home/ab/projects/DnDnD/internal/combat/attack.go:830-839` |
| 200 | E-M12 | Medium | pending | Grapple/shove adjacency uses Chebyshev distance only — no altitude check | `/home/ab/projects/DnDnD/internal/combat/grapple_shove.go:73-80, 201-208` |
| 201 | E-M13 | Medium | pending | Push destination unoccupied-check ignores dead bodies and altitude | `/home/ab/projects/DnDnD/internal/combat/grapple_shove.go:221-233` |
| 202 | E-M14 | Medium | pending | `applyConcentrationOnCast` clears prior concentration even on cast failure later (no rollback) | `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:620-627` |
| 203 | E-M15 | Medium | pending | Cone shape projects from caster center, not tile edge | `/home/ab/projects/DnDnD/internal/combat/aoe.go:113-117` (`ConeAffectedTiles`) |
| 204 | E-M16 | Medium | pending | Concentration save uses `currentConcentration` name-string only (no spell ID) | `/home/ab/projects/DnDnD/internal/combat/concentration.go:36-45` (`CheckConcentr... |
| 205 | E-M17 | Medium | pending | Passive Perception for creatures lacks proficiency when Skills JSONB is empty | `/home/ab/projects/DnDnD/internal/combat/standard_actions.go:516-530` |
| 206 | E-M18 | Medium | pending | Hide's "spotted by" picks highest-PP enemy, but losing tied roll vs second-highest is hidden | `/home/ab/projects/DnDnD/internal/combat/standard_actions.go:347-360` |
| 207 | E-M19 | Medium | pending | Subtle Spell does not actually suppress concentration-break-in-silence | `/home/ab/projects/DnDnD/internal/combat/concentration.go:126-135` (`CheckConcen... |
| 208 | E-M20 | Medium | pending | Cunning Action passes stale `cmd.Turn` to `resolveHide` after consuming bonus action | `/home/ab/projects/DnDnD/internal/combat/standard_actions.go:856-872` |
| 209 | F-M01 | Medium | pending | Readied-spell concentration written with empty SpellID | /home/ab/projects/DnDnD/internal/combat/readied_action.go:126-141 |
| 210 | F-M02 | Medium | pending | Counterspell ability check uses character.ProficiencyBonus on the wrong side | /home/ab/projects/DnDnD/internal/combat/counterspell.go:209-260 |
| 211 | F-M03 | Medium | pending | `/done` unused-resource warning's "Action" branch is logically dead | /home/ab/projects/DnDnD/internal/combat/unused_resources.go:13-33 |
| 212 | F-M04 | Medium | pending | Magical-darkness zone affected-tiles ignore concentration-anchored zone movement | /home/ab/projects/DnDnD/cmd/dndnd/discord_adapters.go:819-833 |
| 213 | F-M05 | Medium | pending | Light cantrip / Continual Flame zones get 20ft uniform — but Daylight is 60ft bright + 60ft dim | /home/ab/projects/DnDnD/cmd/dndnd/discord_adapters.go:918-927 |
| 214 | F-M06 | Medium | pending | Hide-success token visibility doesn't propagate to enemy renders | /home/ab/projects/DnDnD/internal/combat/standard_actions.go:363-371 |
| 215 | F-M07 | Medium | pending | Surprised combatants — round 1 turn skip removes condition immediately, losing reaction-end-of-turn rule | /home/ab/projects/DnDnD/internal/combat/initiative.go:569-593 |
| 216 | F-M08 | Medium | pending | `equipWeapon` accidentally drops the Defense fighting style AC bonus when re-equipping | /home/ab/projects/DnDnD/internal/combat/equip.go:141-184 |
| 217 | F-M09 | Medium | pending | Spell-slot deduction for readied actions runs even when the spell needs no slot | /home/ab/projects/DnDnD/internal/combat/readied_action.go:68-75 |
| 218 | F-M10 | Medium | pending | No reaction-used reset for legendary actions / lair actions in cross-turn sequencing | /home/ab/projects/DnDnD/internal/combat/legendary.go (entire file) |
| 219 | F-M11 | Medium | pending | Auto-resolve cancels reaction declarations instead of marking specific Counterspell/OA as forfeited | /home/ab/projects/DnDnD/internal/combat/timer_resolution.go:314-321 |
| 220 | F-M12 | Medium | pending | `findAdjacentEnemies` uses 0-based row from `int(PositionRow)` directly — off-by-one risk | /home/ab/projects/DnDnD/internal/combat/timer.go:212-230 |
| 221 | F-M13 | Medium | pending | Multiattack parser falls back to "use every attack once" — wrong for skirmishers | /home/ab/projects/DnDnD/internal/combat/turn_builder.go:309-395 |
| 222 | F-M14 | Medium | pending | Reaction one-per-round resets between rounds but not at "creature's turn start" exactly | /home/ab/projects/DnDnD/internal/combat/reactions_panel.go:79-104 |
| 223 | F-M15 | Medium | pending | No range / 60ft check for Counterspell when generating its prompt | /home/ab/projects/DnDnD/internal/combat/counterspell.go (entire file) |
| 224 | F-M16 | Medium | pending | Free interaction matches "open" prefix → routes "open the heavy chest" away from DM | /home/ab/projects/DnDnD/internal/combat/interact.go:11-24 |
| 225 | G-M01 | Medium | pending | LongRest reports HPHealed even when no healing occurred | internal/rest/rest.go:377-384 |
| 226 | G-M02 | Medium | pending | Gold split distributes to ALL approved players, not just encounter participants | internal/loot/service.go:301 |
| 227 | G-M03 | Medium | pending | Long rest does not zero death-save tallies when both are zero | internal/rest/rest.go:444-446 |
| 228 | G-M04 | Medium | pending | Item Picker custom-entry endpoint accepts negative gold/quantity silently | internal/itempicker/handler.go:208-239 |
| 229 | G-M05 | Medium | pending | CastIdentify accepts ritual without 10-minute delay enforcement | internal/inventory/identification.go:81-110 |
| 230 | G-M06 | Medium | pending | CastIdentify silently allows identifying items that aren't magic | internal/inventory/identification.go:24-44 |
| 231 | G-M07 | Medium | pending | Combat recap truncation cuts mid-line and may produce orphan round headers | internal/combat/recap.go:71-78, 93-116 |
| 232 | G-M08 | Medium | pending | PartyShortRest never auto-spends hit dice (always spends 0) | internal/rest/party_handler.go:218-260 |
| 233 | G-M09 | Medium | pending | Loot pool created from "completed" encounter — combatants gold lost if encounter status mismatch | internal/loot/service.go:67-118 |
| 234 | G-M10 | Medium | pending | Loot pool item ItemID null when claimed from custom entries breaks downstream `/use` | internal/loot/service.go:243-256 |
| 235 | G-M11 | Medium | pending | `Equip` blocks re-equipping the same item but silently allows two main-hand items via `OffHand=false` | internal/inventory/equip.go:27-62 |
| 236 | G-M12 | Medium | pending | LongRest doesn't recharge `recharge: "dawn"` features distinct from `"long"` | internal/rest/rest.go:401-407 |
| 237 | G-M13 | Medium | pending | LongRest never zeroes the input.PactMagicSlots when it does mutate them | internal/rest/rest.go:392-398 |
| 238 | G-M14 | Medium | pending | `/rest` doesn't enforce one-long-rest-per-24h even narratively (no warning to DM) | internal/discord/rest_handler.go (no 24h check) |
| 239 | G-M15 | Medium | pending | ShortRest hit-die roll: minimum healing of 0 vs spec's "minimum 1 per HD" framing | internal/rest/rest.go:200-204 |
| 240 | G-M16 | Medium | pending | Item picker custom entry returns `Homebrew: true` but doesn't persist anywhere | internal/itempicker/handler.go:227-238 |
| 241 | H-M01 | Medium | pending | Level-up notification omits new features, spell slots, and class/old→new line | /home/ab/projects/DnDnD/internal/levelup/notify.go:14 (`FormatPrivateLevelUpMess... |
| 242 | H-M02 | Medium | pending | DDB diff is shallow — misses inventory, features, spells, proficiencies | /home/ab/projects/DnDnD/internal/ddbimport/diff.go:9 (`GenerateDiff`) |
| 243 | H-M03 | Medium | pending | DDB AC computation treats any ArmorClass<=3 as a shield | /home/ab/projects/DnDnD/internal/ddbimport/parser.go:285 (`computeAC`) |
| 244 | H-M04 | Medium | pending | DDB import doesn't validate ability scores were generated within 1-30 of submission's "stats" (no override sanity) | /home/ab/projects/DnDnD/internal/ddbimport/parser.go:228 (`parseAbilityScores`) |
| 245 | H-M05 | Medium | pending | DDB import: features below `RequiredLevel` filtered, but subclass features filtered against parent class level (not subclass level) | /home/ab/projects/DnDnD/internal/ddbimport/parser.go:332-345 (`parseFeatures`) |
| 246 | H-M06 | Medium | pending | Token redemption isn't atomic with character creation (race window) | /home/ab/projects/DnDnD/internal/portal/builder_service.go:237 |
| 247 | H-M07 | Medium | pending | No CSRF protection on portal POST /portal/api/characters | /home/ab/projects/DnDnD/internal/portal/api_handler.go:193 (`SubmitCharacter`) |
| 248 | H-M08 | Medium | pending | Multiclass spell-slot table check requires both classes to be casters; Eldritch Knight/Arcane Trickster ignored | /home/ab/projects/DnDnD/internal/character/spellslots.go:54 (`CalculateCasterLev... |
| 249 | H-M09 | Medium | pending | Char sheet template renders `{$level}` for spell slots from `map[string]SlotInfo` | /home/ab/projects/DnDnD/internal/portal/character_sheet_handler.go:374 |
| 250 | H-M10 | Medium | pending | DM denial reason is hard-coded; spec wants DM-supplied message | /home/ab/projects/DnDnD/internal/discord/asi_handler.go:543 |
| 251 | H-M11 | Medium | pending | Builder service does not enforce racial ability cap (no score > 20 at creation) | /home/ab/projects/DnDnD/internal/portal/builder_service.go:444 (`ValidatePointBu... |
| 252 | H-M12 | Medium | pending | HP recalculation on level-up assumes the standard "average+1" formula but spec also offers rolling option (not implemented) | /home/ab/projects/DnDnD/internal/character/stats.go:30 (avg formula) |
| 253 | H-M13 | Medium | pending | Pending ASI choices: storePendingChoice writes to DB asynchronously in background goroutine pattern but uses `context.Background()` | /home/ab/projects/DnDnD/internal/discord/asi_handler.go:314,328 |
| 254 | H-M14 | Medium | pending | Portal Svelte builder doesn't expose subclass selection at the right level | /home/ab/projects/DnDnD/portal/svelte/src/App.svelte (subclass section); /home/a... |
| 255 | H-M15 | Medium | pending | Portal builder calculates HP only for level-1 single-class case; ignores multiclass at submit | /home/ab/projects/DnDnD/portal/svelte/src/App.svelte:281-286 (`derivedHP`) |
| 256 | H-M16 | Medium | pending | DDB import: TempHP can become negative when HPMax overridden lower than removed HP | /home/ab/projects/DnDnD/internal/ddbimport/parser.go:191-198 |
| 257 | I-M01 | Medium | pending | Action Log viewer doesn't flag dm_override_undo entries | /home/ab/projects/DnDnD/internal/combat/action_log_viewer.go:144 |
| 258 | I-M02 | Medium | pending | Undo-of-undo re-applies the same undo instead of redoing | /home/ab/projects/DnDnD/internal/combat/dm_dashboard_undo.go:103-113 |
| 259 | I-M03 | Medium | pending | Pending-action resolve effect: no audit row per effect, after-state misses post-damage hooks | /home/ab/projects/DnDnD/internal/combat/dm_dashboard_handler.go:255-303, 423-441 |
| 260 | I-M04 | Medium | pending | Character Overview lacks live HP/condition snapshots | /home/ab/projects/DnDnD/internal/characteroverview/service.go:24-39 ; dashboard/... |
| 261 | I-M05 | Medium | pending | DM action-resolver "move" effect doesn't write to action log even via effects | /home/ab/projects/DnDnD/internal/combat/dm_dashboard_handler.go:400-421 |
| 262 | I-M06 | Medium | pending | DM character creation does not store skill proficiencies | /home/ab/projects/DnDnD/internal/dashboard/charcreate_service.go:93-116 |
| 263 | I-M07 | Medium | pending | DM character submission allows duplicate class entries | /home/ab/projects/DnDnD/internal/dashboard/charcreate.go:28-79 |
| 264 | I-M08 | Medium | pending | Spell selection isn't filtered by class or capped at max spell level on submit | /home/ab/projects/DnDnD/internal/dashboard/charcreate.go:11-25 ; charcreate_hand... |
| 265 | I-M09 | Medium | pending | Combat workspace omits character_id when emitting combatant for spell-slot override | /home/ab/projects/DnDnD/internal/combat/workspace_handler.go:87-106 |
| 266 | I-M10 | Medium | pending | Reaction panel resolve/cancel calls aren't atomic with turn-lock | /home/ab/projects/DnDnD/internal/combat/handler.go:461-498 (ResolveReaction/Canc... |
| 267 | I-M11 | Medium | pending | Stat block library Get returns SRD entries even when ?source=homebrew | /home/ab/projects/DnDnD/internal/statblocklibrary/service.go:101-119, 147-166 |
| 268 | I-M12 | Medium | pending | Homebrew create/update has no structural validation beyond name | /home/ab/projects/DnDnD/internal/homebrew/service.go:99-145 |
| 269 | I-M13 | Medium | pending | DM Override "spell slots" endpoint does not log per-character before-state for audit diff | /home/ab/projects/DnDnD/internal/combat/dm_dashboard_undo.go:594-659 |
| 270 | I-M14 | Medium | pending | Race lookup is case-sensitive in raceSpeed and feature provider | /home/ab/projects/DnDnD/internal/dashboard/charcreate.go:206-216 ; feature_provi... |
| 271 | I-M15 | Medium | pending | HP/condition tracker doesn't validate damage doesn't go negative for healing path | /home/ab/projects/DnDnD/dashboard/svelte/src/CombatManager.svelte:832-847 |
| 272 | I-M16 | Medium | pending | Encounter Builder does not respect spec's "Auto-generated short ID" uniqueness | /home/ab/projects/DnDnD/dashboard/svelte/src/EncounterBuilder.svelte (lines arou... |
| 273 | I-M17 | Medium | pending | Manual char creation handler ignores starting-equipment "guaranteed:N" quantity | /home/ab/projects/DnDnD/internal/dashboard/charcreate_handler.go (loadStartingEq... |
| 274 | I-M18 | Medium | pending | Manual character creation can submit ability scores violating point-buy without method gating | /home/ab/projects/DnDnD/internal/dashboard/charcreate.go:67-77 |
| 275 | I-M19 | Medium | pending | Reactions panel resolve flow doesn't post correction to #combat-log | /home/ab/projects/DnDnD/internal/combat/handler.go:461-498 |
| 276 | I-M20 | Medium | pending | Combat Manager polls every 5s in addition to WebSocket | /home/ab/projects/DnDnD/dashboard/svelte/src/CombatManager.svelte:101-109 |
| 277 | I-M21 | Medium | pending | HomebrewEditor list endpoint includes `?homebrew=true` but services don't filter by that query param | /home/ab/projects/DnDnD/dashboard/svelte/src/HomebrewEditor.svelte:60-72 ; statb... |
| 278 | J-M01 | Medium | pending | Open5e service caches into globally-visible rows (`campaign_id NULL`) on any auth'd POST | /home/ab/projects/DnDnD/internal/open5e/cache.go:54-66 (`CacheMonster` uses `Cam... |
| 279 | J-M02 | Medium | pending | WS Hub.Register / Unregister channels can deadlock under sustained slow-client traffic | /home/ab/projects/DnDnD/internal/dashboard/ws.go:42-51 (unbuffered channels) |
| 280 | J-M03 | Medium | pending | Crash-recovery loses in-memory once-per-turn slot tracker (Sneak Attack double-use risk) | /home/ab/projects/DnDnD/internal/combat/service.go:323-324 (`usedEffects map`) |
| 281 | J-M04 | Medium | pending | Hub broadcast drops slow clients but never closes their writer goroutine's conn | /home/ab/projects/DnDnD/internal/dashboard/ws.go:77-101 |
| 282 | J-M05 | Medium | pending | Campaign #the-story announcer resolves channel by name on every announce (drift if renamed) | /home/ab/projects/DnDnD/internal/discord/narration_poster.go:71-82 (`resolveStor... |
| 283 | J-M06 | Medium | pending | `LIMIT 1` masks duplicate active-encounter rows for a character (no error) | /home/ab/projects/DnDnD/db/queries/encounters.sql:46-51 (same query) |
| 284 | J-M07 | Medium | pending | Tiled import accepts massive maps up to `HardLimitDimension` with no tile-count guard | /home/ab/projects/DnDnD/internal/gamemap/import.go:88-103 (`checkHardRejections`... |
| 285 | J-M08 | Medium | pending | Tiled `version`/`tiledversion` not validated | /home/ab/projects/DnDnD/internal/gamemap/import.go:55-84 (`ImportTiledJSON`) |
| 286 | J-M09 | Medium | pending | Open5e cache silently rewrites partial monster data with defaults instead of skipping | /home/ab/projects/DnDnD/internal/open5e/cache.go:110-156 (`monsterToParams`) |
| 287 | J-M10 | Medium | pending | dm-queue Post stores `messageID = itemID` placeholder on Send failure → Resolve/Cancel later 404s | /home/ab/projects/DnDnD/internal/dmqueue/notifier.go:163-186 (`Post`) + 200-218 ... |
| 288 | J-M11 | Medium | pending | Tiled import silently strips group layers but doesn't preserve child-layer property inheritance | /home/ab/projects/DnDnD/internal/gamemap/import.go:115-119 (group flattening) |
| 289 | J-M12 | Medium | pending | Action handler exploration cancel doesn't strike-through dm-queue item | /home/ab/projects/DnDnD/internal/discord/action_handler.go:495-509 + `performExp... |
| 290 | J-M13 | Medium | pending | Action handler combat cancel allows incapacitated combatants but block path treats them equally | /home/ab/projects/DnDnD/internal/discord/action_handler.go:368-380 |
| 291 | J-M14 | Medium | pending | Registration dm-queue posts bypass the unified `dmqueue.Notifier` | /home/ab/projects/DnDnD/internal/discord/registration_handler.go:312 (`postDMQue... |
| 292 | J-M15 | Medium | pending | Reaction-handler stash leaks: declarations that get DM-resolved don't clear itemIDs map | /home/ab/projects/DnDnD/internal/discord/reaction_handler.go:178-185 (stash on P... |
| 293 | J-M16 | Medium | pending | dashboardCampaignLookup `IsDM(true)` is not the same as `IsCampaignDM(specific id)` for WS gate | /home/ab/projects/DnDnD/internal/dashboard/dm_middleware.go:48-75 (`RequireDM`);... |
| 294 | J-M17 | Medium | pending | /action freeform combat-mode skips turnGate when no gate is wired (tests-only path leaks to prod) | /home/ab/projects/DnDnD/internal/discord/action_handler.go:300-309 (the `else if... |
| 295 | J-M18 | Medium | pending | /distance handler not visible in scope but several Phase-105 handler tests use a typed-nil interface trap | /home/ab/projects/DnDnD/cmd/dndnd/discord_handlers.go:186-200 (the `if deps.quer... |
| 296 | cross-cut-M01 | Medium | pending | `CalculateHP` only awards the level-1 max die to `classes[0]` | `internal/character/stats.go:21-47`. |
| 297 | cross-cut-M02 | Medium | pending | Duplicate `AbilityModifier` implementations across packages | - `internal/character/stats.go:124-130` (`character.AbilityModifier`) |
| 298 | cross-cut-M03 | Medium | pending | `combatant.AbilityScores` JSON keys are not normalized — `Get` only handles two cases per ability | `internal/character/types.go:20-36` |
| 299 | cross-cut-M04 | Medium | pending | `evaluateACFormula` silently drops unknown tokens (DEX/DEX cap of medium armor not enforced) | `internal/character/stats.go:91-115` |
| 300 | cross-cut-M05 | Medium | pending | Pact magic slot table: levels 11-20 cap at slot level 5 — correct, but `Max` and `Current` are both updated even when the level didn't change | `internal/character/spellslots.go:67-103`. Verified |
| 301 | cross-cut-M06 | Medium | pending | `classHitDie` in rest service hard-codes class IDs; mis-named or homebrew classes fall through to d8 | `internal/rest/rest.go:486-499`. |
| 302 | cross-cut-M07 | Medium | pending | Divine Smite undead/fiend bonus on crit doubles to +2d8 — RAW reading is ambiguous | `internal/combat/divine_smite.go:59-68` |
| 303 | cross-cut-M08 | Medium | pending | `SneakAttack` extra dice list never validated "once per turn" | `internal/combat/feature_integration.go:86-106` |
| 304 | cross-cut-M09 | Medium | pending | Initiative tiebreak does not use DEX score (just DEX modifier) | `internal/combat/initiative.go:166-177` |
| 305 | cross-cut-M10 | Medium | pending | Pact magic slot recovery on short rest restores slot count but not slot level | `internal/rest/rest.go:235-241`. |
| 306 | cross-cut-M11 | Medium | pending | `ApplyDamageAtZeroHP` does not itself enforce the Massive Damage rule | `internal/combat/deathsave.go:157-192` |
| 307 | A-L01 | Low | pending | `SidebarNav` "Errors" path may render before error badge wiring is loaded | internal/dashboard/handler.go:55-68 |
| 308 | A-L02 | Low | pending | `WelcomeMessage` says "Type /help for a full command list" but /help requires the user to be in the guild | internal/discord/welcome.go:18 |
| 309 | A-L03 | Low | pending | Race "Half-Elf" referenced in character card example not seeded | internal/refdata/seed_races.go:1-* (only 9 races as `RaceCount = 9`) |
| 310 | A-L04 | Low | pending | Class entries don't expose Eldritch Knight / Arcane Trickster as third-caster subclasses | internal/refdata/seed_classes.go (Fighter & Rogue blocks) |
| 311 | A-L05 | Low | pending | `Settings.AutoApproveRestEnabled` defaults differ from spec | internal/campaign/service.go:36-42 |
| 312 | A-L06 | Low | pending | CookieSecure defaults to false when `COOKIE_SECURE` is unset | cmd/dndnd/main.go:564 |
| 313 | A-L07 | Low | pending | `RequiredPermissions` omits `Manage Channels` though `/setup` needs it | internal/discord/permissions.go:11-17 |
| 314 | A-L08 | Low | pending | Bot session race in `Bot.HandleGuildCreate` | internal/discord/bot.go:86-90 |
| 315 | A-L09 | Low | pending | Character `level` column not indexed despite spec | db/migrations/20260310120006_create_characters.sql:8-37 |
| 316 | A-L10 | Low | pending | Welcome DM is sent for every guild-join, including bots | internal/discord/bot.go:119-131 |
| 317 | B-L01 | Low | pending | `RollDamage` does not double the modifier-side dice for spells that allow it (e.g. Sneak Attack crit) | `/home/ab/projects/DnDnD/internal/dice/roller.go:100-124`. |
| 318 | B-L02 | Low | pending | D20 result `Total` ignores the "min 1 / max 20" sometimes referenced for nat-1 crits with negative DEX | `/home/ab/projects/DnDnD/internal/dice/d20.go:75-82`. |
| 319 | B-L03 | Low | pending | `ColumnLabel` past column 701 (`ZZ`) becomes 3-letter (`AAA`) | `/home/ab/projects/DnDnD/internal/gamemap/renderer/grid.go:65-75`. |
| 320 | B-L04 | Low | pending | Encounter `display_name` not validated against length / control characters | `/home/ab/projects/DnDnD/internal/encounter/service.go:44-62`. |
| 321 | B-L05 | Low | pending | Asset uploads have no per-campaign quota or count cap | `/home/ab/projects/DnDnD/internal/asset/handler.go:31-83`, |
| 322 | B-L06 | Low | pending | `extractRegion` clipping silently truncates non-aligned wall objects | `/home/ab/projects/DnDnD/dashboard/svelte/src/lib/mapdata.js:193-201, |
| 323 | B-L07 | Low | pending | `UndoStack` push happens *after* mutation, but does not capture the post-paste state for redo correctly | `/home/ab/projects/DnDnD/dashboard/svelte/src/lib/mapdata.js:516-557`, |
| 324 | C-L01 | Low | pending | `colToIndex` is silent on lowercase / empty input | /home/ab/projects/DnDnD/internal/combat/attack.go:1571-1577 |
| 325 | C-L02 | Low | pending | `IsInLongRange` always returns false for melee weapons — but thrown melee in long range handled separately | /home/ab/projects/DnDnD/internal/combat/attack.go:148-155, 538-542 |
| 326 | C-L03 | Low | pending | Conditions JSON empty array marshaling inconsistency | /home/ab/projects/DnDnD/internal/combat/condition.go:122-126 |
| 327 | C-L04 | Low | pending | `ApplyDamageResistances` reason string lowercase-mixed | /home/ab/projects/DnDnD/internal/combat/damage.go:24-46 |
| 328 | C-L05 | Low | pending | Free interaction not tracked across /move + /attack flow boundary | /home/ab/projects/DnDnD/internal/combat/turnresources.go:51-54 |
| 329 | C-L06 | Low | pending | Pathfinding heuristic doesn't account for prone or terrain multipliers (still admissible) | /home/ab/projects/DnDnD/internal/pathfinding/pathfinding.go:165-172 |
| 330 | C-L07 | Low | pending | Condition immunity skips application but doesn't surface to action_log persistently | /home/ab/projects/DnDnD/internal/combat/condition.go:149-152 |
| 331 | C-L08 | Low | pending | Held concentration spells: pre-applied conditions don't auto-end when concentration breaks via crash recovery | /home/ab/projects/DnDnD/internal/combat/concentration.go (general) |
| 332 | C-L09 | Low | pending | `RemoveConditionWithLog` doesn't differentiate "removed by action" vs "expired" | /home/ab/projects/DnDnD/internal/combat/condition.go:297-307 |
| 333 | D-L01 | Low | pending | PaladinAuraRadiusFt = 30 only at L18+, spec says L18 | /home/ab/projects/DnDnD/internal/combat/feature_integration.go:290 |
| 334 | D-L02 | Low | pending | Bardic Inspiration uses CHA mod min 1 — correct, but doesn't account for negative CHA | /home/ab/projects/DnDnD/internal/combat/bardic_inspiration.go:33 |
| 335 | D-L03 | Low | pending | FormatBardicInspirationUse hardcodes the `+` sign | /home/ab/projects/DnDnD/internal/combat/bardic_inspiration.go:75 |
| 336 | D-L04 | Low | pending | WildShape CR limit returns float64 — comparisons against parsed CR strings are exact | /home/ab/projects/DnDnD/internal/combat/wildshape.go:59 |
| 337 | D-L05 | Low | pending | Channel Divinity DM-queue routes class name as `cmd.ClassName` without normalization | /home/ab/projects/DnDnD/internal/combat/channel_divinity.go:533 |
| 338 | D-L06 | Low | pending | Action Surge `ResourceBonusAction` not checked | /home/ab/projects/DnDnD/internal/combat/action_surge.go:25 |
| 339 | E-L01 | Low | pending | OA detection's faction check fails for charm/dominate scenarios | `/home/ab/projects/DnDnD/internal/combat/opportunity_attack.go:106-108` |
| 340 | E-L02 | Low | pending | Stand from prone does not require movement to be available beyond cost | `/home/ab/projects/DnDnD/internal/combat/standard_actions.go:592-595` |
| 341 | E-L03 | Low | pending | Drop Prone ApplyCondition runs immunity check unnecessarily | `/home/ab/projects/DnDnD/internal/combat/standard_actions.go:651-674` |
| 342 | E-L04 | Low | pending | FormatOAPrompt uses target's display name as the slash arg, which can contain spaces/diacritics | `/home/ab/projects/DnDnD/internal/combat/opportunity_attack.go:218-221` |
| 343 | E-L05 | Low | pending | `IsBonusActionSpell` uses substring match — fragile against locale/whitespace | `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:23-25` |
| 344 | E-L06 | Low | pending | `applyMetamagicEffects` swallows unknown metamagic option silently | `/home/ab/projects/DnDnD/internal/combat/metamagic.go:196-214` |
| 345 | E-L07 | Low | pending | CarefulSpellCreatureCount minimum-1 ignores negative CHA mod gracefully | `/home/ab/projects/DnDnD/internal/combat/metamagic.go:217-228` |
| 346 | E-L08 | Low | pending | Distant Spell touch -> 30ft does not propagate into `ValidateSpellRange` | `/home/ab/projects/DnDnD/internal/combat/metamagic.go:136-144`; spellcasting.go ... |
| 347 | E-L09 | Low | pending | Empowered Spell reroll surfaces only the lowest dice (no player choice) | `/home/ab/projects/DnDnD/internal/combat/metamagic.go:248-277` |
| 348 | E-L10 | Low | pending | Heightened Spell + AoE picks "first affected" without exposing target choice in non-Discord paths | `/home/ab/projects/DnDnD/internal/combat/aoe.go:541-562` |
| 349 | E-L11 | Low | pending | Twin Spell consumes (spell_level) sorcery points — but spec says "1 for cantrips" | `/home/ab/projects/DnDnD/internal/combat/sorcery.go:38-50` |
| 350 | E-L12 | Low | pending | FontOfMagic conversion cap check uses `sorcLevel` as max, not feature_uses.max | `/home/ab/projects/DnDnD/internal/combat/sorcery.go:208-211` |
| 351 | E-L13 | Low | pending | Ritual casting allows any class with feature; Bard requires "Ritual Caster" (Lore subclass) | `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:1235-1242` (`HasRitualC... |
| 352 | E-L14 | Low | pending | Spell preparation max calculation uses character.level instead of class.level | `/home/ab/projects/DnDnD/internal/combat/preparation.go:236-240` |
| 353 | E-L15 | Low | pending | AlwaysPreparedSpells subclass list omits Cleric Light, Tempest, etc. and Druid Moon/Stars | `/home/ab/projects/DnDnD/internal/combat/preparation.go:69-93` |
| 354 | E-L16 | Low | pending | OA prompt does not display the OA target's tile or the reach distance | `/home/ab/projects/DnDnD/internal/combat/opportunity_attack.go:218-221` |
| 355 | E-L17 | Low | pending | Empowered + Twinned combo applies Empowered to both targets but only one reroll budget | `/home/ab/projects/DnDnD/internal/combat/aoe.go:980-1010` |
| 356 | E-L18 | Low | pending | Pact Magic check does not respect Sorcerer's slot pool (no cross-pool override) | `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:446-456` |
| 357 | E-L19 | Low | pending | DragMovementCost is always ×2 regardless of number of grappled targets | `/home/ab/projects/DnDnD/internal/combat/grapple_shove.go:373-375` |
| 358 | E-L20 | Low | pending | Concentration save not enqueued for self-damage from spells like Wrathful Smite | `/home/ab/projects/DnDnD/internal/combat/concentration.go:422-448` (`MaybeCreate... |
| 359 | E-L21 | Low | pending | Spell DC calculation uses `ProficiencyBonus` from character row without class-progression check | `/home/ab/projects/DnDnD/internal/combat/spellcasting.go:631` |
| 360 | E-L22 | Low | pending | CastAoE never enforces ValidateSeeTarget on AoE single-creature anchors | `/home/ab/projects/DnDnD/internal/combat/aoe.go:466-468` |
| 361 | F-L01 | Low | pending | `equipShield` auto-stows off-hand weapon at no cost | /home/ab/projects/DnDnD/internal/combat/equip.go:197-200 |
| 362 | F-L02 | Low | pending | DrawFogOfWar uses solid black for Unexplored even on DM render before DMSeesAll check | /home/ab/projects/DnDnD/internal/gamemap/renderer/fog.go:14-45 |
| 363 | F-L03 | Low | pending | `RecalculateAC` evaluates `ac_formula` parts via Sscanf without error handling | /home/ab/projects/DnDnD/internal/combat/equip.go:467-474 |
| 364 | F-L04 | Low | pending | Bonus action parsing misses fully-structured `bonus_actions` rows containing only descriptions | /home/ab/projects/DnDnD/internal/combat/turn_builder.go:473-499 |
| 365 | F-L05 | Low | pending | Summoned-creature short-ID collisions are not detected | /home/ab/projects/DnDnD/internal/combat/summon.go:117-150 |
| 366 | F-L06 | Low | pending | Concentration on readied spell never sets ConcentrationSpellID, breaking spell-ID lookups | /home/ab/projects/DnDnD/internal/combat/readied_action.go:132-141 |
| 367 | F-L07 | Low | pending | `LightSource` and `VisionSource` deduplicate by position but not vision type — flickers | /home/ab/projects/DnDnD/internal/gamemap/renderer/fog_types.go:52-93 |
| 368 | F-L08 | Low | pending | `/action ready` doesn't differentiate readying a Spell vs. a non-Spell action in the panel badge | /home/ab/projects/DnDnD/dashboard/svelte/src/ActiveReactionsPanel.svelte:120-122 |
| 369 | F-L09 | Low | pending | FoW `chebyshevDistance` is correct for square grids but doesn't match the canonical 5e diagonal rule | /home/ab/projects/DnDnD/internal/gamemap/renderer/fog_types.go:160-173 |
| 370 | G-L01 | Low | pending | Save handler combines roll modes via two `CombineRollModes` calls — order-sensitive | internal/save/save.go:90-92 |
| 371 | G-L02 | Low | pending | Group-check success threshold rounds in 5e's favor only for even counts | internal/check/check.go:282-306 |
| 372 | G-L03 | Low | pending | Contested check is "status quo" on tie — initiator's choice not surfaced | internal/check/check.go:332-352 |
| 373 | G-L04 | Low | pending | Equip slot warning text references `/attune cloak-of-protection` even for items already attuned to a different name | internal/inventory/equip.go:51-54 |
| 374 | G-L05 | Low | pending | FormatLootAnnouncement doesn't include item descriptions | internal/loot/service.go:357-380 |
| 375 | G-L06 | Low | pending | `/check` group-check participant modifier accepts raw int — no validation | internal/check/check.go:254-306 |
| 376 | G-L07 | Low | pending | `Attune` error string mixes emoji + format verb prefix | internal/inventory/attunement.go:35 |
| 377 | G-L08 | Low | pending | Inventory `IsPotion` only knows about healing-potion / greater-healing-potion | internal/inventory/service.go:108-114 |
| 378 | G-L09 | Low | pending | SplitGold zeros pool even if no players found (defensive check exists but bug-prone) | internal/loot/service.go:301-329 |
| 379 | G-L10 | Low | pending | Shop announcement doesn't show stock counts when present | internal/shops/service.go:131-155 |
| 380 | G-L11 | Low | pending | Combat recap doesn't tag "[Round X, Turn Y]" inside lines | internal/combat/recap.go:93-116 |
| 381 | H-L01 | Low | pending | DDB import discards spell `Source` for non-class spells; warning text wrong | /home/ab/projects/DnDnD/internal/ddbimport/validator.go:114-125 |
| 382 | H-L02 | Low | pending | LevelUpDetails.NewSpellSlots not surfaced in API response | /home/ab/projects/DnDnD/internal/levelup/handler.go:25 (`LevelUpResponse`) |
| 383 | H-L03 | Low | pending | Pending DDB import map cache not consulted before durable lookup | /home/ab/projects/DnDnD/internal/ddbimport/service.go:222 (`loadPendingImport`) |
| 384 | H-L04 | Low | pending | DiscordUserID race-case in HandleASIFeatSelect: lookupFeatName re-lists all feats | /home/ab/projects/DnDnD/internal/discord/asi_handler.go:668-679,792 |
| 385 | H-L05 | Low | pending | Portal landing page does not list user's existing characters | /home/ab/projects/DnDnD/internal/portal/handler.go:89 (`ServeLanding`) |
| 386 | H-L06 | Low | pending | ASI prompt button on re-trigger doesn't deduplicate or expire | /home/ab/projects/DnDnD/internal/discord/asi_handler.go:307 (`storePendingChoice... |
| 387 | H-L07 | Low | pending | `applyFeatProficiencyChoices` saves proficiencies even when caller didn't change them | /home/ab/projects/DnDnD/internal/levelup/service.go:461-491 |
| 388 | I-L01 | Low | pending | CharCreateHandler accepts campaign_id from request body, not URL | /home/ab/projects/DnDnD/internal/dashboard/charcreate_handler.go:106-138 |
| 389 | I-L02 | Low | pending | DMCharacterSubmission missing subrace / SubraceID | /home/ab/projects/DnDnD/internal/dashboard/charcreate.go:12-25 |
| 390 | I-L03 | Low | pending | Mobile view always renders TurnQueue as read-only — no quick-action to End Turn | /home/ab/projects/DnDnD/dashboard/svelte/src/MobileShell.svelte:47-49 ; QuickAct... |
| 391 | I-L04 | Low | pending | Stat block library handler ignores ?homebrew param | /home/ab/projects/DnDnD/internal/statblocklibrary/handler.go:53-66 |
| 392 | I-L05 | Low | pending | DM display name change for active encounter persists without re-pinning to current turn | /home/ab/projects/DnDnD/dashboard/svelte/src/CombatManager.svelte:178-186 ; upda... |
| 393 | I-L06 | Low | pending | Damage input doesn't allow damage types in HP & Condition Tracker | /home/ab/projects/DnDnD/dashboard/svelte/src/CombatManager.svelte:815-830 |
| 394 | I-L07 | Low | pending | Action Resolver "move" target field uses col+row only — no altitude | /home/ab/projects/DnDnD/dashboard/svelte/src/ActionResolver.svelte:209-217 |
| 395 | I-L08 | Low | pending | ActionLogViewer formatValue stringifies arrays/objects as JSON — hard to read | /home/ab/projects/DnDnD/dashboard/svelte/src/ActionLogViewer.svelte:81-86 |
| 396 | I-L09 | Low | pending | No "Resolve →" deep link inserted into #dm-queue messages | /home/ab/projects/DnDnD/internal/dmqueue/ — not audited line-by-line |
| 397 | I-L10 | Low | pending | Reactions panel doesn't fade once-per-round used reactions until creature's next turn | /home/ab/projects/DnDnD/dashboard/svelte/src/ActiveReactionsPanel.svelte:59-63 |
| 398 | I-L11 | Low | pending | Encounter Builder list query doesn't return display_name in some responses | /home/ab/projects/DnDnD/internal/encounter/handler.go:58-82 |
| 399 | I-L12 | Low | pending | Narration history endpoint accepts unbounded offset | /home/ab/projects/DnDnD/internal/narration/handler.go:107 |
| 400 | I-L13 | Low | pending | Message-player history endpoint similarly unbounded | /home/ab/projects/DnDnD/internal/messageplayer/handler.go:96-101 |
| 401 | I-L14 | Low | pending | CharacterOverview message panel embeds full MessagePlayerPanel — leaks history across cards | /home/ab/projects/DnDnD/dashboard/svelte/src/CharacterOverview.svelte:97-103 |
| 402 | J-L01 | Low | pending | Open5e cache `idPrefix` (`open5e_`) is checked in only one place; no enforcement on stat-block readers | /home/ab/projects/DnDnD/internal/open5e/cache.go:51 |
| 403 | J-L02 | Low | pending | EncounterListerAdapter returns `[]string{}` instead of nil for "no campaign", masking the no-active-campaign distinction | /home/ab/projects/DnDnD/cmd/dndnd/main.go:248-251 + 268-272 |
| 404 | J-L03 | Low | pending | WS client EncounterID query param is not UUID-validated | /home/ab/projects/DnDnD/internal/dashboard/ws.go:135 |
| 405 | J-L04 | Low | pending | dm-queue PgStore.ListPending uses `context.Background()` from `Notifier.ListPending`/`Get` | /home/ab/projects/DnDnD/internal/dmqueue/notifier.go:222-229, 320-327 |
| 406 | J-L05 | Low | pending | E2E `SeedDMApproval` bypasses dashboard approval HTTP endpoint | /home/ab/projects/DnDnD/cmd/dndnd/e2e_harness_test.go:168-184 |
| 407 | J-L06 | Low | pending | Tiled import silently coerces `tilesets` field to `[]any` even when caller supplies an object | /home/ab/projects/DnDnD/internal/gamemap/import.go:69-72 |
| 408 | J-L07 | Low | pending | HelpHandler topic table is case-sensitive but spec advertises class names "rogue", "cleric" | /home/ab/projects/DnDnD/internal/discord/help_handler.go:44-48 (`helpTopics[topi... |
| 409 | J-L08 | Low | pending | Exploration `EndExploration` doesn't notify dashboard / clear PCs | /home/ab/projects/DnDnD/internal/exploration/service.go:172-185 |
| 410 | J-L09 | Low | pending | Reaction handler doesn't return `ErrItemNotFound` from `cancelDMQueueItem` for missing entries | /home/ab/projects/DnDnD/internal/discord/reaction_handler.go:257-269 |
| 411 | J-L10 | Low | pending | Combat enemy-turn notifier label fallback when SetEncounterLookup is unset | /home/ab/projects/DnDnD/internal/discord/enemy_turn_notifier.go (SetEncounterLoo... |
| 412 | J-L11 | Low | pending | Health endpoint `/health` only reports two subsystems (db, discord); spec implies more | /home/ab/projects/DnDnD/cmd/dndnd/main.go:1336 (`health.Register("discord", …)`) |
| 413 | J-L12 | Low | pending | Errorlog Entry has no Severity field; spec asks for severity | /home/ab/projects/DnDnD/internal/errorlog/recorder.go:18-29 |
| 414 | J-L13 | Low | pending | `dmqueue.Sender` doesn't wrap message-too-long; long whispers will fail at Discord's 2000-char limit | /home/ab/projects/DnDnD/internal/dmqueue/sender.go (no splitting) |
| 415 | J-L14 | Low | pending | /action subcommand normalisation misses underscore variants | /home/ab/projects/DnDnD/internal/discord/action_handler.go:315-317 + 321-333 |
| 416 | J-L15 | Low | pending | Snapshot.Build does GetEncounter + ListCombatants + GetTurn in three trips per publish | /home/ab/projects/DnDnD/internal/dashboard/snapshot.go:52-83 |
| 417 | J-L16 | Low | pending | WS writer uses `r.Context()` for write deadlines, but request context is cancelled on disconnect | /home/ab/projects/DnDnD/internal/dashboard/ws.go:146-148 |
| 418 | J-L17 | Low | pending | CampaignAnnouncer announcement is best-effort with no logging | /home/ab/projects/DnDnD/internal/campaign/service.go:192-195 |
| 419 | J-L18 | Low | pending | DM-queue inbox list doesn't paginate | /home/ab/projects/DnDnD/internal/dmqueue/pgstore.go:159-164 (ListAllPendingDMQue... |
| 420 | J-L19 | Low | pending | dmqueueChannelResolver and channel_ids are not validated to be in the bot-accessible guild | /home/ab/projects/DnDnD/cmd/dndnd/main.go:302 (`newDMQueueChannelResolver`) |
| 421 | J-L20 | Low | pending | Phase 118c CI guard comment present but no actual CI workflow file enforces it | /home/ab/projects/DnDnD/docs/phases.md (Phase 118c "load-bearing deliverable") |
| 422 | J-L21 | Low | pending | /reaction in exploration mode is allowed even though spec ties reactions to combat | /home/ab/projects/DnDnD/internal/discord/reaction_handler.go (no encounter mode ... |
| 423 | J-L22 | Low | pending | Open5e cache "manual" resolution default isn't surfaced to DM | /home/ab/projects/DnDnD/internal/open5e/cache.go:215-225 |
| 424 | J-L23 | Low | pending | CampaignAnnouncer story-channel resolution uses Channel*Send rather than the resolved poster's split logic | /home/ab/projects/DnDnD/internal/discord/campaign_announcer.go:30 |
| 425 | J-L24 | Low | pending | Exploration spawn assignment is row-major; spec doesn't dictate ordering and may surprise DMs | /home/ab/projects/DnDnD/internal/exploration/spawn.go (AssignPCsToSpawnZones) |
| 426 | J-L25 | Low | pending | Phase 106f "remove passthroughMiddleware" — still defined and exported in main.go | /home/ab/projects/DnDnD/cmd/dndnd/main.go:319 |
| 427 | cross-cut-L01 | Low | pending | `ProficiencyBonus(0)` returns 0; `ProficiencyBonus(21)` also returns 0 | `internal/character/stats.go:134-139`. |
| 428 | cross-cut-L02 | Low | pending | `RollDeathSave` returns no `DeathSaves` on nat-20 (relies on caller resetting) | `internal/combat/deathsave.go:88-98` returns an |
| 429 | cross-cut-L03 | Low | pending | Twinned Spell cost for cantrips is 1 SP but no AOE / single-target restriction is enforced | `internal/combat/sorcery.go:38-50` |
| 430 | cross-cut-L04 | Low | pending | Diagonal-move cost uses PHB default (5 ft per diagonal) | `internal/pathfinding/pathfinding.go:151-172` — |
| 431 | cross-cut-L05 | Low | pending | Bardic Inspiration die scaling — confirmed correct | `internal/combat/bardic_inspiration.go:14-29`. |
| 432 | cross-cut-L06 | Low | pending | Wild Shape CR cap (standard + Circle of the Moon) — confirmed correct | `internal/combat/wildshape.go:56-73`. |
| 433 | cross-cut-L07 | Low | pending | Cover bonuses match PHB | `internal/combat/cover.go:31-47`. |
| 434 | cross-cut-L08 | Low | pending | Critical hit dice doubling correctly excludes the static modifier | `internal/dice/roller.go:98-124` (`RollDamage`) — |
| 435 | cross-cut-L09 | Low | pending | Concentration save DC = max(10, floor(damage/2)) | `internal/combat/concentration.go:16-24`. |
| 436 | cross-cut-L10 | Low | pending | Cantrip dice multiplier (×2 at L5, ×3 at L11, ×4 at L17) | `internal/combat/spellcasting.go:1258-1271`. |
| 437 | cross-cut-L11 | Low | pending | Class save proficiencies match PHB | `internal/refdata/seed_classes.go` — every class's |
| 438 | cross-cut-L12 | Low | pending | 18-skill list and ability mapping | `internal/character/types.go:203-222` |
| 439 | cross-cut-L13 | Low | pending | Rage damage bonus and uses/day match PHB | `internal/combat/rage.go:13-43`. |
| 440 | cross-cut-L14 | Low | pending | Monk martial-arts die scaling matches PHB | `internal/combat/monk.go:504-523`. |
| 441 | cross-cut-L15 | Low | pending | Sneak attack dice count `(rogueLevel+1)/2` rounds up correctly | `internal/combat/feature_integration.go:466-471`. |
| 442 | cross-cut-L16 | Low | pending | Sorcery point slot creation costs match PHB | `internal/combat/sorcery.go:29-35` |
| 443 | cross-cut-L17 | Low | pending | Armor seed data matches PHB | `internal/refdata/seeder.go:135-152`. |
| 444 | cross-cut-L18 | Low | pending | Unarmored Defense formulas evaluated at runtime via `evaluateACFormula` | seeded as strings in feature `mechanical_effect` |
| 445 | cross-cut-L19 | Low | pending | Exhaustion level effects align with PHB | `internal/combat/damage.go:75-112`. |
| 446 | cross-cut-L20 | Low | pending | Spell save DC and spell attack bonus formulas | `internal/combat/channel_divinity.go:85-89` |
| 447 | cross-cut-L21 | Low | pending | Multiclass caster level uses `level/2` for half / `level/3` for third casters | `internal/character/spellslots.go:47-64`. |
| 448 | cross-cut-L22 | Low | pending | ASI levels include the Fighter (6, 14) and Rogue (10) extras | `internal/levelup/levelup.go:72-79`. |
