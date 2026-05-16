package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/check"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/logging"
	"github.com/ab/dndnd/internal/pathfinding"
	"github.com/ab/dndnd/internal/refdata"
)

// MoveService defines the combat operations needed by the move handler.
type MoveService interface {
	GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
	GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
	ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	UpdateCombatantPosition(ctx context.Context, id uuid.UUID, col string, row, altitude int32) (refdata.Combatant, error)
	UpdateCombatantVisibility(ctx context.Context, arg refdata.UpdateCombatantVisibilityParams) (refdata.Combatant, error)
	UpdateCombatantConditions(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error)
}

// MoveMapProvider retrieves map data for pathfinding.
type MoveMapProvider interface {
	GetByID(ctx context.Context, id uuid.UUID) (refdata.Map, error)
}

// MoveTurnProvider retrieves and updates turn data.
type MoveTurnProvider interface {
	GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	UpdateTurnActions(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error)
}

// MoveEncounterProvider resolves the active encounter a given Discord user is
// participating in (via their character's combatant entry). Phase 105 routes
// commands to the encounter the invoker belongs to rather than picking an
// arbitrary active encounter in the guild, so simultaneous encounters in a
// single campaign are disambiguated per-player.
type MoveEncounterProvider interface {
	ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
}

// MoveCharacterLookup resolves a Discord user to their character within a campaign.
// Used by exploration /move to disambiguate which PC combatant to move when
// multiple PCs share an exploration encounter (Phase 110 it2).
type MoveCharacterLookup interface {
	GetCharacterByCampaignAndDiscord(ctx context.Context, campaignID uuid.UUID, discordUserID string) (refdata.Character, error)
}

// MoveSizeSpeedLookup resolves the creature size category and walk speed for
// a combatant by joining through the character or creature ref id.
// Implementations typically wrap refdata.Queries.GetCharacter / GetCreature
// and parse Creature.Speed via the existing combat helper. med-21 / Phase 30:
// replaces the hardcoded Medium / 30 ft defaults in the prone-stand path so
// halflings (25 ft), tabaxi (40 ft), and Large creatures pathfind correctly.
type MoveSizeSpeedLookup interface {
	LookupSizeAndSpeed(ctx context.Context, combatant refdata.Combatant) (sizeCategory int, speedFt int, err error)
}

// MoveOATurnsLookup returns the per-combatant turn map for an encounter so the
// OA detector can read each hostile's `reaction_used` flag. Implementations
// typically wrap a single `ListTurnsByEncounter`-style query. nil-safe: when
// unset, OA detection skips reaction-used filtering and assumes every hostile
// has a reaction available.
type MoveOATurnsLookup interface {
	ListTurnsByEncounter(ctx context.Context, encounterID uuid.UUID) (map[uuid.UUID]refdata.Turn, error)
}

// MoveOACreatureLookup resolves an NPC creature's parsed attacks (for reach
// in feet). Keyed by `creature_ref_id`. nil-safe: unset means every NPC reach
// defaults to 5ft.
type MoveOACreatureLookup interface {
	GetCreatureAttacks(ctx context.Context, creatureRefID string) ([]combat.CreatureAttackEntry, error)
}

// MoveOAPCWeaponReach resolves the equipped melee weapon's reach for a PC
// hostile (10 if the weapon has the "reach" property, 5 otherwise, 0 when
// no equipped melee weapon). nil-safe: unset means every PC reach defaults
// to 5ft.
type MoveOAPCWeaponReach interface {
	LookupPCReach(ctx context.Context, characterID uuid.UUID) (int, error)
}

// MoveChannelIDProvider resolves the per-encounter Discord channel IDs (e.g.
// "your-turn", "combat-log"). med-24 routes OA prompts to "your-turn".
// nil-safe: when unset, no OA prompts are posted.
type MoveChannelIDProvider interface {
	GetChannelIDs(ctx context.Context, encounterID uuid.UUID) (map[string]string, error)
}

// MoveDragLookup detects whether the mover is currently grappling any other
// combatant. When the returned DragCheckResult.HasTargets is true, /move
// applies the x2 drag movement cost via combat.DragMovementCost. nil-safe:
// when unset, drag costs are never applied (legacy behavior). (D-56 / Phase 56)
type MoveDragLookup interface {
	CheckDragTargets(ctx context.Context, encounterID uuid.UUID, mover refdata.Combatant) (combat.DragCheckResult, error)
}

// MoveDragReleaser releases all grappled targets for a mover. Used by the
// SR-047 interactive "Release & Move" button to end grapples before moving
// at normal speed. nil-safe: when unset, the Release & Move button is not shown.
type MoveDragReleaser interface {
	ReleaseDrag(ctx context.Context, mover refdata.Combatant, targets []refdata.Combatant) (combat.ReleaseDragResult, error)
}

// MovePassiveCheckResolver calculates passive skill totals for a combatant.
// Production uses check.Service.PassiveCheck; tests can inject a recorder to
// prove movement invokes the passive-check path.
type MovePassiveCheckResolver interface {
	PassiveCheckForCombatant(ctx context.Context, combatant refdata.Combatant, skill string) (check.PassiveCheckResult, error)
}

// MoveOANotifier is the minimal dmqueue.Notifier surface the move handler
// uses to post DM-side opportunity-attack prompts for DM-controlled hostile
// reactions. Defined locally so the discord package doesn't take an extra
// dependency on dmqueue at compile time. (SR-028)
type MoveOANotifier interface {
	Post(ctx context.Context, e dmqueue.Event) (itemID string, err error)
}

// MoveOAReactionRecorder records the dm-queue item ID returned by a
// successful OA prompt Post so the round-advance forfeit sweep can cancel
// any prompts that remain pending at end of round. (SR-028)
type MoveOAReactionRecorder interface {
	RecordPendingOA(encounterID uuid.UUID, itemID string)
}

// MoveEncounterPublisher pushes an encounter snapshot after a mutation.
type MoveEncounterPublisher interface {
	PublishEncounterSnapshot(ctx context.Context, encounterID uuid.UUID) error
}

// MoveHandler handles the /move slash command.
type MoveHandler struct {
	session           Session
	combatService     MoveService
	mapProvider       MoveMapProvider
	turnProvider      MoveTurnProvider
	encounterProvider MoveEncounterProvider
	campaignProv      CampaignProvider
	characterLookup   MoveCharacterLookup
	turnGate          TurnGate
	// med-21 / Phase 30: optional size/speed lookup. Nil falls back to the
	// historical Medium / 30 ft defaults so unit tests built before the
	// lookup landed keep working.
	sizeSpeedLookup MoveSizeSpeedLookup
	// med-24 / Phase 55: optional OA fan-out wiring. All four are
	// independently nil-safe — unset means no OA prompts are posted.
	oaTurns     MoveOATurnsLookup
	oaCreatures MoveOACreatureLookup
	oaPCReach   MoveOAPCWeaponReach
	oaChannels  MoveChannelIDProvider
	// SR-028: optional DM-side OA fan-out. When wired, DM-controlled
	// hostile OAs route to #dm-queue via the notifier (and are tracked
	// for the end-of-round forfeit sweep via the recorder) instead of
	// the player-facing #your-turn channel. Both must be non-nil to
	// enable the DM-side branch; if either is unset, DM-controlled
	// hostiles fall back to the legacy #your-turn path (the pre-SR-028
	// behavior).
	oaNotifier MoveOANotifier
	oaRecorder MoveOAReactionRecorder
	// D-56 / Phase 56: drag-cost integration. When set, /move calls
	// CheckDragTargets and doubles the displayed move cost via combat.DragMovementCost.
	dragLookup MoveDragLookup
	// SR-047: optional drag releaser for the interactive "Release & Move" button.
	dragReleaser MoveDragReleaser
	// SR-043: optional passive-check resolver for movement-side hidden enemy
	// detection. Nil keeps legacy movement behavior in minimal test wiring.
	passiveChecks MovePassiveCheckResolver
	// F-20: optional structured logger. nil falls back to slog.Default()
	// so legacy tests built before logger wiring keep working.
	logger *slog.Logger
	// J-H09: optional encounter snapshot publisher. nil-safe.
	publisher MoveEncounterPublisher
}

