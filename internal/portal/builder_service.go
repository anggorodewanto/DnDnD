package portal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
)

var (
	// ErrPointBuyOverspent is returned when point-buy total exceeds 27.
	ErrPointBuyOverspent = errors.New("point buy: spent more than 27 points")
	// ErrScoreOutOfRange is returned when a score is below 8 or above 15.
	ErrScoreOutOfRange = errors.New("point buy: score out of range (must be 8-15)")
	// ErrTokenOwnership is returned when the token does not belong to the user.
	ErrTokenOwnership = errors.New("token does not belong to this user")
	// ErrAlreadyActive is returned when a player submits a built character but
	// already has an approved (active) character in the campaign. They must
	// /retire it first; we never clobber an approved character.
	ErrAlreadyActive = errors.New("you already have an active character in this campaign — retire it first")
)

// CharacterSubmission is the payload sent by the character builder form.
//
// Classes is the multiclass entry — when non-empty it overrides
// Class/Subclass for persistence. Class/Subclass remain on the payload
// so older single-class submitters keep working.
type CharacterSubmission struct {
	Name            string                 `json:"name"`
	Race            string                 `json:"race"`
	Subrace         string                 `json:"subrace,omitempty"`
	Background      string                 `json:"background"`
	Class           string                 `json:"class"`
	Subclass        string                 `json:"subclass,omitempty"`
	Classes         []character.ClassEntry `json:"classes,omitempty"`
	AbilityScores   PointBuyScores         `json:"ability_scores"`
	AbilityMethod   AbilityScoreMethod     `json:"ability_method,omitempty"`
	AbilityRolls    map[string][]int       `json:"ability_rolls,omitempty"`
	Skills          []string               `json:"skills"`
	Equipment       []string               `json:"equipment,omitempty"`
	Spells          []string               `json:"spells,omitempty"`
	WeaponMasteries []string               `json:"weapon_masteries,omitempty"`
	Languages       []string               `json:"languages,omitempty"`
	EquippedWeapon  string                 `json:"equipped_weapon,omitempty"`
	WornArmor       string                 `json:"worn_armor,omitempty"`
}

// CreateMode selects the character-creation workflow.
//
// ModePlayer is the self-serve portal flow: a token is validated and
// redeemed, the player_character row is created "pending" for DM approval,
// and the DM queue is notified.
//
// ModeDM is the dashboard flow: the DM creates the character directly with
// no token, the player_character row is "approved" with no linked player
// (a player claims it later via /register), and no DM-queue notification.
type CreateMode int

const (
	ModePlayer CreateMode = iota
	ModeDM
)

// ValidateSubmission returns a list of validation error messages using the
// player-mode rules (ability scores must satisfy the chosen generation method,
// defaulting to point-buy).
func ValidateSubmission(s CharacterSubmission) []string {
	return ValidateSubmissionMode(s, ModePlayer)
}

// ValidateSubmissionMode validates a submission for the given creation mode.
//
// Structural checks (name, race, class, multiclass entries) are identical for
// both modes. Ability-score checks differ: player mode always enforces the
// generation method (point-buy by default), while DM mode only range-checks
// scores (1-30) and validates the method when one is explicitly chosen — DMs
// build arbitrary stat blocks that need not satisfy point-buy.
func ValidateSubmissionMode(s CharacterSubmission, mode CreateMode) []string {
	var errs []string
	if s.Name == "" {
		errs = append(errs, "name is required")
	}
	if s.Race == "" {
		errs = append(errs, "race is required")
	}
	if s.Class == "" && !hasNonEmptyClass(s.Classes) {
		errs = append(errs, "class is required")
	}
	errs = append(errs, validateClassEntries(s.Classes)...)
	errs = append(errs, validateAbilityForMode(s, mode)...)
	errs = append(errs, validateSpellCount(s)...)
	return errs
}

