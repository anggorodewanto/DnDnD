package discord

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// --- Mocks for /bonus ---

type mockBonusCombatService struct {
	rageCalls       []combat.RageCommand
	endRageCalls    []combat.RageCommand
	offhandCalls    []combat.OffhandAttackCommand
	martialCalls    []combat.MartialArtsBonusAttackCommand
	polearmCalls    []combat.PolearmMasterBonusAttackCommand
	crossbowCalls   []combat.CrossbowExpertBonusAttackCommand
	stepCalls       []combat.StepOfTheWindCommand
	patientCalls    []combat.KiAbilityCommand
	fomConvertCalls []combat.FontOfMagicCommand
	fomCreateCalls  []combat.FontOfMagicCommand
	layCalls        []combat.LayOnHandsCommand
	bardicCalls     []combat.BardicInspirationCommand
	secondWindCalls []combat.SecondWindCommand

	rageResult       combat.RageResult
	endRageResult    combat.RageResult
	offhandResult    combat.AttackResult
	martialResult    combat.AttackResult
	polearmResult    combat.AttackResult
	crossbowResult   combat.AttackResult
	stepResult       combat.KiAbilityResult
	patientResult    combat.KiAbilityResult
	fomResult        combat.FontOfMagicResult
	layResult        combat.LayOnHandsResult
	bardicResult     combat.BardicInspirationResult
	secondWindResult combat.SecondWindResult

	// D-47 / D-48b / D-54-cunning / D-56 / D-57 recordings + canned results.
	wsActivateCalls  []combat.WildShapeCommand
	wsActivateResult combat.WildShapeResult
	wsRevertCalls    []combat.RevertWildShapeCommand
	wsRevertResult   combat.RevertWildShapeResult
	flurryCalls      []combat.FlurryOfBlowsCommand
	flurryResult     combat.FlurryOfBlowsResult
	cunningCalls     []combat.CunningActionCommand
	cunningResult    combat.CunningActionResult
	dragCheckCalls   int
	dragCheckResult  combat.DragCheckResult
	releaseCalls     []releaseDragCall
	releaseResult    combat.ReleaseDragResult
}

type releaseDragCall struct {
	Mover   refdata.Combatant
	Targets []refdata.Combatant
}

func (m *mockBonusCombatService) ActivateRage(_ context.Context, cmd combat.RageCommand) (combat.RageResult, error) {
	m.rageCalls = append(m.rageCalls, cmd)
	return m.rageResult, nil
}

func (m *mockBonusCombatService) EndRage(_ context.Context, cmd combat.RageCommand) (combat.RageResult, error) {
	m.endRageCalls = append(m.endRageCalls, cmd)
	return m.endRageResult, nil
}

func (m *mockBonusCombatService) OffhandAttack(_ context.Context, cmd combat.OffhandAttackCommand, _ *dice.Roller) (combat.AttackResult, error) {
	m.offhandCalls = append(m.offhandCalls, cmd)
	return m.offhandResult, nil
}

func (m *mockBonusCombatService) MartialArtsBonusAttack(_ context.Context, cmd combat.MartialArtsBonusAttackCommand, _ *dice.Roller) (combat.AttackResult, error) {
	m.martialCalls = append(m.martialCalls, cmd)
	return m.martialResult, nil
}

func (m *mockBonusCombatService) PolearmMasterBonusAttack(_ context.Context, cmd combat.PolearmMasterBonusAttackCommand, _ *dice.Roller) (combat.AttackResult, error) {
	m.polearmCalls = append(m.polearmCalls, cmd)
	return m.polearmResult, nil
}

func (m *mockBonusCombatService) CrossbowExpertBonusAttack(_ context.Context, cmd combat.CrossbowExpertBonusAttackCommand, _ *dice.Roller) (combat.AttackResult, error) {
	m.crossbowCalls = append(m.crossbowCalls, cmd)
	return m.crossbowResult, nil
}

func (m *mockBonusCombatService) StepOfTheWind(_ context.Context, cmd combat.StepOfTheWindCommand) (combat.KiAbilityResult, error) {
	m.stepCalls = append(m.stepCalls, cmd)
	return m.stepResult, nil
}

