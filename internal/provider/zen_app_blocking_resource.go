package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

var _ resource.Resource = &ZenAppBlockingResource{}
var _ resource.ResourceWithImportState = &ZenAppBlockingResource{}

// NewZenAppBlockingResource creates a new Zen app blocking resource.
func NewZenAppBlockingResource() resource.Resource {
	return &ZenAppBlockingResource{}
}

// ZenAppBlockingResource manages the blocking mode of a Zen app.
type ZenAppBlockingResource struct {
	client *client.AikidoClient
}

// ZenAppBlockingResourceModel describes the resource data model.
type ZenAppBlockingResourceModel struct {
	ID                      types.String `tfsdk:"id"`
	AppID                   types.String `tfsdk:"app_id"`
	Block                   types.Bool   `tfsdk:"block"`
	DisableMinimumWaitCheck types.Bool   `tfsdk:"disable_minimum_wait_check"`
}

func (r *ZenAppBlockingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zen_app_blocking"
}

func (r *ZenAppBlockingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the blocking mode of a Zen app in Aikido Security.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The Zen app ID.",
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
			"block": schema.BoolAttribute{
				Required:            true,
				MarkdownDescription: "Whether to enable blocking mode.",
			},
			"disable_minimum_wait_check": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Skip the 3-day minimum wait period for production apps. Defaults to `false`.",
			},
		},
	}
}

func (r *ZenAppBlockingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ZenAppBlockingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ZenAppBlockingResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, err := strconv.Atoi(data.AppID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid App ID", fmt.Sprintf("Cannot parse app_id: %s", err))
		return
	}

	blockReq := client.ZenAppBlocking{Block: data.Block.ValueBool()}
	if !data.DisableMinimumWaitCheck.IsNull() {
		blockReq.DisableMinimumWaitCheck = data.DisableMinimumWaitCheck.ValueBool()
	}

	if err := r.client.UpdateZenAppBlocking(ctx, appID, blockReq); err != nil {
		resp.Diagnostics.AddError("Error Updating Blocking", fmt.Sprintf("Unable to update blocking: %s", err))
		return
	}

	data.ID = data.AppID
	tflog.Trace(ctx, "set zen app blocking", map[string]interface{}{"app_id": appID, "block": data.Block.ValueBool()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZenAppBlockingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ZenAppBlockingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Blocking state is write-only — preserve from state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZenAppBlockingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ZenAppBlockingResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, err := strconv.Atoi(data.AppID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid App ID", fmt.Sprintf("Cannot parse app_id: %s", err))
		return
	}

	blockReq := client.ZenAppBlocking{Block: data.Block.ValueBool()}
	if !data.DisableMinimumWaitCheck.IsNull() {
		blockReq.DisableMinimumWaitCheck = data.DisableMinimumWaitCheck.ValueBool()
	}

	if err := r.client.UpdateZenAppBlocking(ctx, appID, blockReq); err != nil {
		resp.Diagnostics.AddError("Error Updating Blocking", fmt.Sprintf("Unable to update blocking: %s", err))
		return
	}

	data.ID = data.AppID
	tflog.Trace(ctx, "updated zen app blocking", map[string]interface{}{"app_id": appID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZenAppBlockingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ZenAppBlockingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, err := strconv.Atoi(data.AppID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid App ID", fmt.Sprintf("Cannot parse app_id: %s", err))
		return
	}

	// Disable blocking on delete.
	if err := r.client.UpdateZenAppBlocking(ctx, appID, client.ZenAppBlocking{Block: false}); err != nil {
		resp.Diagnostics.AddError("Error Disabling Blocking", fmt.Sprintf("Unable to disable blocking: %s", err))
		return
	}

	tflog.Trace(ctx, "disabled zen app blocking (delete)", map[string]interface{}{"app_id": appID})
}

func (r *ZenAppBlockingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("app_id"), req.ID)...)
}
