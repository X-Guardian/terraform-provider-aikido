package client

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsUserInTeam_CachesAcrossCalls(t *testing.T) {
	var calls int32
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		mustEncode(t, w, []User{{ID: 5}, {ID: 7}})
	})
	defer server.Close()

	for i := 0; i < 5; i++ {
		found, err := c.IsUserInTeam(context.Background(), 10, 5)
		if err != nil {
			t.Fatalf("unexpected error on call %d: %v", i, err)
		}
		if !found {
			t.Fatalf("expected user to be found on call %d", i)
		}
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected 1 API call across 5 reads, got %d", got)
	}
}

func TestIsUserInTeam_CacheSharedBetweenDifferentUsers(t *testing.T) {
	var calls int32
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		mustEncode(t, w, []User{{ID: 3}, {ID: 5}, {ID: 7}})
	})
	defer server.Close()

	for _, uid := range []int{3, 5, 7, 99} {
		_, err := c.IsUserInTeam(context.Background(), 10, uid)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected 1 API call for 4 different-user checks in one team, got %d", got)
	}
}

func TestIsUserInTeam_ConcurrentCallsCoalesced(t *testing.T) {
	var calls int32
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		// Hold the response briefly so concurrent callers pile up in singleflight.
		time.Sleep(50 * time.Millisecond)
		mustEncode(t, w, []User{{ID: 5}})
	})
	defer server.Close()

	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = c.IsUserInTeam(context.Background(), 10, 5)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: unexpected error: %v", i, err)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected 1 API call with singleflight coalescing, got %d", got)
	}
}

func TestIsUserInTeam_DifferentTeamsCachedSeparately(t *testing.T) {
	var calls int32
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		team := r.URL.Query().Get("filter_team_id")
		switch team {
		case "10":
			mustEncode(t, w, []User{{ID: 5}})
		case "20":
			mustEncode(t, w, []User{{ID: 7}})
		default:
			mustEncode(t, w, []User{})
		}
	})
	defer server.Close()

	if found, err := c.IsUserInTeam(context.Background(), 10, 5); err != nil || !found {
		t.Fatalf("team 10 user 5: found=%v err=%v", found, err)
	}
	if found, err := c.IsUserInTeam(context.Background(), 20, 7); err != nil || !found {
		t.Fatalf("team 20 user 7: found=%v err=%v", found, err)
	}
	// Repeat — both should hit cache.
	_, _ = c.IsUserInTeam(context.Background(), 10, 5)
	_, _ = c.IsUserInTeam(context.Background(), 20, 7)

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("expected 2 API calls (one per team), got %d", got)
	}
}

func TestAddUserToTeam_InvalidatesCache(t *testing.T) {
	var listCalls int32
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/public/v1/teams/10/addUser" {
			w.WriteHeader(http.StatusOK)
			return
		}
		atomic.AddInt32(&listCalls, 1)
		mustEncode(t, w, []User{{ID: 5}})
	})
	defer server.Close()

	if _, err := c.IsUserInTeam(context.Background(), 10, 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := c.AddUserToTeam(context.Background(), 10, 6); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After the write the cache must be cold for team 10.
	if _, err := c.IsUserInTeam(context.Background(), 10, 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := atomic.LoadInt32(&listCalls); got != 2 {
		t.Errorf("expected 2 list calls (cache invalidated by write), got %d", got)
	}
}

func TestRemoveUserFromTeam_InvalidatesCache(t *testing.T) {
	var listCalls int32
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/public/v1/teams/10/removeUser" {
			w.WriteHeader(http.StatusOK)
			return
		}
		atomic.AddInt32(&listCalls, 1)
		mustEncode(t, w, []User{{ID: 5}})
	})
	defer server.Close()

	if _, err := c.IsUserInTeam(context.Background(), 10, 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := c.RemoveUserFromTeam(context.Background(), 10, 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := c.IsUserInTeam(context.Background(), 10, 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := atomic.LoadInt32(&listCalls); got != 2 {
		t.Errorf("expected 2 list calls (cache invalidated by write), got %d", got)
	}
}

func TestIsResourceLinkedToTeam_CachesAcrossCalls(t *testing.T) {
	var calls int32
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		mustEncode(t, w, []Team{
			{ID: 10, Responsibilities: []Responsibility{{ID: 42, Type: "code_repository"}}},
			{ID: 20, Responsibilities: []Responsibility{{ID: 99, Type: "cloud"}}},
		})
	})
	defer server.Close()

	for _, tc := range []struct {
		teamID       int
		resourceType string
		resourceID   int
		want         bool
	}{
		{10, "code_repository", 42, true},
		{10, "code_repository", 999, false},
		{20, "cloud", 99, true},
		{20, "cloud", 1, false},
	} {
		got, err := c.IsResourceLinkedToTeam(context.Background(), tc.teamID, tc.resourceType, tc.resourceID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != tc.want {
			t.Errorf("team %d %s %d: got %v, want %v", tc.teamID, tc.resourceType, tc.resourceID, got, tc.want)
		}
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected 1 API call across 4 link checks, got %d", got)
	}
}

func TestIsResourceLinkedToTeam_ConcurrentCallsCoalesced(t *testing.T) {
	var calls int32
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(50 * time.Millisecond)
		mustEncode(t, w, []Team{
			{ID: 10, Responsibilities: []Responsibility{{ID: 42, Type: "code_repository"}}},
		})
	})
	defer server.Close()

	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = c.IsResourceLinkedToTeam(context.Background(), 10, "code_repository", 42)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: unexpected error: %v", i, err)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected 1 API call with singleflight coalescing, got %d", got)
	}
}

