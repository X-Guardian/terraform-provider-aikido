package client

import (
	"context"
	"net/http"
	"testing"
)

func TestUpdateZenAppBlocking(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/firewall/apps/42/blocking" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		mustEncode(t, w, map[string]bool{"success": true})
	})
	defer server.Close()

	err := c.UpdateZenAppBlocking(context.Background(), 42, ZenAppBlocking{Block: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetZenAppCountries(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/firewall/apps/42/countries" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, ZenAppCountries{
			Mode: "block",
			List: []ZenAppCountryItem{{Code: "CN", Name: "China"}, {Code: "RU", Name: "Russia"}},
		})
	})
	defer server.Close()

	countries, err := c.GetZenAppCountries(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if countries.Mode != "block" {
		t.Errorf("expected mode 'block', got %q", countries.Mode)
	}
	if len(countries.List) != 2 {
		t.Errorf("expected 2 countries, got %d", len(countries.List))
	}
}

func TestUpdateZenAppCountries(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/firewall/apps/42/countries" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]bool{"success": true})
	})
	defer server.Close()

	err := c.UpdateZenAppCountries(context.Background(), 42, ZenAppCountriesRequest{Mode: "block", List: []string{"CN"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateZenAppIPBlocklist(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/firewall/apps/42/ip-blocklist" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]bool{"success": true})
	})
	defer server.Close()

	err := c.UpdateZenAppIPBlocklist(context.Background(), 42, []string{"192.168.1.1", "10.0.0.0/8"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetZenAppBotLists(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/firewall/apps/42/bot-lists" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, []ZenAppBotListItem{
			{Code: "scrapers", Name: "Web Scrapers", Mode: "block"},
			{Code: "crawlers", Name: "Crawlers", Mode: "ignore"},
		})
	})
	defer server.Close()

	bots, err := c.GetZenAppBotLists(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bots) != 2 {
		t.Errorf("expected 2 bot lists, got %d", len(bots))
	}
}

func TestUpdateZenAppBotLists(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/firewall/apps/42/bot-lists" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]bool{"success": true})
	})
	defer server.Close()

	err := c.UpdateZenAppBotLists(context.Background(), 42, []ZenAppBotListUpdateItem{{Code: "scrapers", Mode: "block"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetZenAppIPLists(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/firewall/apps/42/ip-lists" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, ZenAppIPLists{
			KnownThreatActors: []ZenAppIPListItem{{Code: "apt1", Name: "APT1", Mode: "block"}},
			Tor:               ZenAppTorConfig{Mode: "monitor"},
		})
	})
	defer server.Close()

	lists, err := c.GetZenAppIPLists(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lists.KnownThreatActors) != 1 {
		t.Errorf("expected 1 threat actor list, got %d", len(lists.KnownThreatActors))
	}
	if lists.Tor.Mode != "monitor" {
		t.Errorf("expected tor mode 'monitor', got %q", lists.Tor.Mode)
	}
}

func TestUpdateZenAppIPLists(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/firewall/apps/42/ip-lists" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]bool{"success": true})
	})
	defer server.Close()

	err := c.UpdateZenAppIPLists(context.Background(), 42, ZenAppIPListsRequest{
		KnownThreatActors: []ZenAppIPListUpdateItem{{Code: "apt1", Mode: "block"}},
		Tor:               &ZenAppTorConfig{Mode: "block"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
