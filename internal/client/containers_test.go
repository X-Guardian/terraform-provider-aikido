package client

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"
)

func TestGetContainer(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/containers/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, Container{
			ID:       42,
			Name:     "my-app",
			Provider: "aws",
			Tag:      "latest",
			Distro:   "alpine",
		})
	})
	defer server.Close()

	container, err := c.GetContainer(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if container.Name != "my-app" {
		t.Errorf("expected name 'my-app', got %q", container.Name)
	}
}

func TestGetContainer_WithLinkedCodeRepoID(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		// Simulate API returning linked_code_repo_id as a number (not a string)
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"id":42,"name":"my-app","provider":"aws","tag":"latest","distro":"alpine","linked_code_repo_id":12345}`)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	})
	defer server.Close()

	container, err := c.GetContainer(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if container.LinkedCodeRepoID == nil {
		t.Fatal("expected linked_code_repo_id to be set")
	}
	if *container.LinkedCodeRepoID != 12345 {
		t.Errorf("expected linked_code_repo_id 12345, got %d", *container.LinkedCodeRepoID)
	}
}

func TestGetContainer_NullLinkedCodeRepoID(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"id":42,"name":"my-app","provider":"aws","tag":"latest","distro":"alpine","linked_code_repo_id":null}`)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	})
	defer server.Close()

	container, err := c.GetContainer(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if container.LinkedCodeRepoID != nil {
		t.Errorf("expected linked_code_repo_id to be nil, got %d", *container.LinkedCodeRepoID)
	}
}

func TestGetContainer_NotFound(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	_, err := c.GetContainer(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestListContainers(t *testing.T) {
	page := 0
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch page {
		case 0:
			mustEncode(t, w, []Container{
				{ID: 1, Name: "app-a"},
				{ID: 2, Name: "app-b"},
			})
			page++
		default:
			mustEncode(t, w, []Container{})
		}
	})
	defer server.Close()

	containers, err := c.ListContainers(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(containers) != 2 {
		t.Errorf("expected 2 containers, got %d", len(containers))
	}
}

func TestListContainers_IncludeOptions(t *testing.T) {
	var got url.Values
	page := 0
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if page == 0 {
			got = r.URL.Query()
			mustEncode(t, w, []Container{{ID: 1, Name: "app-a"}})
			page++
			return
		}
		mustEncode(t, w, []Container{})
	})
	defer server.Close()

	reachability := "direct"
	_, err := c.ListContainers(context.Background(), &ListContainersOptions{
		FilterReachability:  reachability,
		IncludeIsRunning:    true,
		IncludeSensitivity:  true,
		IncludeConnectivity: true,
		IncludeLabels:       true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, param := range []string{"include_is_running", "include_sensitivity", "include_connectivity", "include_labels"} {
		if got.Get(param) != "true" {
			t.Errorf("expected %s=true, got %q", param, got.Get(param))
		}
	}
	if got.Get("filter_reachability") != reachability {
		t.Errorf("expected filter_reachability %q, got %q", reachability, got.Get("filter_reachability"))
	}
}

func TestListContainers_OmitsUnsetIncludeOptions(t *testing.T) {
	var got url.Values
	page := 0
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if page == 0 {
			got = r.URL.Query()
			mustEncode(t, w, []Container{{ID: 1}})
			page++
			return
		}
		mustEncode(t, w, []Container{})
	})
	defer server.Close()

	if _, err := c.ListContainers(context.Background(), &ListContainersOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, param := range []string{"include_is_running", "include_sensitivity", "include_connectivity", "include_labels", "filter_reachability"} {
		if got.Has(param) {
			t.Errorf("expected %s to be absent, got %q", param, got.Get(param))
		}
	}
}

