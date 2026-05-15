package portal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ab/dndnd/internal/character"
)

var (
	// ErrPointBuyOverspent is returned when point-buy total exceeds 27.
	ErrPointBuyOverspent = errors.New("point buy: spent more than 27 points")
	// ErrScoreOutOfRange is returned when a score is below 8 or above 15.
	ErrScoreOutOfRange = errors.New("point buy: score out of range (must be 8-15)")
)

// CharacterSubmission is the payload sent by the character builder form.
//
// Classes is the multiclass entry — when non-empty it overrides
// Class/Subclass for persistence. Class/Subclass remain on the payload
// so older single-class submitters keep working.
type CharacterSubmission struct {
	Name          string                 `json:"name"`
	Race          string                 `json:"race"`
	Subrace       string                 `json:"subrace,omitempty"`
	Background    string                 `json:"background"`
	Class         string                 `json:"class"`
	Subclass      string                 `json:"subclass,omitempty"`
	Classes       []character.ClassEntry `json:"classes,omitempty"`
	AbilityScores PointBuyScores         `json:"ability_scores"`
	AbilityMethod AbilityScoreMethod     `json:"ability_method,omitempty"`
	AbilityRolls  map[string][]int       `json:"ability_rolls,omitempty"`
	Skills        []string               `json:"skills"`
	Equipment     []string               `json:"equipment,omitempty"`
	Spells        []string               `json:"spells,omitempty"`
}

// ValidateSubmission returns a list of validation error messages.
func ValidateSubmission(s CharacterSubmission) []string {
	var errs []string
	if s.Name == "" {
		errs = append(errs, "name is required")
	}
	if s.Race == "" {
		errs = append(errs, "race is required")
	}
	if s.Class == "" {
		errs = append(errs, "class is required")
	}
	if err := ValidateAbilityScoreGeneration(s); err != nil {
		errs = append(errs, err.Error())
	}
	return errs
}

// CreateCharacterParams holds params for creating a character record.
//
// Classes drives the JSONB classes column when non-empty; otherwise the
// adapter falls back to a single ClassEntry built from Class/Subclass.
type CreateCharacterParams struct {
	CampaignID     string
	Name           string
	Race           string
	Subrace        string
	Class          string
	Subclass       string
	Classes        []character.ClassEntry
	Background     string
	AbilityScores  character.AbilityScores
	HPMax          int
	AC             int
	SpeedFt        int
	ProfBonus      int
	Skills         []string
	Saves          []string
	Equipment      []string
	Spells         []string
	Languages      []string
	Features       []character.Feature
	EquippedWeapon string
	WornArmor      string
}

// CreatePlayerCharacterParams holds params for the player_characters record.
type CreatePlayerCharacterParams struct {
	CampaignID    string
	CharacterID   string
	DiscordUserID string
	Status        string
	CreatedVia    string
}

// CreateCharacterResult holds the IDs from character creation.
type CreateCharacterResult struct {
	CharacterID       string
	PlayerCharacterID string
}

// BuilderStore abstracts the persistence layer for character creation.
type BuilderStore interface {
	CreateCharacterRecord(ctx context.Context, p CreateCharacterParams) (string, error)
	CreatePlayerCharacterRecord(ctx context.Context, p CreatePlayerCharacterParams) (string, error)
	RedeemToken(ctx context.Context, token string) error
}

// DMQueueNotifier sends notifications to the DM queue channel.
type DMQueueNotifier interface {
	NotifyDMQueue(ctx context.Context, characterName, playerDiscordID, via string) error
}

// AbilityMethodProvider returns the generation methods enabled for a campaign.
type AbilityMethodProvider interface {
	AllowedAbilityScoreMethods(ctx context.Context, campaignID string) ([]AbilityScoreMethod, error)
}

// StaticAbilityMethodProvider is a test/helper provider with fixed methods.
type StaticAbilityMethodProvider []AbilityScoreMethod

