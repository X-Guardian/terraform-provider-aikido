package provider

import (
	"context"
	"fmt"
	"strconv"

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

var _ resource.Resource = &ZenAppBotListsResource{}
var _ resource.ResourceWithImportState = &ZenAppBotListsResource{}

// NewZenAppBotListsResource creates a new Zen app bot lists resource.
func NewZenAppBotListsResource() resource.Resource {
	return &ZenAppBotListsResource{}
}

// ZenAppBotListsResource manages bot list subscriptions.
type ZenAppBotListsResource struct {
	client *client.AikidoClient
}

// ZenAppBotListsResourceModel describes the resource data model.
type ZenAppBotListsResourceModel struct {
	ID    types.String         `tfsdk:"id"`
	AppID types.String         `tfsdk:"app_id"`
	Bots  []ZenAppBotListEntry `tfsdk:"bots"`
}

// ZenAppBotListEntry describes a single bot list subscription.
type ZenAppBotListEntry struct {
	Code types.String `tfsdk:"code"`
	Mode types.String `tfsdk:"mode"`
}

func (r *ZenAppBotListsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zen_app_bot_lists"
}

func (r *ZenAppBotListsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages bot list subscriptions for a Zen app in Aikido Security.",

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
			"bots": schema.ListNestedAttribute{
				Required:            true,
				MarkdownDescription: "Bot list subscriptions. Omitted categories default to `ignore`.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"code": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The bot category code.",
						},
						"mode": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Subscription mode: `monitor` or `block`.",
							Validators: []validator.String{
								stringvalidator.OneOf("monitor", "block"),
							},
						},
					},
				},
			},
		},
	}
}

func (r *ZenAppBotListsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ZenAppBotListsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ZenAppBotListsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, err := strconv.Atoi(data.AppID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid App ID", fmt.Sprintf("Cannot parse app_id: %s", err))
		return
	}

	items := r.modelToItems(&data)
	if err := r.client.UpdateZenAppBotLists(ctx, appID, items); err != nil {
		resp.Diagnostics.AddError("Error Setting Bot Lists", err.Error())
		return
	}

	data.ID = data.AppID
	tflog.Debug(ctx, "set zen app bot lists", map[string]interface{}{"app_id": appID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZenAppBotListsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ZenAppBotListsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, err := strconv.Atoi(data.AppID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid App ID", fmt.Sprintf("Cannot parse app_id: %s", err))
		return
	}

	botLists, err := r.client.GetZenAppBotLists(ctx, appID)
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Only include non-ignore entries (those actively managed).
	var bots []ZenAppBotListEntry
	for _, b := range botLists {
		if b.Mode != "ignore" {
			bots = append(bots, ZenAppBotListEntry{
				Code: types.StringValue(b.Code),
				Mode: types.StringValue(b.Mode),
			})
		}
	}
	data.Bots = bots

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZenAppBotListsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ZenAppBotListsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, err := strconv.Atoi(data.AppID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid App ID", fmt.Sprintf("Cannot parse app_id: %s", err))
		return
	}

	items := r.modelToItems(&data)
	if err := r.client.UpdateZenAppBotLists(ctx, appID, items); err != nil {
		resp.Diagnostics.AddError("Error Updating Bot Lists", err.Error())
		return
	}

	data.ID = data.AppID
	tflog.Debug(ctx, "updated zen app bot lists", map[string]interface{}{"app_id": appID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZenAppBotListsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ZenAppBotListsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, err := strconv.Atoi(data.AppID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid App ID", fmt.Sprintf("Cannot parse app_id: %s", err))
		return
	}

	// Reset all to empty (API will set them to ignore).
	if err := r.client.UpdateZenAppBotLists(ctx, appID, []client.ZenAppBotListUpdateItem{}); err != nil {
		resp.Diagnostics.AddError("Error Clearing Bot Lists", err.Error())
		return
	}

	tflog.Debug(ctx, "cleared zen app bot lists (delete)", map[string]interface{}{"app_id": appID})
}

func (r *ZenAppBotListsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("app_id"), req.ID)...)
}

func (r *ZenAppBotListsResource) modelToItems(data *ZenAppBotListsResourceModel) []client.ZenAppBotListUpdateItem {
	items := make([]client.ZenAppBotListUpdateItem, len(data.Bots))
	for i, b := range data.Bots {
		items[i] = client.ZenAppBotListUpdateItem{
			Code: b.Code.ValueString(),
			Mode: b.Mode.ValueString(),
		}
	}
	return items
}
