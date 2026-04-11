package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

var _ resource.Resource = &ZenAppIPBlocklistResource{}
var _ resource.ResourceWithImportState = &ZenAppIPBlocklistResource{}

// NewZenAppIPBlocklistResource creates a new Zen app IP blocklist resource.
func NewZenAppIPBlocklistResource() resource.Resource {
	return &ZenAppIPBlocklistResource{}
}

// ZenAppIPBlocklistResource manages the custom IP blocklist.
type ZenAppIPBlocklistResource struct {
	client *client.AikidoClient
}

// ZenAppIPBlocklistResourceModel describes the resource data model.
type ZenAppIPBlocklistResourceModel struct {
	ID          types.String `tfsdk:"id"`
	AppID       types.String `tfsdk:"app_id"`
	IPAddresses types.Set    `tfsdk:"ip_addresses"`
}

func (r *ZenAppIPBlocklistResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zen_app_ip_blocklist"
}

func (r *ZenAppIPBlocklistResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the custom IP blocklist for a Zen app in Aikido Security.",

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
			"ip_addresses": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Set of IP addresses or CIDR blocks to block (e.g., `198.51.100.1`, `192.0.2.0/24`).",
			},
		},
	}
}

func (r *ZenAppIPBlocklistResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ZenAppIPBlocklistResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ZenAppIPBlocklistResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, ips, diags := r.parseModel(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.UpdateZenAppIPBlocklist(ctx, appID, ips); err != nil {
		resp.Diagnostics.AddError("Error Setting IP Blocklist", err.Error())
		return
	}

	data.ID = data.AppID
	tflog.Trace(ctx, "set zen app IP blocklist", map[string]interface{}{"app_id": appID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZenAppIPBlocklistResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ZenAppIPBlocklistResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// IP blocklist is write-only — no GET endpoint. Preserve from state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZenAppIPBlocklistResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ZenAppIPBlocklistResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, ips, diags := r.parseModel(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.UpdateZenAppIPBlocklist(ctx, appID, ips); err != nil {
		resp.Diagnostics.AddError("Error Updating IP Blocklist", err.Error())
		return
	}

	data.ID = data.AppID
	tflog.Trace(ctx, "updated zen app IP blocklist", map[string]interface{}{"app_id": appID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZenAppIPBlocklistResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ZenAppIPBlocklistResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, err := strconv.Atoi(data.AppID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid App ID", fmt.Sprintf("Cannot parse app_id: %s", err))
		return
	}

	// Clear the blocklist on delete.
	if err := r.client.UpdateZenAppIPBlocklist(ctx, appID, []string{}); err != nil {
		resp.Diagnostics.AddError("Error Clearing IP Blocklist", err.Error())
		return
	}

	tflog.Trace(ctx, "cleared zen app IP blocklist (delete)", map[string]interface{}{"app_id": appID})
}

func (r *ZenAppIPBlocklistResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("app_id"), req.ID)...)
}

func (r *ZenAppIPBlocklistResource) parseModel(ctx context.Context, data *ZenAppIPBlocklistResourceModel) (int, []string, diag.Diagnostics) {
	var diags diag.Diagnostics

	appID, err := strconv.Atoi(data.AppID.ValueString())
	if err != nil {
		diags.AddError("Invalid App ID", fmt.Sprintf("Cannot parse app_id: %s", err))
		return 0, nil, diags
	}

	var ips []string
	diags.Append(data.IPAddresses.ElementsAs(ctx, &ips, false)...)

	return appID, ips, diags
}