// SetLogger wires the structured logger used to emit per-command
// observability lines via the internal/logging helper. nil-safe: when
// unset the handler falls back to slog.Default().
func (h *MoveHandler) SetLogger(l *slog.Logger) { h.logger = l }

// SetDragLookup wires the D-56 drag-cost integration. nil-safe — when unset,
// /move never applies the x2 drag movement cost.
func (h *MoveHandler) SetDragLookup(l MoveDragLookup) { h.dragLookup = l }

// SetDragReleaser wires the SR-047 interactive "Release & Move" button.
// nil-safe — when unset, the Release & Move choice is not shown.
func (h *MoveHandler) SetDragReleaser(r MoveDragReleaser) { h.dragReleaser = r }

// SetPassiveCheckResolver wires SR-043 movement-side passive Perception
// detection for hidden enemies.
func (h *MoveHandler) SetPassiveCheckResolver(r MovePassiveCheckResolver) { h.passiveChecks = r }

// SetCharacterLookup wires the character lookup used by exploration /move to
// resolve the invoking Discord user's PC combatant.
func (h *MoveHandler) SetCharacterLookup(lookup MoveCharacterLookup) {
	h.characterLookup = lookup
}

// SetPublisher wires the optional encounter snapshot publisher (J-H09).
// nil-safe: when unset, no snapshot is published after a move.
func (h *MoveHandler) SetPublisher(p MoveEncounterPublisher) { h.publisher = p }

// SetTurnGate wires the Phase 27 turn-ownership / advisory-lock gate.
// A nil gate disables the check (preserves backwards-compatibility for
// handlers constructed before Phase 27 wiring rolled out and for unit tests
// that don't care about ownership). Production wiring always supplies one.
func (h *MoveHandler) SetTurnGate(g TurnGate) {
	h.turnGate = g
}

// SetSizeSpeedLookup wires the med-21 size/speed resolver so /move stops
// hardcoding Medium creature size and 30 ft walk speed in the prone-stand
// path. Pass nil to fall back to the historical defaults (used by older
// unit tests).
func (h *MoveHandler) SetSizeSpeedLookup(lookup MoveSizeSpeedLookup) {
	h.sizeSpeedLookup = lookup
}

// SetOpportunityAttackHooks wires the four collaborators needed to fire
// opportunity-attack prompts when /move exits a hostile's reach (med-24 /
// Phase 55). All four arguments may be nil — when any one is unset the
// detection runs with degraded data (e.g. no creature reach, no reaction
// filtering) and prompts are suppressed when channels is nil.
func (h *MoveHandler) SetOpportunityAttackHooks(
	turns MoveOATurnsLookup,
	creatures MoveOACreatureLookup,
	pcReach MoveOAPCWeaponReach,
	channels MoveChannelIDProvider,
) {
	h.oaTurns = turns
	h.oaCreatures = creatures
	h.oaPCReach = pcReach
	h.oaChannels = channels
}

// SetOpportunityAttackNotifier wires the SR-028 DM-side OA fan-out. When
// both notifier and recorder are non-nil, opportunity attacks triggered by
// a DM-controlled hostile (combatant.IsNpc == true) post a
// KindOpportunityAttack event to #dm-queue and register the returned
// item ID with the recorder so the end-of-round forfeit sweep can cancel
// it. PC-controlled hostiles (IsNpc == false) keep using the #your-turn
// path established by SetOpportunityAttackHooks. Passing nil for either
// argument disables the DM-side branch.
func (h *MoveHandler) SetOpportunityAttackNotifier(n MoveOANotifier, r MoveOAReactionRecorder) {
	h.oaNotifier = n
	h.oaRecorder = r
}

// resolveSizeAndSpeed returns (sizeCategory, walkSpeedFt) for the combatant.
// Falls back to (Medium, 30) when no lookup is wired or the lookup errors
// out — pathfinding still works with the defaults.
func (h *MoveHandler) resolveSizeAndSpeed(ctx context.Context, combatant refdata.Combatant) (int, int) {
	if h.sizeSpeedLookup == nil {
		return pathfinding.SizeMedium, 30
	}
	size, speed, err := h.sizeSpeedLookup.LookupSizeAndSpeed(ctx, combatant)
	if err != nil {
		return pathfinding.SizeMedium, 30
	}
	if speed <= 0 {
		speed = 30
	}
	return size, speed
}

// HasCharacterLookup reports whether a non-nil MoveCharacterLookup has been
// wired on this handler. Used by production-wiring tests to detect the
// Phase 110 first-PC bug (nil characterLookup falls back to pcs[0]).
func (h *MoveHandler) HasCharacterLookup() bool {
	return h.characterLookup != nil
}

// NewMoveHandler creates a new MoveHandler.
func NewMoveHandler(
	session Session,
	combatService MoveService,
	mapProvider MoveMapProvider,
	turnProvider MoveTurnProvider,
	encounterProvider MoveEncounterProvider,
	campaignProv CampaignProvider,
) *MoveHandler {
	return &MoveHandler{
		session:           session,
		combatService:     combatService,
		mapProvider:       mapProvider,
		turnProvider:      turnProvider,
		encounterProvider: encounterProvider,
		campaignProv:      campaignProv,
	}
}

