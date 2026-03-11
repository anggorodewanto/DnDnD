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
		srdCreaturesMidCR(),
		srdCreaturesHighCR(),
	)
}
