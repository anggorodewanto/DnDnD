package homebrew

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// --- in-memory fake store ---

type fakeStore struct {
	creatures  map[string]refdata.Creature
	spells     map[string]refdata.Spell
	weapons    map[string]refdata.Weapon
	magicItems map[string]refdata.MagicItem
	races      map[string]refdata.Race
	feats      map[string]refdata.Feat
	classes    map[string]refdata.Class

	// Optional injected errors.
	upsertErr error
	getErr    error
	deleteErr error
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		creatures:  map[string]refdata.Creature{},
		spells:     map[string]refdata.Spell{},
		weapons:    map[string]refdata.Weapon{},
		magicItems: map[string]refdata.MagicItem{},
		races:      map[string]refdata.Race{},
		feats:      map[string]refdata.Feat{},
		classes:    map[string]refdata.Class{},
	}
}

// --- creatures ---

func (f *fakeStore) GetCreature(_ context.Context, id string) (refdata.Creature, error) {
	if f.getErr != nil {
		return refdata.Creature{}, f.getErr
	}
	c, ok := f.creatures[id]
	if !ok {
		return refdata.Creature{}, sql.ErrNoRows
	}
	return c, nil
}

func (f *fakeStore) UpsertCreature(_ context.Context, arg refdata.UpsertCreatureParams) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.creatures[arg.ID] = refdata.Creature{
		ID: arg.ID, Name: arg.Name, Size: arg.Size, Type: arg.Type, Cr: arg.Cr,
		Ac: arg.Ac, HpFormula: arg.HpFormula, HpAverage: arg.HpAverage,
		Speed: arg.Speed, AbilityScores: arg.AbilityScores, Attacks: arg.Attacks,
		CampaignID: arg.CampaignID, Homebrew: arg.Homebrew, Source: arg.Source,
	}
	return nil
}

func (f *fakeStore) DeleteHomebrewCreature(_ context.Context, arg refdata.DeleteHomebrewCreatureParams) (int64, error) {
	if f.deleteErr != nil {
		return 0, f.deleteErr
	}
	c, ok := f.creatures[arg.ID]
	if !ok || !c.Homebrew.Valid || !c.Homebrew.Bool {
		return 0, nil
	}
	if !c.CampaignID.Valid || c.CampaignID.UUID != arg.CampaignID.UUID {
		return 0, nil
	}
	delete(f.creatures, arg.ID)
	return 1, nil
}

// --- spells ---

func (f *fakeStore) GetSpell(_ context.Context, id string) (refdata.Spell, error) {
	if f.getErr != nil {
		return refdata.Spell{}, f.getErr
	}
	s, ok := f.spells[id]
	if !ok {
		return refdata.Spell{}, sql.ErrNoRows
	}
	return s, nil
}

func (f *fakeStore) UpsertSpell(_ context.Context, arg refdata.UpsertSpellParams) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.spells[arg.ID] = refdata.Spell{
		ID: arg.ID, Name: arg.Name, Level: arg.Level, School: arg.School,
		CastingTime: arg.CastingTime, RangeType: arg.RangeType,
		Components: arg.Components, Duration: arg.Duration,
		Description: arg.Description, ResolutionMode: arg.ResolutionMode,
		Classes: arg.Classes, CampaignID: arg.CampaignID, Homebrew: arg.Homebrew, Source: arg.Source,
	}
	return nil
}

func (f *fakeStore) DeleteHomebrewSpell(_ context.Context, arg refdata.DeleteHomebrewSpellParams) (int64, error) {
	if f.deleteErr != nil {
		return 0, f.deleteErr
	}
	s, ok := f.spells[arg.ID]
	if !ok || !s.Homebrew.Valid || !s.Homebrew.Bool {
		return 0, nil
	}
	if !s.CampaignID.Valid || s.CampaignID.UUID != arg.CampaignID.UUID {
		return 0, nil
	}
	delete(f.spells, arg.ID)
	return 1, nil
}

// --- weapons ---

func (f *fakeStore) GetWeapon(_ context.Context, id string) (refdata.Weapon, error) {
	if f.getErr != nil {
		return refdata.Weapon{}, f.getErr
	}
	w, ok := f.weapons[id]
	if !ok {
		return refdata.Weapon{}, sql.ErrNoRows
	}
	return w, nil
}

func (f *fakeStore) UpsertWeapon(_ context.Context, arg refdata.UpsertWeaponParams) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.weapons[arg.ID] = refdata.Weapon{
		ID: arg.ID, Name: arg.Name, Damage: arg.Damage, DamageType: arg.DamageType,
		WeaponType: arg.WeaponType, CampaignID: arg.CampaignID, Homebrew: arg.Homebrew, Source: arg.Source,
	}
	return nil
}

func (f *fakeStore) DeleteHomebrewWeapon(_ context.Context, arg refdata.DeleteHomebrewWeaponParams) (int64, error) {
	if f.deleteErr != nil {
		return 0, f.deleteErr
	}
	w, ok := f.weapons[arg.ID]
	if !ok || !w.Homebrew.Valid || !w.Homebrew.Bool {
		return 0, nil
	}
	if !w.CampaignID.Valid || w.CampaignID.UUID != arg.CampaignID.UUID {
		return 0, nil
	}
	delete(f.weapons, arg.ID)
	return 1, nil
}

