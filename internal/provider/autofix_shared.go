package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

// Identifiers for the autofix settings resources. Each endpoint is a workspace-wide
// singleton with no natural ID, so a fixed sentinel is used instead.
const (
	autofixDependencyID = "dependency"
	autofixSastID       = "sast"
	autofixPentestID    = "pentest"
)

const (
	// autofixSeverityNone is returned by the GET endpoints when autofix is disabled. The
	// PUT endpoints do not accept it, so it is normalised to null in state.
	autofixSeverityNone = "none"

	autofixReposScopeAll      = "all"
	autofixReposScopeSelected = "selected"
)

// autofixSeverityToModel converts an API severity filter into a model value, mapping the
// read-only "none" sentinel to null so it never reaches state or a plan.
func autofixSeverityToModel(severity string) types.String {
	if severity == "" || severity == autofixSeverityNone {
		return types.StringNull()
	}
	return types.StringValue(severity)
}

// autofixRepoIDsToSet converts API repository IDs into a set value. A nil slice becomes
// an empty set rather than null, so a managed resource always holds a concrete set.
func autofixRepoIDsToSet(ctx context.Context, ids []int64) (types.Set, diag.Diagnostics) {
	if ids == nil {
		ids = []int64{}
	}
	return types.SetValueFrom(ctx, types.Int64Type, ids)
}

// autofixRepoIDsFromSet converts a set value into API repository IDs. Null and unknown
// sets become an empty slice, never nil, so the request payload carries an empty array.
func autofixRepoIDsFromSet(ctx context.Context, set types.Set) ([]int64, diag.Diagnostics) {
	var diags diag.Diagnostics

	if set.IsNull() || set.IsUnknown() {
		return []int64{}, diags
	}

	ids := []int64{}
	diags.Append(set.ElementsAs(ctx, &ids, false)...)

	return ids, diags
}

// reconcileRepoIDs decides which repository IDs belong in state after a read.
//
// The API silently filters out repository IDs that are inactive or unknown, so a read can
// legitimately return fewer IDs than were written. Reporting that as drift would produce a
// diff that no apply could ever resolve, because the next write would be filtered the same
// way. When the live IDs are a subset of what state already holds, the server has accepted
// our intent and merely dropped unusable IDs, so prior state is kept.
//
// Anything else — live IDs we never asked for — is a genuine out-of-band change and is
// returned as-is so Terraform reports the drift.
//
// The deliberate trade-off is that removing a single repository ID outside Terraform looks
// like filtering and is not reported. An unresolvable perpetual diff is the worse failure.
func reconcileRepoIDs(prior, live []int64) []int64 {
	priorSet := make(map[int64]struct{}, len(prior))
	for _, id := range prior {
		priorSet[id] = struct{}{}
	}

	for _, id := range live {
		if _, ok := priorSet[id]; !ok {
			return live
		}
	}

	return prior
}

// autofixDisabledDiagnostic reports the workspace-level AutoFix disablement error as an
// actionable diagnostic. It returns true when err was that error and a diagnostic was
// added, so callers can skip their generic error handling.
func autofixDisabledDiagnostic(diags *diag.Diagnostics, err error) bool {
	if !errors.Is(err, client.ErrAutofixDisabledForWorkspace) {
		return false
	}

	diags.AddError(
		"AutoFix Disabled for Workspace",
		"AutoFix is disabled for this Aikido workspace, so its settings cannot be read or changed. "+
			"Enable AutoFix in the Aikido dashboard under Settings then AutoFix before managing it with Terraform.\n\n"+
			"API error: "+err.Error(),
	)

	return true
}

// repoIDsPlanModifier keeps an omitted repo_ids value stable in the plan.
//
// repo_ids is optional and computed, so leaving it out of the configuration makes it plan
// as unknown as soon as any sibling attribute changes, which renders as a spurious
// "[] -> (known after apply)" diff. Reusing the prior state value removes that, but it
// cannot be done unconditionally: when repos_scope is "all" the value is always reset to
// empty, so pinning a stale non-empty set would contradict the applied result.
//
// This is deliberately not stringplanmodifier.UseStateForUnknown: that modifier has no way
// to consult repos_scope.
type repoIDsPlanModifier struct{}

var _ planmodifier.Set = repoIDsPlanModifier{}

func (m repoIDsPlanModifier) Description(ctx context.Context) string {
	return m.MarkdownDescription(ctx)
}

func (m repoIDsPlanModifier) MarkdownDescription(context.Context) string {
	return "Keeps the prior `repo_ids` value when it is omitted, except under the `all` scope where it is always empty."
}

