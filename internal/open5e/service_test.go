package open5e

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_SearchAndCacheMonster_EndToEnd(t *testing.T) {
	server := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/monsters/aboleth/": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"slug":"aboleth","name":"Aboleth","size":"Large","type":"aberration","armor_class":17,"hit_points":135,"hit_dice":"18d10+36","challenge_rating":"10","document__slug":"tome-of-beasts"}`)
		},
	})
	defer server.Close()

	store := &fakeCacheStore{}
	svc := NewService(NewClient(server.URL+"/", nil), NewCache(store))

	id, err := svc.SearchAndCacheMonster(context.Background(), "aboleth")
	require.NoError(t, err)
	assert.Equal(t, "open5e_aboleth", id)
	require.Len(t, store.creatures, 1)
	assert.Equal(t, "open5e:tome-of-beasts", store.creatures[0].Source.String)
}

func TestService_SearchAndCacheMonster_GetError(t *testing.T) {
	server := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/monsters/missing/": func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusNotFound)
		},
	})
	defer server.Close()
	svc := NewService(NewClient(server.URL+"/", nil), NewCache(&fakeCacheStore{}))
	_, err := svc.SearchAndCacheMonster(context.Background(), "missing")
	require.Error(t, err)
}

func TestService_SearchAndCacheSpell_EndToEnd(t *testing.T) {
	server := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/spells/fireball/": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"slug":"fireball","name":"Fireball","level_int":3,"school":"evocation","document__slug":"deep-magic"}`)
		},
	})
	defer server.Close()
	store := &fakeCacheStore{}
	svc := NewService(NewClient(server.URL+"/", nil), NewCache(store))

	id, err := svc.SearchAndCacheSpell(context.Background(), "fireball")
	require.NoError(t, err)
	assert.Equal(t, "open5e_fireball", id)
	require.Len(t, store.spells, 1)
	assert.Equal(t, "open5e:deep-magic", store.spells[0].Source.String)
}

func TestService_SearchAndCacheSpell_GetError(t *testing.T) {
	server := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/spells/missing/": func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusNotFound)
		},
	})
	defer server.Close()
	svc := NewService(NewClient(server.URL+"/", nil), NewCache(&fakeCacheStore{}))
	_, err := svc.SearchAndCacheSpell(context.Background(), "missing")
	require.Error(t, err)
}

func TestService_SearchMonsters_ProxiesWithoutCaching(t *testing.T) {
	server := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/monsters/": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"count":1,"results":[{"slug":"aboleth","name":"Aboleth","document__slug":"tome-of-beasts"}]}`)
		},
	})
	defer server.Close()
	store := &fakeCacheStore{}
	svc := NewService(NewClient(server.URL+"/", nil), NewCache(store))

	res, err := svc.SearchMonsters(context.Background(), SearchQuery{Search: "abol"})
	require.NoError(t, err)
	require.Len(t, res.Results, 1)
	// Proxy — nothing written.
	assert.Empty(t, store.creatures)
}

func TestService_SearchSpells_ProxiesWithoutCaching(t *testing.T) {
	server := newMockAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"/spells/": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"count":1,"results":[{"slug":"fireball","name":"Fireball","level_int":3,"document__slug":"wotc-srd"}]}`)
		},
	})
	defer server.Close()
	store := &fakeCacheStore{}
	svc := NewService(NewClient(server.URL+"/", nil), NewCache(store))

	res, err := svc.SearchSpells(context.Background(), SearchQuery{Search: "fire"})
	require.NoError(t, err)
	require.Len(t, res.Results, 1)
	assert.Empty(t, store.spells)
}
