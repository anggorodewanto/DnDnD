package refdata

import (
	"log/slog"
	"strings"
)

// SpellWarning represents a data quality warning for a spell.
type SpellWarning struct {
	SpellID string
	Check   string
	Message string
}

// ValidateSpells checks spell data for quality issues and returns warnings.
func ValidateSpells(spells []sp) []SpellWarning {
	var warnings []SpellWarning

	for _, s := range spells {
		warnings = appendSpellWarnings(warnings, s)
	}

	return warnings
}

func appendSpellWarnings(warnings []SpellWarning, s sp) []SpellWarning {
	if s.SaveAbility.Valid && !s.SaveEffect.Valid {
		warnings = append(warnings, SpellWarning{
			SpellID: s.ID,
			Check:   "save_ability_without_save_effect",
			Message: "spell has save_ability but no save_effect",
		})
	}

	if s.SaveEffect.Valid && !s.SaveAbility.Valid {
		warnings = append(warnings, SpellWarning{
			SpellID: s.ID,
			Check:   "save_effect_without_save_ability",
			Message: "spell has save_effect but no save_ability",
		})
	}

	if s.MaterialCostGp.Valid && !s.MaterialDescription.Valid {
		warnings = append(warnings, SpellWarning{
			SpellID: s.ID,
			Check:   "material_cost_without_description",
			Message: "spell has material_cost_gp but no material_description",
		})
	}

	if s.MaterialConsumed.Valid && s.MaterialConsumed.Bool && !s.MaterialCostGp.Valid {
		warnings = append(warnings, SpellWarning{
			SpellID: s.ID,
			Check:   "material_consumed_without_cost",
			Message: "spell has material_consumed=true but no material_cost_gp",
		})
	}

	if s.Concentration.Valid && s.Concentration.Bool && !strings.Contains(s.Duration, "Concentration") {
		warnings = append(warnings, SpellWarning{
			SpellID: s.ID,
			Check:   "concentration_duration_mismatch",
			Message: "spell has concentration=true but duration does not mention Concentration",
		})
	}

	if s.Damage.Valid && !s.SaveAbility.Valid && !s.AttackType.Valid {
		warnings = append(warnings, SpellWarning{
			SpellID: s.ID,
			Check:   "damage_without_resolution",
			Message: "spell has damage but no save_ability and no attack_type",
		})
	}

	if s.AreaOfEffect.Valid && !s.SaveAbility.Valid {
		warnings = append(warnings, SpellWarning{
			SpellID: s.ID,
			Check:   "aoe_without_save",
			Message: "spell has area_of_effect but no save_ability",
		})
	}

	if s.ResolutionMode == "auto" && !s.Damage.Valid && !s.Healing.Valid && !s.Teleport.Valid && len(s.ConditionsApplied) == 0 {
		warnings = append(warnings, SpellWarning{
			SpellID: s.ID,
			Check:   "auto_without_mechanical_effect",
			Message: "spell marked auto but has no damage, healing, teleport, or conditions_applied",
		})
	}

	return warnings
}

// LogSpellValidationWarnings validates all SRD spells and logs any data quality warnings.
func LogSpellValidationWarnings(logger *slog.Logger) []SpellWarning {
	spells := srdSpells()
	warnings := ValidateSpells(spells)
	for _, w := range warnings {
		logger.Warn("spell data quality issue",
			"spell_id", w.SpellID,
			"check", w.Check,
			"message", w.Message,
		)
	}
	return warnings
}