// --- magic items ---

func (f *fakeStore) GetMagicItem(_ context.Context, id string) (refdata.MagicItem, error) {
	if f.getErr != nil {
		return refdata.MagicItem{}, f.getErr
	}
	m, ok := f.magicItems[id]
	if !ok {
		return refdata.MagicItem{}, sql.ErrNoRows
	}
	return m, nil
}

func (f *fakeStore) UpsertMagicItem(_ context.Context, arg refdata.UpsertMagicItemParams) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.magicItems[arg.ID] = refdata.MagicItem{
		ID: arg.ID, Name: arg.Name, Rarity: arg.Rarity, Description: arg.Description,
		CampaignID: arg.CampaignID, Homebrew: arg.Homebrew, Source: arg.Source,
	}
	return nil
}

func (f *fakeStore) DeleteHomebrewMagicItem(_ context.Context, arg refdata.DeleteHomebrewMagicItemParams) (int64, error) {
	if f.deleteErr != nil {
		return 0, f.deleteErr
	}
	m, ok := f.magicItems[arg.ID]
	if !ok || !m.Homebrew.Valid || !m.Homebrew.Bool {
		return 0, nil
	}
	if !m.CampaignID.Valid || m.CampaignID.UUID != arg.CampaignID.UUID {
		return 0, nil
	}
	delete(f.magicItems, arg.ID)
	return 1, nil
}

// --- races ---

func (f *fakeStore) GetRace(_ context.Context, id string) (refdata.Race, error) {
	if f.getErr != nil {
		return refdata.Race{}, f.getErr
	}
	r, ok := f.races[id]
	if !ok {
		return refdata.Race{}, sql.ErrNoRows
	}
	return r, nil
}

func (f *fakeStore) UpsertRace(_ context.Context, arg refdata.UpsertRaceParams) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.races[arg.ID] = refdata.Race{
		ID: arg.ID, Name: arg.Name, SpeedFt: arg.SpeedFt, Size: arg.Size,
		AbilityBonuses: arg.AbilityBonuses, Traits: arg.Traits,
		CampaignID: arg.CampaignID, Homebrew: arg.Homebrew, Source: arg.Source,
	}
	return nil
}

func (f *fakeStore) DeleteHomebrewRace(_ context.Context, arg refdata.DeleteHomebrewRaceParams) (int64, error) {
	if f.deleteErr != nil {
		return 0, f.deleteErr
	}
	r, ok := f.races[arg.ID]
	if !ok || !r.Homebrew.Valid || !r.Homebrew.Bool {
		return 0, nil
	}
	if !r.CampaignID.Valid || r.CampaignID.UUID != arg.CampaignID.UUID {
		return 0, nil
	}
	delete(f.races, arg.ID)
	return 1, nil
}

// --- feats ---

func (f *fakeStore) GetFeat(_ context.Context, id string) (refdata.Feat, error) {
	if f.getErr != nil {
		return refdata.Feat{}, f.getErr
	}
	r, ok := f.feats[id]
	if !ok {
		return refdata.Feat{}, sql.ErrNoRows
	}
	return r, nil
}

func (f *fakeStore) UpsertFeat(_ context.Context, arg refdata.UpsertFeatParams) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.feats[arg.ID] = refdata.Feat{
		ID: arg.ID, Name: arg.Name, Description: arg.Description,
		CampaignID: arg.CampaignID, Homebrew: arg.Homebrew, Source: arg.Source,
	}
	return nil
}

func (f *fakeStore) DeleteHomebrewFeat(_ context.Context, arg refdata.DeleteHomebrewFeatParams) (int64, error) {
	if f.deleteErr != nil {
		return 0, f.deleteErr
	}
	r, ok := f.feats[arg.ID]
	if !ok || !r.Homebrew.Valid || !r.Homebrew.Bool {
		return 0, nil
	}
	if !r.CampaignID.Valid || r.CampaignID.UUID != arg.CampaignID.UUID {
		return 0, nil
	}
	delete(f.feats, arg.ID)
	return 1, nil
}

// --- classes ---

func (f *fakeStore) GetClass(_ context.Context, id string) (refdata.Class, error) {
	if f.getErr != nil {
		return refdata.Class{}, f.getErr
	}
	r, ok := f.classes[id]
	if !ok {
		return refdata.Class{}, sql.ErrNoRows
	}
	return r, nil
}

func (f *fakeStore) UpsertClass(_ context.Context, arg refdata.UpsertClassParams) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.classes[arg.ID] = refdata.Class{
		ID: arg.ID, Name: arg.Name, HitDie: arg.HitDie, PrimaryAbility: arg.PrimaryAbility,
		FeaturesByLevel: arg.FeaturesByLevel, AttacksPerAction: arg.AttacksPerAction,
		Subclasses: arg.Subclasses, SubclassLevel: arg.SubclassLevel,
		CampaignID: arg.CampaignID, Homebrew: arg.Homebrew, Source: arg.Source,
	}
	return nil
}

