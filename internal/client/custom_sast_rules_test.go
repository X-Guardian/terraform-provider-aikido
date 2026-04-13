package client

import (
	"context"
	"net/http"
	"testing"
)

func TestCreateCustomRule(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/v1/repositories/sast/custom-rules" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req CustomRuleRequest
		mustDecode(t, r, &req)
		if req.IssueTitle != "Test Rule" {
			t.Errorf("expected issue_title 'Test Rule', got %q", req.IssueTitle)
		}
		mustEncode(t, w, map[string]int{"id": 42})
	})
	defer server.Close()

	id, err := c.CreateCustomRule(context.Background(), CustomRuleRequest{
		SemgrepRule: "rules:\n  - id: test",
		IssueTitle:  "Test Rule",
		TLDR:        "A test rule",
		HowToFix:    "Fix it",
		Priority:    75,
		Language:    "JS",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 42 {
		t.Errorf("expected ID 42, got %d", id)
	}
}

func TestGetCustomRule(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/repositories/sast/custom-rules/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]interface{}{
			"custom_rule": CustomRule{
				ID:          42,
				SemgrepRule: "rules:\n  - id: test",
				IssueTitle:  "Test Rule",
				TLDR:        "A test rule",
				HowToFix:    "Fix it",
				Priority:    75,
				Language:    "JS",
				HasError:    false,
			},
		})
	})
	defer server.Close()

	rule, err := c.GetCustomRule(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule.IssueTitle != "Test Rule" {
		t.Errorf("expected issue_title 'Test Rule', got %q", rule.IssueTitle)
	}
	if rule.Priority != 75 {
		t.Errorf("expected priority 75, got %d", rule.Priority)
	}
}

func TestGetCustomRule_NotFound(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	_, err := c.GetCustomRule(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestListCustomRules(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustEncode(t, w, map[string]interface{}{
			"custom_rules": []CustomRule{
				{ID: 1, IssueTitle: "Rule A", Language: "JS"},
				{ID: 2, IssueTitle: "Rule B", Language: "PY"},
			},
		})
	})
	defer server.Close()

	rules, err := c.ListCustomRules(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}
}

func TestUpdateCustomRule(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/v1/repositories/sast/custom-rules/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req CustomRuleRequest
		mustDecode(t, r, &req)
		if req.Priority != 90 {
			t.Errorf("expected priority 90, got %d", req.Priority)
		}
		mustEncode(t, w, map[string]int{"success": 1})
	})
	defer server.Close()

	err := c.UpdateCustomRule(context.Background(), 42, CustomRuleRequest{
		SemgrepRule: "rules:\n  - id: test",
		IssueTitle:  "Updated Rule",
		TLDR:        "Updated",
		HowToFix:    "Fix it better",
		Priority:    90,
		Language:    "JS",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteCustomRule(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/v1/repositories/sast/custom-rules/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]int{"success": 1})
	})
	defer server.Close()

	err := c.DeleteCustomRule(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteCustomRule_NotFound(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	err := c.DeleteCustomRule(context.Background(), 999)
	if err != nil {
		t.Fatalf("expected no error for already-deleted rule, got: %v", err)
	}
}
