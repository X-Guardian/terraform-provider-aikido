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

var _ resource.Resource = &WebhookResource{}
var _ resource.ResourceWithImportState = &WebhookResource{}

// NewWebhookResource creates a new webhook resource.
func NewWebhookResource() resource.Resource {
	return &WebhookResource{}
}

// WebhookResource defines the resource implementation.
type WebhookResource struct {
	client *client.AikidoClient
}

// WebhookResourceModel describes the resource data model.
type WebhookResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	TargetURL            types.String `tfsdk:"target_url"`
	EventType            types.String `tfsdk:"event_type"`
	HealthStatus         types.String `tfsdk:"health_status"`
	LatestHTTPStatusCode types.Int64  `tfsdk:"latest_http_status_code"`
}

func (r *WebhookResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (r *WebhookResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a webhook in Aikido Security.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the webhook.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"target_url": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The target URL for the webhook.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"event_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The event type that triggers the webhook.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(
						"issue.open.created",
						"issue.snoozed",
						"issue.ignored.manual",
						"issue.closed",
						"issue.unignored",
						"issue.severity.changed.manual",
						"issue.sla.breached",
						"ci.gate.failed",
						"ci.gate.passed",
						"zen.attack",
						"zen.attack_wave",
						"zen.outbound.discovered",
						"scan.image.finished",
					),
				},
			},
			"health_status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The health status of the webhook: `unknown`, `failing`, or `success`.",
			},
			"latest_http_status_code": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "The HTTP status code from the latest webhook delivery.",
			},
		},
	}
}

func (r *WebhookResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *WebhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WebhookResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	webhookID, err := r.client.CreateWebhook(ctx, client.CreateWebhookRequest{
		TargetURL: data.TargetURL.ValueString(),
		EventType: data.EventType.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Webhook", fmt.Sprintf("Unable to create webhook: %s", err))
		return
	}

	data.ID = types.StringValue(strconv.Itoa(webhookID))
	data.HealthStatus = types.StringValue("unknown")
	data.LatestHTTPStatusCode = types.Int64Value(0)

	tflog.Trace(ctx, "created webhook", map[string]interface{}{"id": webhookID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WebhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WebhookResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	webhook, err := r.client.GetWebhook(ctx, data.ID.ValueString())
	if err != nil {
		resp.State.RemoveResource(ctx)
		tflog.Warn(ctx, "webhook not found, removing from state", map[string]interface{}{"id": data.ID.ValueString()})
		return
	}

	data.ID = types.StringValue(webhook.ID)
	data.TargetURL = types.StringValue(webhook.TargetURL)
	data.EventType = types.StringValue(webhook.EventType)
	data.HealthStatus = types.StringValue(webhook.HealthStatus)
	data.LatestHTTPStatusCode = types.Int64Value(int64(webhook.LatestHTTPStatusCode))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WebhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unexpected Update", "Webhook does not support in-place updates. All attributes require replacement.")
}

func (r *WebhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WebhookResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	webhookID, err := strconv.Atoi(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Webhook ID", fmt.Sprintf("Cannot parse webhook ID: %s", err))
		return
	}

	if err := r.client.DeleteWebhook(ctx, webhookID); err != nil {
		resp.Diagnostics.AddError("Error Deleting Webhook", fmt.Sprintf("Unable to delete webhook %d: %s", webhookID, err))
		return
	}

	tflog.Trace(ctx, "deleted webhook", map[string]interface{}{"id": webhookID})
}

func (r *WebhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