func (m *mockBonusCombatService) PatientDefense(_ context.Context, cmd combat.KiAbilityCommand) (combat.KiAbilityResult, error) {
	m.patientCalls = append(m.patientCalls, cmd)
	return m.patientResult, nil
}

func (m *mockBonusCombatService) FontOfMagicConvertSlot(_ context.Context, cmd combat.FontOfMagicCommand) (combat.FontOfMagicResult, error) {
	m.fomConvertCalls = append(m.fomConvertCalls, cmd)
	return m.fomResult, nil
}

func (m *mockBonusCombatService) FontOfMagicCreateSlot(_ context.Context, cmd combat.FontOfMagicCommand) (combat.FontOfMagicResult, error) {
	m.fomCreateCalls = append(m.fomCreateCalls, cmd)
	return m.fomResult, nil
}

func (m *mockBonusCombatService) LayOnHands(_ context.Context, cmd combat.LayOnHandsCommand) (combat.LayOnHandsResult, error) {
	m.layCalls = append(m.layCalls, cmd)
	return m.layResult, nil
}

func (m *mockBonusCombatService) GrantBardicInspiration(_ context.Context, cmd combat.BardicInspirationCommand) (combat.BardicInspirationResult, error) {
	m.bardicCalls = append(m.bardicCalls, cmd)
	return m.bardicResult, nil
}

func (m *mockBonusCombatService) SecondWind(_ context.Context, cmd combat.SecondWindCommand, _ *dice.Roller) (combat.SecondWindResult, error) {
	m.secondWindCalls = append(m.secondWindCalls, cmd)
	return m.secondWindResult, nil
}

func (m *mockBonusCombatService) ActivateWildShape(_ context.Context, cmd combat.WildShapeCommand) (combat.WildShapeResult, error) {
	m.wsActivateCalls = append(m.wsActivateCalls, cmd)
	return m.wsActivateResult, nil
}

func (m *mockBonusCombatService) RevertWildShapeService(_ context.Context, cmd combat.RevertWildShapeCommand) (combat.RevertWildShapeResult, error) {
	m.wsRevertCalls = append(m.wsRevertCalls, cmd)
	return m.wsRevertResult, nil
}

func (m *mockBonusCombatService) FlurryOfBlows(_ context.Context, cmd combat.FlurryOfBlowsCommand, _ *dice.Roller) (combat.FlurryOfBlowsResult, error) {
	m.flurryCalls = append(m.flurryCalls, cmd)
	return m.flurryResult, nil
}

func (m *mockBonusCombatService) CunningAction(_ context.Context, cmd combat.CunningActionCommand, _ ...*dice.Roller) (combat.CunningActionResult, error) {
	m.cunningCalls = append(m.cunningCalls, cmd)
	return m.cunningResult, nil
}

func (m *mockBonusCombatService) CheckDragTargets(_ context.Context, _ uuid.UUID, _ refdata.Combatant) (combat.DragCheckResult, error) {
	m.dragCheckCalls++
	return m.dragCheckResult, nil
}

func (m *mockBonusCombatService) ReleaseDrag(_ context.Context, mover refdata.Combatant, targets []refdata.Combatant) (combat.ReleaseDragResult, error) {
	m.releaseCalls = append(m.releaseCalls, releaseDragCall{Mover: mover, Targets: targets})
	return m.releaseResult, nil
}

type mockBonusProvider struct {
	encID      uuid.UUID
	enc        refdata.Encounter
	turn       refdata.Turn
	actor      refdata.Combatant
	target     refdata.Combatant
	resolveErr error
}

func (m *mockBonusProvider) ActiveEncounterForUser(_ context.Context, _, _ string) (uuid.UUID, error) {
	if m.resolveErr != nil {
		return uuid.Nil, m.resolveErr
	}
	return m.encID, nil
}

func (m *mockBonusProvider) GetEncounter(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
	return m.enc, nil
}

func (m *mockBonusProvider) GetCombatant(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
	if id == m.actor.ID {
		return m.actor, nil
	}
	return m.target, nil
}

func (m *mockBonusProvider) ListCombatantsByEncounterID(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
	return []refdata.Combatant{m.actor, m.target}, nil
}

func (m *mockBonusProvider) GetTurn(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
	return m.turn, nil
}

// --- Helpers ---

