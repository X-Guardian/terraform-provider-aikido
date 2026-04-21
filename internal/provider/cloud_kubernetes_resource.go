package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

var _ resource.Resource = &CloudKubernetesResource{}

// NewCloudKubernetesResource creates a new Kubernetes cloud resource.
func NewCloudKubernetesResource() resource.Resource {
	return &CloudKubernetesResource{}
}

// CloudKubernetesResource defines the resource implementation.
type CloudKubernetesResource struct {
	client *client.AikidoClient
}

// CloudKubernetesResourceModel describes the resource data model.
type CloudKubernetesResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Environment         types.String `tfsdk:"environment"`
	ExcludedNamespaces  types.List   `tfsdk:"excluded_namespaces"`
	IncludedNamespaces  types.List   `tfsdk:"included_namespaces"`
	EnableImageScanning types.Bool   `tfsdk:"enable_image_scanning"`
	Endpoint            types.String `tfsdk:"endpoint"`
	AgentToken          types.String `tfsdk:"agent_token"`
	ExternalID          types.String `tfsdk:"external_id"`
}

func (r *CloudKubernetesResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_kubernetes"
}

func (r *CloudKubernetesResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Connects a Kubernetes cluster to Aikido Security.",

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
				MarkdownDescription: "A name for this Kubernetes cluster.",
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
			"excluded_namespaces": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Namespaces to exclude from monitoring. Mutually exclusive with `included_namespaces`.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"included_namespaces": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Namespaces to include for monitoring. Mutually exclusive with `excluded_namespaces`.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"enable_image_scanning": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Enable container image scanning. Defaults to `false`.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"endpoint": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The agent data endpoint URL.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"agent_token": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "The agent installation token.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"external_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The external identifier.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *CloudKubernetesResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *CloudKubernetesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CloudKubernetesResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.CreateKubernetesCloudRequest{
		Name:        data.Name.ValueString(),
		Environment: data.Environment.ValueString(),
	}

	if !data.ExcludedNamespaces.IsNull() {
		resp.Diagnostics.Append(data.ExcludedNamespaces.ElementsAs(ctx, &createReq.ExcludedNamespaces, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	if !data.IncludedNamespaces.IsNull() {
		resp.Diagnostics.Append(data.IncludedNamespaces.ElementsAs(ctx, &createReq.IncludedNamespaces, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	if !data.EnableImageScanning.IsNull() {
		createReq.EnableImageScanning = data.EnableImageScanning.ValueBool()
	}

	k8sResp, err := r.client.CreateKubernetesCloud(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Kubernetes Cloud", fmt.Sprintf("Unable to create Kubernetes cloud: %s", err))
		return
	}

	data.ID = types.StringValue(strconv.Itoa(k8sResp.ID))
	data.Endpoint = types.StringValue(k8sResp.Endpoint)
	data.AgentToken = types.StringValue(k8sResp.AgentToken)

	cloud, err := r.client.GetCloud(ctx, k8sResp.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Kubernetes Cloud", fmt.Sprintf("Unable to read cloud after create: %s", err))
		return
	}

	data.ExternalID = types.StringValue(cloud.ExternalID)

	tflog.Debug(ctx, "created Kubernetes cloud", map[string]interface{}{"id": k8sResp.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudKubernetesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CloudKubernetesResourceModel

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
		resp.Diagnostics.AddError("Error Reading Kubernetes Cloud", fmt.Sprintf("Unable to read cloud %d: %s", cloudID, err))
		return
	}

	data.Name = types.StringValue(cloud.Name)
	data.Environment = types.StringValue(cloud.Environment)
	data.ExternalID = types.StringValue(cloud.ExternalID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudKubernetesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unexpected Update", "Kubernetes cloud does not support in-place updates. All attributes require replacement.")
}

func (r *CloudKubernetesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CloudKubernetesResourceModel

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