func (f *fakeStore) DeleteHomebrewClass(_ context.Context, arg refdata.DeleteHomebrewClassParams) (int64, error) {
	if f.deleteErr != nil {
		return 0, f.deleteErr
	}
	r, ok := f.classes[arg.ID]
	if !ok || !r.Homebrew.Valid || !r.Homebrew.Bool {
		return 0, nil
	}
	if !r.CampaignID.Valid || r.CampaignID.UUID != arg.CampaignID.UUID {
		return 0, nil
	}
	delete(f.classes, arg.ID)
	return 1, nil
}

// --- helpers ---

func newSvc(store *fakeStore) *Service {
	svc := NewService(store)
	// Deterministic id generator for tests.
	counter := 0
	svc.idGen = func() string {
		counter++
		return "hb_test_" + string(rune('a'+counter-1))
	}
	return svc
}

// seedSRDCreature pre-populates an SRD creature directly in the fake.
func seedSRDCreature(store *fakeStore, id string) {
	store.creatures[id] = refdata.Creature{
		ID: id, Name: "SRD " + id, Type: "humanoid", Size: "Medium", Cr: "1",
		// no homebrew flag, no campaign id
	}
}

// =============================================================================
// CREATURES
// =============================================================================

func TestCreature_Create_Success(t *testing.T) {
	store := newFakeStore()
	svc := newSvc(store)
	cid := uuid.New()
	got, err := svc.CreateHomebrewCreature(context.Background(), cid, refdata.UpsertCreatureParams{
		Name: "Goblin Boss", Size: "Small", Type: "humanoid", Cr: "1",
		Ac: 17, HpFormula: "5d8+5", HpAverage: 27,
		Speed:         json.RawMessage(`{"walk":30}`),
		AbilityScores: json.RawMessage(`{"str":10}`),
		Attacks:       json.RawMessage(`[]`),
	})
	require.NoError(t, err)
	assert.Equal(t, "Goblin Boss", got.Name)
	assert.True(t, got.Homebrew.Bool)
	assert.True(t, got.Homebrew.Valid)
	assert.Equal(t, "homebrew", got.Source.String)
	assert.Equal(t, cid, got.CampaignID.UUID)
	assert.NotEmpty(t, got.ID)
}

func TestCreature_Create_NoCampaign(t *testing.T) {
	svc := newSvc(newFakeStore())
	_, err := svc.CreateHomebrewCreature(context.Background(), uuid.Nil, refdata.UpsertCreatureParams{Name: "x"})
	require.ErrorIs(t, err, ErrInvalidInput)
}

func TestCreature_Create_EmptyName(t *testing.T) {
	svc := newSvc(newFakeStore())
	_, err := svc.CreateHomebrewCreature(context.Background(), uuid.New(), refdata.UpsertCreatureParams{Name: "  "})
	require.ErrorIs(t, err, ErrInvalidInput)
}

func TestCreature_Create_StoreError(t *testing.T) {
	store := newFakeStore()
	store.upsertErr = errors.New("boom")
	svc := newSvc(store)
	_, err := svc.CreateHomebrewCreature(context.Background(), uuid.New(), refdata.UpsertCreatureParams{Name: "x"})
	require.Error(t, err)
}

func TestCreature_Update_Success(t *testing.T) {
	store := newFakeStore()
	svc := newSvc(store)
	cid := uuid.New()
	created, err := svc.CreateHomebrewCreature(context.Background(), cid, refdata.UpsertCreatureParams{Name: "Old"})
	require.NoError(t, err)
	updated, err := svc.UpdateHomebrewCreature(context.Background(), cid, created.ID, refdata.UpsertCreatureParams{Name: "New"})
	require.NoError(t, err)
	assert.Equal(t, "New", updated.Name)
}

func TestCreature_Update_RejectsSRD(t *testing.T) {
	store := newFakeStore()
	seedSRDCreature(store, "goblin")
	svc := newSvc(store)
	_, err := svc.UpdateHomebrewCreature(context.Background(), uuid.New(), "goblin", refdata.UpsertCreatureParams{Name: "Hacked"})
	require.ErrorIs(t, err, ErrNotFound)
}

func TestCreature_Update_WrongCampaign(t *testing.T) {
	store := newFakeStore()
	svc := newSvc(store)
	owner := uuid.New()
	other := uuid.New()
	created, err := svc.CreateHomebrewCreature(context.Background(), owner, refdata.UpsertCreatureParams{Name: "Mine"})
	require.NoError(t, err)
	_, err = svc.UpdateHomebrewCreature(context.Background(), other, created.ID, refdata.UpsertCreatureParams{Name: "Stolen"})
	require.ErrorIs(t, err, ErrNotFound)
}

func TestCreature_Update_NotFound(t *testing.T) {
	svc := newSvc(newFakeStore())
	_, err := svc.UpdateHomebrewCreature(context.Background(), uuid.New(), "missing", refdata.UpsertCreatureParams{Name: "x"})
	require.ErrorIs(t, err, ErrNotFound)
}