// Handle processes the /move command interaction.
func (h *MoveHandler) Handle(interaction *discordgo.Interaction) {
	start := time.Now()
	ctx := context.Background()
	ctx = logging.WithCommand(ctx, "move")
	ctx = logging.WithUserID(ctx, discordUserID(interaction))
	ctx = logging.WithGuildID(ctx, interaction.GuildID)

	base := h.logger
	if base == nil {
		base = slog.Default()
	}
	log := logging.WithContext(ctx, base)
	log.Info("command received")
	defer func() { logging.WithDuration(log, start).Info("command completed") }()

	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	if len(data.Options) == 0 {
		respondEphemeral(h.session, interaction, "Please provide a coordinate (e.g. `/move D4`).")
		return
	}

	coordStr := data.Options[0].StringValue()
	destCol, destRow, err := renderer.ParseCoordinate(coordStr)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Invalid coordinate %q: %v", coordStr, err))
		return
	}

	guildID := interaction.GuildID

	// Phase 105: resolve the encounter via the invoking player's combatant
	// entry rather than a guild-wide "active encounter" lookup, so two
	// simultaneous encounters in the same campaign are disambiguated.
	encounterID, err := h.encounterProvider.ActiveEncounterForUser(ctx, guildID, discordUserID(interaction))
	if err != nil {
		respondEphemeral(h.session, interaction, "No active encounter for you in this server.")
		return
	}

	encounter, err := h.combatService.GetEncounter(ctx, encounterID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to get encounter data.")
		return
	}

	// Phase 110: exploration mode skips turn lookup, resource deduction, and
	// action-economy validation. Pathfinding + wall validation still run.
	if encounter.Mode == "exploration" {
		h.handleExplorationMove(ctx, interaction, encounter, destCol, destRow)
		return
	}

	if !encounter.CurrentTurnID.Valid {
		respondEphemeral(h.session, interaction, "No active turn.")
		return
	}

	// Phase 27 turn-ownership + advisory-lock gate. /move is NOT exempt
	// (combat.IsExemptCommand("move") == false), so two concurrent /move
	// invocations on the same active turn must serialize through
	// pg_advisory_xact_lock and a wrong-owner invocation must be rejected
	// before any turn / combatant lookup runs.
	if h.turnGate != nil {
		if _, gateErr := h.turnGate.AcquireAndRelease(ctx, encounterID, discordUserID(interaction)); gateErr != nil {
			respondEphemeral(h.session, interaction, formatTurnGateError(gateErr))
			return
		}
	}

	// Get turn and combatant
	turn, err := h.turnProvider.GetTurn(ctx, encounter.CurrentTurnID.UUID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to get turn data.")
		return
	}

	combatant, err := h.combatService.GetCombatant(ctx, turn.CombatantID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to get combatant data.")
		return
	}

	// C-43-block-commands: a dying or incapacitated combatant cannot move.
	if msg, blocked := incapacitatedRejection(combatant); blocked {
		respondEphemeral(h.session, interaction, msg)
		return
	}

	// Get map data
	if !encounter.MapID.Valid {
		respondEphemeral(h.session, interaction, "This encounter has no map.")
		return
	}

	mapData, err := h.mapProvider.GetByID(ctx, encounter.MapID.UUID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to load map data.")
		return
	}

	// Parse tiled JSON to get terrain and walls
	md, err := renderer.ParseTiledJSON(mapData.TiledJson, nil, nil)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to parse map data.")
		return
	}

	// Build occupants list from all combatants
	allCombatants, err := h.combatService.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to list combatants.")
		return
	}

	occupants := buildOccupants(allCombatants, combatant, h.occupantSizeFn(ctx))

	// C-40-frightened-move: a frightened combatant cannot move closer to
	// the source of its fear. The validator inspects the combatant's
	// conditions, finds any "frightened" entries with a source_combatant_id,
	// and rejects when the destination tile is closer than the start tile.
	if rejected := rejectFrightenedTowardSource(combatant, allCombatants, destCol, destRow); rejected != "" {
		respondEphemeral(h.session, interaction, rejected)
		return
	}

	grid := &pathfinding.Grid{
		Width:     md.Width,
		Height:    md.Height,
		Terrain:   md.TerrainGrid,
		Walls:     md.Walls,
		Occupants: occupants,
	}

	// med-21: look up actual creature size + walk speed via the wired
	// sizeSpeedLookup; fall back to Medium / 30 ft when no lookup is wired
	// or the lookup errors out.
	sizeCategory, maxSpeed := h.resolveSizeAndSpeed(ctx, combatant)

	moveReq := combat.MoveRequest{
		DestCol:      destCol,
		DestRow:      destRow,
		Turn:         turn,
		Combatant:    combatant,
		Grid:         grid,
		SizeCategory: sizeCategory,
	}

	// Check if combatant is prone and hasn't already stood this turn
	isProne := combat.HasCondition(combatant.Conditions, "prone")
	if isProne && !turn.HasStoodThisTurn {
		// Show Stand & Move / Crawl choice prompt with the looked-up
		// max walk speed encoded in the button custom IDs (med-21).
		standID := fmt.Sprintf("prone_stand:%s:%s:%d:%d:%d",
			turn.ID.String(), combatant.ID.String(), destCol, destRow, maxSpeed)
		crawlID := fmt.Sprintf("prone_crawl:%s:%s:%d:%d:%d",
			turn.ID.String(), combatant.ID.String(), destCol, destRow, maxSpeed)
		cancelID := fmt.Sprintf("move_cancel:%s", turn.ID.String())

		_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You are prone. How do you want to move?",
				Flags:   discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								Label:    "Stand & Move",
								Style:    discordgo.PrimaryButton,
								CustomID: standID,
								Emoji:    &discordgo.ComponentEmoji{Name: "\U0001f9cd"},
							},
							discordgo.Button{
								Label:    "Crawl",
								Style:    discordgo.PrimaryButton,
								CustomID: crawlID,
								Emoji:    &discordgo.ComponentEmoji{Name: "\U0001f41b"},
							},
							discordgo.Button{
								Label:    "Cancel",
								Style:    discordgo.DangerButton,
								CustomID: cancelID,
								Emoji:    &discordgo.ComponentEmoji{Name: "\u274c"},
							},
						},
					},
				},
			},
		})
		return
	}

	result, err := combat.ValidateMove(moveReq)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Move error: %v", err))
		return
	}

	if !result.Valid {
		respondEphemeral(h.session, interaction, result.Reason)
		return
	}

	// SR-047 / D-56 / Phase 56: when the mover is dragging one or more
	// grappled creatures, show a Drag / Release & Move button choice before
	// the standard move confirmation. The player picks whether to drag
	// (doubled cost) or release (normal cost).
	dragPromptPrefix := h.dragPromptForMove(context.Background(), encounterID, combatant)
	if dragPromptPrefix != "" {
		dragID := fmt.Sprintf("drag_choice:drag:%s:%s:%d:%d:%d",
			turn.ID.String(), combatant.ID.String(), destCol, destRow, result.CostFt)
		releaseID := fmt.Sprintf("drag_choice:release:%s:%s:%d:%d:%d",
			turn.ID.String(), combatant.ID.String(), destCol, destRow, result.CostFt)
		cancelID := fmt.Sprintf("move_cancel:%s", turn.ID.String())

		_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: dragPromptPrefix,
				Flags:   discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								Label:    "Drag",
								Style:    discordgo.SuccessButton,
								CustomID: dragID,
								Emoji:    &discordgo.ComponentEmoji{Name: "\u2705"},
							},
							discordgo.Button{
								Label:    "Release & Move",
								Style:    discordgo.PrimaryButton,
								CustomID: releaseID,
								Emoji:    &discordgo.ComponentEmoji{Name: "\u274c"},
							},
							discordgo.Button{
								Label:    "Cancel",
								Style:    discordgo.DangerButton,
								CustomID: cancelID,
								Emoji:    &discordgo.ComponentEmoji{Name: "\U0001f6ab"},
							},
						},
					},
				},
			},
		})
		return
	}

	confirmMsg := combat.FormatMoveConfirmation(result)

	// Encode move data in custom IDs for button callback
	confirmID := fmt.Sprintf("move_confirm:%s:%s:%d:%d:%d",
		turn.ID.String(), combatant.ID.String(), destCol, destRow, result.CostFt)
	cancelID := fmt.Sprintf("move_cancel:%s", turn.ID.String())

	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: confirmMsg,
			Flags:   discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Confirm",
							Style:    discordgo.SuccessButton,
							CustomID: confirmID,
							Emoji: &discordgo.ComponentEmoji{
								Name: "\u2705",
							},
						},
						discordgo.Button{
							Label:    "Cancel",
							Style:    discordgo.DangerButton,
							CustomID: cancelID,
							Emoji: &discordgo.ComponentEmoji{
								Name: "\u274c",
							},
						},
					},
				},
			},
		},
	})
}

// handleExplorationMove handles /move for an exploration-mode encounter (Phase 110).
// Skips turn lookup, movement deduction, and action-economy validation.
// Still enforces pathfinding + wall validation via pathfinding.FindPath.
func (h *MoveHandler) handleExplorationMove(ctx context.Context, interaction *discordgo.Interaction, encounter refdata.Encounter, destCol, destRow int) {
	if !encounter.MapID.Valid {
		respondEphemeral(h.session, interaction, "This encounter has no map.")
		return
	}

	mapData, err := h.mapProvider.GetByID(ctx, encounter.MapID.UUID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to load map data.")
		return
	}

	md, err := renderer.ParseTiledJSON(mapData.TiledJson, nil, nil)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to parse map data.")
		return
	}

	// Exploration has no CurrentTurnID, so resolveExplorationMover matches
	// the invoking Discord user's character against alive PC combatants
	// (falling back to the sole PC when lookup isn't wired).
	allCombatants, err := h.combatService.ListCombatantsByEncounterID(ctx, encounter.ID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to list combatants.")
		return
	}

	mover, ok := h.resolveExplorationMover(ctx, allCombatants, interaction)
	if !ok {
		respondEphemeral(h.session, interaction, "Could not find your character in this encounter.")
		return
	}

	startCol, startRow, err := renderer.ParseCoordinate(fmt.Sprintf("%s%d", mover.PositionCol, mover.PositionRow))
	if err != nil {
		respondEphemeral(h.session, interaction, "Invalid current position.")
		return
	}

	occupants := buildOccupants(allCombatants, mover, h.occupantSizeFn(ctx))
	grid := &pathfinding.Grid{
		Width:     md.Width,
		Height:    md.Height,
		Terrain:   md.TerrainGrid,
		Walls:     md.Walls,
		Occupants: occupants,
	}

	pathReq := pathfinding.PathRequest{
		Start:           pathfinding.Point{Col: startCol, Row: startRow},
		End:             pathfinding.Point{Col: destCol, Row: destRow},
		SizeCategory:    pathfinding.SizeMedium,
		Grid:            grid,
		MoverAltitudeFt: int(mover.AltitudeFt),
	}
	result, err := pathfinding.FindPath(pathReq)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Pathfinding error: %v", err))
		return
	}
	if result == nil || !result.Found {
		respondEphemeral(h.session, interaction, "No path to destination (blocked by walls or occupied tile).")
		return
	}

	destLabel := renderer.ColumnLabel(destCol)
	if _, err := h.combatService.UpdateCombatantPosition(ctx, mover.ID, destLabel, int32(destRow+1), mover.AltitudeFt); err != nil {
		respondEphemeral(h.session, interaction, "Failed to update position.")
		return
	}

	msg := fmt.Sprintf("\U0001f3c3 Moved to %s%d.", destLabel, destRow+1)
	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

