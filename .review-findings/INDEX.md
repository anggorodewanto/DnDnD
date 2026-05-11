# DnDnD Implementation Review — Findings Index

Each chunk review file documents:
- Phase coverage status (checked-off vs implementation reality)
- Spec-vs-impl gaps
- Stubs, deferrals, TODOs
- Test coverage red flags
- Risk items

| Chunk | Phases | File |
|---|---|---|
| 1 | 1–10  Foundation: build, DB, refdata schemas, characters, Discord bot core/queue, OAuth | chunk1_foundation.md |
| 2 | 11–22 Campaign + channels + commands + registration + dashboard skeleton + char cards + dice + maps + assets + editor + rendering | chunk2_campaign_maps.md |
| 3 | 23–38 Encounters + combatants + initiative + lifecycle + concurrency + turn resources + pathfinding + movement + attacks + adv/disadv + extra atk + weapon props + flags | chunk3_combat_core.md |
| 4 | 39–57 Conditions + condition effects + prone + damage + death + FES + class features + standard actions + OA + grapple + stealth | chunk4_conditions_classes.md |
| 5 | 58–72 Spells + AoE + upcast + concentration + teleport + components + pact + prepare + metamagic + zones + FoW + lighting + reactions + ready + counterspell | chunk5_spells_reactions.md |
| 6 | 73–87 Freeform + interact + equip + AC + turn timeout + player turn + enemy turns + summons + recap + check/save + rests + inventory + loot + item picker + shops | chunk6_turn_flow.md |
| 7 | 88–102 Magic items + leveling + DDB + portal + manual char create + dashboard combat manager + action log + undo + stat block + homebrew + narration + char overview + mobile | chunk7_dashboard_portal.md |
| 8 | 103–121 WS + recovery + multi-encounter + DM notif + misc commands + exploration + open5e + observability + invisible + surprise + pause + tiled + testing + concentration cleanup + error log + e2e + playtest | chunk8_polish_e2e.md |

Generated: 2026-05-10.
