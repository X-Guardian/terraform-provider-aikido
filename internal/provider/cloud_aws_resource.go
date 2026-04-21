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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

var _ resource.Resource = &CloudAWSResource{}
var _ resource.ResourceWithImportState = &CloudAWSResource{}

// NewCloudAWSResource creates a new AWS cloud resource.
func NewCloudAWSResource() resource.Resource {
	return &CloudAWSResource{}
}

// CloudAWSResource defines the resource implementation.
type CloudAWSResource struct {
	client *client.AikidoClient
}

// CloudAWSResourceModel describes the resource data model.
type CloudAWSResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Environment types.String `tfsdk:"environment"`
	RoleARN     types.String `tfsdk:"role_arn"`
	ExternalID  types.String `tfsdk:"external_id"`
}

func (r *CloudAWSResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_aws"
}

func (r *CloudAWSResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Connects an AWS cloud environment to Aikido Security.",

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
			"role_arn": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ARN for the IAM role that Aikido can assume to access the AWS account.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"external_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The AWS account ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *CloudAWSResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *CloudAWSResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CloudAWSResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudID, err := r.client.CreateAWSCloud(ctx, client.CreateAWSCloudRequest{
		Name:        data.Name.ValueString(),
		Environment: data.Environment.ValueString(),
		RoleARN:     data.RoleARN.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error Creating AWS Cloud", fmt.Sprintf("Unable to create AWS cloud: %s", err))
		return
	}

	data.ID = types.StringValue(strconv.Itoa(cloudID))

	cloud, err := r.client.GetCloud(ctx, cloudID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading AWS Cloud", fmt.Sprintf("Unable to read cloud after create: %s", err))
		return
	}

	data.ExternalID = types.StringValue(cloud.ExternalID)

	tflog.Debug(ctx, "created AWS cloud", map[string]interface{}{"id": cloudID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudAWSResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CloudAWSResourceModel

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
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			tflog.Warn(ctx, "cloud not found, removing from state", map[string]interface{}{"id": cloudID})
			return
		}
		resp.Diagnostics.AddError("Error Reading AWS Cloud", fmt.Sprintf("Unable to read cloud %d: %s", cloudID, err))
		return
	}

	data.Name = types.StringValue(cloud.Name)
	data.Environment = types.StringValue(cloud.Environment)
	data.ExternalID = types.StringValue(cloud.ExternalID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudAWSResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unexpected Update", "AWS cloud does not support in-place updates. All attributes require replacement.")
}

func (r *CloudAWSResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CloudAWSResourceModel

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

	tflog.Debug(ctx, "deleted cloud", map[string]interface{}{"id": cloudID})
}

func (r *CloudAWSResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid Import ID", "Import ID must be in the format `cloud_id:role_arn` (e.g., `123:arn:aws:iam::000000000000:role/aikido-role`).")
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("role_arn"), parts[1])...)
}