func makeBonusInteraction(action, args string) *discordgo.Interaction {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "action", Value: action, Type: discordgo.ApplicationCommandOptionString},
	}
	if args != "" {
		opts = append(opts, &discordgo.ApplicationCommandInteractionDataOption{
			Name: "args", Value: args, Type: discordgo.ApplicationCommandOptionString,
		})
	}
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "g1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "u1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "bonus",
			Options: opts,
		},
	}
}

func setupBonusHandler() (*BonusHandler, *mockMoveSession, *mockBonusCombatService, *mockBonusProvider) {
	encID := uuid.New()
	turnID := uuid.New()
	actorID := uuid.New()
	targetID := uuid.New()

	provider := &mockBonusProvider{
		encID: encID,
		enc: refdata.Encounter{
			ID:            encID,
			CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
		},
		turn: refdata.Turn{
			ID:          turnID,
			CombatantID: actorID,
		},
		actor: refdata.Combatant{
			ID: actorID, ShortID: "AR", DisplayName: "Aria",
		},
		target: refdata.Combatant{
			ID: targetID, ShortID: "OS", DisplayName: "Orc",
		},
	}
	combatSvc := &mockBonusCombatService{
		rageResult:    combat.RageResult{CombatLog: "🦁 Aria rages!"},
		endRageResult: combat.RageResult{CombatLog: "🦁 Aria's rage ends."},
		offhandResult: combat.AttackResult{
			AttackerName: "Aria",
			TargetName:   "Orc",
			WeaponName:   "Dagger",
			Hit:          true,
			IsMelee:      true,
			DistanceFt:   5,
			DamageTotal:  3,
		},
		martialResult:    combat.AttackResult{AttackerName: "Aria", TargetName: "Orc", WeaponName: "unarmed", Hit: true, IsMelee: true},
		stepResult:       combat.KiAbilityResult{CombatLog: "💨 Aria uses Step of the Wind (dash)"},
		patientResult:    combat.KiAbilityResult{CombatLog: "🛡️ Aria uses Patient Defense"},
		fomResult:        combat.FontOfMagicResult{CombatLog: "🔮 Font of Magic resolved"},
		layResult:        combat.LayOnHandsResult{CombatLog: "💛 Aria heals Orc"},
		bardicResult:     combat.BardicInspirationResult{CombatLog: "🎵 Aria inspires Orc"},
		secondWindResult: combat.SecondWindResult{CombatLog: "💪 Aria uses Second Wind — regains 12 HP"},
	}
	sess := &mockMoveSession{}
	h := NewBonusHandler(sess, combatSvc, provider, dice.NewRoller(func(_ int) int { return 10 }))
	return h, sess, combatSvc, provider
}

// --- Tests ---

func TestBonusHandler_Rage(t *testing.T) {
	h, sess, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("rage", ""))
	if len(svc.rageCalls) != 1 {
		t.Fatalf("expected 1 rage call, got %d", len(svc.rageCalls))
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "rages") {
		t.Errorf("expected rage log, got %q", sess.lastResponse.Data.Content)
	}
	if sess.lastResponse.Data.Flags&discordgo.MessageFlagsEphemeral != 0 {
		t.Errorf("expected rage result to be public (non-ephemeral), got flags %d", sess.lastResponse.Data.Flags)
	}
}

func TestBonusHandler_DragPromptStaysEphemeral(t *testing.T) {
	h, sess, svc, provider := setupBonusHandler()
	svc.dragCheckResult = combat.DragCheckResult{
		HasTargets:      true,
		GrappledTargets: []refdata.Combatant{provider.target},
	}
	h.Handle(makeBonusInteraction("drag", ""))
	if svc.dragCheckCalls != 1 {
		t.Fatalf("expected 1 drag check call, got %d", svc.dragCheckCalls)
	}
	if sess.lastResponse.Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Errorf("expected drag prompt to remain ephemeral, got flags %d", sess.lastResponse.Data.Flags)
	}
}

func TestBonusHandler_EndRage(t *testing.T) {
	h, _, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("end-rage", ""))
	if len(svc.endRageCalls) != 1 {
		t.Fatalf("expected 1 end-rage call, got %d", len(svc.endRageCalls))
	}
}