// resolveExplorationMover picks the PC combatant that belongs to the invoking
// Discord user. Delegates to the shared resolveExplorationPC helper.
func (h *MoveHandler) resolveExplorationMover(ctx context.Context, all []refdata.Combatant, interaction *discordgo.Interaction) (refdata.Combatant, bool) {
	var getCampaign func(ctx context.Context, guildID string) (refdata.Campaign, error)
	if h.campaignProv != nil {
		getCampaign = h.campaignProv.GetCampaignByGuildID
	}
	var getCharacter func(ctx context.Context, campaignID uuid.UUID, discordUserID string) (refdata.Character, error)
	if h.characterLookup != nil {
		getCharacter = h.characterLookup.GetCharacterByCampaignAndDiscord
	}
	return resolveExplorationPC(ctx, all, interaction.GuildID, discordUserID(interaction), getCampaign, getCharacter)
}

// HandleMoveConfirm processes the move confirmation button click.
//
// F-4: the actual write (turn resource deduction + combatant position
// update) runs INSIDE turnGate.AcquireAndRun so the per-turn advisory lock
// is still held across the persistence step. A peer /move on the same turn
// blocks at the lock acquire until our writes commit, eliminating the
// previous "two handlers both pass the gate then race their writes" lost-
// update window. When the gate is unwired (legacy unit tests) the writes
// still happen — only the serialization guarantee is lost.
func (h *MoveHandler) HandleMoveConfirm(interaction *discordgo.Interaction, turnID, combatantID uuid.UUID, destCol, destRow, costFt int) {
	ctx := context.Background()

	turn, err := h.turnProvider.GetTurn(ctx, turnID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Turn no longer active.")
		return
	}

	// confirmMove is the lock-held write body. Errors map to a stable set
	// of sentinel strings so the AcquireAndRun wrapper can translate to a
	// user-facing ephemeral message without leaking DB errors.
	var (
		responseMsg    string
		moverCombatant refdata.Combatant
		moverFetchOK   bool
		updatedTurn    refdata.Turn
		destLabel      = renderer.ColumnLabel(destCol)
	)

	confirmMove := func(ctx context.Context) error {
		var moveErr error
		updatedTurn, moveErr = combat.UseMovement(turn, int32(costFt))
		if moveErr != nil {
			respondEphemeral(h.session, interaction, fmt.Sprintf("Cannot move: %v", moveErr))
			return errAlreadyResponded
		}

		if _, txErr := h.turnProvider.UpdateTurnActions(ctx, combat.TurnToUpdateParams(updatedTurn)); txErr != nil {
			respondEphemeral(h.session, interaction, "Failed to update turn resources.")
			return errAlreadyResponded
		}

		combatant, getErr := h.combatService.GetCombatant(ctx, combatantID)
		currentAltitude := int32(0)
		if getErr == nil {
			currentAltitude = combatant.AltitudeFt
			moverCombatant = combatant
			moverFetchOK = true
		}

		if _, posErr := h.combatService.UpdateCombatantPosition(ctx, combatantID, destLabel, int32(destRow+1), currentAltitude); posErr != nil {
			respondEphemeral(h.session, interaction, "Failed to update position.")
			return errAlreadyResponded
		}

		remaining := combat.FormatRemainingResources(updatedTurn, nil)
		responseMsg = fmt.Sprintf("\U0001f3c3 Moved to %s%d. %s", destLabel, destRow+1, remaining)
		return nil
	}

	if h.turnGate != nil {
		if _, gateErr := h.turnGate.AcquireAndRun(ctx, turn.EncounterID, discordUserID(interaction), confirmMove); gateErr != nil {
			// If confirmMove already wrote a response, respect it; otherwise
			// the failure was at the gate (ownership / lock / DB) and we
			// surface the standard turn-gate translation.
			if gateErr != errAlreadyResponded {
				respondEphemeral(h.session, interaction, formatTurnGateError(gateErr))
			}
			return
		}
	} else if runErr := confirmMove(ctx); runErr != nil {
		// Unit-test path with no gate wired. confirmMove already responded.
		return
	}

	// J-H09: publish encounter snapshot after position write.
	if h.publisher != nil {
		_ = h.publisher.PublishEncounterSnapshot(ctx, turn.EncounterID)
	}

	// D-56-followup: when the mover is dragging grappled targets, sync
	// each target's tile so it stays adjacent (5ft Chebyshev) to the
	// dragger after the move. Best-effort: any failure aborts silently
	// so /move never breaks because of a flaky drag sync. Runs OUTSIDE
	// the lock-held tx because these per-target writes do not need the
	// active-turn serialization guarantee — they target combatants the
	// mover is grappling, which are not the active turn's combatant.
	if moverFetchOK {
		h.syncDragTargetsAlongPath(ctx, moverCombatant, destCol, destRow)
	}

	// med-24 / Phase 55: fire opportunity-attack prompts after the move
	// commits. Best-effort: any failure is silent so a flaky channel post
	// can never break the move flow.
	if moverFetchOK {
		h.fireOpportunityAttacks(ctx, moverCombatant, updatedTurn, destCol, destRow, interaction.GuildID)
	}

	if moverFetchOK {
		if spotted := h.revealHiddenEnemiesByPassiveCheck(ctx, moverCombatant, updatedTurn, destCol, destRow); spotted != "" {
			responseMsg += "\n" + spotted
		}
	}

	// F-24: mirror move to #combat-log (best-effort, nil-safe).
	if moverFetchOK {
		logMsg := fmt.Sprintf("\U0001f3c3 %s moves to %s%d.", moverCombatant.DisplayName, destLabel, destRow+1)
		postCombatLogChannel(ctx, h.session, h.oaChannels, turn.EncounterID, logMsg)
	}

	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    responseMsg,
			Components: []discordgo.MessageComponent{}, // remove buttons
		},
	})
}

// fireOpportunityAttacks runs combat.DetectOpportunityAttacks against the
// just-confirmed move and routes each prompt to the appropriate surface:
// PC-controlled hostiles get a #your-turn ping (med-24 / Phase 55 legacy
// behavior); DM-controlled hostiles get a #dm-queue notification via the
// SR-028 notifier wiring so the DM (not the PC who owns the reaction)
// decides whether to spend the reaction. Best-effort throughout: any
// failure aborts silently so OA wiring can never block /move from
// completing. The guildID is threaded from the invoking interaction so
// the dm-queue notifier can resolve the per-guild #dm-queue channel
// without an extra DB round-trip.
func (h *MoveHandler) fireOpportunityAttacks(ctx context.Context, mover refdata.Combatant, moverTurn refdata.Turn, destCol, destRow int, guildID string) {
	// If neither surface is wired there's nothing to do.
	if h.oaChannels == nil && h.oaNotifier == nil {
		return
	}

	startCol, startRow, err := renderer.ParseCoordinate(mover.PositionCol + fmt.Sprintf("%d", mover.PositionRow))
	if err != nil {
		return
	}

	allCombatants, err := h.combatService.ListCombatantsByEncounterID(ctx, moverTurn.EncounterID)
	if err != nil {
		return
	}

	path := h.buildOAPath(ctx, mover, moverTurn, startCol, startRow, destCol, destRow, allCombatants)
	if len(path) < 2 {
		return
	}

	hostileTurns := h.lookupHostileTurns(ctx, moverTurn.EncounterID)
	creatureAttacks := h.lookupCreatureAttacks(ctx, allCombatants, mover)
	pcReach := h.lookupPCReach(ctx, allCombatants, mover)

	triggers := combat.DetectOpportunityAttacksWithReach(
		mover, path, allCombatants, moverTurn, hostileTurns, creatureAttacks, pcReach,
	)
	if len(triggers) == 0 {
		return
	}

	hostileByID := indexHostilesByID(allCombatants)
	yourTurnCh := h.resolveYourTurnChannel(ctx, moverTurn.EncounterID)
	campaignID := h.resolveCampaignIDForEncounter(ctx, moverTurn.EncounterID)

	for _, trigger := range triggers {
		hostile, ok := hostileByID[trigger.HostileID]
		dmControlled := ok && hostile.IsNpc
		if dmControlled && h.oaNotifier != nil {
			h.postOAToDMQueue(ctx, moverTurn.EncounterID, trigger, guildID, campaignID)
			continue
		}
		if yourTurnCh == "" {
			continue
		}
		_, _ = h.session.ChannelMessageSend(yourTurnCh, combat.FormatOAPrompt(trigger))
	}
}

