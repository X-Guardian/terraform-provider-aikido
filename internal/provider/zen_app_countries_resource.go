package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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

var _ resource.Resource = &ZenAppCountriesResource{}
var _ resource.ResourceWithImportState = &ZenAppCountriesResource{}

// NewZenAppCountriesResource creates a new Zen app countries resource.
func NewZenAppCountriesResource() resource.Resource {
	return &ZenAppCountriesResource{}
}

// ZenAppCountriesResource manages country-based IP blocking.
type ZenAppCountriesResource struct {
	client *client.AikidoClient
}

// ZenAppCountriesResourceModel describes the resource data model.
type ZenAppCountriesResourceModel struct {
	ID    types.String `tfsdk:"id"`
	AppID types.String `tfsdk:"app_id"`
	Mode  types.String `tfsdk:"mode"`
	List  types.Set    `tfsdk:"list"`
}

func (r *ZenAppCountriesResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zen_app_countries"
}

func (r *ZenAppCountriesResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages country-based IP blocking for a Zen app in Aikido Security.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"app_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the Zen app.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"mode": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Blocking mode: `block` (block listed countries) or `allow` (only allow listed countries).",
				Validators: []validator.String{
					stringvalidator.OneOf("block", "allow"),
				},
			},
			"list": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Set of country codes (e.g., `CN`, `RU`).",
			},
		},
	}
}

func (r *ZenAppCountriesResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	aikidoClient, ok := req.ProviderData.(*client.AikidoClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.AikidoClient, got: %T.", req.ProviderData))
		return
	}
	r.client = aikidoClient
}

func (r *ZenAppCountriesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ZenAppCountriesResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, countries, diags := r.parseModel(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.UpdateZenAppCountries(ctx, appID, client.ZenAppCountriesRequest{
		Mode: data.Mode.ValueString(),
		List: countries,
	}); err != nil {
		resp.Diagnostics.AddError("Error Setting Countries", err.Error())
		return
	}

	data.ID = data.AppID
	tflog.Debug(ctx, "set zen app countries", map[string]interface{}{"app_id": appID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZenAppCountriesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ZenAppCountriesResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, err := strconv.Atoi(data.AppID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid App ID", fmt.Sprintf("Cannot parse app_id: %s", err))
		return
	}

	countries, err := r.client.GetZenAppCountries(ctx, appID)
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.Mode = types.StringValue(countries.Mode)
	codes := make([]string, len(countries.List))
	for i, c := range countries.List {
		codes[i] = c.Code
	}
	codeSet, d := types.SetValueFrom(ctx, types.StringType, codes)
	resp.Diagnostics.Append(d...)
	data.List = codeSet

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZenAppCountriesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ZenAppCountriesResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, countries, diags := r.parseModel(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.UpdateZenAppCountries(ctx, appID, client.ZenAppCountriesRequest{
		Mode: data.Mode.ValueString(),
		List: countries,
	}); err != nil {
		resp.Diagnostics.AddError("Error Updating Countries", err.Error())
		return
	}

	data.ID = data.AppID
	tflog.Debug(ctx, "updated zen app countries", map[string]interface{}{"app_id": appID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZenAppCountriesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ZenAppCountriesResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, err := strconv.Atoi(data.AppID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid App ID", fmt.Sprintf("Cannot parse app_id: %s", err))
		return
	}

	// Clear the country list on delete.
	if err := r.client.UpdateZenAppCountries(ctx, appID, client.ZenAppCountriesRequest{
		Mode: "block",
		List: []string{},
	}); err != nil {
		resp.Diagnostics.AddError("Error Clearing Countries", err.Error())
		return
	}

	tflog.Debug(ctx, "cleared zen app countries (delete)", map[string]interface{}{"app_id": appID})
}

func (r *ZenAppCountriesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("app_id"), req.ID)...)
}

func (r *ZenAppCountriesResource) parseModel(ctx context.Context, data *ZenAppCountriesResourceModel) (int, []string, diag.Diagnostics) {
	var diags diag.Diagnostics

	appID, err := strconv.Atoi(data.AppID.ValueString())
	if err != nil {
		diags.AddError("Invalid App ID", fmt.Sprintf("Cannot parse app_id: %s", err))
		return 0, nil, diags
	}

	var countries []string
	diags.Append(data.List.ElementsAs(ctx, &countries, false)...)

	return appID, countries, diags
}