func TestBonusHandler_OffhandRoutesToOffhandAttack(t *testing.T) {
	h, sess, svc, provider := setupBonusHandler()

	h.Handle(makeBonusInteraction("offhand", "OS"))

	if len(svc.offhandCalls) != 1 {
		t.Fatalf("expected 1 offhand call, got %d", len(svc.offhandCalls))
	}
	got := svc.offhandCalls[0]
	if got.Attacker.ID != provider.actor.ID {
		t.Errorf("expected attacker %s, got %s", provider.actor.ID, got.Attacker.ID)
	}
	if got.Target.ShortID != "OS" {
		t.Errorf("expected target OS, got %s", got.Target.ShortID)
	}
	if got.Turn.ID != provider.turn.ID {
		t.Errorf("expected turn %s, got %s", provider.turn.ID, got.Turn.ID)
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Dagger") {
		t.Errorf("expected offhand attack log, got %q", sess.lastResponse.Data.Content)
	}
}

func TestBonusHandler_OffhandForwardsMapWalls(t *testing.T) {
	h, _, svc, provider := setupBonusHandler()
	mapID := uuid.New()
	provider.enc.MapID = uuid.NullUUID{UUID: mapID, Valid: true}
	tiled := json.RawMessage(`{
		"height": 3, "width": 3, "tilewidth": 48, "tileheight": 48,
		"layers": [
			{"name": "terrain", "type": "tilelayer", "width": 3, "height": 3,
			 "data": [1,1,1, 1,1,1, 1,1,1]},
			{"name": "walls", "type": "objectgroup",
			 "objects": [{"x": 96, "y": 0, "width": 0, "height": 48}]}
		],
		"tilesets": [{"firstgid": 1, "name": "base", "tiles": [{"id": 0, "type": "open_ground"}]}]
	}`)
	h.SetMapProvider(&stubAttackMapProvider{mapData: refdata.Map{ID: mapID, TiledJson: tiled}})

	h.Handle(makeBonusInteraction("offhand", "OS"))

	if len(svc.offhandCalls) != 1 {
		t.Fatalf("expected 1 offhand call, got %d", len(svc.offhandCalls))
	}
	if len(svc.offhandCalls[0].Walls) == 0 {
		t.Fatal("expected /bonus offhand to forward encounter map walls")
	}
}

type bonusOffhandCommandPathStore struct {
	combat.Store

	char       refdata.Character
	attacker   refdata.Combatant
	target     refdata.Combatant
	savedTurns []refdata.UpdateTurnActionsParams
	hpWrites   []refdata.UpdateCombatantHPParams
}

// CreateActionLog is a no-op: ISSUE-014 made OffhandAttack best-effort record
// the swing to action_log, but this path-test only asserts the bonus-action /
// damage-mod economy, so the embedded (nil) combat.Store must not be invoked.
func (s *bonusOffhandCommandPathStore) CreateActionLog(_ context.Context, _ refdata.CreateActionLogParams) (refdata.ActionLog, error) {
	return refdata.ActionLog{}, nil
}

func (s *bonusOffhandCommandPathStore) GetCharacter(_ context.Context, id uuid.UUID) (refdata.Character, error) {
	if id == s.char.ID {
		return s.char, nil
	}
	return refdata.Character{}, sql.ErrNoRows
}

func (s *bonusOffhandCommandPathStore) GetWeapon(_ context.Context, id string) (refdata.Weapon, error) {
	switch id {
	case "shortsword":
		return refdata.Weapon{
			ID:         "shortsword",
			Name:       "Shortsword",
			Damage:     "1d6",
			DamageType: "piercing",
			WeaponType: "martial_melee",
			Properties: []string{"finesse", "light"},
		}, nil
	case "dagger":
		return refdata.Weapon{
			ID:         "dagger",
			Name:       "Dagger",
			Damage:     "1d4",
			DamageType: "piercing",
			WeaponType: "simple_melee",
			Properties: []string{"finesse", "light", "thrown"},
		}, nil
	default:
		return refdata.Weapon{}, sql.ErrNoRows
	}
}

func (s *bonusOffhandCommandPathStore) UpdateTurnActions(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
	s.savedTurns = append(s.savedTurns, arg)
	return refdata.Turn{
		ID:               arg.ID,
		CombatantID:      s.attacker.ID,
		AttacksRemaining: arg.AttacksRemaining,
		ActionUsed:       arg.ActionUsed,
		BonusActionUsed:  arg.BonusActionUsed,
		ReactionUsed:     arg.ReactionUsed,
	}, nil
}