// indexHostilesByID returns a map of combatantID → combatant for O(1)
// lookups by trigger.HostileID inside fireOpportunityAttacks.
func indexHostilesByID(all []refdata.Combatant) map[uuid.UUID]refdata.Combatant {
	out := make(map[uuid.UUID]refdata.Combatant, len(all))
	for _, c := range all {
		out[c.ID] = c
	}
	return out
}

// resolveYourTurnChannel returns the encounter's #your-turn channel ID or
// "" when the channels provider is unwired / errors / lacks a your-turn
// mapping. nil-safe: an unwired oaChannels degrades to "" so the caller
// can still fan DM-controlled hostiles to the notifier.
func (h *MoveHandler) resolveYourTurnChannel(ctx context.Context, encounterID uuid.UUID) string {
	if h.oaChannels == nil {
		return ""
	}
	channelIDs, err := h.oaChannels.GetChannelIDs(ctx, encounterID)
	if err != nil {
		return ""
	}
	return channelIDs["your-turn"]
}

// resolveCampaignIDForEncounter returns the encounter's campaign_id as a
// string, or "" on lookup failure. Used to populate the SR-028 dm-queue
// event so the dashboard can scope items per campaign.
func (h *MoveHandler) resolveCampaignIDForEncounter(ctx context.Context, encounterID uuid.UUID) string {
	if h.combatService == nil {
		return ""
	}
	enc, err := h.combatService.GetEncounter(ctx, encounterID)
	if err != nil {
		return ""
	}
	return enc.CampaignID.String()
}

// postOAToDMQueue builds a KindOpportunityAttack dmqueue.Event and posts
// it via the wired notifier, then records the returned item ID with the
// SR-028 forfeit-sweep recorder. Errors are intentionally swallowed —
// the prompt is best-effort and must never block /move.
func (h *MoveHandler) postOAToDMQueue(ctx context.Context, encounterID uuid.UUID, trigger combat.OATrigger, guildID, campaignID string) {
	itemID, err := h.oaNotifier.Post(ctx, dmqueue.Event{
		Kind:       dmqueue.KindOpportunityAttack,
		PlayerName: trigger.HostileName,
		Summary:    fmt.Sprintf("%s left reach at %s — react?", trigger.TargetName, trigger.ExitLabel),
		GuildID:    guildID,
		CampaignID: campaignID,
	})
	if err != nil {
		return
	}
	if h.oaRecorder != nil {
		h.oaRecorder.RecordPendingOA(encounterID, itemID)
	}
}

func (h *MoveHandler) revealHiddenEnemiesByPassiveCheck(ctx context.Context, mover refdata.Combatant, moverTurn refdata.Turn, destCol, destRow int) string {
	if h.passiveChecks == nil {
		return ""
	}

	startCol, startRow, err := renderer.ParseCoordinate(mover.PositionCol + fmt.Sprintf("%d", mover.PositionRow))
	if err != nil {
		return ""
	}

	allCombatants, err := h.combatService.ListCombatantsByEncounterID(ctx, moverTurn.EncounterID)
	if err != nil {
		return ""
	}

	path := h.buildOAPath(ctx, mover, moverTurn, startCol, startRow, destCol, destRow, allCombatants)
	if len(path) == 0 {
		return ""
	}

	perception, err := h.passiveChecks.PassiveCheckForCombatant(ctx, mover, "perception")
	if err != nil {
		return ""
	}

	var spotted []string
	for _, hidden := range allCombatants {
		if !shouldCheckHiddenEnemy(mover, hidden) {
			continue
		}
		if !pathComesNearCombatant(path, hidden, 1) {
			continue
		}
		stealth, err := h.passiveChecks.PassiveCheckForCombatant(ctx, hidden, "stealth")
		if err != nil {
			continue
		}
		if perception.Total < stealth.Total {
			continue
		}
		if _, err := h.combatService.UpdateCombatantVisibility(ctx, refdata.UpdateCombatantVisibilityParams{ID: hidden.ID, IsVisible: true}); err != nil {
			continue
		}
		spotted = append(spotted, hidden.DisplayName)
	}

	if len(spotted) == 0 {
		return ""
	}
	return "Spotted " + strings.Join(spotted, ", ")
}

func shouldCheckHiddenEnemy(mover, candidate refdata.Combatant) bool {
	if candidate.ID == mover.ID {
		return false
	}
	if !candidate.IsAlive || candidate.IsVisible {
		return false
	}
	return candidate.IsNpc != mover.IsNpc
}