func TestCreature_Update_Validation(t *testing.T) {
	svc := newSvc(newFakeStore())
	_, err := svc.UpdateHomebrewCreature(context.Background(), uuid.Nil, "id", refdata.UpsertCreatureParams{Name: "x"})
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = svc.UpdateHomebrewCreature(context.Background(), uuid.New(), "  ", refdata.UpsertCreatureParams{Name: "x"})
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = svc.UpdateHomebrewCreature(context.Background(), uuid.New(), "id", refdata.UpsertCreatureParams{Name: ""})
	require.ErrorIs(t, err, ErrInvalidInput)
}

func TestCreature_Update_GetError(t *testing.T) {
	store := newFakeStore()
	store.getErr = errors.New("db down")
	svc := newSvc(store)
	_, err := svc.UpdateHomebrewCreature(context.Background(), uuid.New(), "id", refdata.UpsertCreatureParams{Name: "x"})
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)
}

func TestCreature_Update_UpsertError(t *testing.T) {
	store := newFakeStore()
	svc := newSvc(store)
	cid := uuid.New()
	created, err := svc.CreateHomebrewCreature(context.Background(), cid, refdata.UpsertCreatureParams{Name: "x"})
	require.NoError(t, err)
	store.upsertErr = errors.New("boom")
	_, err = svc.UpdateHomebrewCreature(context.Background(), cid, created.ID, refdata.UpsertCreatureParams{Name: "y"})
	require.Error(t, err)
}

func TestCreature_Delete_Success(t *testing.T) {
	store := newFakeStore()
	svc := newSvc(store)
	cid := uuid.New()
	created, err := svc.CreateHomebrewCreature(context.Background(), cid, refdata.UpsertCreatureParams{Name: "x"})
	require.NoError(t, err)
	require.NoError(t, svc.DeleteHomebrewCreature(context.Background(), cid, created.ID))
	_, ok := store.creatures[created.ID]
	assert.False(t, ok)
}

func TestCreature_Delete_RejectsSRD(t *testing.T) {
	store := newFakeStore()
	seedSRDCreature(store, "goblin")
	svc := newSvc(store)
	err := svc.DeleteHomebrewCreature(context.Background(), uuid.New(), "goblin")
	require.ErrorIs(t, err, ErrNotFound)
	_, ok := store.creatures["goblin"]
	assert.True(t, ok, "SRD must remain")
}

func TestCreature_Delete_WrongCampaign(t *testing.T) {
	store := newFakeStore()
	svc := newSvc(store)
	owner := uuid.New()
	created, err := svc.CreateHomebrewCreature(context.Background(), owner, refdata.UpsertCreatureParams{Name: "x"})
	require.NoError(t, err)
	err = svc.DeleteHomebrewCreature(context.Background(), uuid.New(), created.ID)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestCreature_Delete_NotFound(t *testing.T) {
	svc := newSvc(newFakeStore())
	err := svc.DeleteHomebrewCreature(context.Background(), uuid.New(), "missing")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestCreature_Delete_Validation(t *testing.T) {
	svc := newSvc(newFakeStore())
	require.ErrorIs(t, svc.DeleteHomebrewCreature(context.Background(), uuid.Nil, "id"), ErrInvalidInput)
	require.ErrorIs(t, svc.DeleteHomebrewCreature(context.Background(), uuid.New(), "  "), ErrInvalidInput)
}

func TestCreature_Delete_StoreError(t *testing.T) {
	store := newFakeStore()
	store.deleteErr = errors.New("boom")
	svc := newSvc(store)
	err := svc.DeleteHomebrewCreature(context.Background(), uuid.New(), "id")
	require.Error(t, err)
}

// =============================================================================
// SPELLS
// =============================================================================

func TestSpell_FullCRUD(t *testing.T) {
	store := newFakeStore()
	svc := newSvc(store)
	cid := uuid.New()

	created, err := svc.CreateHomebrewSpell(context.Background(), cid, refdata.UpsertSpellParams{Name: "Eldritch Glow", Level: 1, School: "evocation"})
	require.NoError(t, err)
	assert.True(t, created.Homebrew.Bool)
	assert.Equal(t, cid, created.CampaignID.UUID)

	updated, err := svc.UpdateHomebrewSpell(context.Background(), cid, created.ID, refdata.UpsertSpellParams{Name: "Eldritch Glow II", Level: 2, School: "evocation"})
	require.NoError(t, err)
	assert.Equal(t, "Eldritch Glow II", updated.Name)

	require.NoError(t, svc.DeleteHomebrewSpell(context.Background(), cid, created.ID))
	_, ok := store.spells[created.ID]
	assert.False(t, ok)
}

func TestSpell_ValidationAndOwnership(t *testing.T) {
	svc := newSvc(newFakeStore())
	_, err := svc.CreateHomebrewSpell(context.Background(), uuid.Nil, refdata.UpsertSpellParams{Name: "x"})
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = svc.CreateHomebrewSpell(context.Background(), uuid.New(), refdata.UpsertSpellParams{Name: ""})
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = svc.UpdateHomebrewSpell(context.Background(), uuid.New(), "id", refdata.UpsertSpellParams{Name: ""})
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = svc.UpdateHomebrewSpell(context.Background(), uuid.New(), "missing", refdata.UpsertSpellParams{Name: "x"})
	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, svc.DeleteHomebrewSpell(context.Background(), uuid.Nil, "id"), ErrInvalidInput)
	require.ErrorIs(t, svc.DeleteHomebrewSpell(context.Background(), uuid.New(), "missing"), ErrNotFound)
}

func TestSpell_StoreErrors(t *testing.T) {
	store := newFakeStore()
	store.upsertErr = errors.New("boom")
	svc := newSvc(store)
	_, err := svc.CreateHomebrewSpell(context.Background(), uuid.New(), refdata.UpsertSpellParams{Name: "x"})
	require.Error(t, err)

	store2 := newFakeStore()
	store2.deleteErr = errors.New("boom")
	svc2 := newSvc(store2)
	require.Error(t, svc2.DeleteHomebrewSpell(context.Background(), uuid.New(), "id"))
}

func TestSpell_RejectsSRD(t *testing.T) {
	store := newFakeStore()
	store.spells["fireball"] = refdata.Spell{ID: "fireball", Name: "Fireball"}
	svc := newSvc(store)
	_, err := svc.UpdateHomebrewSpell(context.Background(), uuid.New(), "fireball", refdata.UpsertSpellParams{Name: "x"})
	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, svc.DeleteHomebrewSpell(context.Background(), uuid.New(), "fireball"), ErrNotFound)
}

