package refdata

import (
	"sort"
	"testing"
)

// knownExceptions documents seeded spells whose hand-coded ResolutionMode
// disagrees with ClassifyResolutionMode. Each entry is an explicit audit
// note explaining why the human-assigned mode is correct.
//
// Two failure modes:
//
//  1. seed=auto, algo=dm_required — buff/utility spells with mechanical
//     effect (AC, HP, advantage, speed, condition cure) but no Damage /
//     Healing / Save / AttackType field. The algorithm cannot tell these
//     apart from RP-only utilities.
//
//  2. seed=dm_required, algo=auto — save-based charm / domination /
//     illusion / social spells. They have a save, but the consequence of
//     failing (changed behavior, altered memory, lost identity) needs DM
//     adjudication, not engine resolution.
//
// New spells SHOULD align with the algorithm. If they cannot, add a row
// here with a short reason. The size of this map (audit baseline ~122)
// captures Phase-5-era seed authorship; the value of the test is that
// any FUTURE mismatch outside this map fails the build.
var knownExceptions = map[string]string{
	// --- seed=auto, algo=dm_required: mechanical buffs / debuffs with no Damage/Save/Attack ---
	"absorb-elements":               "auto: mechanical resistance to triggering damage",
	"aid":                           "auto: deterministic +5 HP max & current",
	"antilife-shell":                "auto: engine can enforce 10-ft anti-creature barrier",
	"arcane-gate":                   "auto: portal opens between two engine-tracked points",
	"aura-of-life":                  "auto: resistance/HP regen aura, mechanical",
	"aura-of-purity":                "auto: condition suppression aura, mechanical",
	"barkskin":                      "auto: AC floor of 16, mechanical",
	"beacon-of-hope":                "auto: advantage on WIS + death saves, max heal — mechanical",
	"blade-ward":                    "auto: resistance to bludgeoning/piercing/slashing",
	"bless":                         "auto: +1d4 to attacks & saves — mechanical bonus",
	"blink":                         "auto: 50% chance to vanish each turn — mechanical roll",
	"blur":                          "auto: disadvantage on attacks against caster",
	"circle-of-power":               "auto: advantage on magic saves within 30 ft",
	"color-spray":                   "auto: applies blinded by HP-pool mechanic",
	"continual-flame":               "auto: deterministic light source",
	"counterspell":                  "auto: spell-vs-spell ability check, mechanical",
	"create-food-and-water":         "auto: deterministic resource creation",
	"darkvision":                    "auto: grants 60ft darkvision — mechanical sense",
	"daylight":                      "auto: 60-ft sphere of bright light, mechanical",
	"death-ward":                    "auto: drops to 1 HP instead of 0 once, mechanical",
	"dimension-door":                "auto: 500-ft teleport, mechanical position change",
	"dispel-evil-and-good":          "auto: advantage on attacks vs specified creatures, mechanical",
	"dispel-magic":                  "auto: spell-vs-spell ability check, mechanical",
	"divine-word":                   "auto: HP-pool table → conditions, mechanical",
	"elemental-weapon":              "auto: +1 to hit and bonus damage die, mechanical",
	"enhance-ability":               "auto: advantage on ability checks, mechanical",
	"expeditious-retreat":           "auto: Dash as bonus action, mechanical",
	"false-life":                    "auto: 1d4+4 temp HP, mechanical",
	"far-step":                      "auto: 60-ft teleport bonus action, mechanical",
	"feather-fall":                  "auto: descent slows to 60 ft/round, mechanical",
	"feign-death":                   "auto: target appears dead with set states, mechanical",
	"fly":                           "auto: 60-ft flying speed, mechanical",
	"foresight":                     "auto: advantage on attacks/checks/saves, mechanical",
	"freedom-of-movement":           "auto: immunity to speed reduction, mechanical",
	"glibness":                      "auto: floor of 15 on Charisma checks, mechanical",
	"globe-of-invulnerability":      "auto: blocks spells of level <= 5, mechanical",
	"greater-invisibility":          "auto: full invisibility + concentration, mechanical",
	"greater-restoration":           "auto: removes specified conditions, mechanical",
	"guidance":                      "auto: +1d4 to one ability check, mechanical",
	"haste":                         "auto: +2 AC, doubled speed, extra action, mechanical",
	"heroes-feast":                  "auto: temp HP + immunities, mechanical",
	"heroism":                       "auto: immune to frightened + temp HP, mechanical",
	"holy-aura":                     "auto: advantage on saves + disadvantage on attacks vs allies, mechanical",
	"invisibility":                  "auto: target turns invisible, mechanical",
	"jump":                          "auto: jump distance tripled, mechanical",
	"lesser-restoration":            "auto: removes one disease or condition, mechanical",
	"light":                         "auto: deterministic light source, mechanical",
	"longstrider":                   "auto: +10 ft speed, mechanical",
	"mage-armor":                    "auto: base AC becomes 13 + Dex, mechanical",
	"magic-weapon":                  "auto: +1 / +2 / +3 to attack & damage rolls, mechanical",
	"maze":                          "auto: target removed to demiplane, INT check to escape, mechanical",
	"mind-blank":                    "auto: immunity to psychic damage & charm, mechanical",
	"mirror-image":                  "auto: duplicates redirect attacks, mechanical roll",
	"misty-step":                    "auto: 30-ft teleport bonus action, mechanical",
	"nondetection":                  "auto: target hidden from divination, mechanical",
	"pass-without-trace":            "auto: +10 to Stealth checks, mechanical",
	"power-word-kill":               "auto: HP-threshold kill (<= 100 HP), mechanical",
	"power-word-stun":               "auto: HP-threshold stun (<= 150 HP), mechanical",
	"protection-from-energy":        "auto: resistance to chosen damage type, mechanical",
	"protection-from-evil-and-good": "auto: disadvantage on attacks from named creature types, mechanical",
	"protection-from-poison":        "auto: advantage on saves vs poison + resistance, mechanical",
	"purify-food-and-drink":         "auto: deterministic food cleansing in radius, mechanical",
	"raise-dead":                    "auto: dead -> alive with -4 penalty for 4 long rests, mechanical",
	"remove-curse":                  "auto: removes curses on target, mechanical",
	"resistance":                    "auto: +1d4 to one saving throw, mechanical",
	"resurrection":                  "auto: target returns to life, mechanical",
	"see-invisibility":              "auto: caster sees invisible + ethereal creatures, mechanical",
	"shield":                        "auto: +5 AC reaction + immune to magic missile, mechanical",
	"shield-of-faith":               "auto: +2 AC concentration, mechanical",
	"shillelagh":                    "auto: weapon damage die becomes d8 + spell mod, mechanical",
	"sleep":                         "auto: HP-pool unconscious mechanic, mechanical",
	"spare-the-dying":               "auto: 0-HP creature becomes stable, mechanical",
	"spider-climb":                  "auto: climbing speed = walking speed, mechanical",
	"stoneskin":                     "auto: resistance to non-magical b/p/s damage, mechanical",
	"swift-quiver":                  "auto: extra two attacks per turn, mechanical",
	"teleportation-circle":          "auto: deterministic portal to permanent sigil, mechanical",
	"tongues":                       "auto: target understands all spoken languages, mechanical",
	"transport-via-plants":          "auto: deterministic transport between plants, mechanical",
	"tree-stride":                   "auto: tree-to-tree teleport per turn, mechanical",
	"true-resurrection":             "auto: target restored fully, mechanical",
	"true-seeing":                   "auto: target sees through illusions/invisibility, mechanical",
	"true-strike":                   "auto: advantage on first attack next turn, mechanical",
	"warding-bond":                  "auto: shared damage + +1 AC/saves + resistance, mechanical",
	"water-breathing":               "auto: breathe underwater for 24h, mechanical",
	"water-walk":                    "auto: walk on liquid surfaces, mechanical",
	"wind-walk":                     "auto: gaseous form 300 ft fly, mechanical",
	"word-of-recall":                "auto: deterministic teleport to sanctuary, mechanical",

	// --- seed=dm_required, algo=auto: save mechanics with DM-interpreted consequences ---
	"animal-friendship":  "dm_required: charmed beast behavior, RP/DM call",
	"antipathy-sympathy": "dm_required: target seeks/avoids area, DM movement/RP",
	"bestow-curse":       "dm_required: curse choice picks one of four DM-tracked effects",
	"calm-emotions":      "dm_required: suppresses charmed/frightened OR pacifies — DM call",
	"charm-person":       "dm_required: charmed humanoid attitude shift, DM RP",
	"command":            "dm_required: caster speaks 1-word command; DM adjudicates interpretation",
	"compulsion":         "dm_required: caster directs movement each round, DM positioning",
	"confusion":          "dm_required: random behavior table needs DM narration",
	"contagion":          "dm_required: caster picks one of seven diseases, DM-tracked symptoms",
	"crown-of-madness":   "dm_required: caster directs target's attacks each round, DM call",
	"detect-thoughts":    "dm_required: caster reads thoughts — DM provides info",
	"dominate-beast":     "dm_required: caster issues commands telepathically, DM RP",
	"dominate-monster":   "dm_required: caster issues commands telepathically, DM RP",
	"dominate-person":    "dm_required: caster issues commands telepathically, DM RP",
	"earthquake":         "dm_required: terrain-changing effect, fissures/structures collapse — DM call",
	"enthrall":           "dm_required: disadvantage on Perception, DM judges audibility",
	"geas":               "dm_required: caster issues a command/restraint, DM enforces",
	"gust-of-wind":       "dm_required: disperses gas/blocks flight/pushes — DM-judged terrain effects",
	"imprisonment":       "dm_required: caster picks 1 of 5 imprisonment forms, DM-tracked",
	"mass-suggestion":    "dm_required: AoE Suggestion variant, DM RP",
	"modify-memory":      "dm_required: caster rewrites memories, DM narrates",
	"phantasmal-force":   "dm_required: caster crafts illusion, DM rules on plausibility",
	"planar-binding":     "dm_required: bound creature serves caster, DM RP",
	"polymorph":          "dm_required: target transforms into chosen CR-capped beast, DM stat block",
	"reverse-gravity":    "dm_required: terrain/position chaos, DM adjudicates falling damage",
	"scrying":            "dm_required: caster observes a target, DM narrates",
	"seeming":            "dm_required: appearance change for many creatures, DM RP",
	"sleet-storm":        "dm_required: difficult terrain + concentration + obscure, DM movement calls",
	"suggestion":         "dm_required: caster suggests a course of action, DM RP",
	"true-polymorph":     "dm_required: permanent shape change, DM stat block / RP",
	"tsunami":            "dm_required: terrain-altering wave, DM call on objects/structures",
	"wall-of-ice":        "dm_required: wall creates terrain, DM call on movement/breaks",
	"wall-of-thorns":     "dm_required: wall creates terrain, DM call on traversal damage",
	"wind-wall":          "dm_required: blocks gas/arrows/small creatures, DM environmental calls",
	"zone-of-truth":      "dm_required: detects lying in zone, DM enforces honesty",
}