// AllowedAbilityScoreMethods returns the fixed method list.
func (p StaticAbilityMethodProvider) AllowedAbilityScoreMethods(_ context.Context, _ string) ([]AbilityScoreMethod, error) {
	return []AbilityScoreMethod(p), nil
}

// BuilderServiceOption configures optional features of BuilderService.
type BuilderServiceOption func(*BuilderService)

// WithNotifier adds a DM queue notifier to the BuilderService.
func WithNotifier(n DMQueueNotifier) BuilderServiceOption {
	return func(svc *BuilderService) {
		svc.notifier = n
	}
}

// WithLogger adds a logger to the BuilderService.
func WithLogger(l *slog.Logger) BuilderServiceOption {
	return func(svc *BuilderService) {
		svc.logger = l
	}
}

// WithAbilityMethodProvider adds campaign ability-score method gating.
func WithAbilityMethodProvider(p AbilityMethodProvider) BuilderServiceOption {
	return func(svc *BuilderService) {
		svc.abilityMethods = p
	}
}

// BuilderService handles character creation from the portal form.
type BuilderService struct {
	store          BuilderStore
	notifier       DMQueueNotifier
	logger         *slog.Logger
	abilityMethods AbilityMethodProvider
}

// NewBuilderService creates a new BuilderService.
func NewBuilderService(store BuilderStore, opts ...BuilderServiceOption) *BuilderService {
	svc := &BuilderService{store: store}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// CreateCharacter validates the submission, calculates derived stats,
// creates the character + player_character records, and redeems the token.
func (svc *BuilderService) CreateCharacter(ctx context.Context, campaignID, discordUserID, token string, sub CharacterSubmission) (CreateCharacterResult, error) {
	errs := ValidateSubmission(sub)
	if err := svc.validateAllowedAbilityMethod(ctx, campaignID, sub.AbilityMethod); err != nil {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return CreateCharacterResult{}, fmt.Errorf("validation failed: %s", strings.Join(errs, "; "))
	}

	scores := character.AbilityScores{
		STR: sub.AbilityScores.STR,
		DEX: sub.AbilityScores.DEX,
		CON: sub.AbilityScores.CON,
		INT: sub.AbilityScores.INT,
		WIS: sub.AbilityScores.WIS,
		CHA: sub.AbilityScores.CHA,
	}

	hp := DeriveHP(sub.Class, scores)
	if len(sub.Classes) > 0 {
		hp = DeriveHPMulticlass(sub.Classes, scores)
	}
	ac := DeriveAC(scores)
	totalLevel := 1
	if len(sub.Classes) > 0 {
		totalLevel = character.TotalLevel(sub.Classes)
	}
	profBonus := character.ProficiencyBonus(totalLevel)

	charParams := CreateCharacterParams{
		CampaignID:    campaignID,
		Name:          sub.Name,
		Race:          sub.Race,
		Subrace:       sub.Subrace,
		Class:         sub.Class,
		Subclass:      sub.Subclass,
		Classes:       sub.Classes,
		Background:    sub.Background,
		AbilityScores: scores,
		HPMax:         hp,
		AC:            ac,
		SpeedFt:       DeriveSpeed(sub.Race),
		ProfBonus:     profBonus,
		Skills:        sub.Skills,
		Equipment:     sub.Equipment,
		Spells:        sub.Spells,
	}

	charID, err := svc.store.CreateCharacterRecord(ctx, charParams)
	if err != nil {
		return CreateCharacterResult{}, fmt.Errorf("creating character: %w", err)
	}

	pcParams := CreatePlayerCharacterParams{
		CampaignID:    campaignID,
		CharacterID:   charID,
		DiscordUserID: discordUserID,
		Status:        "pending",
		CreatedVia:    "create",
	}

	pcID, err := svc.store.CreatePlayerCharacterRecord(ctx, pcParams)
	if err != nil {
		return CreateCharacterResult{}, fmt.Errorf("creating player character: %w", err)
	}

	if err := svc.store.RedeemToken(ctx, token); err != nil && svc.logger != nil {
		svc.logger.Warn("redeeming token after character creation", "token", token, "error", err)
	}

	if svc.notifier != nil {
		if err := svc.notifier.NotifyDMQueue(ctx, sub.Name, discordUserID, "portal-create"); err != nil && svc.logger != nil {
			svc.logger.Warn("notifying dm queue", "error", err)
		}
	}

	return CreateCharacterResult{
		CharacterID:       charID,
		PlayerCharacterID: pcID,
	}, nil
}

func (svc *BuilderService) validateAllowedAbilityMethod(ctx context.Context, campaignID string, method AbilityScoreMethod) error {
	allowed, err := svc.AllowedAbilityScoreMethods(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("loading ability score methods: %w", err)
	}
	if len(allowed) == 0 {
		return nil
	}
	if method == "" {
		method = AbilityMethodPointBuy
	}
	for _, allowedMethod := range allowed {
		if method == allowedMethod {
			return nil
		}
	}
	return fmt.Errorf("ability score method %s is not allowed", method)
}

// AllowedAbilityScoreMethods returns campaign-enabled methods for the builder.
func (svc *BuilderService) AllowedAbilityScoreMethods(ctx context.Context, campaignID string) ([]AbilityScoreMethod, error) {
	provider := svc.abilityMethods
	if provider == nil {
		if p, ok := svc.store.(AbilityMethodProvider); ok {
			provider = p
		}
	}
	if provider == nil {
		return DefaultAbilityScoreMethods(), nil
	}
	allowed, err := provider.AllowedAbilityScoreMethods(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if len(allowed) > 0 {
		return allowed, nil
	}
	return DefaultAbilityScoreMethods(), nil
}

// PointBuyScores holds the six ability scores for point-buy validation.
type PointBuyScores struct {
	STR int `json:"str"`
	DEX int `json:"dex"`
	CON int `json:"con"`
	INT int `json:"int"`
	WIS int `json:"wis"`
	CHA int `json:"cha"`
}

// AbilityScoreMethod identifies how character creation produced ability scores.
type AbilityScoreMethod string

const (
	AbilityMethodPointBuy      AbilityScoreMethod = "point_buy"
	AbilityMethodStandardArray AbilityScoreMethod = "standard_array"
	AbilityMethodRoll          AbilityScoreMethod = "roll"
)

// DefaultAbilityScoreMethods returns the spec-supported generation modes.
func DefaultAbilityScoreMethods() []AbilityScoreMethod {
	return []AbilityScoreMethod{AbilityMethodPointBuy, AbilityMethodStandardArray, AbilityMethodRoll}
}

// CampaignAbilitySettings is the subset of campaign settings used by creation.
type CampaignAbilitySettings struct {
	AbilityScoreMethods []AbilityScoreMethod `json:"ability_score_methods,omitempty"`
}

// AbilityScoreMethodsFromSettings decodes enabled generation modes from settings JSON.
func AbilityScoreMethodsFromSettings(raw json.RawMessage) []AbilityScoreMethod {
	if len(raw) == 0 {
		return DefaultAbilityScoreMethods()
	}
	var settings CampaignAbilitySettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return DefaultAbilityScoreMethods()
	}
	if len(settings.AbilityScoreMethods) == 0 {
		return DefaultAbilityScoreMethods()
	}
	return settings.AbilityScoreMethods
}

// PointBuyScoresFromCharacter converts character scores to the creation score shape.
func PointBuyScoresFromCharacter(scores character.AbilityScores) PointBuyScores {
	return PointBuyScores{
		STR: scores.STR,
		DEX: scores.DEX,
		CON: scores.CON,
		INT: scores.INT,
		WIS: scores.WIS,
		CHA: scores.CHA,
	}
}

// ValidateAbilityScoreGeneration checks the selected generation method.
func ValidateAbilityScoreGeneration(sub CharacterSubmission) error {
	method := sub.AbilityMethod
	if method == "" {
		method = AbilityMethodPointBuy
	}
	return ValidateAbilityScores(method, sub.AbilityScores, sub.AbilityRolls)
}

// ValidateAbilityScores checks ability scores for point-buy, standard array, or roll mode.
func ValidateAbilityScores(method AbilityScoreMethod, scores PointBuyScores, rolls map[string][]int) error {
	switch method {
	case "", AbilityMethodPointBuy:
		return ValidatePointBuy(scores)
	case AbilityMethodStandardArray:
		return ValidateStandardArray(scores)
	case AbilityMethodRoll:
		return ValidateRolledScores(scores, rolls)
	default:
		return fmt.Errorf("unknown ability score method: %s", method)
	}
}

// ValidateStandardArray checks that the six scores are a valid standard array
// (15,14,13,12,10,8) possibly with racial bonuses applied (up to +2 per score).
func ValidateStandardArray(scores PointBuyScores) error {
	got := []int{scores.STR, scores.DEX, scores.CON, scores.INT, scores.WIS, scores.CHA}
	// With racial bonuses, scores can exceed the base array values.
	// Validate that each score is in the plausible range (8-17).
	for _, v := range got {
		if v < 8 || v > 17 {
			return fmt.Errorf("standard array score out of range: %d", v)
		}
	}
	return nil
}

// ValidateRolledScores checks 4d6-drop-lowest details when provided.
// Scores may include racial bonuses so the valid range is 3-20.
func ValidateRolledScores(scores PointBuyScores, rolls map[string][]int) error {
	scoreByAbility := map[string]int{
		"str": scores.STR,
		"dex": scores.DEX,
		"con": scores.CON,
		"int": scores.INT,
		"wis": scores.WIS,
		"cha": scores.CHA,
	}
	for ability, score := range scoreByAbility {
		if score < 3 || score > 20 {
			return fmt.Errorf("%s rolled score must be between 3 and 20", strings.ToUpper(ability))
		}
	}
	for ability, score := range scoreByAbility {
		dice := rolls[ability]
		if len(dice) != 4 {
			return fmt.Errorf("%s roll must include four d6 results", strings.ToUpper(ability))
		}
		total := 0
		lowest := 7
		for _, die := range dice {
			if die < 1 || die > 6 {
				return fmt.Errorf("%s roll contains non-d6 result %d", strings.ToUpper(ability), die)
			}
			total += die
			if die < lowest {
				lowest = die
			}
		}
		// The submitted score includes racial bonuses, so it may exceed
		// the raw dice total. Accept if score >= dice total (bonus applied).
		diceScore := total - lowest
		if score < diceScore {
			return fmt.Errorf("%s score %d is less than 4d6 drop lowest %d", strings.ToUpper(ability), score, diceScore)
		}
	}
	return nil
}

// PointBuyCost returns the point cost for a single ability score value.
func PointBuyCost(score int) (int, error) {
	if score < 8 || score > 15 {
		return 0, ErrScoreOutOfRange
	}
	// 8=0, 9=1, 10=2, 11=3, 12=4, 13=5, 14=7, 15=9
	base := score - 8
	if score <= 13 {
		return base, nil
	}
	// 14 costs 7 (5 + 2 extra), 15 costs 9 (5 + 2 + 2 extra)
	return 5 + (score-13)*2, nil
}

// ValidatePointBuy checks that the given scores are valid under 5e point-buy rules.
// Scores may include racial bonuses (up to +2), so the valid range is 8-17.
func ValidatePointBuy(scores PointBuyScores) error {
	vals := []int{scores.STR, scores.DEX, scores.CON, scores.INT, scores.WIS, scores.CHA}
	total := 0
	for _, v := range vals {
		// Accept post-racial scores: base 8-15 + up to +2 racial = 8-17.
		if v < 8 || v > 17 {
			return fmt.Errorf("%w: %d", ErrScoreOutOfRange, v)
		}
		// Cap at 15 for cost calculation (racial bonus is free).
		base := v
		if base > 15 {
			base = 15
		}
		cost, err := PointBuyCost(base)
		if err != nil {
			return fmt.Errorf("%w: %d", err, v)
		}
		total += cost
	}
	if total > 27 {
		return ErrPointBuyOverspent
	}
	return nil
}