func (s *bonusOffhandCommandPathStore) ListCombatantsByEncounterID(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
	return []refdata.Combatant{s.attacker, s.target}, nil
}

func (s *bonusOffhandCommandPathStore) ListEncounterZonesByEncounterID(_ context.Context, _ uuid.UUID) ([]refdata.EncounterZone, error) {
	return nil, nil
}

// UpdateCombatantHP records the off-hand hit's HP write. The embedded nil
// combat.Store cannot service this call, so the off-hand ApplyDamage pipeline
// (added when every primary hit began applying its damage) needs a faithful
// implementation here that returns a non-nil combatant.
func (s *bonusOffhandCommandPathStore) UpdateCombatantHP(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
	s.hpWrites = append(s.hpWrites, arg)
	return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, TempHp: arg.TempHp, IsAlive: arg.IsAlive, Conditions: json.RawMessage(`[]`)}, nil
}

// GetCombatantConcentration is consulted by the ApplyDamage → concentration
// pipeline. The target holds no concentration, so return an empty row.
func (s *bonusOffhandCommandPathStore) GetCombatantConcentration(_ context.Context, _ uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
	return refdata.GetCombatantConcentrationRow{}, nil
}

func TestBonusHandler_OffhandCommandPathConsumesBonusActionAndOmitsDamageMod(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	scores, err := json.Marshal(combat.AbilityScores{Str: 16, Dex: 14, Con: 10, Int: 10, Wis: 10, Cha: 10})
	if err != nil {
		t.Fatalf("marshal ability scores: %v", err)
	}
	attacker := refdata.Combatant{
		ID:          attackerID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		ShortID:     "AR",
		DisplayName: "Aria",
		PositionCol: "A",
		PositionRow: 1,
		HpCurrent:   10,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID:          targetID,
		EncounterID: encounterID,
		ShortID:     "OS",
		DisplayName: "Orc",
		PositionCol: "B",
		PositionRow: 1,
		Ac:          13,
		HpCurrent:   10,
		IsAlive:     true,
		IsNpc:       true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	provider := &mockBonusProvider{
		encID: encounterID,
		enc: refdata.Encounter{
			ID:            encounterID,
			CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
		},
		turn: refdata.Turn{
			ID:          turnID,
			CombatantID: attackerID,
		},
		actor:  attacker,
		target: target,
	}
	store := &bonusOffhandCommandPathStore{
		char: refdata.Character{
			ID:               charID,
			AbilityScores:    scores,
			ProficiencyBonus: 2,
			EquippedMainHand: sql.NullString{String: "shortsword", Valid: true},
			EquippedOffHand:  sql.NullString{String: "dagger", Valid: true},
		},
		attacker: attacker,
		target:   target,
	}
	sess := &mockMoveSession{}
	h := NewBonusHandler(sess, combat.NewService(store), provider, dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 3
	}))

	h.Handle(makeBonusInteraction("offhand", "OS"))

	if len(store.savedTurns) != 1 {
		t.Fatalf("expected one persisted turn update, got %d; response: %q", len(store.savedTurns), sess.lastResponse.Data.Content)
	}
	if !store.savedTurns[0].BonusActionUsed {
		t.Fatal("expected /bonus offhand to consume the bonus action")
	}
	if got := sess.lastResponse.Data.Content; !strings.Contains(got, "Damage: 3 piercing") {
		t.Fatalf("expected off-hand damage to omit STR mod and deal 3 piercing, got %q", got)
	}
}