// validateSpellCount enforces the 5e prepared/known spell-count cap on a
// caster submission. The cap is a rules constraint (not mode-specific), so it
// applies to both player and DM submissions. Non-casters and submissions with
// no spells pass unchanged.
func validateSpellCount(s CharacterSubmission) []string {
	maxSpells, ok := spellCountCap(s)
	if !ok {
		return nil
	}
	if len(s.Spells) <= maxSpells {
		return nil
	}
	return []string{fmt.Sprintf("too many spells selected: %d chosen, maximum %d", len(s.Spells), maxSpells)}
}

// spellCountCap returns the maximum number of spells the submission's primary
// class may have. The second return value is false when no cap applies: either
// there is no primary class or the primary class is not a spellcaster.
//
// The cap mirrors combat.MaxPreparedSpells (spellcasting-ability modifier +
// primary class level, minimum 1).
func spellCountCap(s CharacterSubmission) (int, bool) {
	primary := primaryClassEntry(SubmissionClasses(s))
	if primary == nil {
		return 0, false
	}
	sc := classSpellcasting(primary.Class)
	if sc.SlotProgression == "none" {
		return 0, false
	}
	score := s.AbilityScores.Character().Get(sc.SpellAbility)
	abilityMod := character.AbilityModifier(score)
	return combat.MaxPreparedSpells(abilityMod, primary.Level), true
}

// hasNonEmptyClass reports whether any multiclass entry names a class.
func hasNonEmptyClass(classes []character.ClassEntry) bool {
	for _, c := range classes {
		if c.Class != "" {
			return true
		}
	}
	return false
}

// validateClassEntries checks the multiclass list: each entry must name a
// class, levels must be at least 1, classes must be unique, and the combined
// level cannot exceed 20. An empty list is valid (legacy single-class path).
func validateClassEntries(classes []character.ClassEntry) []string {
	if len(classes) == 0 {
		return nil
	}
	var errs []string
	totalLevel := 0
	seen := make(map[string]bool, len(classes))
	for i, c := range classes {
		if c.Class == "" {
			errs = append(errs, fmt.Sprintf("class entry %d: class name is required", i+1))
		} else if seen[strings.ToLower(c.Class)] {
			errs = append(errs, fmt.Sprintf("class entry %d: duplicate class %q", i+1, c.Class))
		} else {
			seen[strings.ToLower(c.Class)] = true
		}
		if c.Level < 1 {
			errs = append(errs, fmt.Sprintf("class entry %d: level must be at least 1", i+1))
		}
		totalLevel += c.Level
	}
	if totalLevel > 20 {
		errs = append(errs, "total level cannot exceed 20")
	}
	return errs
}

// validateAbilityForMode applies the mode-specific ability-score rules.
func validateAbilityForMode(s CharacterSubmission, mode CreateMode) []string {
	if mode == ModePlayer {
		if err := ValidateAbilityScoreGeneration(s); err != nil {
			return []string{err.Error()}
		}
		return nil
	}

	var errs []string
	abilities := []struct {
		name  string
		value int
	}{
		{"STR", s.AbilityScores.STR}, {"DEX", s.AbilityScores.DEX}, {"CON", s.AbilityScores.CON},
		{"INT", s.AbilityScores.INT}, {"WIS", s.AbilityScores.WIS}, {"CHA", s.AbilityScores.CHA},
	}
	for _, a := range abilities {
		if a.value < 1 || a.value > 30 {
			errs = append(errs, fmt.Sprintf("%s must be between 1 and 30", a.name))
		}
	}
	if s.AbilityMethod != "" {
		if err := ValidateAbilityScores(s.AbilityMethod, s.AbilityScores, s.AbilityRolls); err != nil {
			errs = append(errs, err.Error())
		}
	}
	return errs
}

