package homebrew

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

// typeOps abstracts the per-type plumbing needed by the generic homebrew
// CRUD helpers. Params (P) and Row (R) are the sqlc-generated Upsert and
// row types; because those types differ field-by-field per refdata entity,
// each per-type file supplies small closures instead of trying to treat
// them as a common struct.
type typeOps[P any, R any] struct {
	// get fetches a row by id.
	get func(ctx context.Context, id string) (R, error)
	// upsert writes the given params.
	upsert func(ctx context.Context, params P) error
	// deleteHomebrew deletes the owned homebrew row and returns rows affected.
	deleteHomebrew func(ctx context.Context, id string, campaignID uuid.UUID) (int64, error)

	// nameOf reads the Name field off the params.
	nameOf func(P) string
	// setIdentity stamps id + homebrew columns onto the params.
	setIdentity func(p *P, id string, campaignID uuid.NullUUID, homebrew sql.NullBool, source sql.NullString)
	// ownership reads (homebrew, campaign_id) from an existing row for
	// ownership checks.
	ownership func(R) (sql.NullBool, uuid.NullUUID)
}

// genericCreate runs the shared create flow: validate, stamp identity,
// upsert, re-fetch with ownership check.
func genericCreate[P any, R any](ctx context.Context, s *Service, ops typeOps[P, R], campaignID uuid.UUID, params P) (R, error) {
	var zero R
	if err := s.requireCreate(campaignID, ops.nameOf(params)); err != nil {
		return zero, err
	}
	id := s.idGen()
	cid, hb, src := s.homebrewCols(campaignID)
	ops.setIdentity(&params, id, cid, hb, src)
	if err := ops.upsert(ctx, params); err != nil {
		return zero, err
	}
	return genericFetch(ctx, ops, id, campaignID)
}

// genericUpdate runs the shared update flow: validate, verify ownership,
// stamp identity, upsert, re-fetch.
func genericUpdate[P any, R any](ctx context.Context, s *Service, ops typeOps[P, R], campaignID uuid.UUID, id string, params P) (R, error) {
	var zero R
	if err := s.requireUpdate(campaignID, id, ops.nameOf(params)); err != nil {
		return zero, err
	}
	existing, err := ops.get(ctx, id)
	if err != nil {
		return zero, translateGetErr(err)
	}
	hb, owner := ops.ownership(existing)
	if err := ownsRow(hb, owner, campaignID); err != nil {
		return zero, err
	}
	cid, hbCol, src := s.homebrewCols(campaignID)
	ops.setIdentity(&params, id, cid, hbCol, src)
	if err := ops.upsert(ctx, params); err != nil {
		return zero, err
	}
	return genericFetch(ctx, ops, id, campaignID)
}

// genericDelete runs the shared delete flow: validate, delete, translate
// zero-rows to ErrNotFound.
func genericDelete[P any, R any](ctx context.Context, s *Service, ops typeOps[P, R], campaignID uuid.UUID, id string) error {
	if err := s.requireDelete(campaignID, id); err != nil {
		return err
	}
	rows, err := ops.deleteHomebrew(ctx, id, campaignID)
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// genericFetch re-reads a row and enforces ownership, mapping missing or
// foreign rows to ErrNotFound.
func genericFetch[P any, R any](ctx context.Context, ops typeOps[P, R], id string, campaignID uuid.UUID) (R, error) {
	var zero R
	row, err := ops.get(ctx, id)
	if err != nil {
		return zero, translateGetErr(err)
	}
	hb, owner := ops.ownership(row)
	if err := ownsRow(hb, owner, campaignID); err != nil {
		return zero, err
	}
	return row, nil
}
