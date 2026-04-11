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

var _ resource.Resource = &CloudAzureResource{}

// NewCloudAzureResource creates a new Azure cloud resource.
func NewCloudAzureResource() resource.Resource {
	return &CloudAzureResource{}
}

// CloudAzureResource defines the resource implementation.
type CloudAzureResource struct {
	client *client.AikidoClient
}

// CloudAzureResourceModel describes the resource data model.
type CloudAzureResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Environment    types.String `tfsdk:"environment"`
	ApplicationID  types.String `tfsdk:"application_id"`
	DirectoryID    types.String `tfsdk:"directory_id"`
	SubscriptionID types.String `tfsdk:"subscription_id"`
	KeyValue       types.String `tfsdk:"key_value"`
	ExternalID     types.String `tfsdk:"external_id"`
}

func (r *CloudAzureResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_azure"
}

func (r *CloudAzureResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Connects an Azure cloud environment to Aikido Security.",

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
			"application_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The Application ID of the registered application for Aikido scanning.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"directory_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The Directory ID of the registered application.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subscription_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The relevant Azure Subscription ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"key_value": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "The generated secret for the registered application.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"external_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The external identifier from Azure.",
			},
		},
	}
}

func (r *CloudAzureResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *CloudAzureResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CloudAzureResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudID, err := r.client.CreateAzureCloud(ctx, client.CreateAzureCloudRequest{
		Name:           data.Name.ValueString(),
		Environment:    data.Environment.ValueString(),
		ApplicationID:  data.ApplicationID.ValueString(),
		DirectoryID:    data.DirectoryID.ValueString(),
		SubscriptionID: data.SubscriptionID.ValueString(),
		KeyValue:       data.KeyValue.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Azure Cloud", fmt.Sprintf("Unable to create Azure cloud: %s", err))
		return
	}

	data.ID = types.StringValue(strconv.Itoa(cloudID))

	cloud, err := r.client.GetCloud(ctx, cloudID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Azure Cloud", fmt.Sprintf("Unable to read cloud after create: %s", err))
		return
	}

	data.ExternalID = types.StringValue(cloud.ExternalID)

	tflog.Trace(ctx, "created Azure cloud", map[string]interface{}{"id": cloudID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudAzureResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CloudAzureResourceModel

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

func (r *CloudAzureResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unexpected Update", "Azure cloud does not support in-place updates. All attributes require replacement.")
}

func (r *CloudAzureResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CloudAzureResourceModel

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
