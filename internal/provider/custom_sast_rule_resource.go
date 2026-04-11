package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

var _ resource.Resource = &CustomRuleResource{}
var _ resource.ResourceWithImportState = &CustomRuleResource{}

// NewCustomRuleResource creates a new custom rule resource.
func NewCustomRuleResource() resource.Resource {
	return &CustomRuleResource{}
}

// CustomRuleResource defines the resource implementation.
type CustomRuleResource struct {
	client *client.AikidoClient
}

// CustomRuleResourceModel describes the resource data model.
type CustomRuleResourceModel struct {
	ID          types.String `tfsdk:"id"`
	SemgrepRule types.String `tfsdk:"semgrep_rule"`
	IssueTitle  types.String `tfsdk:"issue_title"`
	TLDR        types.String `tfsdk:"tldr"`
	HowToFix    types.String `tfsdk:"how_to_fix"`
	Priority    types.Int64  `tfsdk:"priority"`
	Language    types.String `tfsdk:"language"`
	HasError    types.Bool   `tfsdk:"has_error"`
}

func (r *CustomRuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_sast_rule"
}

func (r *CustomRuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a custom SAST (semgrep) rule in Aikido Security.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the custom rule.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"semgrep_rule": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The semgrep rule definition.",
			},
			"issue_title": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The title of the issue raised when this rule matches.",
			},
			"tldr": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "A summary of the issue.",
			},
			"how_to_fix": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "A description of how to fix the issue.",
			},
			"priority": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Severity from 1 (low) to 100 (critical).",
				Validators: []validator.Int64{
					int64validator.Between(1, 100),
				},
			},
			"language": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The programming language for this rule.",
				Validators: []validator.String{
					stringvalidator.OneOf(
						"JS", "TS", "PHP", "Java", "Scala", "GO", "PY",
						"Ruby", ".NET", "RUST", "Dart", "Swift", "Elixir",
						"C", "Clojure", "kotlin", "terraform", "generic",
					),
				},
			},
			"has_error": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this rule caused an error during a scan.",
			},
		},
	}
}

func (r *CustomRuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	aikidoClient, ok := req.ProviderData.(*client.AikidoClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.AikidoClient, got: %T.", req.ProviderData),
		)
		return
	}

	r.client = aikidoClient
}

func (r *CustomRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CustomRuleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ruleID, err := r.client.CreateCustomRule(ctx, r.modelToRequest(&data))
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Custom Rule", fmt.Sprintf("Unable to create custom rule: %s", err))
		return
	}

	data.ID = types.StringValue(strconv.Itoa(ruleID))

	// Read back to get has_error and server-authoritative state.
	rule, err := r.client.GetCustomRule(ctx, ruleID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Custom Rule", fmt.Sprintf("Unable to read custom rule after create: %s", err))
		return
	}

	r.ruleToModel(rule, &data)

	tflog.Trace(ctx, "created custom rule", map[string]interface{}{"id": ruleID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CustomRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CustomRuleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ruleID, err := strconv.Atoi(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Rule ID", fmt.Sprintf("Cannot parse rule ID: %s", err))
		return
	}

	rule, err := r.client.GetCustomRule(ctx, ruleID)
	if err != nil {
		resp.State.RemoveResource(ctx)
		tflog.Warn(ctx, "custom rule not found, removing from state", map[string]interface{}{"id": ruleID})
		return
	}

	r.ruleToModel(rule, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CustomRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data CustomRuleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ruleID, err := strconv.Atoi(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Rule ID", fmt.Sprintf("Cannot parse rule ID: %s", err))
		return
	}

	if err := r.client.UpdateCustomRule(ctx, ruleID, r.modelToRequest(&data)); err != nil {
		resp.Diagnostics.AddError("Error Updating Custom Rule", fmt.Sprintf("Unable to update custom rule: %s", err))
		return
	}

	// Read back server-authoritative state.
	rule, err := r.client.GetCustomRule(ctx, ruleID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Custom Rule", fmt.Sprintf("Unable to read custom rule after update: %s", err))
		return
	}

	r.ruleToModel(rule, &data)

	tflog.Trace(ctx, "updated custom rule", map[string]interface{}{"id": ruleID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CustomRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CustomRuleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ruleID, err := strconv.Atoi(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Rule ID", fmt.Sprintf("Cannot parse rule ID: %s", err))
		return
	}

	if err := r.client.DeleteCustomRule(ctx, ruleID); err != nil {
		resp.Diagnostics.AddError("Error Deleting Custom Rule", fmt.Sprintf("Unable to delete custom rule %d: %s", ruleID, err))
		return
	}

	tflog.Trace(ctx, "deleted custom rule", map[string]interface{}{"id": ruleID})
}

func (r *CustomRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *CustomRuleResource) modelToRequest(data *CustomRuleResourceModel) client.CustomRuleRequest {
	return client.CustomRuleRequest{
		SemgrepRule: data.SemgrepRule.ValueString(),
		IssueTitle:  data.IssueTitle.ValueString(),
		TLDR:        data.TLDR.ValueString(),
		HowToFix:    data.HowToFix.ValueString(),
		Priority:    int(data.Priority.ValueInt64()),
		Language:    data.Language.ValueString(),
	}
}

func (r *CustomRuleResource) ruleToModel(rule *client.CustomRule, data *CustomRuleResourceModel) {
	data.ID = types.StringValue(strconv.Itoa(rule.ID))
	data.SemgrepRule = types.StringValue(rule.SemgrepRule)
	data.IssueTitle = types.StringValue(rule.IssueTitle)
	data.TLDR = types.StringValue(rule.TLDR)
	data.HowToFix = types.StringValue(rule.HowToFix)
	data.Priority = types.Int64Value(int64(rule.Priority))
	data.Language = types.StringValue(rule.Language)
	data.HasError = types.BoolValue(rule.HasError)
}
