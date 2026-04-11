// Copyright IBM Corp. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

var _ datasource.DataSource = &CodeReposDataSource{}

// NewCodeReposDataSource creates a new code repos data source.
func NewCodeReposDataSource() datasource.DataSource {
	return &CodeReposDataSource{}
}

// CodeReposDataSource defines the data source implementation.
type CodeReposDataSource struct {
	client *client.AikidoClient
}

// CodeReposDataSourceModel describes the data source data model.
type CodeReposDataSourceModel struct {
	IncludeInactive types.Bool                `tfsdk:"include_inactive"`
	FilterName      types.String              `tfsdk:"filter_name"`
	FilterBranch    types.String              `tfsdk:"filter_branch"`
	Repos           []CodeRepoDataSourceModel `tfsdk:"repos"`
}

// CodeRepoDataSourceModel describes a single code repository.
type CodeRepoDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Provider     types.String `tfsdk:"provider_name"`
	ExternalID   types.String `tfsdk:"external_repo_id"`
	Active       types.Bool   `tfsdk:"active"`
	URL          types.String `tfsdk:"url"`
	Branch       types.String `tfsdk:"branch"`
	Connectivity types.String `tfsdk:"connectivity"`
	Sensitivity  types.String `tfsdk:"sensitivity"`
}

func (d *CodeReposDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_code_repos"
}

func (d *CodeReposDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists code repositories in the Aikido workspace.",

		Attributes: map[string]schema.Attribute{
			"include_inactive": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Include inactive repositories. Defaults to `false`.",
			},
			"filter_name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Filter repositories by name.",
			},
			"filter_branch": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Filter repositories by branch.",
			},
			"repos": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The list of code repositories.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique identifier of the repository.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The name of the repository.",
						},
						"provider_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The source provider (e.g., github, gitlab, bitbucket).",
						},
						"external_repo_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The external identifier from the source provider.",
						},
						"active": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether scanning is active for this repository.",
						},
						"url": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The API URL of the repository.",
						},
						"branch": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The default branch being scanned.",
						},
						"connectivity": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The connectivity status (e.g., connected).",
						},
						"sensitivity": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The sensitivity level (e.g., normal).",
						},
					},
				},
			},
		},
	}
}

func (d *CodeReposDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	aikidoClient, ok := req.ProviderData.(*client.AikidoClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.AikidoClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = aikidoClient
}

func (d *CodeReposDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data CodeReposDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := &client.ListCodeReposOptions{}

	if !data.IncludeInactive.IsNull() && data.IncludeInactive.ValueBool() {
		opts.IncludeInactive = true
	}
	if !data.FilterName.IsNull() {
		opts.FilterName = data.FilterName.ValueString()
	}
	if !data.FilterBranch.IsNull() {
		opts.FilterBranch = data.FilterBranch.ValueString()
	}

	repos, err := d.client.ListCodeRepos(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error Listing Code Repositories", fmt.Sprintf("Unable to list code repositories: %s", err))
		return
	}

	data.Repos = make([]CodeRepoDataSourceModel, len(repos))
	for i, repo := range repos {
		data.Repos[i] = CodeRepoDataSourceModel{
			ID:           types.StringValue(strconv.Itoa(repo.ID)),
			Name:         types.StringValue(repo.Name),
			Provider:     types.StringValue(repo.Provider),
			ExternalID:   types.StringValue(repo.ExternalRepoID),
			Active:       types.BoolValue(repo.Active),
			URL:          types.StringValue(repo.URL),
			Branch:       types.StringValue(repo.Branch),
			Connectivity: types.StringValue(repo.Connectivity),
			Sensitivity:  types.StringValue(repo.Sensitivity),
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
