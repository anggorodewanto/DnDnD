package portal

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"sort"
	"strconv"

	"github.com/ab/dndnd/internal/auth"
	"github.com/go-chi/chi/v5"
)

// CharacterSheetLoader loads character data for the sheet view.
type CharacterSheetLoader interface {
	LoadCharacterSheet(ctx context.Context, characterID, userID string) (*CharacterSheetData, error)
}

// CharacterSheetHandler serves the character sheet page.
type CharacterSheetHandler struct {
	logger    *slog.Logger
	svc       CharacterSheetLoader
	sheetTmpl *template.Template
}

// NewCharacterSheetHandler creates a new CharacterSheetHandler.
func NewCharacterSheetHandler(logger *slog.Logger, svc CharacterSheetLoader) *CharacterSheetHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &CharacterSheetHandler{
		logger:    logger,
		svc:       svc,
		sheetTmpl: template.Must(template.New("sheet").Funcs(sheetFuncMap()).Parse(characterSheetTemplate)),
	}
}

// SetSheetTemplate overrides the character sheet template (for testing).
func (h *CharacterSheetHandler) SetSheetTemplate(t *template.Template) {
	h.sheetTmpl = t
}

// ServeCharacterSheet renders the full character sheet page.
func (h *CharacterSheetHandler) ServeCharacterSheet(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	characterID := chi.URLParam(r, "characterID")
	if characterID == "" {
		http.Error(w, "missing character ID", http.StatusBadRequest)
		return
	}

	data, err := h.svc.LoadCharacterSheet(r.Context(), characterID, userID)
	if err != nil {
		h.handleSheetError(w, err)
		return
	}

	h.renderSheet(w, data)
}

