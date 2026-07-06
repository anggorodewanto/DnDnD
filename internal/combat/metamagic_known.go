package combat

import (
	"fmt"
	"strings"

	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

// HasMetamagic reports whether the character's features include a Sorcerer
// Metamagic option with the given clean-slug mechanical_effect (e.g.
// "quickened"). Metamagic picks are persisted as
// Feature{Source:"metamagic", MechanicalEffect:<slug>} by the builder
// (internal/portal/metamagic.go), so the existing slug matcher recognizes them —
// exactly like HasInvocation / HasFightingStyle.
func HasMetamagic(features pqtype.NullRawMessage, slug string) bool {
	return hasFeatureEffect(features, slug)
}

// validateKnownMetamagic rejects any requested metamagic option the caster has
// not learned. Before COV-15 the only cast-time gate on /cast <option> was
// sorcery-point cost, so any sorcerer with points could apply ANY of the eight
// options regardless of which two (three/four) they actually picked. This
// enforces the builder-captured picks: an option must be present in the
// character's features (combat.HasMetamagic) to be usable. Called at the top of
// the Cast / CastAoE metamagic block, BEFORE any slot or sorcery-point is
// deducted, so a rejected cast burns nothing.
//
// A character with no metamagic features (a legacy sorcerer never rebuilt in the
// picker, or a non-sorcerer) knows zero options and so cannot apply any — the
// remedy is to (re)pick metamagics in the builder or DM dashboard.
func validateKnownMetamagic(features pqtype.NullRawMessage, metamagics []string) error {
	for _, m := range metamagics {
		if HasMetamagic(features, m) {
			continue
		}
		return fmt.Errorf("your character hasn't learned the %s metamagic option — pick it in the character builder", metamagicDisplayName(m))
	}
	return nil
}

// metamagicDisplayName returns the catalog display name for a metamagic slug
// (e.g. "quickened" -> "Quickened Spell"), falling back to the raw slug for an
// unrecognized value so the error is always intelligible.
func metamagicDisplayName(slug string) string {
	if m, ok := refdata.MetamagicByID()[strings.ToLower(slug)]; ok {
		return m.Name
	}
	return slug
}
