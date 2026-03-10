package refdata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// mockDBTX implements the DBTX interface for unit testing.
// It returns errToReturn on every ExecContext call, or after
// failAfterN successful calls if failAfterN > 0.
type mockDBTX struct {
	errToReturn error
	failAfterN  int
	callCount   int
	queryErr    error
}

func (m *mockDBTX) ExecContext(_ context.Context, _ string, _ ...any) (sql.Result, error) {
	m.callCount++
	if m.failAfterN > 0 && m.callCount <= m.failAfterN {
		return nil, nil
	}
	return nil, m.errToReturn
}

func (m *mockDBTX) PrepareContext(_ context.Context, _ string) (*sql.Stmt, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockDBTX) QueryContext(_ context.Context, _ string, _ ...any) (*sql.Rows, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockDBTX) QueryRowContext(_ context.Context, _ string, _ ...any) *sql.Row {
	return nil
}

func TestMustJSON(t *testing.T) {
	effects := []MechanicalEffect{
		{EffectType: "cant_see"},
		{EffectType: "auto_fail_ability_check", Condition: "requires_sight"},
	}

	result := mustJSON(effects)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	var parsed []MechanicalEffect
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(parsed) != 2 {
		t.Fatalf("expected 2 effects, got %d", len(parsed))
	}
	if parsed[0].EffectType != "cant_see" {
		t.Fatalf("expected effect_type cant_see, got %q", parsed[0].EffectType)
	}
	if parsed[1].Condition != "requires_sight" {
		t.Fatalf("expected condition requires_sight, got %q", parsed[1].Condition)
	}
}

func TestMechanicalEffectJSON(t *testing.T) {
	effect := MechanicalEffect{
		EffectType:  "grant_advantage",
		Description: "Attacks have advantage",
		Target:      "attack_rolls",
		Condition:   "within_5ft",
		Value:       "2",
	}

	b, err := json.Marshal(effect)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed MechanicalEffect
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.EffectType != "grant_advantage" {
		t.Fatalf("expected effect_type grant_advantage, got %q", parsed.EffectType)
	}
	if parsed.Target != "attack_rolls" {
		t.Fatalf("expected target attack_rolls, got %q", parsed.Target)
	}
}

func TestMechanicalEffectJSON_OmitsEmpty(t *testing.T) {
	effect := MechanicalEffect{
		EffectType: "cant_see",
	}

	b, err := json.Marshal(effect)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if _, ok := m["description"]; ok {
		t.Fatal("expected description to be omitted when empty")
	}
	if _, ok := m["target"]; ok {
		t.Fatal("expected target to be omitted when empty")
	}
	if _, ok := m["condition"]; ok {
		t.Fatal("expected condition to be omitted when empty")
	}
	if _, ok := m["value"]; ok {
		t.Fatal("expected value to be omitted when empty")
	}
}

func TestOptHelpers(t *testing.T) {
	f := optFloat(3.5)
	if !f.Valid || f.Float64 != 3.5 {
		t.Fatalf("optFloat failed: %v", f)
	}

	i := optInt(10)
	if !i.Valid || i.Int32 != 10 {
		t.Fatalf("optInt failed: %v", i)
	}

	s := optStr("hello")
	if !s.Valid || s.String != "hello" {
		t.Fatalf("optStr failed: %v", s)
	}

	b := optBool(true)
	if !b.Valid || !b.Bool {
		t.Fatalf("optBool(true) failed: %v", b)
	}

	b2 := optBool(false)
	if !b2.Valid || b2.Bool {
		t.Fatalf("optBool(false) failed: %v", b2)
	}
}

func TestSeedWeapons_ErrorWrapping(t *testing.T) {
	dbErr := errors.New("exec failed")
	mock := &mockDBTX{errToReturn: dbErr}
	q := New(mock)

	err := seedWeapons(context.Background(), q)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "upserting weapon") {
		t.Fatalf("expected error to contain 'upserting weapon', got %q", err.Error())
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected wrapped error to contain original, got %v", err)
	}
}

func TestSeedArmor_ErrorWrapping(t *testing.T) {
	dbErr := errors.New("exec failed")
	mock := &mockDBTX{errToReturn: dbErr}
	q := New(mock)

	err := seedArmor(context.Background(), q)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "upserting armor") {
		t.Fatalf("expected error to contain 'upserting armor', got %q", err.Error())
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected wrapped error to contain original, got %v", err)
	}
}

func TestSeedConditions_ErrorWrapping(t *testing.T) {
	dbErr := errors.New("exec failed")
	mock := &mockDBTX{errToReturn: dbErr}
	q := New(mock)

	err := seedConditions(context.Background(), q)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "upserting condition") {
		t.Fatalf("expected error to contain 'upserting condition', got %q", err.Error())
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected wrapped error to contain original, got %v", err)
	}
}

func TestSeedAll_WeaponsErrorWrapping(t *testing.T) {
	dbErr := errors.New("weapons exec failed")
	mock := &mockDBTX{errToReturn: dbErr}
	err := SeedAll(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "seeding weapons") {
		t.Fatalf("expected error to contain 'seeding weapons', got %q", err.Error())
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected wrapped error to contain original, got %v", err)
	}
}

func TestSeedAll_ArmorErrorWrapping(t *testing.T) {
	dbErr := errors.New("armor exec failed")
	// All weapons succeed, then armor fails on first call
	mock := &mockDBTX{errToReturn: dbErr, failAfterN: WeaponCount}
	err := SeedAll(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "seeding armor") {
		t.Fatalf("expected error to contain 'seeding armor', got %q", err.Error())
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected wrapped error to contain original, got %v", err)
	}
}