func TestBonusHandler_OffhandCommandPathRejectsFullCoverBeforeBonusAction(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	mapID := uuid.New()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	scores, err := json.Marshal(combat.AbilityScores{Str: 16, Dex: 14, Con: 10, Int: 10, Wis: 10, Cha: 10})
	if err != nil {
		t.Fatalf("marshal ability scores: %v", err)
	}
	attacker := refdata.Combatant{
		ID:          attackerID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		ShortID:     "AR",
		DisplayName: "Aria",
		PositionCol: "A",
		PositionRow: 3,
		HpCurrent:   10,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID:          targetID,
		EncounterID: encounterID,
		ShortID:     "OS",
		DisplayName: "Orc",
		PositionCol: "D",
		PositionRow: 3,
		Ac:          13,
		HpCurrent:   10,
		IsAlive:     true,
		IsNpc:       true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	provider := &mockBonusProvider{
		encID: encounterID,
		enc: refdata.Encounter{
			ID:            encounterID,
			CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
			MapID:         uuid.NullUUID{UUID: mapID, Valid: true},
		},
		turn: refdata.Turn{
			ID:          turnID,
			CombatantID: attackerID,
		},
		actor:  attacker,
		target: target,
	}
	store := &bonusOffhandCommandPathStore{
		char: refdata.Character{
			ID:               charID,
			AbilityScores:    scores,
			ProficiencyBonus: 2,
			EquippedMainHand: sql.NullString{String: "shortsword", Valid: true},
			EquippedOffHand:  sql.NullString{String: "dagger", Valid: true},
		},
		attacker: attacker,
		target:   target,
	}
	tiled := json.RawMessage(`{
		"height": 5, "width": 5, "tilewidth": 48, "tileheight": 48,
		"layers": [
			{"name": "terrain", "type": "tilelayer", "width": 5, "height": 5,
			 "data": [1,1,1,1,1, 1,1,1,1,1, 1,1,1,1,1, 1,1,1,1,1, 1,1,1,1,1]},
			{"name": "walls", "type": "objectgroup",
			 "objects": [{"x": 96, "y": 0, "width": 0, "height": 240}]}
		],
		"tilesets": [{"firstgid": 1, "name": "base", "tiles": [{"id": 0, "type": "open_ground"}]}]
	}`)
	sess := &mockMoveSession{}
	h := NewBonusHandler(sess, combat.NewService(store), provider, dice.NewRoller(func(max int) int { return max }))
	h.SetMapProvider(&stubAttackMapProvider{mapData: refdata.Map{ID: mapID, TiledJson: tiled}})

	h.Handle(makeBonusInteraction("offhand", "OS"))

	if len(store.savedTurns) != 0 {
		t.Fatalf("expected full-cover rejection before bonus-action persistence, got %d turn updates", len(store.savedTurns))
	}
	if got := sess.lastResponse.Data.Content; !strings.Contains(got, "total cover") {
		t.Fatalf("expected total-cover rejection, got %q", got)
	}
}

func TestBonusHandler_MartialArts(t *testing.T) {
	h, _, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("martial-arts", "OS"))
	if len(svc.martialCalls) != 1 {
		t.Fatalf("expected 1 martial-arts call, got %d", len(svc.martialCalls))
	}
	if svc.martialCalls[0].Target.ShortID != "OS" {
		t.Errorf("expected target OS, got %s", svc.martialCalls[0].Target.ShortID)
	}
}

func TestBonusHandler_Polearm(t *testing.T) {
	h, _, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("polearm", "OS"))
	if len(svc.polearmCalls) != 1 {
		t.Fatalf("expected 1 polearm call, got %d", len(svc.polearmCalls))
	}
	if svc.polearmCalls[0].Target.ShortID != "OS" {
		t.Errorf("expected target OS, got %s", svc.polearmCalls[0].Target.ShortID)
	}
}

func TestBonusHandler_Polearm_MissingTarget(t *testing.T) {
	h, sess, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("polearm", ""))
	if len(svc.polearmCalls) != 0 {
		t.Error("expected no service call without target")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Missing target") {
		t.Errorf("expected missing-target rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestBonusHandler_CrossbowExpert(t *testing.T) {
	h, _, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("crossbow", "OS"))
	if len(svc.crossbowCalls) != 1 {
		t.Fatalf("expected 1 crossbow call, got %d", len(svc.crossbowCalls))
	}
	if svc.crossbowCalls[0].Target.ShortID != "OS" {
		t.Errorf("expected target OS, got %s", svc.crossbowCalls[0].Target.ShortID)
	}
}