func TestFindContainer(t *testing.T) {
	var got url.Values
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		sensitivity := "sensitive"
		connectivity := "connected"
		isRunning := true
		mustEncode(t, w, []Container{
			{ID: 1, Name: "app-a"},
			{
				ID:           42,
				Name:         "my-app",
				IsActive:     true,
				Sensitivity:  &sensitivity,
				Connectivity: &connectivity,
				IsRunning:    &isRunning,
				Labels:       []ContainerLabel{{ID: 7, Name: "prod", IsImported: true}},
			},
		})
	})
	defer server.Close()

	container, err := c.FindContainer(context.Background(), 42, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if container.ID != 42 {
		t.Errorf("expected container 42, got %d", container.ID)
	}
	if !container.IsActive {
		t.Error("expected is_active to be true")
	}
	if container.Sensitivity == nil || *container.Sensitivity != "sensitive" {
		t.Errorf("expected sensitivity 'sensitive', got %v", container.Sensitivity)
	}
	if container.Connectivity == nil || *container.Connectivity != "connected" {
		t.Errorf("expected connectivity 'connected', got %v", container.Connectivity)
	}
	if len(container.Labels) != 1 || container.Labels[0].Name != "prod" {
		t.Errorf("expected one label 'prod', got %v", container.Labels)
	}

	// A deactivated container must still be findable, so the status filter has to be widened.
	if got.Get("filter_status") != "all" {
		t.Errorf("expected filter_status=all, got %q", got.Get("filter_status"))
	}
	for _, param := range []string{"include_sensitivity", "include_connectivity"} {
		if got.Get(param) != "true" {
			t.Errorf("expected %s=true, got %q", param, got.Get(param))
		}
	}
}

func TestFindContainer_NameHintFiltersRequest(t *testing.T) {
	var queries []url.Values
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query())
		mustEncode(t, w, []Container{{ID: 42, Name: "my-app"}})
	})
	defer server.Close()

	container, err := c.FindContainer(context.Background(), 42, "my-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if container.ID != 42 {
		t.Errorf("expected container 42, got %d", container.ID)
	}

	// The hint should produce a single filtered request, with no unfiltered fallback.
	if len(queries) != 1 {
		t.Fatalf("expected 1 request, got %d", len(queries))
	}
	if queries[0].Get("filter_name") != "my-app" {
		t.Errorf("expected filter_name=my-app, got %q", queries[0].Get("filter_name"))
	}
}

func TestFindContainer_StaleNameHintFallsBack(t *testing.T) {
	var queries []url.Values
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query())
		// The filtered request misses, as it would after the container was renamed.
		if r.URL.Query().Get("filter_name") != "" {
			mustEncode(t, w, []Container{})
			return
		}
		mustEncode(t, w, []Container{{ID: 42, Name: "renamed"}})
	})
	defer server.Close()

	container, err := c.FindContainer(context.Background(), 42, "old-name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if container.Name != "renamed" {
		t.Errorf("expected to find 'renamed', got %q", container.Name)
	}

	if len(queries) != 2 {
		t.Fatalf("expected a filtered request then an unfiltered one, got %d requests", len(queries))
	}
	if queries[1].Has("filter_name") {
		t.Errorf("expected the fallback to drop filter_name, got %q", queries[1].Get("filter_name"))
	}
}

func TestFindContainer_NameHintDoesNotSelectWrongContainer(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		// A substring filter can return several containers; only the ID decides the match.
		if r.URL.Query().Get("filter_name") != "" {
			mustEncode(t, w, []Container{
				{ID: 7, Name: "my-app-staging"},
				{ID: 42, Name: "my-app"},
				{ID: 99, Name: "my-app-prod"},
			})
			return
		}
		mustEncode(t, w, []Container{})
	})
	defer server.Close()

	container, err := c.FindContainer(context.Background(), 42, "my-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if container.ID != 42 {
		t.Errorf("expected container 42, got %d", container.ID)
	}
}

func TestFindContainer_NameHintErrorDoesNotFallBack(t *testing.T) {
	requests := 0
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer server.Close()

	// A transport or server failure is not a miss, so it must not trigger a second scan.
	if _, err := c.FindContainer(context.Background(), 42, "my-app"); err == nil {
		t.Fatal("expected an error")
	}
	if requests != 1 {
		t.Errorf("expected 1 request, got %d", requests)
	}
}