func TestLinkResourceToTeam_InvalidatesTeamsCache(t *testing.T) {
	var listCalls int32
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/public/v1/teams/10/linkResource" {
			w.WriteHeader(http.StatusOK)
			return
		}
		atomic.AddInt32(&listCalls, 1)
		mustEncode(t, w, []Team{
			{ID: 10, Responsibilities: []Responsibility{{ID: 42, Type: "code_repository"}}},
		})
	})
	defer server.Close()

	if _, err := c.IsResourceLinkedToTeam(context.Background(), 10, "code_repository", 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := c.LinkResourceToTeam(context.Background(), 10, "cloud", 99); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := c.IsResourceLinkedToTeam(context.Background(), 10, "code_repository", 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := atomic.LoadInt32(&listCalls); got != 2 {
		t.Errorf("expected 2 list calls (cache invalidated by link), got %d", got)
	}
}

func TestUnlinkResourceFromTeam_InvalidatesTeamsCache(t *testing.T) {
	var listCalls int32
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/public/v1/teams/10/unlinkResource" {
			w.WriteHeader(http.StatusOK)
			return
		}
		atomic.AddInt32(&listCalls, 1)
		mustEncode(t, w, []Team{
			{ID: 10, Responsibilities: []Responsibility{{ID: 42, Type: "code_repository"}}},
		})
	})
	defer server.Close()

	if _, err := c.IsResourceLinkedToTeam(context.Background(), 10, "code_repository", 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := c.UnlinkResourceFromTeam(context.Background(), 10, "code_repository", 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := c.IsResourceLinkedToTeam(context.Background(), 10, "code_repository", 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := atomic.LoadInt32(&listCalls); got != 2 {
		t.Errorf("expected 2 list calls (cache invalidated by unlink), got %d", got)
	}
}

func TestTeamsCache_TTLExpiry(t *testing.T) {
	var calls int32
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		mustEncode(t, w, []Team{{ID: 10}})
	})
	defer server.Close()

	c.teamsCache = newTeamsCache(10 * time.Millisecond)

	if _, err := c.GetTeam(context.Background(), 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	time.Sleep(25 * time.Millisecond)
	if _, err := c.GetTeam(context.Background(), 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("expected 2 API calls after TTL expiry, got %d", got)
	}
}

func TestUsersCache_TTLExpiry(t *testing.T) {
	var calls int32
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		mustEncode(t, w, []User{{ID: 5}})
	})
	defer server.Close()

	c.usersCache = newUsersCache(10 * time.Millisecond)

	if _, err := c.IsUserInTeam(context.Background(), 10, 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	time.Sleep(25 * time.Millisecond)
	if _, err := c.IsUserInTeam(context.Background(), 10, 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("expected 2 API calls after TTL expiry, got %d", got)
	}
}
