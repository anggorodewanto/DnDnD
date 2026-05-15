package dashboard

import (
	"testing"

	"github.com/ab/dndnd/internal/character"
)

func TestRefDataFeatureProvider_ImplementsInterface(t *testing.T) {
	// Verify the type satisfies the FeatureProvider interface at compile time.
	var _ FeatureProvider = (*RefDataFeatureProvider)(nil)
}

func TestRefDataFeatureProvider_EmptyMaps(t *testing.T) {
	fp := &RefDataFeatureProvider{
		classFeatures:    make(map[string]map[string][]character.Feature),
		subclassFeatures: make(map[string]map[string]map[string][]character.Feature),
		races:            make(map[string][]character.Feature),
	}
	if cf := fp.ClassFeatures(); cf == nil {
		t.Error("ClassFeatures() should not be nil")
	}
	if sf := fp.SubclassFeatures(); sf == nil {
		t.Error("SubclassFeatures() should not be nil")
	}
	if rt := fp.RacialTraits("elf"); rt != nil {
		t.Errorf("RacialTraits for unknown race should be nil, got %v", rt)
	}
}