// TestClassifyResolutionMode_SeedInvariant asserts every seeded spell's
// ResolutionMode matches ClassifyResolutionMode, except for an explicit
// audit list. The test catches drift: any new spell whose resolution
// mode disagrees with the algorithm must be added to knownExceptions
// (with a justification) or its mode corrected.
func TestClassifyResolutionMode_SeedInvariant(t *testing.T) {
	seeds := srdSpells()
	type mm struct{ id, want, got string }
	var unexpected []mm
	allIDs := make(map[string]bool, len(seeds))

	for _, s := range seeds {
		allIDs[s.ID] = true
		got := ClassifyResolutionMode(s)
		_, isException := knownExceptions[s.ID]
		if got == s.ResolutionMode {
			if isException {
				t.Errorf("spell %q is in knownExceptions but now matches algorithm — remove the exception", s.ID)
			}
			continue
		}
		if isException {
			continue
		}
		unexpected = append(unexpected, mm{s.ID, s.ResolutionMode, got})
	}

	// Detect stale exception entries (spell removed/renamed).
	var stale []string
	for id := range knownExceptions {
		if !allIDs[id] {
			stale = append(stale, id)
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Errorf("knownExceptions has %d stale entries (no matching seed): %v", len(stale), stale)
	}

	if len(unexpected) > 0 {
		sort.Slice(unexpected, func(i, j int) bool { return unexpected[i].id < unexpected[j].id })
		for _, m := range unexpected {
			t.Errorf("ResolutionMode mismatch for %q: seed=%s algorithm=%s — fix the seed or add to knownExceptions with a justification", m.id, m.want, m.got)
		}
	}
}

// TestClassifyResolutionMode_Algorithm sanity-checks the classifier on
// hand-crafted inputs covering each branch.
func TestClassifyResolutionMode_Algorithm(t *testing.T) {
	cases := []struct {
		name string
		seed sp
		want string
	}{
		{"damage", sp{Damage: optJSON(map[string]any{"dice": "1d6"})}, "auto"},
		{"healing", sp{Healing: optJSON(map[string]any{"dice": "1d4"})}, "auto"},
		{"attack", sp{AttackType: optStr("ranged")}, "auto"},
		{"save+effect", sp{SaveAbility: optStr("dex"), SaveEffect: optStr("half_damage")}, "auto"},
		{"save only (no effect)", sp{SaveAbility: optStr("dex")}, "dm_required"},
		{"effect only (no ability)", sp{SaveEffect: optStr("half_damage")}, "dm_required"},
		{"bare utility", sp{}, "dm_required"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClassifyResolutionMode(c.seed)
			if got != c.want {
				t.Errorf("ClassifyResolutionMode(%s) = %q, want %q", c.name, got, c.want)
			}
		})
	}
}
