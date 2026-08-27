package provider

import (
	"context"
	"fmt"

	"github.com/gr8-toolkit/terraform-provider-starrocks/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &starrocksProvider{}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &starrocksProvider{version: version}
	}
}

type starrocksProvider struct {
	version string
}

type starrocksProviderModel struct {
	Host     types.String `tfsdk:"host"`
	Port     types.Int64  `tfsdk:"port"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

func (p *starrocksProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "starrocks"
	resp.Version = p.version
}

func (p *starrocksProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Required: true,
			},
			"port": schema.Int64Attribute{
				Required: true,
			},
			"username": schema.StringAttribute{
				Required: true,
			},
			"password": schema.StringAttribute{
				Required:  true,
				Sensitive: true,
			},
		},
	}
}

func (p *starrocksProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config starrocksProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	host := config.Host.ValueString()
	port := fmt.Sprintf("%d", config.Port.ValueInt64())
	username := config.Username.ValueString()
	password := config.Password.ValueString()

	hostPort := fmt.Sprintf("%s:%s", host, port)
	c, err := client.NewClient(hostPort, username, password)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create StarRocks Client", err.Error())
		return
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *starrocksProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func (p *starrocksProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewResourceGroupResource,
		NewCatalogResource,
		NewTableResource,
		NewIndexResource,
		NewDatabaseResource,
		NewGlobalVariableResource,
	}
}
