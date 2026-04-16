package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

var _ resource.Resource = &ZenAppResource{}
var _ resource.ResourceWithImportState = &ZenAppResource{}

// NewZenAppResource creates a new Zen app resource.
func NewZenAppResource() resource.Resource {
	return &ZenAppResource{}
}

// ZenAppResource defines the resource implementation.
type ZenAppResource struct {
	client *client.AikidoClient
}

// ZenAppResourceModel describes the resource data model.
type ZenAppResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Environment  types.String `tfsdk:"environment"`
	RepoID       types.String `tfsdk:"repo_id"`
	Token        types.String `tfsdk:"token"`
	TokenHint    types.String `tfsdk:"token_hint"`
	HasToken     types.Bool   `tfsdk:"has_token"`
	Blocking     types.Bool   `tfsdk:"blocking"`
	CodeRepoName types.String `tfsdk:"code_repo_name"`
}

func (r *ZenAppResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zen_app"
}

func (r *ZenAppResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Zen runtime application firewall (WAF) app in Aikido Security.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the Zen app.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the Zen app.",
			},
			"environment": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The environment: `production`, `staging`, or `development`.",
				Validators: []validator.String{
					stringvalidator.OneOf("production", "staging", "development"),
				},
			},
			"repo_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The ID of a code repository to link to this app.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"token": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "The Zen app token. Only available after initial creation.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"token_hint": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "A hint of the current active token.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"has_token": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether a token is set for this app.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"blocking": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether blocking mode is enabled.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"code_repo_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The name of the linked code repository.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *ZenAppResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ZenAppResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ZenAppResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.CreateZenAppRequest{
		Name:        data.Name.ValueString(),
		Environment: data.Environment.ValueString(),
	}
	if !data.RepoID.IsNull() {
		repoID, err := strconv.Atoi(data.RepoID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid Repo ID", fmt.Sprintf("Cannot parse repo_id: %s", err))
			return
		}
		createReq.RepoID = &repoID
	}

	createResp, err := r.client.CreateZenApp(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Zen App", fmt.Sprintf("Unable to create Zen app: %s", err))
		return
	}

	data.ID = types.StringValue(strconv.Itoa(createResp.AppID))
	data.Token = types.StringValue(createResp.Token)

	// Read back to populate computed fields.
	app, err := r.client.GetZenApp(ctx, createResp.AppID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Zen App", fmt.Sprintf("Unable to read Zen app after create: %s", err))
		return
	}

	r.mapAppToModel(app, &data)

	tflog.Trace(ctx, "created zen app", map[string]interface{}{"id": createResp.AppID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZenAppResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ZenAppResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, err := strconv.Atoi(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid App ID", fmt.Sprintf("Cannot parse app ID: %s", err))
		return
	}

	app, err := r.client.GetZenApp(ctx, appID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			tflog.Warn(ctx, "zen app not found, removing from state", map[string]interface{}{"id": appID})
			return
		}
		resp.Diagnostics.AddError("Error Reading Zen App", fmt.Sprintf("Unable to read zen app %d: %s", appID, err))
		return
	}

	r.mapAppToModel(app, &data)
	// Token is preserved from state — not returned by GET.

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZenAppResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ZenAppResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, err := strconv.Atoi(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid App ID", fmt.Sprintf("Cannot parse app ID: %s", err))
		return
	}

	if err := r.client.UpdateZenApp(ctx, appID, client.UpdateZenAppRequest{
		Name:        data.Name.ValueString(),
		Environment: data.Environment.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Error Updating Zen App", fmt.Sprintf("Unable to update Zen app: %s", err))
		return
	}

	app, err := r.client.GetZenApp(ctx, appID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Zen App", fmt.Sprintf("Unable to read Zen app after update: %s", err))
		return
	}

	r.mapAppToModel(app, &data)

	tflog.Trace(ctx, "updated zen app", map[string]interface{}{"id": appID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZenAppResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ZenAppResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID, err := strconv.Atoi(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid App ID", fmt.Sprintf("Cannot parse app ID: %s", err))
		return
	}

	if err := r.client.DeleteZenApp(ctx, appID); err != nil {
		resp.Diagnostics.AddError("Error Deleting Zen App", fmt.Sprintf("Unable to delete Zen app %d: %s", appID, err))
		return
	}

	tflog.Trace(ctx, "deleted zen app", map[string]interface{}{"id": appID})
}

func (r *ZenAppResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapAppToModel populates the model from an API response. Does NOT overwrite Token.
func (r *ZenAppResource) mapAppToModel(app *client.ZenAppDetail, data *ZenAppResourceModel) {
	data.ID = types.StringValue(app.ID)
	data.Name = types.StringValue(app.Name)
	data.Environment = types.StringValue(app.Environment)
	data.TokenHint = types.StringValue(app.TokenHint)
	data.HasToken = types.BoolValue(app.HasToken)

	if app.CodeRepoID > 0 {
		data.RepoID = types.StringValue(strconv.Itoa(app.CodeRepoID))
		if app.CodeRepoName != nil {
			data.CodeRepoName = types.StringValue(*app.CodeRepoName)
		} else {
			data.CodeRepoName = types.StringNull()
		}
	} else {
		data.CodeRepoName = types.StringNull()
	}
}