// =============================================================================
// WEAPONS
// =============================================================================

func TestWeapon_FullCRUD(t *testing.T) {
	store := newFakeStore()
	svc := newSvc(store)
	cid := uuid.New()

	created, err := svc.CreateHomebrewWeapon(context.Background(), cid, refdata.UpsertWeaponParams{Name: "Vorpal Fork", Damage: "1d8", DamageType: "slashing", WeaponType: "martial-melee"})
	require.NoError(t, err)
	assert.True(t, created.Homebrew.Bool)

	updated, err := svc.UpdateHomebrewWeapon(context.Background(), cid, created.ID, refdata.UpsertWeaponParams{Name: "Vorpal Spoon", Damage: "1d10", DamageType: "bludgeoning", WeaponType: "martial-melee"})
	require.NoError(t, err)
	assert.Equal(t, "Vorpal Spoon", updated.Name)

	require.NoError(t, svc.DeleteHomebrewWeapon(context.Background(), cid, created.ID))
}

func TestWeapon_Errors(t *testing.T) {
	svc := newSvc(newFakeStore())
	_, err := svc.CreateHomebrewWeapon(context.Background(), uuid.Nil, refdata.UpsertWeaponParams{Name: "x"})
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = svc.UpdateHomebrewWeapon(context.Background(), uuid.New(), "missing", refdata.UpsertWeaponParams{Name: "x"})
	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, svc.DeleteHomebrewWeapon(context.Background(), uuid.New(), "missing"), ErrNotFound)
	require.ErrorIs(t, svc.DeleteHomebrewWeapon(context.Background(), uuid.Nil, "id"), ErrInvalidInput)

	store := newFakeStore()
	store.upsertErr = errors.New("boom")
	svc2 := newSvc(store)
	_, err = svc2.CreateHomebrewWeapon(context.Background(), uuid.New(), refdata.UpsertWeaponParams{Name: "x"})
	require.Error(t, err)

	store3 := newFakeStore()
	store3.deleteErr = errors.New("boom")
	require.Error(t, newSvc(store3).DeleteHomebrewWeapon(context.Background(), uuid.New(), "x"))
}

func TestWeapon_RejectsSRD(t *testing.T) {
	store := newFakeStore()
	store.weapons["longsword"] = refdata.Weapon{ID: "longsword", Name: "Longsword"}
	svc := newSvc(store)
	_, err := svc.UpdateHomebrewWeapon(context.Background(), uuid.New(), "longsword", refdata.UpsertWeaponParams{Name: "x"})
	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, svc.DeleteHomebrewWeapon(context.Background(), uuid.New(), "longsword"), ErrNotFound)
}

// =============================================================================
// MAGIC ITEMS
// =============================================================================

func TestMagicItem_FullCRUD(t *testing.T) {
	store := newFakeStore()
	svc := newSvc(store)
	cid := uuid.New()
	created, err := svc.CreateHomebrewMagicItem(context.Background(), cid, refdata.UpsertMagicItemParams{Name: "Bag of Doom", Rarity: "rare", Description: "scary"})
	require.NoError(t, err)
	assert.Equal(t, cid, created.CampaignID.UUID)
	updated, err := svc.UpdateHomebrewMagicItem(context.Background(), cid, created.ID, refdata.UpsertMagicItemParams{Name: "Bag of More Doom", Rarity: "very rare", Description: "scarier"})
	require.NoError(t, err)
	assert.Equal(t, "Bag of More Doom", updated.Name)
	require.NoError(t, svc.DeleteHomebrewMagicItem(context.Background(), cid, created.ID))
}