func pathComesNearCombatant(path []pathfinding.Point, combatant refdata.Combatant, reachTiles int) bool {
	col, row, err := renderer.ParseCoordinate(combatant.PositionCol + fmt.Sprintf("%d", combatant.PositionRow))
	if err != nil {
		return false
	}
	for _, p := range path {
		if maxInt(absInt(p.Col-col), absInt(p.Row-row)) <= reachTiles {
			return true
		}
	}
	return false
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// buildOAPath re-runs pathfinding for the just-confirmed move so OA detection
// can walk the same path. Falls back to a 2-point start→dest segment when
// pathfinding fails (still detects OAs from hostiles adjacent to the start
// tile in most layouts).
func (h *MoveHandler) buildOAPath(ctx context.Context, mover refdata.Combatant, moverTurn refdata.Turn, startCol, startRow, destCol, destRow int, allCombatants []refdata.Combatant) []pathfinding.Point {
	fallback := []pathfinding.Point{
		{Col: startCol, Row: startRow},
		{Col: destCol, Row: destRow},
	}

	encounter, err := h.combatService.GetEncounter(ctx, moverTurn.EncounterID)
	if err != nil || !encounter.MapID.Valid {
		return fallback
	}
	mapData, err := h.mapProvider.GetByID(ctx, encounter.MapID.UUID)
	if err != nil {
		return fallback
	}
	md, err := renderer.ParseTiledJSON(mapData.TiledJson, nil, nil)
	if err != nil {
		return fallback
	}

	grid := &pathfinding.Grid{
		Width:     md.Width,
		Height:    md.Height,
		Terrain:   md.TerrainGrid,
		Walls:     md.Walls,
		Occupants: buildOccupants(allCombatants, mover, h.occupantSizeFn(ctx)),
	}
	sizeCategory, _ := h.resolveSizeAndSpeed(ctx, mover)
	pathReq := pathfinding.PathRequest{
		Start:           pathfinding.Point{Col: startCol, Row: startRow},
		End:             pathfinding.Point{Col: destCol, Row: destRow},
		SizeCategory:    sizeCategory,
		Grid:            grid,
		MoverAltitudeFt: int(mover.AltitudeFt),
	}
	result, err := pathfinding.FindPath(pathReq)
	if err != nil || result == nil || len(result.Path) < 2 {
		return fallback
	}
	return result.Path
}

// lookupHostileTurns fans out to the wired turn lookup. Returns an empty map
// (not nil) on any failure so DetectOpportunityAttacks's reaction-used filter
// degrades gracefully (every hostile assumed reaction-available).
func (h *MoveHandler) lookupHostileTurns(ctx context.Context, encounterID uuid.UUID) map[uuid.UUID]refdata.Turn {
	if h.oaTurns == nil {
		return map[uuid.UUID]refdata.Turn{}
	}
	turns, err := h.oaTurns.ListTurnsByEncounter(ctx, encounterID)
	if err != nil || turns == nil {
		return map[uuid.UUID]refdata.Turn{}
	}
	return turns
}

// lookupCreatureAttacks fans out per unique NPC creature_ref_id in the
// encounter so DetectOpportunityAttacks can resolve each NPC's max reach. Any
// per-creature failure is skipped (creature defaults to 5ft reach).
func (h *MoveHandler) lookupCreatureAttacks(ctx context.Context, all []refdata.Combatant, mover refdata.Combatant) map[string][]combat.CreatureAttackEntry {
	if h.oaCreatures == nil {
		return nil
	}
	out := make(map[string][]combat.CreatureAttackEntry)
	seen := make(map[string]bool)
	for _, c := range all {
		if c.ID == mover.ID || !c.IsAlive || !c.IsNpc || !c.CreatureRefID.Valid {
			continue
		}
		if seen[c.CreatureRefID.String] {
			continue
		}
		seen[c.CreatureRefID.String] = true
		attacks, err := h.oaCreatures.GetCreatureAttacks(ctx, c.CreatureRefID.String)
		if err != nil {
			continue
		}
		out[c.CreatureRefID.String] = attacks
	}
	return out
}

// lookupPCReach fans out per hostile PC in the encounter so DetectOpportunityAttacks
// can honor reach-weapon equipment. Skips NPCs and PCs without a wired lookup.
func (h *MoveHandler) lookupPCReach(ctx context.Context, all []refdata.Combatant, mover refdata.Combatant) map[uuid.UUID]int {
	if h.oaPCReach == nil {
		return nil
	}
	out := make(map[uuid.UUID]int)
	for _, c := range all {
		if c.ID == mover.ID || !c.IsAlive || c.IsNpc || !c.CharacterID.Valid {
			continue
		}
		// Same-faction combatants are skipped by DetectOpportunityAttacks
		// itself; we still skip here to avoid a wasted DB hit.
		if c.IsNpc == mover.IsNpc {
			continue
		}
		reach, err := h.oaPCReach.LookupPCReach(ctx, c.CharacterID.UUID)
		if err != nil || reach <= 0 {
			continue
		}
		out[c.ID] = reach
	}
	return out
}

// syncDragTargetsAlongPath persists each grappled target's tile so it
// stays within 5ft of the dragger after a /move confirmation. For the
// minimal-correct step, the target lands on the dragger's PRIOR tile
// (one step behind along the path) — that places the target adjacent to
// the dragger's destination, satisfying the 5ft invariant for both
// opportunity-attack detection and visibility checks. Multi-target drags
// fan each target onto the same prior tile; ties are acceptable since the
// grid renderer stacks tokens by altitude.
//
// Best-effort: any failure (no drag lookup, lookup error, no targets,
// persistence error) aborts silently so /move can't break because of a
// flaky drag sync. D-56-followup-drag-tile-sync.
func (h *MoveHandler) syncDragTargetsAlongPath(ctx context.Context, dragger refdata.Combatant, destCol, destRow int) {
	if h.dragLookup == nil {
		return
	}
	check, err := h.dragLookup.CheckDragTargets(ctx, dragger.EncounterID, dragger)
	if err != nil || !check.HasTargets {
		return
	}

	// Compute the prior tile (one step back toward the dragger's start).
	// The dragger row is 1-indexed (PositionRow) in DB; destRow is
	// 0-indexed grid row from the caller.
	startCol, startRow, perr := renderer.ParseCoordinate(dragger.PositionCol + fmt.Sprintf("%d", dragger.PositionRow))
	if perr != nil {
		// Fall back to placing target on the dragger's destination tile
		// (still satisfies the 5ft invariant; just no longer "behind").
		startCol = destCol
		startRow = destRow
	}

	priorCol, priorRow := tileOneStepBack(startCol, startRow, destCol, destRow)
	priorLabel := renderer.ColumnLabel(priorCol)
	for _, target := range check.GrappledTargets {
		_, _ = h.combatService.UpdateCombatantPosition(
			ctx, target.ID, priorLabel, int32(priorRow+1), target.AltitudeFt,
		)
	}
}

// tileOneStepBack returns the tile one step from the destination back
// toward the start. For a Chebyshev grid the answer is dest - sign(dest-start)
// along each axis. When start==dest (no movement) the destination is
// returned unchanged.
func tileOneStepBack(startCol, startRow, destCol, destRow int) (int, int) {
	colStep := 0
	switch {
	case destCol > startCol:
		colStep = 1
	case destCol < startCol:
		colStep = -1
	}
	rowStep := 0
	switch {
	case destRow > startRow:
		rowStep = 1
	case destRow < startRow:
		rowStep = -1
	}
	return destCol - colStep, destRow - rowStep
}

// dragPromptForMove returns the drag confirmation prompt when the mover is
// currently grappling other creatures, or "" when no drag is active or the
// drag lookup is unwired / errors out. The caller doubles the move cost only
// when a prompt is returned. (D-56 / Phase 56)
func (h *MoveHandler) dragPromptForMove(ctx context.Context, encounterID uuid.UUID, mover refdata.Combatant) string {
	if h.dragLookup == nil {
		return ""
	}
	check, err := h.dragLookup.CheckDragTargets(ctx, encounterID, mover)
	if err != nil || !check.HasTargets {
		return ""
	}
	return combat.FormatDragPrompt(check.GrappledTargets)
}

// HandleMoveCancel processes the move cancel button click.
func (h *MoveHandler) HandleMoveCancel(interaction *discordgo.Interaction) {
	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    "Move cancelled.",
			Components: []discordgo.MessageComponent{},
		},
	})
}

// HandleDragChoice processes the Drag / Release & Move button click (SR-047).
// mode is "drag" or "release". For "drag", the move confirmation is shown
// with doubled cost. For "release", grappled targets are released first, then
// the move confirmation is shown at normal cost.
func (h *MoveHandler) HandleDragChoice(interaction *discordgo.Interaction, mode string, turnID, combatantID uuid.UUID, destCol, destRow, baseCostFt int) {
	ctx := context.Background()

	if mode == "release" {
		// Release all grappled targets, then show normal-cost confirmation.
		if h.dragReleaser != nil && h.dragLookup != nil {
			combatant, err := h.combatService.GetCombatant(ctx, combatantID)
			if err != nil {
				respondEphemeral(h.session, interaction, "Failed to get combatant data.")
				return
			}
			check, err := h.dragLookup.CheckDragTargets(ctx, combatant.EncounterID, combatant)
			if err == nil && check.HasTargets {
				_, _ = h.dragReleaser.ReleaseDrag(ctx, combatant, check.GrappledTargets)
			}
		}
		// Show move confirmation at normal cost.
		h.showMoveConfirmation(interaction, turnID, combatantID, destCol, destRow, baseCostFt)
		return
	}

	// "drag" — show move confirmation with doubled cost.
	h.showMoveConfirmation(interaction, turnID, combatantID, destCol, destRow, combat.DragMovementCost(baseCostFt))
}

// showMoveConfirmation renders the standard Confirm/Cancel move prompt with
// the given cost. Used by HandleDragChoice to present the final move step.
func (h *MoveHandler) showMoveConfirmation(interaction *discordgo.Interaction, turnID, combatantID uuid.UUID, destCol, destRow, costFt int) {
	confirmMsg := combat.FormatMoveConfirmationFromParts(destCol, destRow, costFt)
	confirmID := fmt.Sprintf("move_confirm:%s:%s:%d:%d:%d",
		turnID.String(), combatantID.String(), destCol, destRow, costFt)
	cancelID := fmt.Sprintf("move_cancel:%s", turnID.String())

	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content: confirmMsg,
			Flags:   discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Confirm",
							Style:    discordgo.SuccessButton,
							CustomID: confirmID,
							Emoji:    &discordgo.ComponentEmoji{Name: "\u2705"},
						},
						discordgo.Button{
							Label:    "Cancel",
							Style:    discordgo.DangerButton,
							CustomID: cancelID,
							Emoji:    &discordgo.ComponentEmoji{Name: "\u274c"},
						},
					},
				},
			},
		},
	})
}

