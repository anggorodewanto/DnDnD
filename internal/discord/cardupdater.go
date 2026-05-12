package discord

import (
	"context"
	"log"

	"github.com/google/uuid"
)

// CardUpdater is the minimal callback non-combat handlers fire after a
// successful character-state mutation so the persistent #character-cards
// message stays in sync with the new state (SR-007 — spec line 216).
//
// charactercard.Service.OnCharacterUpdated satisfies this interface and is
// the production binding wired in cmd/dndnd/main.go. The interface is
// declared locally to avoid an import cycle with the charactercard package
// and to keep the handler-level test seam narrow. Errors are intentionally
// swallowed by the call site (notifyCardUpdate); a card-edit failure must
// never undo the underlying mutation.
type CardUpdater interface {
	OnCharacterUpdated(ctx context.Context, characterID uuid.UUID) error
}

// notifyCardUpdate fires the CardUpdater for the given character if a
// non-nil updater is wired. Nil-updater and uuid.Nil are silent no-ops.
// Errors are logged and swallowed so a Discord hiccup cannot undo a
// committed DB mutation.
func notifyCardUpdate(ctx context.Context, u CardUpdater, characterID uuid.UUID) {
	if u == nil || characterID == uuid.Nil {
		return
	}
	if err := u.OnCharacterUpdated(ctx, characterID); err != nil {
		log.Printf("character card auto-update failed for %s: %v", characterID, err)
	}
}