func TestSeedAll_ConditionsErrorWrapping(t *testing.T) {
	dbErr := errors.New("condition exec failed")
	// All weapons + armor succeed, then conditions fails
	mock := &mockDBTX{errToReturn: dbErr, failAfterN: WeaponCount + ArmorCount}
	err := SeedAll(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "seeding conditions") {
		t.Fatalf("expected error to contain 'seeding conditions', got %q", err.Error())
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected wrapped error to contain original, got %v", err)
	}
}

func TestSeedClasses_ErrorWrapping(t *testing.T) {
	dbErr := errors.New("exec failed")
	mock := &mockDBTX{errToReturn: dbErr}
	q := New(mock)

	err := seedClasses(context.Background(), q)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "upserting class") {
		t.Fatalf("expected error to contain 'upserting class', got %q", err.Error())
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected wrapped error to contain original, got %v", err)
	}
}

func TestSeedRaces_ErrorWrapping(t *testing.T) {
	dbErr := errors.New("exec failed")
	mock := &mockDBTX{errToReturn: dbErr}
	q := New(mock)

	err := seedRaces(context.Background(), q)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "upserting race") {
		t.Fatalf("expected error to contain 'upserting race', got %q", err.Error())
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected wrapped error to contain original, got %v", err)
	}
}

func TestSeedFeats_ErrorWrapping(t *testing.T) {
	dbErr := errors.New("exec failed")
	mock := &mockDBTX{errToReturn: dbErr}
	q := New(mock)

	err := seedFeats(context.Background(), q)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "upserting feat") {
		t.Fatalf("expected error to contain 'upserting feat', got %q", err.Error())
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected wrapped error to contain original, got %v", err)
	}
}

func TestSeedAll_ClassesErrorWrapping(t *testing.T) {
	dbErr := errors.New("class exec failed")
	mock := &mockDBTX{errToReturn: dbErr, failAfterN: WeaponCount + ArmorCount + ConditionCount}
	err := SeedAll(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "seeding classes") {
		t.Fatalf("expected error to contain 'seeding classes', got %q", err.Error())
	}
}

func TestSeedAll_RacesErrorWrapping(t *testing.T) {
	dbErr := errors.New("race exec failed")
	mock := &mockDBTX{errToReturn: dbErr, failAfterN: WeaponCount + ArmorCount + ConditionCount + ClassCount}
	err := SeedAll(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "seeding races") {
		t.Fatalf("expected error to contain 'seeding races', got %q", err.Error())
	}
}

func TestSeedAll_FeatsErrorWrapping(t *testing.T) {
	dbErr := errors.New("feat exec failed")
	mock := &mockDBTX{errToReturn: dbErr, failAfterN: WeaponCount + ArmorCount + ConditionCount + ClassCount + RaceCount}
	err := SeedAll(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "seeding feats") {
		t.Fatalf("expected error to contain 'seeding feats', got %q", err.Error())
	}
}

func TestListClasses_QueryError(t *testing.T) {
	dbErr := errors.New("query failed")
	mock := &mockDBTX{queryErr: dbErr}
	q := New(mock)

	_, err := q.ListClasses(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestListRaces_QueryError(t *testing.T) {
	dbErr := errors.New("query failed")
	mock := &mockDBTX{queryErr: dbErr}
	q := New(mock)

	_, err := q.ListRaces(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestListFeats_QueryError(t *testing.T) {
	dbErr := errors.New("query failed")
	mock := &mockDBTX{queryErr: dbErr}
	q := New(mock)

	_, err := q.ListFeats(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestSeedAll_NilDB(t *testing.T) {
	err := SeedAll(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil db, got nil")
	}
	if !strings.Contains(err.Error(), "database connection must not be nil") {
		t.Fatalf("expected nil db error message, got %q", err.Error())
	}
}

func TestSeedAll_Success(t *testing.T) {
	mock := &mockDBTX{}
	err := SeedAll(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	totalUpserts := WeaponCount + ArmorCount + ConditionCount + ClassCount + RaceCount + FeatCount
	if mock.callCount != totalUpserts {
		t.Fatalf("expected %d ExecContext calls, got %d", totalUpserts, mock.callCount)
	}
}

func TestListWeapons_QueryError(t *testing.T) {
	dbErr := errors.New("query failed")
	mock := &mockDBTX{queryErr: dbErr}
	q := New(mock)

	_, err := q.ListWeapons(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestListArmor_QueryError(t *testing.T) {
	dbErr := errors.New("query failed")
	mock := &mockDBTX{queryErr: dbErr}
	q := New(mock)

	_, err := q.ListArmor(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestListConditions_QueryError(t *testing.T) {
	dbErr := errors.New("query failed")
	mock := &mockDBTX{queryErr: dbErr}
	q := New(mock)

	_, err := q.ListConditions(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestMustJSON_Panic(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T", r)
		}
		if !strings.Contains(msg, "failed to marshal JSON") {
			t.Fatalf("expected panic message to contain 'failed to marshal JSON', got %q", msg)
		}
	}()

	// channels are not JSON-serializable and will cause json.Marshal to fail
	mustJSON(make(chan int))
}

func TestOptJSON(t *testing.T) {
	result := optJSON(map[string]int{"str": 13})
	if !result.Valid {
		t.Fatal("expected valid NullRawMessage")
	}
	var parsed map[string]int
	if err := json.Unmarshal(result.RawMessage, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if parsed["str"] != 13 {
		t.Fatalf("expected str 13, got %d", parsed["str"])
	}
}

func TestOptJSON_Panic(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
	}()

	optJSON(make(chan int))
}
