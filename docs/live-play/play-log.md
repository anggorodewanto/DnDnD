# Play Log — chronological

Append a short entry per beat (setup steps, narration delivered, player commands,
outcomes, decisions). Newest at the bottom. This is the story-and-mechanics
history; `game-state.md` is the current snapshot.

---

## Session 1 — 2026-06-24

**Setup**

- Stood up the stack with `make local-up` (docker compose; app built from current
  source so combat fixes are present). Verified: app `:8080` → 307, Postgres
  healthy, `discord session opened`, `channel-bindings passed` for guild `DnDnD`.
- Inspected the DB: a campaign **already existed** from a prior playtest
  (`/setup` run on 2026-05-24) — DM = the user, all 10 channels created, one 10×10
  map imported. **0 characters, 0 encounters** → clean slate for fresh play.
- Decided the play shape (via interview): **sandbox**, Claude drives the dashboard
  in the browser, user **builds a fresh character**, Claude picks the setting.
- Created this `docs/live-play/` doc set (README / runbook / game-state / play-log)
  so any fresh agent can resume as DM.
- DM picked an opening frame: **"Ashfall Waystation"** (see `game-state.md`), to be
  tailored to the character the player builds.

- **Clean-slate decision (user):** keep the campaign shell + working Discord
  channels, delete leftovers. Deleted the old map, the resolved test-whisper, and
  3 stale portal tokens. DB now: 0 characters / 0 encounters / 0 maps; campaign +
  channels + DM ownership intact.

**Character build + first bug**

- Claude took over the authenticated DM dashboard (claude-in-chrome). Confirmed
  clean campaign home (0 approvals / encounters).
- User started building a **level-3 warlock** in the portal and hit a wall: only
  cantrips selectable, no leveled spells. → Investigated (subagent + hand-verified
  code/data): **ISSUE-001** — Pact Magic never wired into the builder's
  max-spell-level derivation (`derive_stats.go`) nor persisted at creation
  (`builder_store_adapter.go`); `PactMagicSlotsForLevel` existed but had zero
  callers. Data was fine (warlock spells seeded).
- User chose **fix now (TDD)**. Delegated a bounded TDD fix; reviewed the diff,
  confirmed build/vet/tests + `make cover-check` green, rebuilt + restarted the
  app so the fix is live. Logged the fix + a surfaced latent gap (**ISSUE-002**:
  standard-caster `spell_slots` may not persist at creation — unconfirmed).

**Class-creation audit (probe for more warlock-style gaps)**

- User asked to probe whether other classes have similar "data/consumer exists
  but builder never wires it" gaps. Ran 3 parallel read-only investigators
  (persistence / spell-level gating / non-spell resources). Found + logged
  ISSUE-002..ISSUE-007: full/half-caster `spell_slots` dropped at creation
  (confirmed, major); EK/AT not recognized as casters by the frontend (spell step
  skipped); Unarmored Defense AC never wired for Barbarian/Monk; Expertise never
  wired for Rogue/Bard; L1 paladin/ranger phantom slot; multiclass UI spell-budget
  uses primary class only. None block the warlock. See `issues.md`.

**Submit 500 → ISSUE-008 fix**

- User submitted the warlock for DM approval → **HTTP 500**. Traced to
  `characters.languages TEXT[] NOT NULL` + the builder sending no languages
  (`pq.Array(nil)` → SQL NULL). Guaranteed blocker for *all* portal builds, first
  triggered now (campaign's first portal character). User chose **fix now (TDD)**.
  Coerced nil→`[]` in `CreateCharacterRecord` (2 red→green tests), `cover-check`
  green, app rebuilt + restarted. Logged ISSUE-008 + the deeper gap (builder
  collects no concrete languages at all).

**Next:** user **resubmits** the warlock (fix is live) → Claude approves on the
dashboard → open the scene (tailor "Ashfall Waystation" to the warlock).
</content>
