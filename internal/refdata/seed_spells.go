package refdata

import (
	"context"
	"log/slog"
)

// sp is a shorthand builder for UpsertSpellParams to reduce verbosity.
type sp = UpsertSpellParams

func seedSpells(ctx context.Context, q *Queries) error {
	spells := srdSpells()

	warnings := ValidateSpells(spells)
	for _, w := range warnings {
		slog.Warn("spell data quality issue",
			"spell_id", w.SpellID,
			"check", w.Check,
			"message", w.Message,
		)
	}

	return seedEntities(ctx, spells, q.UpsertSpell, "spell")
}

func srdSpells() []sp {
	var all []sp
	all = append(all, srdCantrips()...)
	all = append(all, srdLevel1()...)
	all = append(all, srdLevel2()...)
	all = append(all, srdLevel3()...)
	all = append(all, srdLevel4()...)
	all = append(all, srdLevel5()...)
	all = append(all, srdLevel6()...)
	all = append(all, srdLevel7()...)
	all = append(all, srdLevel8()...)
	all = append(all, srdLevel9()...)
	return all
}
