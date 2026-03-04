package provider

import (
	"context"

	"github.com/rushiii/terraform-provider-zabbix/internal/zabbix"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &itemResource{}
	_ resource.ResourceWithConfigure   = &itemResource{}
	_ resource.ResourceWithImportState = &itemResource{}
)

type itemResource struct {
	client *zabbix.Client
}

type itemResourceModel struct {
	ID        types.String `tfsdk:"id"`
	HostID    types.String `tfsdk:"host_id"`
	Name      types.String `tfsdk:"name"`
	Key       types.String `tfsdk:"key"`
	Type      types.Int64  `tfsdk:"type"`       
	ValueType types.Int64  `tfsdk:"value_type"` 
	SNMPOid   types.String `tfsdk:"snmp_oid"`
	Units     types.String `tfsdk:"units"`
	Delay     types.String `tfsdk:"delay"`
	History   types.String `tfsdk:"history"`
	Trends    types.String `tfsdk:"trends"`
	DelayFlex types.String `tfsdk:"delay_flex"`
	Enabled   types.Bool   `tfsdk:"enabled"`
}

func NewItemResource() resource.Resource {
	return &itemResource{}
}

func (r *itemResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_item"
}

func (r *itemResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Zabbix item resource (collection element). Supports Zabbix agent (0), SNMP (1,2,3), simple check (5), etc.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"host_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				MarkdownDescription: "ID of the host or template to attach the item to. Changing this forces recreation.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Item name (e.g. Lamp time, ICMP ping).",
			},
			"key": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Item key (e.g. epson.lamp.hours, agent.ping, icmpping).",
			},
			"type": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(0),
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
				MarkdownDescription: "Item type (Zabbix 6.4): 0=Zabbix agent, 2=Zabbix trapper, 5=Simple check, 7=Zabbix agent (active), 20=SNMP agent. Changing this forces recreation.",
			},
			"value_type": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(3),
				MarkdownDescription: "Value type: 0=float, 1=string, 2=log, 3=unsigned, 4=text.",
			},
			"snmp_oid": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "SNMP OID. Required only for SNMP types (1, 2, 3).",
			},
			"units": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Display units (e.g. !h for hours).",
			},
			"delay": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("10m"),
				MarkdownDescription: "Update interval (e.g. 10m, 60s).",
			},
			"history": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("90d"),
				MarkdownDescription: "History storage period (e.g. 90d).",
			},
			"trends": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("365d"),
				MarkdownDescription: "Trends storage period (e.g. 365d).",
			},
			"delay_flex": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Flexible interval, e.g. 50s;1-7,00:00-24:00 (50s Mon-Sun 24/7).",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether the item is enabled.",
			},
		},
	}
}

func (r *itemResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	providerData, ok := req.ProviderData.(*providerData)
	if !ok || providerData.Client == nil {
		resp.Diagnostics.AddError("Invalid provider", "Zabbix client unavailable.")
		return
	}
	r.client = providerData.Client
}

func (r *itemResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan itemResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zreq := zabbix.ItemCreateRequest{
		HostID:    plan.HostID.ValueString(),
		Name:      plan.Name.ValueString(),
		Key:       plan.Key.ValueString(),
		Type:      int(plan.Type.ValueInt64()),
		ValueType: int(plan.ValueType.ValueInt64()),
		SNMPOid:   plan.SNMPOid.ValueString(),
		Units:     plan.Units.ValueString(),
		Delay:     plan.Delay.ValueString(),
		History:   plan.History.ValueString(),
		Trends:    plan.Trends.ValueString(),
		DelayFlex: plan.DelayFlex.ValueString(),
		Enabled:   plan.Enabled.ValueBool(),
	}

	id, err := r.client.ItemCreate(ctx, zreq)
	if err != nil {
		resp.Diagnostics.AddError("item.create error", err.Error())
		return
	}

	plan.ID = types.StringValue(id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *itemResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state itemResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	item, err := r.client.ItemGetByID(ctx, state.ID.ValueString())
	if err != nil {
		if zabbix.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("item.get error", err.Error())
		return
	}

	state.HostID = types.StringValue(item.HostID)
	state.Name = types.StringValue(item.Name)
	state.Key = types.StringValue(item.Key)
	state.Type = types.Int64Value(int64(item.Type))
	state.ValueType = types.Int64Value(int64(item.ValueType))
	if item.SNMPOid != "" {
		state.SNMPOid = types.StringValue(item.SNMPOid)
	} else if int64(item.Type) == zabbix.ItemTypeSNMPAgent && isNumericOIDString(item.Key) {
		// Safety net: Zabbix may return empty snmp_oid for template SNMP items; reconstruct from key_.
		state.SNMPOid = types.StringValue(item.Key)
	} else {
		state.SNMPOid = types.StringNull()
	}
	if item.Units != "" {
		state.Units = types.StringValue(item.Units)
	} else {
		state.Units = types.StringNull()
	}
	state.Delay = types.StringValue(item.Delay)
	state.History = types.StringValue(item.History)
	state.Trends = types.StringValue(item.Trends)
	if item.DelayFlex != "" {
		state.DelayFlex = types.StringValue(item.DelayFlex)
	} else {
		state.DelayFlex = types.StringNull()
	}
	state.Enabled = types.BoolValue(zabbix.StatusToEnabled(item.Status))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *itemResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan itemResourceModel
	var state itemResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zreq := zabbix.ItemCreateRequest{
		HostID:    plan.HostID.ValueString(),
		Name:      plan.Name.ValueString(),
		Key:       plan.Key.ValueString(),
		Type:      int(plan.Type.ValueInt64()),
		ValueType: int(plan.ValueType.ValueInt64()),
		SNMPOid:   plan.SNMPOid.ValueString(),
		Units:     plan.Units.ValueString(),
		Delay:     plan.Delay.ValueString(),
		History:   plan.History.ValueString(),
		Trends:    plan.Trends.ValueString(),
		DelayFlex: plan.DelayFlex.ValueString(),
		Enabled:   plan.Enabled.ValueBool(),
	}

	if err := r.client.ItemUpdate(ctx, state.ID.ValueString(), zreq); err != nil {
		resp.Diagnostics.AddError("item.update error", err.Error())
		return
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *itemResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state itemResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.ItemDelete(ctx, state.ID.ValueString())
	if err != nil && !zabbix.IsNotFound(err) {
		resp.Diagnostics.AddError("item.delete error", err.Error())
	}
}

func (r *itemResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func isNumericOIDString(value string) bool {
	if value == "" {
		return false
	}
	start := 0
	if value[0] == '.' {
		start = 1
	}
	if start >= len(value) {
		return false
	}
	for i := start; i < len(value); i++ {
		ch := value[i]
		if (ch < '0' || ch > '9') && ch != '.' {
			return false
		}
	}
	return true
}
