package discord

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// ResumeCombatService is the minimal combat lookup surface required to
// reconstruct a turn-start prompt when a campaign resumes.
type ResumeCombatService interface {
	ListEncountersByCampaignID(ctx context.Context, campaignID uuid.UUID) ([]refdata.Encounter, error)
	GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
}

// ResumePlayerLookup resolves a PC's Discord user id from the (campaign,
// character) pair so the resume ping can @mention the right Discord user.
type ResumePlayerLookup interface {
	GetPlayerCharacterByCharacter(ctx context.Context, campaignID, characterID uuid.UUID) (refdata.PlayerCharacter, error)
}

// ResumeTurnPinger posts a turn-start prompt to #your-turn when a paused
// campaign is resumed mid-combat, so the acting player can see it's still
// their move. Implements campaign.TurnPinger. All failures are absorbed
// silently: resume is authoritative, the ping is best-effort.
type ResumeTurnPinger struct {
	session Session
	combat  ResumeCombatService
	players ResumePlayerLookup
}

// NewResumeTurnPinger constructs a ResumeTurnPinger. Any of session, combat,
// players may be nil in tests; runtime calls are guarded.
func NewResumeTurnPinger(session Session, combat ResumeCombatService, players ResumePlayerLookup) *ResumeTurnPinger {
	return &ResumeTurnPinger{session: session, combat: combat, players: players}
}

// RePingCurrentTurn finds the campaign's active encounter (if any), resolves
// its current turn and the owning PC, then posts the turn-start prompt (with
// an @mention of the player's discord user) to #your-turn. NPC current turns
// are skipped because the spec only mentions re-pinging the "current turn
// player".
func (p *ResumeTurnPinger) RePingCurrentTurn(ctx context.Context, c refdata.Campaign) {
	if p.session == nil || p.combat == nil || p.players == nil {
		return
	}
	channelID, ok := yourTurnChannel(c)
	if !ok {
		return
	}

	enc, ok := p.findActiveEncounterWithTurn(ctx, c.ID)
	if !ok {
		return
	}

	turn, err := p.combat.GetTurn(ctx, enc.CurrentTurnID.UUID)
	if err != nil {
		return
	}

	comb, err := p.combat.GetCombatant(ctx, turn.CombatantID)
	if err != nil {
		return
	}
	if comb.IsNpc || !comb.CharacterID.Valid {
		return
	}

	pc, err := p.players.GetPlayerCharacterByCharacter(ctx, c.ID, comb.CharacterID.UUID)
	if err != nil || pc.DiscordUserID == "" {
		return
	}

	content := buildResumeTurnContent(enc, turn, comb, pc.DiscordUserID)
	_, _ = p.session.ChannelMessageSend(channelID, content)
}

// findActiveEncounterWithTurn returns the first encounter in a campaign that
// is status=active and has a CurrentTurnID set.
func (p *ResumeTurnPinger) findActiveEncounterWithTurn(ctx context.Context, campaignID uuid.UUID) (refdata.Encounter, bool) {
	encounters, err := p.combat.ListEncountersByCampaignID(ctx, campaignID)
	if err != nil {
		return refdata.Encounter{}, false
	}
	for _, e := range encounters {
		if e.Status == "active" && e.CurrentTurnID.Valid {
			return e, true
		}
	}
	return refdata.Encounter{}, false
}

// buildResumeTurnContent composes the turn-start prompt and prefixes it with
// a Discord @mention of the owning player's user id so they get a ping on the
// resume. Uses the existing FormatTurnStartPrompt formatter for parity with
// a normal turn-start notification.
func buildResumeTurnContent(enc refdata.Encounter, turn refdata.Turn, comb refdata.Combatant, discordUserID string) string {
	name := combat.EncounterDisplayName(enc)
	prompt := combat.FormatTurnStartPrompt(name, enc.RoundNumber, comb.DisplayName, turn, &comb)
	mention := fmt.Sprintf("<@%s>", discordUserID)
	return mention + " — the campaign has resumed. It's still your turn.\n" + prompt
}

// yourTurnChannel extracts the "your-turn" channel id from a campaign's
// settings JSONB. Returns ok=false if settings are missing, unparseable, or
// lack the key.
func yourTurnChannel(c refdata.Campaign) (string, bool) {
	if !c.Settings.Valid {
		return "", false
	}
	var s campaign.Settings
	if err := json.Unmarshal(c.Settings.RawMessage, &s); err != nil {
		return "", false
	}
	id, ok := s.ChannelIDs["your-turn"]
	if !ok || id == "" {
		return "", false
	}
	return id, true
}
