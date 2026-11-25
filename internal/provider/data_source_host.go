package provider

import (
	"context"

	"github.com/rushiii/terraform-provider-zabbix/internal/zabbix"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &hostDataSource{}
	_ datasource.DataSourceWithConfigure = &hostDataSource{}
)

// hostDataSource looks up an existing host, e.g. one created in the Zabbix UI or by discovery,
// to reference it without managing it.
type hostDataSource struct {
	client *zabbix.Client
}

type hostDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Host         types.String `tfsdk:"host"`
	VisibleName  types.String `tfsdk:"visible_name"`
	HostGroupIDs types.Set    `tfsdk:"host_group_ids"`
}

func NewHostDataSource() datasource.DataSource {
	return &hostDataSource{}
}

func (d *hostDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host"
}

func (d *hostDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up an existing Zabbix host by name, e.g. a host created in the Zabbix UI or by discovery.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Technical name (`host`) of the host, or its visible name.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Host ID.",
			},
			"host": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Technical name of the host.",
			},
			"visible_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Visible name of the host.",
			},
			"host_group_ids": schema.SetAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "IDs of the host groups the host belongs to.",
			},
		},
	}
}

func (d *hostDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	providerData, ok := req.ProviderData.(*providerData)
	if !ok || providerData.Client == nil {
		resp.Diagnostics.AddError("Invalid provider", "Zabbix client unavailable.")
		return
	}
	d.client = providerData.Client
}

func (d *hostDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config hostDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	host, err := d.client.HostGetByName(ctx, config.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("name"), "Host not found", err.Error())
		return
	}

	groupIDs := make([]string, 0, len(host.Groups))
	for _, g := range host.Groups {
		groupIDs = append(groupIDs, g.GroupID)
	}
	config.ID = types.StringValue(host.HostID)
	config.Host = types.StringValue(host.Host)
	config.VisibleName = types.StringValue(host.Name)
	config.HostGroupIDs = stringSetValue(ctx, groupIDs)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
