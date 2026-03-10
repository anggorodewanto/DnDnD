package refdata

import (
	"context"
)

// sp is a shorthand builder for UpsertSpellParams to reduce verbosity.
type sp = UpsertSpellParams

func seedSpells(ctx context.Context, q *Queries) error {
	spells := srdSpells()
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
