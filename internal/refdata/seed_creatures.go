package refdata

import (
	"context"
	"slices"
)

// cr is a shorthand builder for UpsertCreatureParams to reduce verbosity.
type cr = UpsertCreatureParams

func seedCreatures(ctx context.Context, q *Queries) error {
	return seedEntities(ctx, srdCreatures(), q.UpsertCreature, "creature")
}

func srdCreatures() []cr {
	return slices.Concat(
		srdCreaturesLowCR(),
		srdCreaturesCR3to5(),
		srdCreaturesCR6to7(),
		srdCreaturesCR8to11(),
		srdCreaturesCR12to17(),
		srdCreaturesCR18to30(),
	)
}