func TestBonusHandler_CrossbowExpert_MissingTarget(t *testing.T) {
	h, sess, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("crossbow", ""))
	if len(svc.crossbowCalls) != 0 {
		t.Error("expected no service call without target")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Missing target") {
		t.Errorf("expected missing-target rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestBonusHandler_MartialArts_MissingTarget(t *testing.T) {
	h, sess, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("martial-arts", ""))
	if len(svc.martialCalls) != 0 {
		t.Error("expected no service call without target")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Missing target") {
		t.Errorf("expected missing-target rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestBonusHandler_StepOfTheWind_Dash(t *testing.T) {
	h, _, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("step-of-the-wind", "dash"))
	if len(svc.stepCalls) != 1 {
		t.Fatalf("expected 1 step call, got %d", len(svc.stepCalls))
	}
	if svc.stepCalls[0].Mode != "dash" {
		t.Errorf("expected mode dash, got %s", svc.stepCalls[0].Mode)
	}
}

func TestBonusHandler_StepOfTheWind_InvalidMode(t *testing.T) {
	h, sess, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("step-of-the-wind", "fly"))
	if len(svc.stepCalls) != 0 {
		t.Error("expected no service call for invalid mode")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "dash") {
		t.Errorf("expected mode hint, got %q", sess.lastResponse.Data.Content)
	}
}

func TestBonusHandler_PatientDefense(t *testing.T) {
	h, _, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("patient-defense", ""))
	if len(svc.patientCalls) != 1 {
		t.Fatalf("expected 1 patient defense call, got %d", len(svc.patientCalls))
	}
}

func TestBonusHandler_FontOfMagic_Convert(t *testing.T) {
	h, _, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("font-of-magic", "convert 2"))
	if len(svc.fomConvertCalls) != 1 {
		t.Fatalf("expected 1 fom convert call, got %d", len(svc.fomConvertCalls))
	}
	if svc.fomConvertCalls[0].SlotLevel != 2 {
		t.Errorf("expected slot level 2, got %d", svc.fomConvertCalls[0].SlotLevel)
	}
}

func TestBonusHandler_FontOfMagic_Create(t *testing.T) {
	h, _, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("font-of-magic", "create 3"))
	if len(svc.fomCreateCalls) != 1 {
		t.Fatalf("expected 1 fom create call, got %d", len(svc.fomCreateCalls))
	}
	if svc.fomCreateCalls[0].CreateSlotLevel != 3 {
		t.Errorf("expected create level 3, got %d", svc.fomCreateCalls[0].CreateSlotLevel)
	}
}

func TestBonusHandler_FontOfMagic_BadArgs(t *testing.T) {
	h, sess, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("font-of-magic", "convert"))
	if len(svc.fomConvertCalls) != 0 {
		t.Error("expected no call with missing slot level")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Font of Magic") {
		t.Errorf("expected fom usage hint, got %q", sess.lastResponse.Data.Content)
	}
}

func TestBonusHandler_LayOnHands(t *testing.T) {
	h, _, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("lay-on-hands", "OS 10 poison"))
	if len(svc.layCalls) != 1 {
		t.Fatalf("expected 1 lay call, got %d", len(svc.layCalls))
	}
	got := svc.layCalls[0]
	if got.HP != 10 {
		t.Errorf("expected HP 10, got %d", got.HP)
	}
	if !got.CurePoison {
		t.Error("expected CurePoison=true")
	}
	if got.CureDisease {
		t.Error("expected CureDisease=false")
	}
}

func TestBonusHandler_LayOnHands_BadArgs(t *testing.T) {
	h, sess, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("lay-on-hands", "OS"))
	if len(svc.layCalls) != 0 {
		t.Error("expected no call without HP")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Lay on Hands") {
		t.Errorf("expected lay-on-hands usage hint, got %q", sess.lastResponse.Data.Content)
	}
}

func TestBonusHandler_BardicInspiration(t *testing.T) {
	h, _, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("bardic-inspiration", "OS"))
	if len(svc.bardicCalls) != 1 {
		t.Fatalf("expected 1 bardic call, got %d", len(svc.bardicCalls))
	}
	if svc.bardicCalls[0].Target.ShortID != "OS" {
		t.Errorf("expected target OS, got %s", svc.bardicCalls[0].Target.ShortID)
	}
}

