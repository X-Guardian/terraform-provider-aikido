package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestReconcileRepoIDs(t *testing.T) {
	tests := []struct {
		name     string
		prior    []int64
		live     []int64
		expected []int64
	}{
		{
			name:     "live is a strict subset so prior intent is kept",
			prior:    []int64{1, 2, 999},
			live:     []int64{1, 2},
			expected: []int64{1, 2, 999},
		},
		{
			name:     "live contains unknown IDs so live wins as real drift",
			prior:    []int64{1, 2},
			live:     []int64{1, 2, 3},
			expected: []int64{1, 2, 3},
		},
		{
			name:     "disjoint sets are real drift",
			prior:    []int64{1, 2},
			live:     []int64{7, 8},
			expected: []int64{7, 8},
		},
		{
			name:     "equal sets keep prior",
			prior:    []int64{1, 2},
			live:     []int64{2, 1},
			expected: []int64{1, 2},
		},
		{
			name:     "empty live with non-empty prior is treated as filtering",
			prior:    []int64{1, 2},
			live:     []int64{},
			expected: []int64{1, 2},
		},
		{
			name:     "empty prior takes live",
			prior:    []int64{},
			live:     []int64{5},
			expected: []int64{5},
		},
		{
			name:     "both empty",
			prior:    []int64{},
			live:     []int64{},
			expected: []int64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reconcileRepoIDs(tt.prior, tt.live)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Fatalf("expected %v, got %v", tt.expected, got)
				}
			}
		})
	}
}

func TestAutofixSeverityToModel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantNull bool
		wantVal  string
	}{
		{name: "none becomes null", input: "none", wantNull: true},
		{name: "empty becomes null", input: "", wantNull: true},
		{name: "real value is preserved", input: "critical_and_high_only", wantVal: "critical_and_high_only"},
		{name: "all is preserved", input: "all", wantVal: "all"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := autofixSeverityToModel(tt.input)
			if tt.wantNull {
				if !got.IsNull() {
					t.Errorf("expected null, got %v", got)
				}
				return
			}
			if got.ValueString() != tt.wantVal {
				t.Errorf("expected %q, got %q", tt.wantVal, got.ValueString())
			}
		})
	}
}

func TestAutofixRepoIDsFromSet(t *testing.T) {
	ctx := context.Background()

	// A null set must produce an empty slice, never nil, so the JSON payload is [].
	ids, diags := autofixRepoIDsFromSet(ctx, types.SetNull(types.Int64Type))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if ids == nil {
		t.Fatal("expected an empty slice, got nil")
	}
	if len(ids) != 0 {
		t.Errorf("expected an empty slice, got %v", ids)
	}

	set, d := autofixRepoIDsToSet(ctx, []int64{4, 5})
	if d.HasError() {
		t.Fatalf("unexpected diagnostics: %v", d)
	}
	ids, diags = autofixRepoIDsFromSet(ctx, set)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %v", ids)
	}
}

func TestAutofixRepoIDsToSet_NilBecomesEmptySet(t *testing.T) {
	set, diags := autofixRepoIDsToSet(context.Background(), nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if set.IsNull() {
		t.Error("expected an empty set, got null")
	}
	if len(set.Elements()) != 0 {
		t.Errorf("expected no elements, got %v", set.Elements())
	}
}
