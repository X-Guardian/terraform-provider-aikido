package client

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"golang.org/x/sync/singleflight"
)

// usersCache holds recent ListUsers results keyed by filter parameters.
// Concurrent callers for the same key are coalesced through singleflight,
// so N parallel reads for the same team perform one paginated fetch.
type usersCache struct {
	ttl     time.Duration
	sf      singleflight.Group
	mu      sync.RWMutex
	entries map[string]*usersCacheEntry
}

type usersCacheEntry struct {
	users   []User
	userIDs map[int]struct{}
	expires time.Time
}

func newUsersCache(ttl time.Duration) *usersCache {
	return &usersCache{
		ttl:     ttl,
		entries: make(map[string]*usersCacheEntry),
	}
}

func usersCacheKey(opts *ListUsersOptions) string {
	teamID := 0
	inactive := false
	if opts != nil {
		if opts.TeamID != nil {
			teamID = *opts.TeamID
		}
		inactive = opts.IncludeInactive
	}
	return fmt.Sprintf("team=%d,inactive=%t", teamID, inactive)
}

func (c *usersCache) get(key string) (*usersCacheEntry, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(entry.expires) {
		return nil, false
	}
	return entry, true
}

func (c *usersCache) put(key string, users []User) *usersCacheEntry {
	ids := make(map[int]struct{}, len(users))
	for _, u := range users {
		ids[u.ID] = struct{}{}
	}
	entry := &usersCacheEntry{
		users:   users,
		userIDs: ids,
		expires: time.Now().Add(c.ttl),
	}
	c.mu.Lock()
	c.entries[key] = entry
	c.mu.Unlock()
	return entry
}

// getOrFetch returns a fresh cached entry if available, otherwise invokes
// loader through singleflight so concurrent callers share one fetch.
func (c *usersCache) getOrFetch(ctx context.Context, key string, loader func() ([]User, error)) (*usersCacheEntry, error) {
	if entry, ok := c.get(key); ok {
		tflog.Debug(ctx, "users cache hit", map[string]interface{}{
			"key":        key,
			"user_count": len(entry.users),
			"expires_in": time.Until(entry.expires).String(),
		})
		return entry, nil
	}
	v, err, shared := c.sf.Do(key, func() (any, error) {
		if entry, ok := c.get(key); ok {
			tflog.Debug(ctx, "users cache hit (inside singleflight)", map[string]interface{}{
				"key":        key,
				"user_count": len(entry.users),
			})
			return entry, nil
		}
		tflog.Debug(ctx, "users cache miss, fetching", map[string]interface{}{"key": key})
		start := time.Now()
		users, err := loader()
		if err != nil {
			return nil, err
		}
		entry := c.put(key, users)
		tflog.Debug(ctx, "users cache populated", map[string]interface{}{
			"key":        key,
			"user_count": len(users),
			"fetch_ms":   time.Since(start).Milliseconds(),
		})
		return entry, nil
	})
	if err != nil {
		return nil, err
	}
	if shared {
		tflog.Debug(ctx, "users cache fetch coalesced via singleflight", map[string]interface{}{"key": key})
	}
	return v.(*usersCacheEntry), nil
}

// invalidateTeam drops every cached entry whose key references the given team.
// Called after writes that modify team membership so the next read refetches.
func (c *usersCache) invalidateTeam(ctx context.Context, teamID int) {
	prefix := fmt.Sprintf("team=%d,", teamID)
	c.mu.Lock()
	dropped := 0
	for key := range c.entries {
		if strings.HasPrefix(key, prefix) {
			delete(c.entries, key)
			dropped++
		}
	}
	c.mu.Unlock()
	tflog.Debug(ctx, "users cache invalidated", map[string]interface{}{
		"team_id":         teamID,
		"entries_dropped": dropped,
	})
}

// teamsCache holds the full paginated team list for a short TTL. The team list
// endpoint does not accept filter parameters and caps per_page at 20, so the
// cheapest way to serve N independent team lookups is to share one listing.
type teamsCache struct {
	ttl   time.Duration
	sf    singleflight.Group
	mu    sync.RWMutex
	entry *teamsCacheEntry
}

type teamsCacheEntry struct {
	teams     []Team
	teamsByID map[int]*Team
	expires   time.Time
}

func newTeamsCache(ttl time.Duration) *teamsCache {
	return &teamsCache{ttl: ttl}
}

const teamsCacheKey = "all"

func (c *teamsCache) get() (*teamsCacheEntry, bool) {
	c.mu.RLock()
	entry := c.entry
	c.mu.RUnlock()
	if entry == nil || time.Now().After(entry.expires) {
		return nil, false
	}
	return entry, true
}

func (c *teamsCache) put(teams []Team) *teamsCacheEntry {
	byID := make(map[int]*Team, len(teams))
	for i := range teams {
		byID[teams[i].ID] = &teams[i]
	}
	entry := &teamsCacheEntry{
		teams:     teams,
		teamsByID: byID,
		expires:   time.Now().Add(c.ttl),
	}
	c.mu.Lock()
	c.entry = entry
	c.mu.Unlock()
	return entry
}

// getOrFetch returns a fresh cached entry if available, otherwise invokes
// loader through singleflight so concurrent callers share one fetch.
func (c *teamsCache) getOrFetch(ctx context.Context, loader func() ([]Team, error)) (*teamsCacheEntry, error) {
	if entry, ok := c.get(); ok {
		tflog.Debug(ctx, "teams cache hit", map[string]interface{}{
			"team_count": len(entry.teams),
			"expires_in": time.Until(entry.expires).String(),
		})
		return entry, nil
	}
	v, err, shared := c.sf.Do(teamsCacheKey, func() (any, error) {
		if entry, ok := c.get(); ok {
			return entry, nil
		}
		tflog.Debug(ctx, "teams cache miss, fetching", nil)
		start := time.Now()
		teams, err := loader()
		if err != nil {
			return nil, err
		}
		entry := c.put(teams)
		tflog.Debug(ctx, "teams cache populated", map[string]interface{}{
			"team_count": len(teams),
			"fetch_ms":   time.Since(start).Milliseconds(),
		})
		return entry, nil
	})
	if err != nil {
		return nil, err
	}
	if shared {
		tflog.Debug(ctx, "teams cache fetch coalesced via singleflight", nil)
	}
	return v.(*teamsCacheEntry), nil
}

// invalidate drops the cached team list. Called after writes that modify teams
// or their responsibilities so the next read refetches.
func (c *teamsCache) invalidate(ctx context.Context) {
	c.mu.Lock()
	had := c.entry != nil
	c.entry = nil
	c.mu.Unlock()
	if had {
		tflog.Debug(ctx, "teams cache invalidated", nil)
	}
}
