package refdata

// ClassifyResolutionMode infers whether a spell can be resolved automatically
// by the engine or requires DM adjudication, based on its mechanical fields.
//
// The algorithm: a spell is "auto" iff it has a quantifiable mechanical effect
// expressible as damage, healing, a saving throw with a defined effect, or an
// attack roll. Otherwise it is "dm_required" — utility, social, illusion, or
// roleplay spells whose outcome the engine cannot mechanically resolve.
//
// Buff/condition spells that lack any of those fields (e.g. Bless, Shield,
// Mage Armor) are listed in the test's knownExceptions map: they are "auto"
// because their effect is mechanical (AC, advantage, temp HP) even though
// they don't fit the four primary signals.
func ClassifyResolutionMode(s sp) string {
	if s.Damage.Valid {
		return "auto"
	}
	if s.Healing.Valid {
		return "auto"
	}
	if s.AttackType.Valid {
		return "auto"
	}
	if s.SaveAbility.Valid && s.SaveEffect.Valid {
		return "auto"
	}
	return "dm_required"
}