func TestFindContainer_SecondLookupServedFromCache(t *testing.T) {
	requests := 0
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Query().Get("page") == "0" {
			mustEncode(t, w, []Container{{ID: 42, Name: "my-app"}, {ID: 43, Name: "other"}})
			return
		}
		mustEncode(t, w, []Container{})
	})
	defer server.Close()

	if _, err := c.FindContainer(context.Background(), 42, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	afterFirst := requests

	// A different container from the same listing must not cost another fetch.
	container, err := c.FindContainer(context.Background(), 43, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if container.Name != "other" {
		t.Errorf("expected 'other', got %q", container.Name)
	}
	if requests != afterFirst {
		t.Errorf("expected no further requests, got %d extra", requests-afterFirst)
	}
}

func TestFindContainer_CacheMissIsAuthoritative(t *testing.T) {
	requests := 0
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Query().Get("page") == "0" {
			mustEncode(t, w, []Container{{ID: 42, Name: "my-app"}})
			return
		}
		mustEncode(t, w, []Container{})
	})
	defer server.Close()

	if _, err := c.FindContainer(context.Background(), 42, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	afterFirst := requests

	// The cached listing covers every container, so an absent ID needs no extra requests to
	// confirm — including no fallback scan for the name hint.
	if _, err := c.FindContainer(context.Background(), 999, "some-name"); !errors.Is(err, ErrContainerNotFound) {
		t.Fatalf("expected ErrContainerNotFound, got %v", err)
	}
	if requests != afterFirst {
		t.Errorf("expected no further requests, got %d extra", requests-afterFirst)
	}
}

func TestFindContainer_ConcurrentLookupsCoalesce(t *testing.T) {
	var mu sync.Mutex
	pageRequests := 0
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		pageRequests++
		mu.Unlock()
		// Hold the response open so every goroutine is waiting when the fetch starts.
		time.Sleep(50 * time.Millisecond)
		if r.URL.Query().Get("page") == "0" {
			mustEncode(t, w, []Container{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}})
			return
		}
		mustEncode(t, w, []Container{})
	})
	defer server.Close()

	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = c.FindContainer(context.Background(), i+1, "")
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("lookup %d: unexpected error: %v", i+1, err)
		}
	}

	// Four parallel refreshes should share a single paginated listing, not repeat it each.
	mu.Lock()
	got := pageRequests
	mu.Unlock()
	if got > 2 {
		t.Errorf("expected the listing to be fetched once (at most 2 page requests), got %d", got)
	}
}

func TestFindContainer_WriteInvalidatesCache(t *testing.T) {
	sensitivity := "normal"
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/public/v1/containers/42/sensitivity" {
			sensitivity = "extreme"
			mustEncode(t, w, map[string]int{"success": 1})
			return
		}
		if r.URL.Query().Get("page") == "0" {
			current := sensitivity
			mustEncode(t, w, []Container{{ID: 42, Name: "my-app", Sensitivity: &current}})
			return
		}
		mustEncode(t, w, []Container{})
	})
	defer server.Close()

	container, err := c.FindContainer(context.Background(), 42, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *container.Sensitivity != "normal" {
		t.Fatalf("expected 'normal' before the write, got %q", *container.Sensitivity)
	}

	if err := c.UpdateContainerSensitivity(context.Background(), 42, "extreme"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Without invalidation this read would serve the pre-write value and produce a phantom diff.
	container, err = c.FindContainer(context.Background(), 42, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *container.Sensitivity != "extreme" {
		t.Errorf("expected 'extreme' after the write, got %q", *container.Sensitivity)
	}
}

func TestContainerWritesInvalidateCache(t *testing.T) {
	writes := map[string]func(*AikidoClient) error{
		"activate":    func(c *AikidoClient) error { return c.ActivateContainer(context.Background(), 42) },
		"deactivate":  func(c *AikidoClient) error { return c.DeactivateContainer(context.Background(), 42) },
		"sensitivity": func(c *AikidoClient) error { return c.UpdateContainerSensitivity(context.Background(), 42, "normal") },
		"connectivity": func(c *AikidoClient) error {
			return c.UpdateContainerConnectivity(context.Background(), 42, "connected")
		},
		"tag_filter": func(c *AikidoClient) error { return c.UpdateContainerTagFilter(context.Background(), 42, "v*") },
		"link":       func(c *AikidoClient) error { return c.LinkCodeRepoToContainer(context.Background(), 42, 7) },
		"unlink":     func(c *AikidoClient) error { return c.UnlinkCodeRepoFromContainer(context.Background(), 42) },
	}

	for name, write := range writes {
		t.Run(name, func(t *testing.T) {
			server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("page") == "0" {
					mustEncode(t, w, []Container{{ID: 42, Name: "my-app"}})
					return
				}
				if r.Method == http.MethodGet {
					mustEncode(t, w, []Container{})
					return
				}
				mustEncode(t, w, map[string]int{"success": 1})
			})
			defer server.Close()

			if _, err := c.FindContainer(context.Background(), 42, ""); err != nil {
				t.Fatalf("priming the cache: %v", err)
			}
			if _, ok := c.containersCache.get(); !ok {
				t.Fatal("expected the cache to be populated")
			}

			if err := write(c); err != nil {
				t.Fatalf("write failed: %v", err)
			}

			if _, ok := c.containersCache.get(); ok {
				t.Error("expected the write to invalidate the cache")
			}
		})
	}
}

