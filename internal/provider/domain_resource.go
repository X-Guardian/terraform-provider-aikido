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

var _ resource.Resource = &DomainResource{}
var _ resource.ResourceWithImportState = &DomainResource{}

// NewDomainResource creates a new domain resource.
func NewDomainResource() resource.Resource {
	return &DomainResource{}
}

// DomainResource defines the resource implementation.
type DomainResource struct {
	client *client.AikidoClient
}

// DomainResourceModel describes the resource data model.
type DomainResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Domain         types.String `tfsdk:"domain"`
	Kind           types.String `tfsdk:"kind"`
	OpenAPISpecURL types.String `tfsdk:"openapi_spec_url"`
}

func (r *DomainResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *DomainResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a domain in Aikido Security for surface monitoring and DAST scanning.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the domain.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"domain": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The domain name (e.g., `example.com`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"kind": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The type of domain: `front_end`, `rest_api`, or `graphql_api`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("front_end", "rest_api", "graphql_api"),
				},
			},
			"openapi_spec_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "URL to a JSON OpenAPI spec. Only applicable for `rest_api` or `graphql_api` kinds.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *DomainResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DomainResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.CreateDomainRequest{
		Domain: data.Domain.ValueString(),
		Kind:   data.Kind.ValueString(),
	}
	if !data.OpenAPISpecURL.IsNull() {
		createReq.OpenAPISpecURL = data.OpenAPISpecURL.ValueString()
	}

	domainID, err := r.client.CreateDomain(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Domain", fmt.Sprintf("Unable to create domain: %s", err))
		return
	}

	data.ID = types.StringValue(strconv.Itoa(domainID))

	tflog.Debug(ctx, "created domain", map[string]interface{}{"id": domainID, "domain": data.Domain.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DomainResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID, err := strconv.Atoi(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Domain ID", fmt.Sprintf("Cannot parse domain ID: %s", err))
		return
	}

	domain, err := r.client.GetDomain(ctx, domainID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			tflog.Warn(ctx, "domain not found, removing from state", map[string]interface{}{"id": domainID})
			return
		}
		resp.Diagnostics.AddError("Error Reading Domain", fmt.Sprintf("Unable to read domain %d: %s", domainID, err))
		return
	}

	data.Domain = types.StringValue(domain.Domain)
	data.Kind = types.StringValue(domain.Kind)
	// openapi_spec_url is write-only — preserve from state

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unexpected Update", "Domain does not support in-place updates. All attributes require replacement.")
}

func (r *DomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DomainResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID, err := strconv.Atoi(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Domain ID", fmt.Sprintf("Cannot parse domain ID: %s", err))
		return
	}

	if err := r.client.DeleteDomain(ctx, domainID); err != nil {
		resp.Diagnostics.AddError("Error Deleting Domain", fmt.Sprintf("Unable to delete domain %d: %s", domainID, err))
		return
	}

	tflog.Debug(ctx, "deleted domain", map[string]interface{}{"id": domainID})
}

func (r *DomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