func TestMagicItem_Errors(t *testing.T) {
	svc := newSvc(newFakeStore())
	_, err := svc.CreateHomebrewMagicItem(context.Background(), uuid.Nil, refdata.UpsertMagicItemParams{Name: "x"})
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = svc.UpdateHomebrewMagicItem(context.Background(), uuid.New(), "missing", refdata.UpsertMagicItemParams{Name: "x"})
	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, svc.DeleteHomebrewMagicItem(context.Background(), uuid.New(), "missing"), ErrNotFound)
	require.ErrorIs(t, svc.DeleteHomebrewMagicItem(context.Background(), uuid.Nil, "id"), ErrInvalidInput)
}

func TestMagicItem_RejectsSRD(t *testing.T) {
	store := newFakeStore()
	store.magicItems["bag-of-holding"] = refdata.MagicItem{ID: "bag-of-holding", Name: "Bag of Holding"}
	svc := newSvc(store)
	_, err := svc.UpdateHomebrewMagicItem(context.Background(), uuid.New(), "bag-of-holding", refdata.UpsertMagicItemParams{Name: "x"})
	require.ErrorIs(t, err, ErrNotFound)
}

func TestMagicItem_StoreErrors(t *testing.T) {
	store := newFakeStore()
	store.upsertErr = errors.New("boom")
	svc := newSvc(store)
	_, err := svc.CreateHomebrewMagicItem(context.Background(), uuid.New(), refdata.UpsertMagicItemParams{Name: "x"})
	require.Error(t, err)
	store.upsertErr = nil
	store.deleteErr = errors.New("boom")
	require.Error(t, svc.DeleteHomebrewMagicItem(context.Background(), uuid.New(), "x"))
}

// =============================================================================
// RACES
// =============================================================================

func TestRace_FullCRUD(t *testing.T) {
	store := newFakeStore()
	svc := newSvc(store)
	cid := uuid.New()
	created, err := svc.CreateHomebrewRace(context.Background(), cid, refdata.UpsertRaceParams{
		Name: "Catfolk", SpeedFt: 30, Size: "Medium",
		AbilityBonuses: json.RawMessage(`{"dex":2}`),
		Traits:         json.RawMessage(`[]`),
	})
	require.NoError(t, err)
	assert.True(t, created.Homebrew.Bool)
	updated, err := svc.UpdateHomebrewRace(context.Background(), cid, created.ID, refdata.UpsertRaceParams{
		Name: "Catfolk Royal", SpeedFt: 35, Size: "Medium",
		AbilityBonuses: json.RawMessage(`{"dex":2,"cha":1}`),
		Traits:         json.RawMessage(`[]`),
	})
	require.NoError(t, err)
	assert.Equal(t, "Catfolk Royal", updated.Name)
	require.NoError(t, svc.DeleteHomebrewRace(context.Background(), cid, created.ID))
}

func TestRace_Errors(t *testing.T) {
	svc := newSvc(newFakeStore())
	_, err := svc.CreateHomebrewRace(context.Background(), uuid.Nil, refdata.UpsertRaceParams{Name: "x"})
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = svc.UpdateHomebrewRace(context.Background(), uuid.New(), "missing", refdata.UpsertRaceParams{Name: "x"})
	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, svc.DeleteHomebrewRace(context.Background(), uuid.New(), "missing"), ErrNotFound)
	require.ErrorIs(t, svc.DeleteHomebrewRace(context.Background(), uuid.Nil, "id"), ErrInvalidInput)
}

func TestRace_StoreErrors(t *testing.T) {
	store := newFakeStore()
	store.upsertErr = errors.New("boom")
	svc := newSvc(store)
	_, err := svc.CreateHomebrewRace(context.Background(), uuid.New(), refdata.UpsertRaceParams{Name: "x"})
	require.Error(t, err)
	store.upsertErr = nil
	store.deleteErr = errors.New("boom")
	require.Error(t, svc.DeleteHomebrewRace(context.Background(), uuid.New(), "x"))
}

func TestRace_RejectsSRD(t *testing.T) {
	store := newFakeStore()
	store.races["elf"] = refdata.Race{ID: "elf", Name: "Elf"}
	svc := newSvc(store)
	_, err := svc.UpdateHomebrewRace(context.Background(), uuid.New(), "elf", refdata.UpsertRaceParams{Name: "x"})
	require.ErrorIs(t, err, ErrNotFound)
}

// =============================================================================
// FEATS
// =============================================================================

func TestFeat_FullCRUD(t *testing.T) {
	store := newFakeStore()
	svc := newSvc(store)
	cid := uuid.New()
	created, err := svc.CreateHomebrewFeat(context.Background(), cid, refdata.UpsertFeatParams{Name: "Lucky Boot", Description: "Reroll a 1"})
	require.NoError(t, err)
	updated, err := svc.UpdateHomebrewFeat(context.Background(), cid, created.ID, refdata.UpsertFeatParams{Name: "Lucky Slipper", Description: "Reroll any roll"})
	require.NoError(t, err)
	assert.Equal(t, "Lucky Slipper", updated.Name)
	require.NoError(t, svc.DeleteHomebrewFeat(context.Background(), cid, created.ID))
}

