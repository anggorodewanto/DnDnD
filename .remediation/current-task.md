finding_id: D-C04
severity: Critical
title: Save handler never sets IsRaging in EffectContext
location: internal/discord/save_handler.go:199
spec_ref: spec §Feature Effect System example "Rage" trigger on_save with when_raging; Phase 46
problem: |
  The EffectContext built for /save populates only AbilityUsed and WearingArmor. IsRaging is left as false, so the rage save-advantage effect is filtered out by EvaluateConditions (c.WhenRaging && !ctx.IsRaging). A raging barbarian rolling a STR save loses the rage advantage.
suggested_fix: |
  Look up the saver's active combatant via the existing combatantLookup, copy IsRaging into the EffectContext. The save handler already has access to the combatant data.
acceptance_criterion: |
  When a raging barbarian makes a STR saving throw, the EffectContext.IsRaging is true and the rage advantage effect applies. A test demonstrates this.
