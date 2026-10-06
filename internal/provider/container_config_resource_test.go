package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

// Omitted optional and computed attributes are unknown in the plan, not null. applyConfig must
// skip them: acting on one sent an empty sensitivity, which the API rejects with a 400, and
// read an unknown active as false, deactivating the container.
func TestContainerConfigApplyConfig_SkipsUnknownAttributes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/oauth/token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "test-token",
				"expires_in":   3600,
				"token_type":   "bearer",
			})
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	aikidoClient := client.NewAikidoClient(server.URL, "test-id", "test-secret", client.RateLimitTierStandard)
	aikidoClient.SetRateLimitForTesting(1000)
	r := &ContainerConfigResource{client: aikidoClient}

	data := ContainerConfigResourceModel{
		ContainerRepoID:  types.StringValue("42"),
		Active:           types.BoolUnknown(),
		Sensitivity:      types.StringUnknown(),
		InternetExposed:  types.StringUnknown(),
		TagFilter:        types.StringNull(),
		LinkedCodeRepoID: types.StringNull(),
	}

	var diags diag.Diagnostics
	r.applyConfig(context.Background(), 42, &data, &client.ContainerDetail{}, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

// When the API omits sensitivity and connectivity, a value still unknown from the plan must be
// set to null, as Terraform rejects unknown values in state after apply. A known prior value is
// kept.
func TestMapListContainerToModel_OmittedSensitivityAndConnectivity(t *testing.T) {
	r := &ContainerConfigResource{}
	container := &client.Container{ID: 42, Name: "my-container"}

	unknown := ContainerConfigResourceModel{
		Sensitivity:     types.StringUnknown(),
		InternetExposed: types.StringUnknown(),
	}
	r.mapListContainerToModel(container, &unknown)
	if !unknown.Sensitivity.IsNull() {
		t.Errorf("sensitivity: got %s, want null", unknown.Sensitivity)
	}
	if !unknown.InternetExposed.IsNull() {
		t.Errorf("internet_exposed: got %s, want null", unknown.InternetExposed)
	}

	prior := ContainerConfigResourceModel{
		Sensitivity:     types.StringValue("sensitive"),
		InternetExposed: types.StringValue("connected"),
	}
	r.mapListContainerToModel(container, &prior)
	if prior.Sensitivity.ValueString() != "sensitive" {
		t.Errorf("sensitivity: got %s, want sensitive", prior.Sensitivity)
	}
	if prior.InternetExposed.ValueString() != "connected" {
		t.Errorf("internet_exposed: got %s, want connected", prior.InternetExposed)
	}
}