func TestFeat_Errors(t *testing.T) {
	svc := newSvc(newFakeStore())
	_, err := svc.CreateHomebrewFeat(context.Background(), uuid.Nil, refdata.UpsertFeatParams{Name: "x"})
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = svc.UpdateHomebrewFeat(context.Background(), uuid.New(), "missing", refdata.UpsertFeatParams{Name: "x"})
	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, svc.DeleteHomebrewFeat(context.Background(), uuid.New(), "missing"), ErrNotFound)
	require.ErrorIs(t, svc.DeleteHomebrewFeat(context.Background(), uuid.Nil, "id"), ErrInvalidInput)
}

func TestFeat_StoreErrors(t *testing.T) {
	store := newFakeStore()
	store.upsertErr = errors.New("boom")
	svc := newSvc(store)
	_, err := svc.CreateHomebrewFeat(context.Background(), uuid.New(), refdata.UpsertFeatParams{Name: "x"})
	require.Error(t, err)
	store.upsertErr = nil
	store.deleteErr = errors.New("boom")
	require.Error(t, svc.DeleteHomebrewFeat(context.Background(), uuid.New(), "x"))
}

func TestFeat_RejectsSRD(t *testing.T) {
	store := newFakeStore()
	store.feats["lucky"] = refdata.Feat{ID: "lucky", Name: "Lucky"}
	svc := newSvc(store)
	_, err := svc.UpdateHomebrewFeat(context.Background(), uuid.New(), "lucky", refdata.UpsertFeatParams{Name: "x"})
	require.ErrorIs(t, err, ErrNotFound)
}

// =============================================================================
// CLASSES
// =============================================================================

func TestClass_FullCRUD(t *testing.T) {
	store := newFakeStore()
	svc := newSvc(store)
	cid := uuid.New()
	created, err := svc.CreateHomebrewClass(context.Background(), cid, refdata.UpsertClassParams{
		Name: "Pugilist", HitDie: "1d10", PrimaryAbility: "STR",
		FeaturesByLevel:  json.RawMessage(`{}`),
		AttacksPerAction: json.RawMessage(`{}`),
		Subclasses:       json.RawMessage(`[]`),
		SubclassLevel:    3,
	})
	require.NoError(t, err)
	assert.True(t, created.Homebrew.Bool)

	updated, err := svc.UpdateHomebrewClass(context.Background(), cid, created.ID, refdata.UpsertClassParams{
		Name: "Pugilist 2.0", HitDie: "1d12", PrimaryAbility: "STR",
		FeaturesByLevel:  json.RawMessage(`{}`),
		AttacksPerAction: json.RawMessage(`{}`),
		Subclasses:       json.RawMessage(`[]`),
		SubclassLevel:    3,
	})
	require.NoError(t, err)
	assert.Equal(t, "Pugilist 2.0", updated.Name)

	require.NoError(t, svc.DeleteHomebrewClass(context.Background(), cid, created.ID))
}

func TestClass_Errors(t *testing.T) {
	svc := newSvc(newFakeStore())
	_, err := svc.CreateHomebrewClass(context.Background(), uuid.Nil, refdata.UpsertClassParams{Name: "x"})
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = svc.UpdateHomebrewClass(context.Background(), uuid.New(), "missing", refdata.UpsertClassParams{Name: "x"})
	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, svc.DeleteHomebrewClass(context.Background(), uuid.New(), "missing"), ErrNotFound)
	require.ErrorIs(t, svc.DeleteHomebrewClass(context.Background(), uuid.Nil, "id"), ErrInvalidInput)
}

func TestClass_StoreErrors(t *testing.T) {
	store := newFakeStore()
	store.upsertErr = errors.New("boom")
	svc := newSvc(store)
	_, err := svc.CreateHomebrewClass(context.Background(), uuid.New(), refdata.UpsertClassParams{Name: "x"})
	require.Error(t, err)
	store.upsertErr = nil
	store.deleteErr = errors.New("boom")
	require.Error(t, svc.DeleteHomebrewClass(context.Background(), uuid.New(), "x"))
}

func TestClass_RejectsSRD(t *testing.T) {
	store := newFakeStore()
	store.classes["fighter"] = refdata.Class{ID: "fighter", Name: "Fighter"}
	svc := newSvc(store)
	_, err := svc.UpdateHomebrewClass(context.Background(), uuid.New(), "fighter", refdata.UpsertClassParams{Name: "x"})
	require.ErrorIs(t, err, ErrNotFound)
}

// =============================================================================
// shared helpers
// =============================================================================

func TestDefaultIDGenIsRandomAndPrefixed(t *testing.T) {
	a := defaultIDGen()
	b := defaultIDGen()
	assert.NotEqual(t, a, b)
	assert.Contains(t, a, "hb_")
}