// buildOccupants creates a pathfinding occupant list from combatants, excluding the mover.
// The sizeFn callback resolves each occupant's actual size category; pass nil
// to fall back to Medium for every occupant.
//
// C-30 / Phase 30: previously hardcoded SizeMedium for every occupant, which
// broke Phase 29's "size diff ≥ 2 pass-through" rule (Tiny and Large/Huge
// blockers behaved identically to Medium). Now each occupant's size is
// resolved per-combatant via sizeFn so the pathfinder sees the real shape.
func buildOccupants(all []refdata.Combatant, mover refdata.Combatant, sizeFn func(refdata.Combatant) int) []pathfinding.Occupant {
	var occupants []pathfinding.Occupant
	for _, c := range all {
		if c.ID == mover.ID || !c.IsAlive {
			continue
		}
		col, row, err := renderer.ParseCoordinate(c.PositionCol + fmt.Sprintf("%d", c.PositionRow))
		if err != nil {
			continue
		}
		size := pathfinding.SizeMedium
		if sizeFn != nil {
			size = sizeFn(c)
		}
		occupants = append(occupants, pathfinding.Occupant{
			Col:          col,
			Row:          row,
			IsAlly:       c.IsNpc == mover.IsNpc, // ally if same faction
			SizeCategory: size,
			AltitudeFt:   int(c.AltitudeFt),
		})
	}
	return occupants
}

// rejectFrightenedTowardSource returns the ephemeral rejection message when
// the mover's conditions include `frightened` with a source_combatant_id and
// the proposed destination is closer to that source than the start tile.
// Returns "" when the move is allowed (mover is not frightened, the source
// is absent from the encounter, or the move is parallel/away).
//
// C-40-frightened-move / Phase 40: ValidateFrightenedMovement is defined in
// internal/combat but the validation was never invoked from the slash-command
// pipeline. Now wired so a frightened combatant cannot consume movement to
// approach its fear source.
func rejectFrightenedTowardSource(mover refdata.Combatant, all []refdata.Combatant, destCol, destRow int) string {
	conds, err := combat.ListConditions(mover.Conditions)
	if err != nil || len(conds) == 0 {
		return ""
	}
	startCol, startRow, err := renderer.ParseCoordinate(mover.PositionCol + fmt.Sprintf("%d", mover.PositionRow))
	if err != nil {
		return ""
	}
	fearSources := make(map[string][2]int, len(all))
	for _, c := range all {
		col, row, perr := renderer.ParseCoordinate(c.PositionCol + fmt.Sprintf("%d", c.PositionRow))
		if perr != nil {
			continue
		}
		fearSources[c.ID.String()] = [2]int{col, row}
	}
	if verr := combat.ValidateFrightenedMovement(conds, startCol, startRow, destCol, destRow, fearSources); verr != nil {
		return "Cannot move closer to the source of your fear."
	}
	return ""
}

// occupantSizeFn returns a closure that resolves an occupant's size via the
// wired sizeSpeedLookup. When no lookup is wired (legacy unit tests), the
// closure is nil so buildOccupants falls back to Medium.
func (h *MoveHandler) occupantSizeFn(ctx context.Context) func(refdata.Combatant) int {
	if h.sizeSpeedLookup == nil {
		return nil
	}
	return func(c refdata.Combatant) int {
		size, _, err := h.sizeSpeedLookup.LookupSizeAndSpeed(ctx, c)
		if err != nil {
			return pathfinding.SizeMedium
		}
		return size
	}
}

// buildGridForTurn fetches the map and combatant data needed to build a pathfinding grid.
// Returns an error string for the user if any step fails (empty string on success).
func (h *MoveHandler) buildGridForTurn(ctx context.Context, turn refdata.Turn, mover refdata.Combatant) (*pathfinding.Grid, string) {
	encounter, err := h.combatService.GetEncounter(ctx, turn.EncounterID)
	if err != nil || !encounter.MapID.Valid {
		return nil, "Failed to get map data."
	}

	mapData, err := h.mapProvider.GetByID(ctx, encounter.MapID.UUID)
	if err != nil {
		return nil, "Failed to load map data."
	}

	md, err := renderer.ParseTiledJSON(mapData.TiledJson, nil, nil)
	if err != nil {
		return nil, "Failed to parse map data."
	}

	allCombatants, err := h.combatService.ListCombatantsByEncounterID(ctx, turn.EncounterID)
	if err != nil {
		return nil, "Failed to list combatants."
	}

	return &pathfinding.Grid{
		Width:     md.Width,
		Height:    md.Height,
		Terrain:   md.TerrainGrid,
		Walls:     md.Walls,
		Occupants: buildOccupants(allCombatants, mover, h.occupantSizeFn(ctx)),
	}, ""
}

// respondUpdateConfirmCancel sends an update-message response with Confirm and Cancel buttons.
func (h *MoveHandler) respondUpdateConfirmCancel(interaction *discordgo.Interaction, msg, confirmID, cancelID string) {
	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Confirm",
							Style:    discordgo.SuccessButton,
							CustomID: confirmID,
							Emoji:    &discordgo.ComponentEmoji{Name: "\u2705"},
						},
						discordgo.Button{
							Label:    "Cancel",
							Style:    discordgo.DangerButton,
							CustomID: cancelID,
							Emoji:    &discordgo.ComponentEmoji{Name: "\u274c"},
						},
					},
				},
			},
		},
	})
}

// HandleProneStandAndMove handles the Stand & Move button click for a prone combatant.
// It validates the move with stand cost deducted, then shows a confirmation prompt.
func (h *MoveHandler) HandleProneStandAndMove(interaction *discordgo.Interaction, turnID, combatantID uuid.UUID, destCol, destRow, maxSpeed int) {
	ctx := context.Background()

	turn, err := h.turnProvider.GetTurn(ctx, turnID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Turn no longer active.")
		return
	}

	combatant, err := h.combatService.GetCombatant(ctx, combatantID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to get combatant data.")
		return
	}

	grid, errMsg := h.buildGridForTurn(ctx, turn, combatant)
	if errMsg != "" {
		respondEphemeral(h.session, interaction, errMsg)
		return
	}

	moveReq := combat.MoveRequest{
		DestCol:      destCol,
		DestRow:      destRow,
		Turn:         turn,
		Combatant:    combatant,
		Grid:         grid,
		SizeCategory: pathfinding.SizeMedium,
	}

	result, err := combat.ValidateProneMoveStandAndMove(moveReq, maxSpeed)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Move error: %v", err))
		return
	}

	if !result.Valid {
		respondEphemeral(h.session, interaction, result.Reason)
		return
	}

	confirmID := fmt.Sprintf("move_confirm:%s:%s:%d:%d:%d:stand_and_move:%d",
		turnID.String(), combatantID.String(), destCol, destRow, result.CostFt, result.StandCostFt)
	cancelID := fmt.Sprintf("move_cancel:%s", turnID.String())

	h.respondUpdateConfirmCancel(interaction, combat.FormatMoveConfirmation(result), confirmID, cancelID)
}

// HandleProneCrawl handles the Crawl button click for a prone combatant.
// It validates the move with crawl costs, then shows a confirmation prompt.
func (h *MoveHandler) HandleProneCrawl(interaction *discordgo.Interaction, turnID, combatantID uuid.UUID, destCol, destRow int) {
	ctx := context.Background()

	turn, err := h.turnProvider.GetTurn(ctx, turnID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Turn no longer active.")
		return
	}

	combatant, err := h.combatService.GetCombatant(ctx, combatantID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to get combatant data.")
		return
	}

	grid, errMsg := h.buildGridForTurn(ctx, turn, combatant)
	if errMsg != "" {
		respondEphemeral(h.session, interaction, errMsg)
		return
	}

	moveReq := combat.MoveRequest{
		DestCol:      destCol,
		DestRow:      destRow,
		Turn:         turn,
		Combatant:    combatant,
		Grid:         grid,
		SizeCategory: pathfinding.SizeMedium,
	}

	result, err := combat.ValidateProneMoveCrawl(moveReq)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Move error: %v", err))
		return
	}

	if !result.Valid {
		respondEphemeral(h.session, interaction, result.Reason)
		return
	}

	confirmID := fmt.Sprintf("move_confirm:%s:%s:%d:%d:%d:crawl:0",
		turnID.String(), combatantID.String(), destCol, destRow, result.CostFt)
	cancelID := fmt.Sprintf("move_cancel:%s", turnID.String())

	h.respondUpdateConfirmCancel(interaction, combat.FormatMoveConfirmation(result), confirmID, cancelID)
}

