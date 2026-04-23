package open5e

import "context"

// Service bundles the Open5e Client and the local Cache so callers can
// both proxy live searches and persist individual entries with one call.
type Service struct {
	client *Client
	cache  *Cache
}

// NewService constructs a Service from a Client + Cache.
func NewService(client *Client, cache *Cache) *Service {
	return &Service{client: client, cache: cache}
}

// SearchMonsters proxies Client.SearchMonsters. It deliberately does
// NOT cache the whole page — callers cache individually via
// SearchAndCacheMonster to avoid filling the DB with rows a DM never
// actually intends to use.
func (s *Service) SearchMonsters(ctx context.Context, q SearchQuery) (MonsterListResponse, error) {
	return s.client.SearchMonsters(ctx, q)
}

// SearchSpells proxies Client.SearchSpells.
func (s *Service) SearchSpells(ctx context.Context, q SearchQuery) (SpellListResponse, error) {
	return s.client.SearchSpells(ctx, q)
}

// SearchAndCacheMonster fetches one monster by slug and caches it.
// Returns the cached creature id on success.
func (s *Service) SearchAndCacheMonster(ctx context.Context, slug string) (string, error) {
	m, err := s.client.GetMonster(ctx, slug)
	if err != nil {
		return "", err
	}
	return s.cache.CacheMonster(ctx, m)
}

// SearchAndCacheSpell fetches one spell by slug and caches it.
func (s *Service) SearchAndCacheSpell(ctx context.Context, slug string) (string, error) {
	sp, err := s.client.GetSpell(ctx, slug)
	if err != nil {
		return "", err
	}
	return s.cache.CacheSpell(ctx, sp)
}