// TestUpdate_UpsertErrorAllTypes covers the "upsert returns error during
// update" branch for every type (the create path is already covered for
// every type, but updating exercises a separate code path that re-calls
// upsert after a successful Get).
func TestUpdate_UpsertErrorAllTypes(t *testing.T) {
	cid := uuid.New()
	ctx := context.Background()

	t.Run("creature", func(t *testing.T) {
		store := newFakeStore()
		svc := newSvc(store)
		created, err := svc.CreateHomebrewCreature(ctx, cid, refdata.UpsertCreatureParams{Name: "x"})
		require.NoError(t, err)
		store.upsertErr = errors.New("boom")
		_, err = svc.UpdateHomebrewCreature(ctx, cid, created.ID, refdata.UpsertCreatureParams{Name: "y"})
		require.Error(t, err)
	})
	t.Run("spell", func(t *testing.T) {
		store := newFakeStore()
		svc := newSvc(store)
		created, err := svc.CreateHomebrewSpell(ctx, cid, refdata.UpsertSpellParams{Name: "x"})
		require.NoError(t, err)
		store.upsertErr = errors.New("boom")
		_, err = svc.UpdateHomebrewSpell(ctx, cid, created.ID, refdata.UpsertSpellParams{Name: "y"})
		require.Error(t, err)
	})
	t.Run("weapon", func(t *testing.T) {
		store := newFakeStore()
		svc := newSvc(store)
		created, err := svc.CreateHomebrewWeapon(ctx, cid, refdata.UpsertWeaponParams{Name: "x"})
		require.NoError(t, err)
		store.upsertErr = errors.New("boom")
		_, err = svc.UpdateHomebrewWeapon(ctx, cid, created.ID, refdata.UpsertWeaponParams{Name: "y"})
		require.Error(t, err)
	})
	t.Run("magicitem", func(t *testing.T) {
		store := newFakeStore()
		svc := newSvc(store)
		created, err := svc.CreateHomebrewMagicItem(ctx, cid, refdata.UpsertMagicItemParams{Name: "x"})
		require.NoError(t, err)
		store.upsertErr = errors.New("boom")
		_, err = svc.UpdateHomebrewMagicItem(ctx, cid, created.ID, refdata.UpsertMagicItemParams{Name: "y"})
		require.Error(t, err)
	})
	t.Run("race", func(t *testing.T) {
		store := newFakeStore()
		svc := newSvc(store)
		created, err := svc.CreateHomebrewRace(ctx, cid, refdata.UpsertRaceParams{Name: "x"})
		require.NoError(t, err)
		store.upsertErr = errors.New("boom")
		_, err = svc.UpdateHomebrewRace(ctx, cid, created.ID, refdata.UpsertRaceParams{Name: "y"})
		require.Error(t, err)
	})
	t.Run("feat", func(t *testing.T) {
		store := newFakeStore()
		svc := newSvc(store)
		created, err := svc.CreateHomebrewFeat(ctx, cid, refdata.UpsertFeatParams{Name: "x"})
		require.NoError(t, err)
		store.upsertErr = errors.New("boom")
		_, err = svc.UpdateHomebrewFeat(ctx, cid, created.ID, refdata.UpsertFeatParams{Name: "y"})
		require.Error(t, err)
	})
	t.Run("class", func(t *testing.T) {
		store := newFakeStore()
		svc := newSvc(store)
		created, err := svc.CreateHomebrewClass(ctx, cid, refdata.UpsertClassParams{Name: "x"})
		require.NoError(t, err)
		store.upsertErr = errors.New("boom")
		_, err = svc.UpdateHomebrewClass(ctx, cid, created.ID, refdata.UpsertClassParams{Name: "y"})
		require.Error(t, err)
	})
}

// TestFetchAfterCreateGetError covers the "fetch returns get error" branch
// on every per-type fetch helper.
func TestFetchAfterCreateGetError(t *testing.T) {
	ctx := context.Background()
	cid := uuid.New()
	for _, name := range []string{"creature", "spell", "weapon", "magicitem", "race", "feat", "class"} {
		t.Run(name, func(t *testing.T) {
			store := newFakeStore()
			svc := newSvc(store)
			// Make Get fail, Upsert succeed.
			store.getErr = errors.New("post-write read failure")
			var err error
			switch name {
			case "creature":
				_, err = svc.CreateHomebrewCreature(ctx, cid, refdata.UpsertCreatureParams{Name: "x"})
			case "spell":
				_, err = svc.CreateHomebrewSpell(ctx, cid, refdata.UpsertSpellParams{Name: "x"})
			case "weapon":
				_, err = svc.CreateHomebrewWeapon(ctx, cid, refdata.UpsertWeaponParams{Name: "x"})
			case "magicitem":
				_, err = svc.CreateHomebrewMagicItem(ctx, cid, refdata.UpsertMagicItemParams{Name: "x"})
			case "race":
				_, err = svc.CreateHomebrewRace(ctx, cid, refdata.UpsertRaceParams{Name: "x"})
			case "feat":
				_, err = svc.CreateHomebrewFeat(ctx, cid, refdata.UpsertFeatParams{Name: "x"})
			case "class":
				_, err = svc.CreateHomebrewClass(ctx, cid, refdata.UpsertClassParams{Name: "x"})
			}
			require.Error(t, err)
		})
	}
}

// Cover the "fetch after create returns get error" branch.
func TestCreate_GetAfterUpsertError(t *testing.T) {
	store := newFakeStore()
	svc := newSvc(store)
	store.getErr = errors.New("post-write read failure")
	// upsert succeeds, get fails
	_, err := svc.CreateHomebrewSpell(context.Background(), uuid.New(), refdata.UpsertSpellParams{Name: "x"})
	require.Error(t, err)
}
