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

var _ resource.Resource = &ZenAppIPListsResource{}
var _ resource.ResourceWithImportState = &ZenAppIPListsResource{}

// NewZenAppIPListsResource creates a new Zen app IP lists resource.
func NewZenAppIPListsResource() resource.Resource {
	return &ZenAppIPListsResource{}
}

// ZenAppIPListsResource manages threat actor and Tor traffic configuration.
type ZenAppIPListsResource struct {
	client *client.AikidoClient
}

// ZenAppIPListsResourceModel describes the resource data model.
type ZenAppIPListsResourceModel struct {
	ID                types.String        `tfsdk:"id"`
	AppID             types.String        `tfsdk:"app_id"`
	KnownThreatActors []ZenAppIPListEntry `tfsdk:"known_threat_actors"`
	TorMode           types.String        `tfsdk:"tor_mode"`
}

// ZenAppIPListEntry describes a single threat actor list subscription.
type ZenAppIPListEntry struct {
	Code types.String `tfsdk:"code"`
	Mode types.String `tfsdk:"mode"`
}

func (r *ZenAppIPListsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zen_app_ip_lists"
}

func (r *ZenAppIPListsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages threat actor IP lists and Tor traffic configuration for a Zen app in Aikido Security.",

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
			"known_threat_actors": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Known threat actor IP list subscriptions.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"code": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The threat actor list code.",
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
			"tor_mode": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Tor traffic mode: `ignore`, `monitor`, or `block`.",
				Validators: []validator.String{
					stringvalidator.OneOf("ignore", "monitor", "block"),
				},
			},
		},
	}
}

func (r *ZenAppIPListsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ZenAppIPListsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ZenAppIPListsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, err := strconv.Atoi(data.AppID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid App ID", fmt.Sprintf("Cannot parse app_id: %s", err))
		return
	}

	if err := r.client.UpdateZenAppIPLists(ctx, appID, r.modelToRequest(&data)); err != nil {
		resp.Diagnostics.AddError("Error Setting IP Lists", err.Error())
		return
	}

	data.ID = data.AppID
	tflog.Debug(ctx, "set zen app IP lists", map[string]interface{}{"app_id": appID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZenAppIPListsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ZenAppIPListsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, err := strconv.Atoi(data.AppID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid App ID", fmt.Sprintf("Cannot parse app_id: %s", err))
		return
	}

	ipLists, err := r.client.GetZenAppIPLists(ctx, appID)
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	var actors []ZenAppIPListEntry
	for _, a := range ipLists.KnownThreatActors {
		if a.Mode != "ignore" {
			actors = append(actors, ZenAppIPListEntry{
				Code: types.StringValue(a.Code),
				Mode: types.StringValue(a.Mode),
			})
		}
	}
	data.KnownThreatActors = actors
	data.TorMode = types.StringValue(ipLists.Tor.Mode)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZenAppIPListsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ZenAppIPListsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, err := strconv.Atoi(data.AppID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid App ID", fmt.Sprintf("Cannot parse app_id: %s", err))
		return
	}

	if err := r.client.UpdateZenAppIPLists(ctx, appID, r.modelToRequest(&data)); err != nil {
		resp.Diagnostics.AddError("Error Updating IP Lists", err.Error())
		return
	}

	data.ID = data.AppID
	tflog.Debug(ctx, "updated zen app IP lists", map[string]interface{}{"app_id": appID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZenAppIPListsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ZenAppIPListsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, err := strconv.Atoi(data.AppID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid App ID", fmt.Sprintf("Cannot parse app_id: %s", err))
		return
	}

	// Reset to defaults.
	if err := r.client.UpdateZenAppIPLists(ctx, appID, client.ZenAppIPListsRequest{
		KnownThreatActors: []client.ZenAppIPListUpdateItem{},
		Tor:               &client.ZenAppTorConfig{Mode: "ignore"},
	}); err != nil {
		resp.Diagnostics.AddError("Error Clearing IP Lists", err.Error())
		return
	}

	tflog.Debug(ctx, "cleared zen app IP lists (delete)", map[string]interface{}{"app_id": appID})
}

func (r *ZenAppIPListsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("app_id"), req.ID)...)
}

func (r *ZenAppIPListsResource) modelToRequest(data *ZenAppIPListsResourceModel) client.ZenAppIPListsRequest {
	req := client.ZenAppIPListsRequest{}

	if data.KnownThreatActors != nil {
		req.KnownThreatActors = make([]client.ZenAppIPListUpdateItem, len(data.KnownThreatActors))
		for i, a := range data.KnownThreatActors {
			req.KnownThreatActors[i] = client.ZenAppIPListUpdateItem{
				Code: a.Code.ValueString(),
				Mode: a.Mode.ValueString(),
			}
		}
	}

	if !data.TorMode.IsNull() {
		req.Tor = &client.ZenAppTorConfig{Mode: data.TorMode.ValueString()}
	}

	return req
}