// HandleMoveConfirmWithMode processes a move confirmation with an explicit move mode.
// For stand_and_move, it deducts stand cost + path cost and sets HasStoodThisTurn.
// For crawl, it deducts the crawl cost only.
func (h *MoveHandler) HandleMoveConfirmWithMode(interaction *discordgo.Interaction, turnID, combatantID uuid.UUID, destCol, destRow, costFt int, mode string, standCostFt int) {
	ctx := context.Background()

	turn, err := h.turnProvider.GetTurn(ctx, turnID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Turn no longer active.")
		return
	}

	totalCost := int32(costFt)
	if mode == "stand_and_move" {
		totalCost += int32(standCostFt)
	}

	updatedTurn, err := combat.UseMovement(turn, totalCost)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Cannot move: %v", err))
		return
	}

	if mode == "stand_and_move" {
		updatedTurn.HasStoodThisTurn = true

		// Remove prone condition from combatant
		combatant, getErr := h.combatService.GetCombatant(ctx, combatantID)
		if getErr == nil {
			newConds, removeErr := combat.RemoveCondition(combatant.Conditions, "prone")
			if removeErr == nil {
				_, _ = h.combatService.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
					ID:              combatantID,
					Conditions:      newConds,
					ExhaustionLevel: combatant.ExhaustionLevel,
				})
			}
		}
	}

	_, err = h.turnProvider.UpdateTurnActions(ctx, combat.TurnToUpdateParams(updatedTurn))
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to update turn resources.")
		return
	}

	combatant, getErr := h.combatService.GetCombatant(ctx, combatantID)
	currentAltitude := int32(0)
	if getErr == nil {
		currentAltitude = combatant.AltitudeFt
	}

	destLabel := renderer.ColumnLabel(destCol)
	_, err = h.combatService.UpdateCombatantPosition(ctx, combatantID, destLabel, int32(destRow+1), currentAltitude)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to update position.")
		return
	}

	remaining := combat.FormatRemainingResources(updatedTurn, nil)
	var msg string
	if mode == "stand_and_move" {
		msg = fmt.Sprintf("\U0001f3c3 Stood up and moved to %s%d. %s", destLabel, destRow+1, remaining)
	} else {
		msg = fmt.Sprintf("\U0001f41b Crawled to %s%d. %s", destLabel, destRow+1, remaining)
	}

	if getErr == nil {
		if spotted := h.revealHiddenEnemiesByPassiveCheck(ctx, combatant, updatedTurn, destCol, destRow); spotted != "" {
			msg += "\n" + spotted
		}
	}

	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    msg,
			Components: []discordgo.MessageComponent{},
		},
	})
}

// ParseMoveConfirmData parses the custom ID of a move confirm button.
func ParseMoveConfirmData(customID string) (turnID, combatantID uuid.UUID, destCol, destRow, costFt int, err error) {
	var turnStr, combatantStr string
	n, scanErr := fmt.Sscanf(customID, "move_confirm:%36s:%36s:%d:%d:%d",
		&turnStr, &combatantStr, &destCol, &destRow, &costFt)
	if scanErr != nil || n != 5 {
		return uuid.Nil, uuid.Nil, 0, 0, 0, fmt.Errorf("invalid move confirm data: %q", customID)
	}
	turnID, err = uuid.Parse(turnStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, 0, 0, 0, fmt.Errorf("invalid turn ID: %w", err)
	}
	combatantID, err = uuid.Parse(combatantStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, 0, 0, 0, fmt.Errorf("invalid combatant ID: %w", err)
	}
	return turnID, combatantID, destCol, destRow, costFt, nil
}

// ParseMoveConfirmWithModeData parses a move confirm custom ID that includes mode and stand cost.
// Format: move_confirm:<turnID>:<combatantID>:<col>:<row>:<cost>:<mode>:<standCost>
func ParseMoveConfirmWithModeData(customID string) (turnID, combatantID uuid.UUID, destCol, destRow, costFt int, mode string, standCostFt int, err error) {
	var turnStr, combatantStr string
	n, scanErr := fmt.Sscanf(customID, "move_confirm:%36s:%36s:%d:%d:%d:%s",
		&turnStr, &combatantStr, &destCol, &destRow, &costFt, &mode)
	if scanErr != nil || n != 6 {
		return uuid.Nil, uuid.Nil, 0, 0, 0, "", 0, fmt.Errorf("invalid move confirm with mode data: %q", customID)
	}

	// mode may contain ":standCost" suffix — split at the last colon
	if idx := strings.LastIndex(mode, ":"); idx >= 0 {
		_, scanErr = fmt.Sscanf(mode[idx+1:], "%d", &standCostFt)
		if scanErr != nil {
			return uuid.Nil, uuid.Nil, 0, 0, 0, "", 0, fmt.Errorf("invalid stand cost in: %q", customID)
		}
		mode = mode[:idx]
	}

	turnID, err = uuid.Parse(turnStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, 0, 0, 0, "", 0, fmt.Errorf("invalid turn ID: %w", err)
	}
	combatantID, err = uuid.Parse(combatantStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, 0, 0, 0, "", 0, fmt.Errorf("invalid combatant ID: %w", err)
	}
	return turnID, combatantID, destCol, destRow, costFt, mode, standCostFt, nil
}

// ParseDragChoiceData parses a drag choice button custom ID.
// Format: drag_choice:<mode>:<turnID>:<combatantID>:<col>:<row>:<cost>
func ParseDragChoiceData(customID string) (mode string, turnID, combatantID uuid.UUID, destCol, destRow, costFt int, err error) {
	var modeStr string
	n, scanErr := fmt.Sscanf(customID, "drag_choice:%s", &modeStr)
	if scanErr != nil || n != 1 {
		return "", uuid.Nil, uuid.Nil, 0, 0, 0, fmt.Errorf("invalid drag choice data: %q", customID)
	}
	// modeStr captured everything after "drag_choice:" — split on ":"
	parts := strings.SplitN(modeStr, ":", 6)
	if len(parts) != 6 {
		return "", uuid.Nil, uuid.Nil, 0, 0, 0, fmt.Errorf("invalid drag choice data: %q", customID)
	}
	mode = parts[0]
	turnID, err = uuid.Parse(parts[1])
	if err != nil {
		return "", uuid.Nil, uuid.Nil, 0, 0, 0, fmt.Errorf("invalid turn ID: %w", err)
	}
	combatantID, err = uuid.Parse(parts[2])
	if err != nil {
		return "", uuid.Nil, uuid.Nil, 0, 0, 0, fmt.Errorf("invalid combatant ID: %w", err)
	}
	_, scanErr = fmt.Sscanf(parts[3]+":"+parts[4]+":"+parts[5], "%d:%d:%d", &destCol, &destRow, &costFt)
	if scanErr != nil {
		return "", uuid.Nil, uuid.Nil, 0, 0, 0, fmt.Errorf("invalid coordinates in drag choice: %q", customID)
	}
	return mode, turnID, combatantID, destCol, destRow, costFt, nil
}

// ParseProneMoveData parses a prone movement button custom ID.
// Format: <prefix>:<turnID>:<combatantID>:<col>:<row>:<maxSpeed>
func ParseProneMoveData(customID string, prefix string) (turnID, combatantID uuid.UUID, destCol, destRow, maxSpeed int, err error) {
	var turnStr, combatantStr string
	format := prefix + ":%36s:%36s:%d:%d:%d"
	n, scanErr := fmt.Sscanf(customID, format, &turnStr, &combatantStr, &destCol, &destRow, &maxSpeed)
	if scanErr != nil || n != 5 {
		return uuid.Nil, uuid.Nil, 0, 0, 0, fmt.Errorf("invalid prone move data: %q", customID)
	}
	turnID, err = uuid.Parse(turnStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, 0, 0, 0, fmt.Errorf("invalid turn ID: %w", err)
	}
	combatantID, err = uuid.Parse(combatantStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, 0, 0, 0, fmt.Errorf("invalid combatant ID: %w", err)
	}
	return turnID, combatantID, destCol, destRow, maxSpeed, nil
}
