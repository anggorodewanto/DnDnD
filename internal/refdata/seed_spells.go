package refdata

import (
	"context"
	"log/slog"
	"slices"
)

// sp is a shorthand builder for UpsertSpellParams to reduce verbosity.
type sp = UpsertSpellParams

func seedSpells(ctx context.Context, q *Queries) error {
	LogSpellValidationWarnings(slog.Default())
	return seedEntities(ctx, srdSpells(), q.UpsertSpell, "spell")
}

func srdSpells() []sp {
	return slices.Concat(
		srdCantrips(),
		srdLevel1(),
		srdLevel2(),
		srdLevel3(),
		srdLevel4(),
		srdLevel5(),
		srdLevel6(),
		srdLevel7(),
		srdLevel8(),
		srdLevel9(),
	)
}
