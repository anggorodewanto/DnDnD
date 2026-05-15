// Package open5e integrates the Open5e extended content API
// (https://api.open5e.com/) into DnDnD. It provides a thin HTTP client,
// a cache that writes fetched content into the refdata creatures/spells
// tables with a source attribution like "open5e:<document_slug>", and
// HTTP handlers the DM dashboard can drive.
//
// The cache deliberately stores Open5e rows globally (campaign_id NULL)
// so that every campaign sees the same cached payload once fetched.
// Per-campaign visibility is enforced by the statblocklibrary filter,
// which only surfaces open5e:* rows whose document slug appears in the
// campaign settings' open5e_sources list.
package open5e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client is a thin wrapper around http.Client for the Open5e REST API.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient returns a Client. An empty baseURL defaults to the public
// Open5e endpoint; a nil httpClient defaults to http.DefaultClient.
// The baseURL always ends with a single trailing slash.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = "https://api.open5e.com/v1/"
	}
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{http: httpClient, baseURL: baseURL}
}

// SearchQuery is the set of filters accepted by list endpoints.
type SearchQuery struct {
	Search       string
	DocumentSlug string
	Limit        int
	Offset       int
}

// MonsterListResponse is a paginated Open5e monster list.
type MonsterListResponse struct {
	Count   int       `json:"count"`
	Results []Monster `json:"results"`
}

// SpellListResponse is a paginated Open5e spell list.
type SpellListResponse struct {
	Count   int     `json:"count"`
	Results []Spell `json:"results"`
}

// Monster is the subset of the Open5e monster schema that DnDnD maps
// into the creatures table. Fields not listed here are intentionally
// dropped because they have no column mapping in the DnDnD schema.
type Monster struct {
	Slug         string          `json:"slug"`
	Name         string          `json:"name"`
	Size         string          `json:"size"`
	Type         string          `json:"type"`
	Alignment    string          `json:"alignment"`
	ArmorClass   int             `json:"armor_class"`
	ArmorDesc    string          `json:"armor_desc"`
	HitPoints    int             `json:"hit_points"`
	HitDice      string          `json:"hit_dice"`
	Speed        json.RawMessage `json:"speed"`
	Strength     int             `json:"strength"`
	Dexterity    int             `json:"dexterity"`
	Constitution int             `json:"constitution"`
	Intelligence int             `json:"intelligence"`
	Wisdom       int             `json:"wisdom"`
	Charisma     int             `json:"charisma"`
	// ChallengeRating is a string because Open5e encodes fractional CR
	// like "1/4" (matches DnDnD's refdata.creatures.cr).
	ChallengeRating  string          `json:"challenge_rating"`
	Actions          json.RawMessage `json:"actions"`
	SpecialAbilities json.RawMessage `json:"special_abilities"`
	// DocumentSlug identifies the publishing book, e.g. "tome-of-beasts".
	// Used to build the creatures.source column: "open5e:<DocumentSlug>".
	DocumentSlug string `json:"document__slug"`
}

// Spell is the subset of the Open5e spell schema DnDnD maps into spells.
type Spell struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	LevelInt      int    `json:"level_int"`
	School        string `json:"school"`
	CastingTime   string `json:"casting_time"`
	Range         string `json:"range"`
	Duration      string `json:"duration"`
	Components    string `json:"components"`
	Material      string `json:"material"`
	Ritual        bool   `json:"ritual"`
	Concentration bool   `json:"concentration"`
	Description   string `json:"desc"`
	HigherLevel   string `json:"higher_level"`
	// DndClass is comma-joined in Open5e ("Sorcerer, Wizard").
	DndClass     string `json:"dnd_class"`
	DocumentSlug string `json:"document__slug"`
}

// SearchMonsters calls GET {base}/monsters/ with the given filters.
func (c *Client) SearchMonsters(ctx context.Context, q SearchQuery) (MonsterListResponse, error) {
	var resp MonsterListResponse
	err := c.doJSON(ctx, "monsters/", searchParams(q), &resp)
	return resp, err
}

// SearchSpells calls GET {base}/spells/ with the given filters.
func (c *Client) SearchSpells(ctx context.Context, q SearchQuery) (SpellListResponse, error) {
	var resp SpellListResponse
	err := c.doJSON(ctx, "spells/", searchParams(q), &resp)
	return resp, err
}

// GetMonster calls GET {base}/monsters/{slug}/.
func (c *Client) GetMonster(ctx context.Context, slug string) (Monster, error) {
	if strings.TrimSpace(slug) == "" {
		return Monster{}, errors.New("open5e: empty monster slug")
	}
	var m Monster
	err := c.doJSON(ctx, "monsters/"+url.PathEscape(slug)+"/", nil, &m)
	return m, err
}

// GetSpell calls GET {base}/spells/{slug}/.
func (c *Client) GetSpell(ctx context.Context, slug string) (Spell, error) {
	if strings.TrimSpace(slug) == "" {
		return Spell{}, errors.New("open5e: empty spell slug")
	}
	var s Spell
	err := c.doJSON(ctx, "spells/"+url.PathEscape(slug)+"/", nil, &s)
	return s, err
}

// doJSON performs a GET request and decodes the JSON body into out.
func (c *Client) doJSON(ctx context.Context, path string, params url.Values, out any) error {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("open5e: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("open5e: request %s: %w", u, err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("open5e: GET %s returned %d: %s", u, res.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("open5e: decode %s: %w", u, err)
	}
	return nil
}

// searchParams builds url.Values for the list endpoints. Empty fields
// are omitted so the server-side defaults apply.
func searchParams(q SearchQuery) url.Values {
	v := url.Values{}
	if q.Search != "" {
		v.Set("search", q.Search)
	}
	if q.DocumentSlug != "" {
		v.Set("document__slug", q.DocumentSlug)
	}
	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.Offset > 0 {
		v.Set("offset", strconv.Itoa(q.Offset))
	}
	return v
}