func TestFindContainer_SearchesLaterPages(t *testing.T) {
	requests := 0
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Query().Get("page") == "0" {
			// A full page forces the search to continue onto the next one.
			full := make([]Container, containersPageSize)
			for i := range full {
				full[i] = Container{ID: i + 1}
			}
			mustEncode(t, w, full)
			return
		}
		mustEncode(t, w, []Container{{ID: 500, Name: "late"}})
	})
	defer server.Close()

	container, err := c.FindContainer(context.Background(), 500, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if container.Name != "late" {
		t.Errorf("expected to find 'late', got %q", container.Name)
	}
	if requests != 2 {
		t.Errorf("expected 2 requests, got %d", requests)
	}
}

func TestFindContainer_NotFound(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustEncode(t, w, []Container{{ID: 1}, {ID: 2}})
	})
	defer server.Close()

	_, err := c.FindContainer(context.Background(), 999, "")
	if !errors.Is(err, ErrContainerNotFound) {
		t.Fatalf("expected ErrContainerNotFound, got %v", err)
	}
}

func TestGetContainer_NotFoundIsSentinel(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	_, err := c.GetContainer(context.Background(), 999)
	if !errors.Is(err, ErrContainerNotFound) {
		t.Fatalf("expected ErrContainerNotFound, got %v", err)
	}
}

func TestActivateContainer(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/containers/activate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]int
		mustDecode(t, r, &body)
		if body["container_repo_id"] != 42 {
			t.Errorf("expected container_repo_id 42, got %d", body["container_repo_id"])
		}
		mustEncode(t, w, map[string]int{"success": 1})
	})
	defer server.Close()

	err := c.ActivateContainer(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeactivateContainer(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/containers/deactivate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]int{"success": 1})
	})
	defer server.Close()

	err := c.DeactivateContainer(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateContainerSensitivity(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/containers/42/sensitivity" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]interface{}{"success": true})
	})
	defer server.Close()

	err := c.UpdateContainerSensitivity(context.Background(), 42, "extreme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateContainerConnectivity(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/containers/42/internetConnection" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]interface{}{"success": true})
	})
	defer server.Close()

	err := c.UpdateContainerConnectivity(context.Background(), 42, "connected")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLinkCodeRepoToContainer(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/containers/linkCodeRepo" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]int
		mustDecode(t, r, &body)
		if body["container_repo_id"] != 42 {
			t.Errorf("expected container_repo_id 42, got %d", body["container_repo_id"])
		}
		if body["code_repo_id"] != 7 {
			t.Errorf("expected code_repo_id 7, got %d", body["code_repo_id"])
		}
		mustEncode(t, w, map[string]int{"success": 1})
	})
	defer server.Close()

	err := c.LinkCodeRepoToContainer(context.Background(), 42, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnlinkCodeRepoFromContainer(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/containers/unlinkCodeRepo" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]int
		mustDecode(t, r, &body)
		if body["container_repo_id"] != 42 {
			t.Errorf("expected container_repo_id 42, got %d", body["container_repo_id"])
		}
		if _, ok := body["code_repo_id"]; ok {
			t.Errorf("unlink should not send code_repo_id")
		}
		mustEncode(t, w, map[string]int{"success": 1})
	})
	defer server.Close()

	err := c.UnlinkCodeRepoFromContainer(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateContainerTagFilter(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/containers/updateTagFilter" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]int{"success": 1})
	})
	defer server.Close()

	err := c.UpdateContainerTagFilter(context.Background(), 42, "v*")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