func (m repoIDsPlanModifier) PlanModifySet(ctx context.Context, req planmodifier.SetRequest, resp *planmodifier.SetResponse) {
	// Only unknown values need resolving. A configured value always wins.
	if !resp.PlanValue.IsUnknown() {
		return
	}

	// On create there is no prior state to reuse, and on destroy there is nothing to plan.
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var plannedScope types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("repos_scope"), &plannedScope)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Under the "all" scope the API ignores repo_ids and the resource stores an empty set,
	// so plan that directly rather than reusing a possibly stale list.
	if plannedScope.ValueString() == autofixReposScopeAll {
		empty, diags := autofixRepoIDsToSet(ctx, []int64{})
		resp.Diagnostics.Append(diags...)
		if !resp.Diagnostics.HasError() {
			resp.PlanValue = empty
		}
		return
	}

	// Otherwise reuse the prior value so an unrelated change does not show a false diff.
	if !req.StateValue.IsNull() {
		resp.PlanValue = req.StateValue
	}
}

// errorsIsAutofixDisabled reports whether err is the workspace-level AutoFix disablement
// error. Delete uses this to treat an already-disabled workspace as success.
func errorsIsAutofixDisabled(err error) bool {
	return errors.Is(err, client.ErrAutofixDisabledForWorkspace)
}

// autofixFieldsValidator enforces the API's conditional requirements: the severity and
// repository scope fields are mandatory when autofix is enabled and are ignored otherwise.
type autofixFieldsValidator struct {
	// hasReposScope is false for the pentest resource, which has no repository scoping.
	hasReposScope bool
	// hasAikidoLibrary is true only for the dependency resource.
	hasAikidoLibrary bool
}

var _ resource.ConfigValidator = &autofixFieldsValidator{}

func (v *autofixFieldsValidator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

func (v *autofixFieldsValidator) MarkdownDescription(context.Context) string {
	if !v.hasReposScope {
		return "`severity_filter` must be set when `enabled` is `true`."
	}
	return "`severity_filter` and `repos_scope` must be set when `enabled` is `true`, " +
		"`repo_ids` must be non-empty when `repos_scope` is `selected`, and empty when it is `all`."
}

func (v *autofixFieldsValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var enabled types.Bool
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("enabled"), &enabled)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Values that are not yet known cannot be validated. This is common when they are
	// derived from another resource or a data source.
	if enabled.IsNull() || enabled.IsUnknown() || !enabled.ValueBool() {
		return
	}

	var severityFilter types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("severity_filter"), &severityFilter)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if severityFilter.IsNull() {
		resp.Diagnostics.AddAttributeError(
			path.Root("severity_filter"),
			"Missing Required Attribute",
			"`severity_filter` must be set when `enabled` is `true`.",
		)
	}

	if v.hasAikidoLibrary {
		var useAikidoLibrary types.Bool
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("use_aikido_library_for_major"), &useAikidoLibrary)...)
		if !resp.Diagnostics.HasError() && useAikidoLibrary.IsNull() {
			resp.Diagnostics.AddAttributeError(
				path.Root("use_aikido_library_for_major"),
				"Missing Required Attribute",
				"`use_aikido_library_for_major` must be set when `enabled` is `true`.",
			)
		}
	}

	if !v.hasReposScope {
		return
	}

	var reposScope types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("repos_scope"), &reposScope)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if reposScope.IsNull() {
		resp.Diagnostics.AddAttributeError(
			path.Root("repos_scope"),
			"Missing Required Attribute",
			"`repos_scope` must be set when `enabled` is `true`.",
		)
		return
	}
	if reposScope.IsUnknown() {
		return
	}

	var repoIDs types.Set
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("repo_ids"), &repoIDs)...)
	if resp.Diagnostics.HasError() || repoIDs.IsUnknown() {
		return
	}

	switch reposScope.ValueString() {
	case autofixReposScopeSelected:
		if repoIDs.IsNull() || len(repoIDs.Elements()) == 0 {
			resp.Diagnostics.AddAttributeError(
				path.Root("repo_ids"),
				"Missing Required Attribute",
				fmt.Sprintf("`repo_ids` must contain at least one repository ID when `repos_scope` is %q.", autofixReposScopeSelected),
			)
		}
	case autofixReposScopeAll:
		if !repoIDs.IsNull() && len(repoIDs.Elements()) > 0 {
			resp.Diagnostics.AddAttributeError(
				path.Root("repo_ids"),
				"Conflicting Attribute Configuration",
				fmt.Sprintf("`repo_ids` must not be set when `repos_scope` is %q, because the API ignores it.", autofixReposScopeAll),
			)
		}
	}
}
