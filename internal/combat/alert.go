package combat

// alertInitiativeBonusValue is the flat initiative bonus the 2014 Alert feat
// grants ("+5 bonus to initiative"). The seeded feat carries the 2014 shape; the
// 2024 rewrite (bonus = proficiency bonus, plus an initiative-swap option) is a
// deferred follow-up.
const alertInitiativeBonusValue = 5

// alertInitiativeBonus returns the Alert feat's flat initiative bonus for a
// character (5 when the feat is present, 0 otherwise). Detection is name-based
// via HasFeatureByName, mirroring the GWM 2024 / Savage Attacker precedent —
// level-up feats land in the features JSON by name without a mechanical_effect
// slug. Alert's other 2014 benefits (can't be surprised, no advantage from
// unseen attackers) need surprise / attacker-visibility tracking and are left as
// deferred follow-ups.
func alertInitiativeBonus(featuresJSON []byte) int {
	if HasFeatureByName(featuresJSON, "Alert") {
		return alertInitiativeBonusValue
	}
	return 0
}