func TestBonusHandler_SecondWind(t *testing.T) {
	h, sess, svc, provider := setupBonusHandler()
	h.Handle(makeBonusInteraction("second-wind", ""))
	if len(svc.secondWindCalls) != 1 {
		t.Fatalf("expected 1 second-wind call, got %d", len(svc.secondWindCalls))
	}
	if svc.secondWindCalls[0].Fighter.ID != provider.actor.ID {
		t.Errorf("expected fighter %s, got %s", provider.actor.ID, svc.secondWindCalls[0].Fighter.ID)
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Second Wind") {
		t.Errorf("expected second-wind log, got %q", sess.lastResponse.Data.Content)
	}
	if sess.lastResponse.Data.Flags&discordgo.MessageFlagsEphemeral != 0 {
		t.Errorf("expected second-wind result to be public, got flags %d", sess.lastResponse.Data.Flags)
	}
}

func TestBonusHandler_UnknownAction(t *testing.T) {
	h, sess, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("yeet", ""))
	// no service call from any branch
	if len(svc.rageCalls)+len(svc.martialCalls) != 0 {
		t.Error("expected no service call for unknown action")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Unknown bonus action") {
		t.Errorf("expected unknown-action rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestBonusHandler_MissingAction(t *testing.T) {
	h, sess, _, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("", ""))
	if !strings.Contains(sess.lastResponse.Data.Content, "specify a bonus action") {
		t.Errorf("expected action prompt, got %q", sess.lastResponse.Data.Content)
	}
}

func TestBonusHandler_TurnGate_RejectsWrongOwner(t *testing.T) {
	h, sess, svc, _ := setupBonusHandler()
	h.SetTurnGate(&stubTurnGate{err: &combat.ErrNotYourTurn{
		CurrentCharacterName: "Bob",
		CurrentDiscordUserID: "u-bob",
	}})
	h.Handle(makeBonusInteraction("rage", ""))
	if len(svc.rageCalls) != 0 {
		t.Error("expected no service call when gate rejects")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Bob") {
		t.Errorf("expected wrong-owner rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestBonusHandler_NoEncounter(t *testing.T) {
	h, sess, _, provider := setupBonusHandler()
	provider.resolveErr = errNoEncounter
	h.Handle(makeBonusInteraction("rage", ""))
	if !strings.Contains(sess.lastResponse.Data.Content, "not in an active encounter") {
		t.Errorf("expected no-encounter error, got %q", sess.lastResponse.Data.Content)
	}
}

func TestBonusHandler_FontOfMagic_UnknownMode(t *testing.T) {
	h, sess, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("font-of-magic", "yeet 2"))
	if len(svc.fomConvertCalls)+len(svc.fomCreateCalls) != 0 {
		t.Error("expected no fom call for unknown mode")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Unknown Font of Magic mode") {
		t.Errorf("expected unknown-mode rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestBonusHandler_FontOfMagic_BadLevel(t *testing.T) {
	h, sess, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("font-of-magic", "convert abc"))
	if len(svc.fomConvertCalls) != 0 {
		t.Error("expected no fom call for non-numeric level")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Invalid slot level") {
		t.Errorf("expected invalid-level rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestBonusHandler_LayOnHands_BadHP(t *testing.T) {
	h, sess, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("lay-on-hands", "OS notanumber"))
	if len(svc.layCalls) != 0 {
		t.Error("expected no lay call for non-numeric HP")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Invalid HP") {
		t.Errorf("expected invalid-HP rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestBonusHandler_LayOnHands_BadTarget(t *testing.T) {
	h, sess, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("lay-on-hands", "ZZZ 5"))
	if len(svc.layCalls) != 0 {
		t.Error("expected no lay call when target missing")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "not found") {
		t.Errorf("expected target-missing rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestBonusHandler_PostsToCombatLog(t *testing.T) {
	h, _, _, _ := setupBonusHandler()
	captured := []string{}
	h.session = &capturingSession{
		mockMoveSession: &mockMoveSession{},
		sendFn: func(channelID, content string) {
			captured = append(captured, channelID+":"+content)
		},
	}
	h.SetChannelIDProvider(&mockDeathSaveCSP{
		fn: func(_ context.Context, _ uuid.UUID) (map[string]string, error) {
			return map[string]string{"combat-log": "ch-cl"}, nil
		},
	})
	h.Handle(makeBonusInteraction("rage", ""))
	if len(captured) != 1 || !strings.HasPrefix(captured[0], "ch-cl:") {
		t.Errorf("expected combat-log post to ch-cl, got %v", captured)
	}
}