func (h *CharacterSheetHandler) handleSheetError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotOwner) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if errors.Is(err, ErrCharacterNotFound) {
		http.Error(w, "character not found", http.StatusNotFound)
		return
	}
	h.logger.Error("character sheet load error", "error", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func (h *CharacterSheetHandler) renderSheet(w http.ResponseWriter, data *CharacterSheetData) {
	var buf bytes.Buffer
	if err := h.sheetTmpl.Execute(&buf, data); err != nil {
		h.logger.Error("failed to render character sheet template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

// SpellLevelGroup groups spells by their level for template rendering.
type SpellLevelGroup struct {
	Level  int
	Label  string
	Spells []SpellDisplayEntry
}

func sheetFuncMap() template.FuncMap {
	return template.FuncMap{
		"formatModifier": func(mod int) string {
			if mod >= 0 {
				return "+" + template.HTMLEscapeString(strconv.Itoa(mod))
			}
			return template.HTMLEscapeString(strconv.Itoa(mod))
		},
		"profIcon": func(proficient bool) string {
			if proficient {
				return "●"
			}
			return "○"
		},
		"groupSpellsByLevel": groupSpellsByLevel,
	}
}

// groupSpellsByLevel organizes spells into groups by spell level, sorted ascending.
func groupSpellsByLevel(spells []SpellDisplayEntry) []SpellLevelGroup {
	if len(spells) == 0 {
		return nil
	}

	grouped := make(map[int][]SpellDisplayEntry)
	for _, s := range spells {
		grouped[s.Level] = append(grouped[s.Level], s)
	}

	levels := make([]int, 0, len(grouped))
	for lvl := range grouped {
		levels = append(levels, lvl)
	}
	sort.Ints(levels)

	result := make([]SpellLevelGroup, 0, len(levels))
	for _, lvl := range levels {
		label := "Level " + strconv.Itoa(lvl)
		if lvl == 0 {
			label = "Cantrips"
		}
		result = append(result, SpellLevelGroup{
			Level:  lvl,
			Label:  label,
			Spells: grouped[lvl],
		})
	}
	return result
}

const characterSheetTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>DnDnD — {{.Name}}</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: system-ui, -apple-system, sans-serif; min-height: 100vh; background: #1a1a2e; color: #e0e0e0; }
        .header { background: #16213e; border-bottom: 1px solid #0f3460; padding: 1rem 2rem; display: flex; align-items: center; justify-content: space-between; }
        .header h1 { color: #e94560; font-size: 1.4rem; }
        .header nav { display: flex; gap: 1.5rem; }
        .header nav a { color: #e0e0e0; text-decoration: none; padding: 0.5rem 0; }
        .header nav a:hover { color: #e94560; }
        .main { max-width: 960px; margin: 1.5rem auto; padding: 0 1rem; }
        .char-header { background: #16213e; border-radius: 8px; padding: 1.5rem; border: 1px solid #0f3460; margin-bottom: 1rem; }
        .char-name { color: #e94560; font-size: 1.8rem; margin-bottom: 0.25rem; }
        .char-subtitle { color: #a0a0b0; font-size: 1rem; }
        .stats-bar { display: grid; grid-template-columns: repeat(auto-fit, minmax(120px, 1fr)); gap: 0.75rem; margin-bottom: 1rem; }
        .stat-box { background: #16213e; border-radius: 8px; padding: 1rem; border: 1px solid #0f3460; text-align: center; }
        .stat-label { color: #a0a0b0; font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em; }
        .stat-value { color: #e94560; font-size: 1.5rem; font-weight: bold; }
        .section { background: #16213e; border-radius: 8px; padding: 1.5rem; border: 1px solid #0f3460; margin-bottom: 1rem; }
        .section h3 { color: #e94560; margin-bottom: 0.75rem; font-size: 1.1rem; border-bottom: 1px solid #0f3460; padding-bottom: 0.5rem; }
        .grid-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; }
        .ability-grid { display: grid; grid-template-columns: repeat(6, 1fr); gap: 0.5rem; margin-bottom: 1rem; }
        .ability-box { background: #1a1a2e; border-radius: 8px; padding: 0.75rem; border: 1px solid #0f3460; text-align: center; }
        .ability-name { color: #a0a0b0; font-size: 0.7rem; text-transform: uppercase; letter-spacing: 0.05em; }
        .ability-score { color: #e0e0e0; font-size: 1.3rem; font-weight: bold; }
        .ability-mod { color: #e94560; font-size: 0.9rem; }
        .skill-list, .save-list { list-style: none; }
        .skill-list li, .save-list li { display: flex; justify-content: space-between; padding: 0.25rem 0; border-bottom: 1px solid #0f346033; font-size: 0.9rem; }
        .prof-dot { margin-right: 0.5rem; }
        .prof-yes { color: #e94560; }
        .prof-no { color: #555; }
        .mod-value { font-weight: bold; color: #e94560; min-width: 2rem; text-align: right; }
        .feature { margin-bottom: 0.75rem; }
        .feature-name { color: #e94560; font-weight: bold; }
        .feature-source { color: #a0a0b0; font-size: 0.8rem; margin-left: 0.5rem; }
        .feature-desc { color: #c0c0d0; font-size: 0.9rem; margin-top: 0.25rem; }
        .item-row { display: flex; justify-content: space-between; padding: 0.3rem 0; border-bottom: 1px solid #0f346033; }
        .item-name { }
        .item-equipped { color: #e94560; font-size: 0.8rem; }
        .item-magic { color: #9b59b6; }
        .slot-row { display: flex; justify-content: space-between; padding: 0.25rem 0; }
        .lang-list { display: flex; flex-wrap: wrap; gap: 0.5rem; }
        .lang-tag { background: #1a1a2e; border: 1px solid #0f3460; border-radius: 4px; padding: 0.25rem 0.5rem; font-size: 0.85rem; }
        .attune-item { padding: 0.25rem 0; }
        .empty-msg { color: #666; font-style: italic; }
        @media (max-width: 700px) {
            .grid-2 { grid-template-columns: 1fr; }
            .ability-grid { grid-template-columns: repeat(3, 1fr); }
            .header { flex-direction: column; gap: 0.5rem; text-align: center; }
            .main { padding: 0 0.5rem; }
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>DnDnD Player Portal</h1>
        <nav>
            <a href="/portal/">Home</a>
            <a href="/portal/auth/logout">Logout</a>
        </nav>
    </div>
    <div class="main">
        <div class="char-header">
            <div class="char-name">{{.Name}}</div>
            <div class="char-subtitle">Level {{.Level}} {{.Race}} — {{.ClassSummary}}</div>
        </div>

        <div class="stats-bar">
            <div class="stat-box">
                <div class="stat-label">HP</div>
                <div class="stat-value">{{.HpCurrent}}/{{.HpMax}}</div>
                {{if gt .TempHP 0}}<div class="stat-label">+{{.TempHP}} temp</div>{{end}}
            </div>
            <div class="stat-box">
                <div class="stat-label">AC</div>
                <div class="stat-value">{{.AC}}</div>
            </div>
            <div class="stat-box">
                <div class="stat-label">Speed</div>
                <div class="stat-value">{{.SpeedFt}}ft</div>
            </div>
            <div class="stat-box">
                <div class="stat-label">Prof. Bonus</div>
                <div class="stat-value">+{{.ProficiencyBonus}}</div>
            </div>
            <div class="stat-box">
                <div class="stat-label">Gold</div>
                <div class="stat-value">{{.Gold}}gp</div>
            </div>
        </div>

        {{if .HitDiceRemaining}}
        <div class="section">
            <h3>Hit Dice</h3>
            {{range $die, $remaining := .HitDiceRemaining}}
            <div class="slot-row">
                <span>{{$die}}</span>
                <span>{{$remaining}} remaining</span>
            </div>
            {{end}}
        </div>
        {{end}}

        <div class="ability-grid">
            <div class="ability-box">
                <div class="ability-name">STR</div>
                <div class="ability-score">{{.AbilityScores.STR}}</div>
                <div class="ability-mod">{{formatModifier (index .AbilityModifiers "STR")}}</div>
            </div>
            <div class="ability-box">
                <div class="ability-name">DEX</div>
                <div class="ability-score">{{.AbilityScores.DEX}}</div>
                <div class="ability-mod">{{formatModifier (index .AbilityModifiers "DEX")}}</div>
            </div>
            <div class="ability-box">
                <div class="ability-name">CON</div>
                <div class="ability-score">{{.AbilityScores.CON}}</div>
                <div class="ability-mod">{{formatModifier (index .AbilityModifiers "CON")}}</div>
            </div>
            <div class="ability-box">
                <div class="ability-name">INT</div>
                <div class="ability-score">{{.AbilityScores.INT}}</div>
                <div class="ability-mod">{{formatModifier (index .AbilityModifiers "INT")}}</div>
            </div>
            <div class="ability-box">
                <div class="ability-name">WIS</div>
                <div class="ability-score">{{.AbilityScores.WIS}}</div>
                <div class="ability-mod">{{formatModifier (index .AbilityModifiers "WIS")}}</div>
            </div>
            <div class="ability-box">
                <div class="ability-name">CHA</div>
                <div class="ability-score">{{.AbilityScores.CHA}}</div>
                <div class="ability-mod">{{formatModifier (index .AbilityModifiers "CHA")}}</div>
            </div>
        </div>

        <div class="grid-2">
            <div class="section">
                <h3>Saving Throws</h3>
                <ul class="save-list">
                {{range .SavingThrows}}
                    <li>
                        <span>
                            <span class="prof-dot {{if .Proficient}}prof-yes{{else}}prof-no{{end}}">{{profIcon .Proficient}}</span>
                            {{.Ability}}
                        </span>
                        <span class="mod-value">{{formatModifier .Modifier}}</span>
                    </li>
                {{end}}
                </ul>
            </div>
            <div class="section">
                <h3>Skills</h3>
                <ul class="skill-list">
                {{range .Skills}}
                    <li>
                        <span>
                            <span class="prof-dot {{if .Proficient}}prof-yes{{else}}prof-no{{end}}">{{profIcon .Proficient}}</span>
                            {{.Name}} <span style="color:#666">({{.Ability}})</span>
                        </span>
                        <span class="mod-value">{{formatModifier .Modifier}}</span>
                    </li>
                {{end}}
                </ul>
            </div>
        </div>

        <div class="section">
            <h3>Equipment</h3>
            <div style="margin-bottom: 0.5rem;">
                <strong>Main Hand:</strong> {{if .EquippedMainHand}}{{.EquippedMainHand}}{{else}}<span class="empty-msg">empty</span>{{end}} |
                <strong>Off Hand:</strong> {{if .EquippedOffHand}}{{.EquippedOffHand}}{{else}}<span class="empty-msg">empty</span>{{end}} |
                <strong>Armor:</strong> {{if .EquippedArmor}}{{.EquippedArmor}}{{else}}<span class="empty-msg">none</span>{{end}}
            </div>
        </div>

        <div class="section">
            <h3>Languages</h3>
            {{if .Languages}}
            <div class="lang-list">
                {{range .Languages}}<span class="lang-tag">{{.}}</span>{{end}}
            </div>
            {{else}}
            <p class="empty-msg">None</p>
            {{end}}
        </div>

        <div class="section">
            <h3>Features</h3>
            {{if .Features}}
            {{range .Features}}
            <div class="feature">
                <span class="feature-name">{{.Name}}</span>
                {{if .Source}}<span class="feature-source">({{.Source}}, Level {{.Level}})</span>{{end}}
                {{if .Description}}<div class="feature-desc">{{.Description}}</div>{{end}}
                {{if .MechanicalEffect}}<div class="feature-desc"><em>{{.MechanicalEffect}}</em></div>{{end}}
            </div>
            {{end}}
            {{else}}
            <p class="empty-msg">No features</p>
            {{end}}
        </div>

        {{if .FeatureUses}}
        <div class="section">
            <h3>Feature Uses</h3>
            {{range $name, $use := .FeatureUses}}
            <div class="slot-row">
                <span>{{$name}}</span>
                <span>{{$use.Current}}/{{$use.Max}} ({{$use.Recharge}})</span>
            </div>
            {{end}}
        </div>
        {{end}}

        {{if .SpellSlots}}
        <div class="section">
            <h3>Spell Slots</h3>
            {{range $level, $slot := .SpellSlots}}
            <div class="slot-row">
                <span>Level {{$level}}</span>
                <span>{{$slot.Current}}/{{$slot.Max}}</span>
            </div>
            {{end}}
        </div>
        {{end}}

        {{if .PactMagicSlots}}
        <div class="section">
            <h3>Pact Magic</h3>
            <div class="slot-row">
                <span>Level {{.PactMagicSlots.SlotLevel}} Pact Slots</span>
                <span>{{.PactMagicSlots.Current}}/{{.PactMagicSlots.Max}}</span>
            </div>
        </div>
        {{end}}

        {{if .Spells}}
        <div class="section">
            <h3>Spell List</h3>
            {{range groupSpellsByLevel .Spells}}
            <div style="margin-bottom: 0.75rem;">
                <div style="color: #e94560; font-weight: bold; margin-bottom: 0.25rem;">{{.Label}}</div>
                {{range .Spells}}
                <div class="item-row">
                    <span>
                        {{if .Prepared}}<span class="prof-dot prof-yes">●</span>{{end}}
                        {{.Name}}
                        {{if .School}}<span style="color:#a0a0b0; font-size:0.8rem;">({{.School}})</span>{{end}}
                        {{if .Homebrew}}<span style="color:#ffd166; font-size:0.75rem; margin-left:0.35rem;">Homebrew</span>{{end}}
                        {{if .OffList}}<span style="color:#ffd166; font-size:0.75rem; margin-left:0.35rem;">Off-list</span>{{end}}
                    </span>
                    <span style="color:#a0a0b0; font-size:0.8rem;">
                        {{if .CastingTime}}{{.CastingTime}}{{end}}
                        {{if .Range}} | {{.Range}}{{end}}
                    </span>
                </div>
                {{end}}
            </div>
            {{end}}
        </div>
        {{end}}

        <div class="section">
            <h3>Inventory</h3>
            {{if .Inventory}}
            {{range .Inventory}}
            <div class="item-row">
                <span class="item-name {{if .IsMagic}}item-magic{{end}}">
                    {{.Name}}
                    {{if gt .Quantity 1}}(x{{.Quantity}}){{end}}
                    {{if .IsMagic}}✦{{end}}
                    {{if .Homebrew}}<span style="color:#ffd166; font-size:0.75rem; margin-left:0.35rem;">Homebrew</span>{{end}}
                    {{if .Source}}<span style="color:#a0a0b0; font-size:0.75rem; margin-left:0.35rem;">{{.Source}}</span>{{end}}
                </span>
                <span>
                    {{if .Equipped}}<span class="item-equipped">Equipped</span>{{end}}
                    {{if .Rarity}}<span style="color:#a0a0b0; font-size:0.8rem;">{{.Rarity}}</span>{{end}}
                </span>
            </div>
            {{end}}
            {{else}}
            <p class="empty-msg">No items</p>
            {{end}}
        </div>

        {{if .AttunementSlots}}
        <div class="section">
            <h3>Attunement</h3>
            {{range .AttunementSlots}}
            <div class="attune-item">{{.Name}}</div>
            {{end}}
        </div>
        {{end}}
    </div>
</body>
</html>`
