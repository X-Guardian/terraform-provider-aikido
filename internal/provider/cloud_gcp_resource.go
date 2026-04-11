// Copyright IBM Corp. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

var _ resource.Resource = &CloudGCPResource{}

// NewCloudGCPResource creates a new GCP cloud resource.
func NewCloudGCPResource() resource.Resource {
	return &CloudGCPResource{}
}

// CloudGCPResource defines the resource implementation.
type CloudGCPResource struct {
	client *client.AikidoClient
}

// CloudGCPResourceModel describes the resource data model.
type CloudGCPResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Environment types.String `tfsdk:"environment"`
	ProjectID   types.String `tfsdk:"project_id"`
	AccessKey   types.String `tfsdk:"access_key"`
	ExternalID  types.String `tfsdk:"external_id"`
}

func (r *CloudGCPResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_gcp"
}

func (r *CloudGCPResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Connects a GCP cloud environment to Aikido Security.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the cloud environment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "A name for this cloud environment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"environment": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The environment classification: `production`, `staging`, `development`, or `mixed`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("production", "staging", "development", "mixed"),
				},
			},
			"project_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The GCP project identifier.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"access_key": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "Stringified JSON of the service account access key or Workload Identity Federation config.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"external_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The external identifier from GCP.",
			},
		},
	}
}

func (r *CloudGCPResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *CloudGCPResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CloudGCPResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudID, err := r.client.CreateGCPCloud(ctx, client.CreateGCPCloudRequest{
		Name:        data.Name.ValueString(),
		Environment: data.Environment.ValueString(),
		ProjectID:   data.ProjectID.ValueString(),
		AccessKey:   data.AccessKey.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error Creating GCP Cloud", fmt.Sprintf("Unable to create GCP cloud: %s", err))
		return
	}

	data.ID = types.StringValue(strconv.Itoa(cloudID))

	cloud, err := r.client.GetCloud(ctx, cloudID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading GCP Cloud", fmt.Sprintf("Unable to read cloud after create: %s", err))
		return
	}

	data.ExternalID = types.StringValue(cloud.ExternalID)

	tflog.Trace(ctx, "created GCP cloud", map[string]interface{}{"id": cloudID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudGCPResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CloudGCPResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudID, err := strconv.Atoi(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Cloud ID", fmt.Sprintf("Cannot parse cloud ID: %s", err))
		return
	}

	cloud, err := r.client.GetCloud(ctx, cloudID)
	if err != nil {
		resp.State.RemoveResource(ctx)
		tflog.Warn(ctx, "cloud not found, removing from state", map[string]interface{}{"id": cloudID})
		return
	}

	data.Name = types.StringValue(cloud.Name)
	data.Environment = types.StringValue(cloud.Environment)
	data.ExternalID = types.StringValue(cloud.ExternalID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudGCPResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unexpected Update", "GCP cloud does not support in-place updates. All attributes require replacement.")
}

func (r *CloudGCPResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CloudGCPResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudID, err := strconv.Atoi(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Cloud ID", fmt.Sprintf("Cannot parse cloud ID: %s", err))
		return
	}

	if err := r.client.DeleteCloud(ctx, cloudID); err != nil {
		resp.Diagnostics.AddError("Error Deleting Cloud", fmt.Sprintf("Unable to delete cloud %d: %s", cloudID, err))
		return
	}

	tflog.Trace(ctx, "deleted cloud", map[string]interface{}{"id": cloudID})
}