// CreateCharacterParams holds params for creating a character record.
//
// Classes drives the JSONB classes column when non-empty; otherwise the
// adapter falls back to a single ClassEntry built from Class/Subclass.
type CreateCharacterParams struct {
	CampaignID      string
	Name            string
	Race            string
	Subrace         string
	Class           string
	Subclass        string
	Classes         []character.ClassEntry
	Background      string
	AbilityScores   character.AbilityScores
	HPMax           int
	AC              int
	SpeedFt         int
	ProfBonus       int
	Skills          []string
	Saves           []string
	Equipment       []string
	Spells          []string
	WeaponMasteries []string
	Languages       []string
	Features        []character.Feature
	EquippedWeapon  string
	WornArmor       string
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

// ActivePlayerCharacter is the minimal view of an existing non-retired
// player_characters row used to decide whether a submission inserts a new row
// or re-links the existing one.
type ActivePlayerCharacter struct {
	ID     string
	Status string
}

// BuilderStore abstracts the persistence layer for character creation.
type BuilderStore interface {
	CreateCharacterRecord(ctx context.Context, p CreateCharacterParams) (string, error)
	CreatePlayerCharacterRecord(ctx context.Context, p CreatePlayerCharacterParams) (string, error)
	// ActivePlayerCharacter returns the non-retired player_characters row for
	// the (campaign, player), or (nil, nil) when none exists.
	ActivePlayerCharacter(ctx context.Context, campaignID, discordUserID string) (*ActivePlayerCharacter, error)
	// RelinkPlayerCharacterRecord re-points an existing row at a freshly built
	// character and resets it to pending. Returns the row's ID.
	RelinkPlayerCharacterRecord(ctx context.Context, pcID, characterID, createdVia string) (string, error)
	ValidateToken(ctx context.Context, token string) (*PortalToken, error)
	RedeemToken(ctx context.Context, token string) error
	// SaveCharacterDraft upserts the in-progress builder draft blob for
	// (campaign, player, mode). The draft is opaque JSON owned by the client.
	SaveCharacterDraft(ctx context.Context, campaignID, discordUserID, mode string, draft json.RawMessage) error
	// LoadCharacterDraft returns the stored draft blob for (campaign, player,
	// mode), or (nil, nil) when none exists.
	LoadCharacterDraft(ctx context.Context, campaignID, discordUserID, mode string) (json.RawMessage, error)
}

// DMQueueNotifier sends notifications to the DM queue channel. campaignID is
// included so a multi-guild deployment can resolve which guild's #dm-queue to
// post to (the builder only knows the campaign, not the Discord guild).
type DMQueueNotifier interface {
	NotifyDMQueue(ctx context.Context, campaignID, characterName, playerDiscordID, via string) error
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

// WithFeatureProvider supplies class/subclass/racial features so created
// characters (and previews) carry their feature list.
func WithFeatureProvider(fp FeatureProvider) BuilderServiceOption {
	return func(svc *BuilderService) {
		svc.featureProvider = fp
	}
}

// WithRaceSpeedLookup supplies a DB-backed race→speed map used by stat
// derivation, overriding the hardcoded table.
func WithRaceSpeedLookup(fn func(ctx context.Context) map[string]int) BuilderServiceOption {
	return func(svc *BuilderService) {
		svc.raceSpeedFn = fn
	}
}

// BuilderService handles character creation for both the player portal and
// the DM dashboard. The workflow is selected per call via CreateMode.
type BuilderService struct {
	store           BuilderStore
	notifier        DMQueueNotifier
	logger          *slog.Logger
	abilityMethods  AbilityMethodProvider
	featureProvider FeatureProvider
	raceSpeedFn     func(ctx context.Context) map[string]int
}

// SetFeatureProvider wires the feature provider after construction.
func (svc *BuilderService) SetFeatureProvider(fp FeatureProvider) {
	svc.featureProvider = fp
}

// racialTraits returns the parsed trait list for a race, or nil when no
// feature provider is wired. Used by skill-proficiency validation to resolve
// race fixed/choose skill grants.
func (svc *BuilderService) racialTraits(race string) []character.Feature {
	if svc.featureProvider == nil {
		return nil
	}
	return svc.featureProvider.RacialTraits(race)
}

// SetRaceSpeedLookup wires the race→speed lookup after construction.
func (svc *BuilderService) SetRaceSpeedLookup(fn func(ctx context.Context) map[string]int) {
	svc.raceSpeedFn = fn
}

// NewBuilderService creates a new BuilderService.
func NewBuilderService(store BuilderStore, opts ...BuilderServiceOption) *BuilderService {
	svc := &BuilderService{store: store}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// CreateCharacter runs the player-portal creation flow: it validates token
// ownership, creates a "pending" character for DM approval, redeems the token,
// and notifies the DM queue.
func (svc *BuilderService) CreateCharacter(ctx context.Context, campaignID, discordUserID, token string, sub CharacterSubmission) (CreateCharacterResult, error) {
	return svc.create(ctx, createInput{
		campaignID:    campaignID,
		discordUserID: discordUserID,
		token:         token,
		sub:           sub,
		mode:          ModePlayer,
	})
}

// CreateCharacterDM runs the dashboard creation flow: the DM creates the
// character directly (no token), pre-approved, with no linked player.
func (svc *BuilderService) CreateCharacterDM(ctx context.Context, campaignID string, sub CharacterSubmission) (CreateCharacterResult, error) {
	return svc.create(ctx, createInput{
		campaignID: campaignID,
		sub:        sub,
		mode:       ModeDM,
	})
}

// SaveDraft persists the player's in-progress builder draft so the "request
// changes" cycle and a cross-device resume can rehydrate the form instead of
// starting blank (T11 / Finding 4·b). An empty blob is a no-op: there is
// nothing to preserve and we avoid writing an empty row. Callers treat this as
// best-effort — a save failure must never block the submit it piggybacks on.
func (svc *BuilderService) SaveDraft(ctx context.Context, campaignID, discordUserID, mode string, draft json.RawMessage) error {
	if len(draft) == 0 {
		return nil
	}
	return svc.store.SaveCharacterDraft(ctx, campaignID, discordUserID, mode, draft)
}

// LoadDraft returns the stored builder draft blob for (campaign, player, mode),
// or (nil, nil) when none exists. An empty campaignID yields no draft rather
// than an error, so a malformed request degrades to a blank form.
func (svc *BuilderService) LoadDraft(ctx context.Context, campaignID, discordUserID, mode string) (json.RawMessage, error) {
	if campaignID == "" {
		return nil, nil
	}
	return svc.store.LoadCharacterDraft(ctx, campaignID, discordUserID, mode)
}

// createInput bundles the per-call parameters for the unified creation core.
type createInput struct {
	campaignID    string
	discordUserID string
	token         string
	sub           CharacterSubmission
	mode          CreateMode
}

// create is the unified creation core shared by the player and DM flows.
func (svc *BuilderService) create(ctx context.Context, in createInput) (CreateCharacterResult, error) {
	sub := in.sub
	if in.mode == ModeDM && sub.AbilityMethod == "" && svc.hasAbilityMethodProvider() {
		sub.AbilityMethod = AbilityMethodPointBuy
	}
	// Back-fill the legacy single-class fields from the primary multiclass
	// entry so the stored record always names a primary class even when the
	// builder only sent the Classes array (the DM dashboard does this).
	if sub.Class == "" {
		if primary := primaryClassEntry(SubmissionClasses(sub)); primary != nil {
			sub.Class = primary.Class
			sub.Subclass = primary.Subclass
		}
	}

	errs := ValidateSubmissionMode(sub, in.mode)
	if err := svc.validateAllowedAbilityMethod(ctx, in.campaignID, sub.AbilityMethod); err != nil {
		errs = append(errs, err.Error())
	}
	// Reject illegal client-submitted skill sets (e.g. a crafted POST claiming
	// all 18 skills). Race grants come from the feature provider when wired;
	// when it is absent race fixed/choose budgets resolve to ∅/0.
	if err := validateSubmittedSkills(sub, svc.racialTraits(sub.Race)); err != nil {
		errs = append(errs, err.Error())
	}
	// A missing campaign_id (both modes) or token (player flow) is a bad
	// request, not a server error: reject it here so the handler returns 400
	// instead of letting a token lookup miss or campaign_id parse failure
	// surface as a generic 500.
	if in.campaignID == "" {
		errs = append(errs, "campaign_id is required")
	}
	if in.mode == ModePlayer && in.token == "" {
		errs = append(errs, "token is required")
	}
	if len(errs) > 0 {
		return CreateCharacterResult{}, fmt.Errorf("validation failed: %s", strings.Join(errs, "; "))
	}

	stats := DeriveStats(sub, svc.lookupRaceSpeeds(ctx))

	var features []character.Feature
	if svc.featureProvider != nil {
		features = CollectFeatures(
			SubmissionClasses(sub),
			svc.featureProvider.ClassFeatures(),
			svc.featureProvider.SubclassFeatures(),
			svc.featureProvider.RacialTraits(sub.Race),
		)
	}

	// Honour explicitly chosen skills (the builder UI lets the player/DM
	// pick them); fall back to the class+background defaults when none were
	// submitted so DM-created characters still get sensible proficiencies.
	skills := sub.Skills
	if len(skills) == 0 {
		skills = append(classSkillProficiencies(SubmissionClasses(sub)), backgroundSkillProficiencies(sub.Background)...)
	}

	charParams := CreateCharacterParams{
		CampaignID:      in.campaignID,
		Name:            sub.Name,
		Race:            sub.Race,
		Subrace:         sub.Subrace,
		Class:           sub.Class,
		Subclass:        sub.Subclass,
		Classes:         sub.Classes,
		Background:      sub.Background,
		AbilityScores:   sub.AbilityScores.Character(),
		HPMax:           stats.HPMax,
		AC:              stats.AC,
		SpeedFt:         stats.SpeedFt,
		ProfBonus:       stats.ProficiencyBonus,
		Skills:          skills,
		Saves:           stats.SaveProficiencies,
		Equipment:       sub.Equipment,
		Spells:          sub.Spells,
		WeaponMasteries: sub.WeaponMasteries,
		Languages:       sub.Languages,
		Features:        features,
		EquippedWeapon:  sub.EquippedWeapon,
		WornArmor:       sub.WornArmor,
	}

	if in.mode == ModePlayer {
		// Validate token ownership before creating character.
		tok, err := svc.store.ValidateToken(ctx, in.token)
		if err != nil {
			return CreateCharacterResult{}, fmt.Errorf("validating token: %w", err)
		}
		if tok != nil && tok.DiscordUserID != in.discordUserID {
			return CreateCharacterResult{}, ErrTokenOwnership
		}
		// Reject an already-approved player BEFORE creating the character
		// record or redeeming the token (Finding 4·d). A player approved
		// mid-build would otherwise burn their token on the 409, and the
		// retry would 500 with "token already used". persistPlayerCharacter
		// re-checks (and relinks pending rows), so this only short-circuits
		// the approved case.
		existing, err := svc.store.ActivePlayerCharacter(ctx, in.campaignID, in.discordUserID)
		if err != nil {
			return CreateCharacterResult{}, fmt.Errorf("checking existing player character: %w", err)
		}
		if existing != nil && existing.Status == "approved" {
			return CreateCharacterResult{}, ErrAlreadyActive
		}
	}

	charID, err := svc.store.CreateCharacterRecord(ctx, charParams)
	if err != nil {
		return CreateCharacterResult{}, fmt.Errorf("creating character: %w", err)
	}

	if in.mode == ModePlayer {
		// Redeem token AFTER character creation succeeds to avoid consuming
		// the token when creation fails (H-M06).
		if err := svc.store.RedeemToken(ctx, in.token); err != nil {
			return CreateCharacterResult{}, fmt.Errorf("redeeming token: %w", err)
		}
	}

	pcID, err := svc.persistPlayerCharacter(ctx, in, charID)
	if err != nil {
		return CreateCharacterResult{}, err
	}

	if in.mode == ModePlayer && svc.notifier != nil {
		if err := svc.notifier.NotifyDMQueue(ctx, in.campaignID, sub.Name, in.discordUserID, "portal-create"); err != nil && svc.logger != nil {
			svc.logger.Warn("notifying dm queue", "error", err)
		}
	}

	return CreateCharacterResult{
		CharacterID:       charID,
		PlayerCharacterID: pcID,
	}, nil
}

// playerCharacterParams builds the player_characters row for the given mode.
// Player submissions are "pending" and linked to the submitter; DM creations
// are "approved" with no linked player (claimed later via /register).
func playerCharacterParams(in createInput, charID string) CreatePlayerCharacterParams {
	if in.mode == ModeDM {
		return CreatePlayerCharacterParams{
			CampaignID:  in.campaignID,
			CharacterID: charID,
			Status:      "approved",
			CreatedVia:  "dm_dashboard",
		}
	}
	return CreatePlayerCharacterParams{
		CampaignID:    in.campaignID,
		CharacterID:   charID,
		DiscordUserID: in.discordUserID,
		Status:        "pending",
		CreatedVia:    "create",
	}
}

// persistPlayerCharacter writes the player_characters row for a submission.
//
// For player-portal submissions it first looks for an existing non-retired row
// for the (campaign, player): an approved one is never overwritten (returns
// ErrAlreadyActive — the player must /retire first), while a pending /
// changes_requested / rejected row is re-linked to the freshly built character
// and reset to pending. This makes resubmits and resumed builds reuse the row
// instead of INSERTing a second one, which the partial unique index forbids.
// DM-mode creations always insert (they carry no discord_user_id).
func (svc *BuilderService) persistPlayerCharacter(ctx context.Context, in createInput, charID string) (string, error) {
	if in.mode == ModePlayer {
		existing, err := svc.store.ActivePlayerCharacter(ctx, in.campaignID, in.discordUserID)
		if err != nil {
			return "", fmt.Errorf("checking existing player character: %w", err)
		}
		if existing != nil {
			if existing.Status == "approved" {
				return "", ErrAlreadyActive
			}
			pcID, err := svc.store.RelinkPlayerCharacterRecord(ctx, existing.ID, charID, "create")
			if err != nil {
				return "", fmt.Errorf("relinking player character: %w", err)
			}
			return pcID, nil
		}
	}
	pcID, err := svc.store.CreatePlayerCharacterRecord(ctx, playerCharacterParams(in, charID))
	if err != nil {
		return "", fmt.Errorf("creating player character: %w", err)
	}
	return pcID, nil
}

// Preview computes derived stats (and features, when a provider is wired)
// for a submission without persisting anything. It is shared by the player
// portal and DM dashboard preview endpoints.
func (svc *BuilderService) Preview(ctx context.Context, sub CharacterSubmission) DerivedStats {
	stats := DeriveStats(sub, svc.lookupRaceSpeeds(ctx))
	if svc.featureProvider != nil {
		stats.Features = CollectFeatures(
			SubmissionClasses(sub),
			svc.featureProvider.ClassFeatures(),
			svc.featureProvider.SubclassFeatures(),
			svc.featureProvider.RacialTraits(sub.Race),
		)
	}
	return stats
}

// hasAbilityMethodProvider reports whether an ability-method provider is wired.
func (svc *BuilderService) hasAbilityMethodProvider() bool {
	if svc.abilityMethods != nil {
		return true
	}
	_, ok := svc.store.(AbilityMethodProvider)
	return ok
}

// lookupRaceSpeeds returns the DB race→speed map, or nil when unavailable.
func (svc *BuilderService) lookupRaceSpeeds(ctx context.Context) map[string]int {
	if svc.raceSpeedFn == nil {
		return nil
	}
	return svc.raceSpeedFn(ctx)
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
	if slices.Contains(allowed, method) {
		return nil
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

// Character converts the creation score shape to character.AbilityScores.
func (s PointBuyScores) Character() character.AbilityScores {
	return character.AbilityScores{
		STR: s.STR,
		DEX: s.DEX,
		CON: s.CON,
		INT: s.INT,
		WIS: s.WIS,
		CHA: s.CHA,
	}
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
		base := min(v, 15)
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
