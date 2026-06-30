package characteroverview

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestApplyFeatureUses_Success_PersistsMergedMap(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store)
	existing := json.RawMessage(`{"rage":{"current":1,"max":3,"recharge":"long"}}`)

	uses, err := svc.ApplyFeatureUses(context.Background(), uuid.New(), existing,
		[]FeatureUseChange{{Feature: "rage", Current: 2}})
	if err != nil {
		t.Fatalf("ApplyFeatureUses: %v", err)
	}
	if got := uses["rage"]; got.Current != 2 || got.Max != 3 || got.Recharge != "long" {
		t.Fatalf("rage = %+v (Max/Recharge must be preserved)", got)
	}
	if store.persistedFeatureUses == nil {
		t.Fatal("expected feature_uses to be persisted")
	}
	if body := string(store.persistedFeatureUses.FeatureUses); !strings.Contains(body, `"current":2`) {
		t.Fatalf("persisted body = %s", body)
	}
}

func TestApplyFeatureUses_OnlyTouchesNamedFeature(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store)
	existing := json.RawMessage(`{"rage":{"current":1,"max":3,"recharge":"long"},"ki":{"current":0,"max":4,"recharge":"short"}}`)

	uses, err := svc.ApplyFeatureUses(context.Background(), uuid.New(), existing,
		[]FeatureUseChange{{Feature: "ki", Current: 4}})
	if err != nil {
		t.Fatalf("ApplyFeatureUses: %v", err)
	}
	if uses["rage"].Current != 1 {
		t.Fatalf("rage untouched expected current 1, got %+v", uses["rage"])
	}
	if uses["ki"].Current != 4 {
		t.Fatalf("ki = %+v", uses["ki"])
	}
}

func TestApplyFeatureUses_UnlimitedPool_OnlyLowerBound(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store)
	// max -1 marks an unlimited pool (e.g. L20 barbarian rage): any current >= 0 ok.
	existing := json.RawMessage(`{"rage":{"current":3,"max":-1,"recharge":"long"}}`)

	uses, err := svc.ApplyFeatureUses(context.Background(), uuid.New(), existing,
		[]FeatureUseChange{{Feature: "rage", Current: 99}})
	if err != nil {
		t.Fatalf("ApplyFeatureUses: %v", err)
	}
	if uses["rage"].Current != 99 {
		t.Fatalf("rage = %+v", uses["rage"])
	}
}

func TestApplyFeatureUses_ValidationErrors(t *testing.T) {
	existing := json.RawMessage(`{"rage":{"current":1,"max":3,"recharge":"long"}}`)
	cases := []struct {
		name    string
		changes []FeatureUseChange
	}{
		{"empty changes", nil},
		{"unknown feature", []FeatureUseChange{{Feature: "ki", Current: 1}}},
		{"negative current", []FeatureUseChange{{Feature: "rage", Current: -1}}},
		{"current exceeds max", []FeatureUseChange{{Feature: "rage", Current: 4}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeStore{}
			svc := NewService(store)
			_, err := svc.ApplyFeatureUses(context.Background(), uuid.New(), existing, tc.changes)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput", err)
			}
			if store.persistedFeatureUses != nil {
				t.Fatal("must not persist on validation failure")
			}
		})
	}
}

func TestApplyFeatureUses_ParseErrorNotInvalidInput(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store)
	_, err := svc.ApplyFeatureUses(context.Background(), uuid.New(),
		json.RawMessage(`{not json`), []FeatureUseChange{{Feature: "rage", Current: 2}})
	if err == nil || errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want a non-ErrInvalidInput parse error", err)
	}
}

func TestApplyFeatureUses_PersistErrorPropagates(t *testing.T) {
	store := &fakeStore{persistFeatureUsesErr: errors.New("db down")}
	svc := NewService(store)
	_, err := svc.ApplyFeatureUses(context.Background(), uuid.New(),
		json.RawMessage(`{"rage":{"current":1,"max":3,"recharge":"long"}}`),
		[]FeatureUseChange{{Feature: "rage", Current: 2}})
	if err == nil || errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want a wrapped persist error", err)
	}
}
