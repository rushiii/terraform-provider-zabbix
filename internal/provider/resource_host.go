package provider

import (
	"context"
	"strconv"
	"strings"

	"github.com/rushiii/terraform-provider-zabbix/internal/zabbix"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &hostResource{}
	_ resource.ResourceWithConfigure      = &hostResource{}
	_ resource.ResourceWithImportState    = &hostResource{}
	_ resource.ResourceWithModifyPlan     = &hostResource{}
	_ resource.ResourceWithValidateConfig = &hostResource{}
)

type hostResource struct {
	client *zabbix.Client
}

type hostResourceModel struct {
	ID             types.String         `tfsdk:"id"`
	Name           types.String         `tfsdk:"name"`
	VisibleName    types.String         `tfsdk:"visible_name"`
	Description    types.String         `tfsdk:"description"`
	Enabled        types.Bool           `tfsdk:"enabled"`
	HostGroupIDs   types.Set            `tfsdk:"host_group_ids"`
	HostGroupNames types.Set            `tfsdk:"host_group_names"`
	TemplateIDs    types.Set            `tfsdk:"template_ids"`
	TemplateNames  types.Set            `tfsdk:"template_names"`
	Tags           types.Map            `tfsdk:"tags"`
	Interfaces     []hostInterfaceModel `tfsdk:"interfaces"`
}

type hostInterfaceModel struct {
	Type        types.Int64           `tfsdk:"type"` // 1=Agent,2=SNMP,3=IPMI,4=JMX
	Main        types.Bool            `tfsdk:"main"`
	UseIP       types.Bool            `tfsdk:"use_ip"`
	IP          types.String          `tfsdk:"ip"`
	DNS         types.String          `tfsdk:"dns"`
	Port        types.String          `tfsdk:"port"`
	SNMPDetails *hostSNMPDetailsModel `tfsdk:"snmp_details"`
}

type hostSNMPDetailsModel struct {
	Version   types.Int64  `tfsdk:"version"`
	Community types.String `tfsdk:"community"`
}

func NewHostResource() resource.Resource {
	return &hostResource{}
}

func (r *hostResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host"
}

func (r *hostResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Zabbix host resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Internal Zabbix ID.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Host technical name (`host`).",
			},
			"visible_name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Visible name (`name`) in Zabbix.",
			},
			"description": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Host description. When omitted, the description set in Zabbix is left unchanged.",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether the host is enabled.",
			},
			"host_group_ids": schema.SetAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Host group IDs. Computed from host_group_names when not set.",
			},
			"host_group_names": schema.SetAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Host group names, resolved to IDs at apply time. Alternative to host_group_ids; computed from it when not set.",
			},
			"template_ids": schema.SetAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Linked template IDs. Computed from template_names when not set; an empty set unlinks all templates.",
			},
			"template_names": schema.SetAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Template names (technical or visible), resolved to IDs at apply time. Alternative to template_ids; computed from it when not set.",
			},
			"tags": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Tags map (tag => value). When not set, the host's tags are left unchanged; `{}` removes them all.",
			},
		},
		Blocks: map[string]schema.Block{
			"interfaces": schema.ListNestedBlock{
				MarkdownDescription: "Host interfaces.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.Int64Attribute{
							Required:            true,
							MarkdownDescription: "1=Agent, 2=SNMP, 3=IPMI, 4=JMX.",
						},
						"main": schema.BoolAttribute{
							Optional: true,
							Computed: true,
							Default:  booldefault.StaticBool(true),
						},
						"use_ip": schema.BoolAttribute{
							Optional: true,
							Computed: true,
							Default:  booldefault.StaticBool(true),
						},
						"ip": schema.StringAttribute{
							Optional: true,
						},
						"dns": schema.StringAttribute{
							Optional: true,
						},
						"port": schema.StringAttribute{
							Optional: true,
							Computed: true,
							Default:  stringdefault.StaticString("10050"),
						},
					},
					Blocks: map[string]schema.Block{
						"snmp_details": schema.SingleNestedBlock{
							MarkdownDescription: "SNMP details (v2 supported). Used mainly with type=2.",
							Attributes: map[string]schema.Attribute{
								"version": schema.Int64Attribute{
									Optional:            true,
									Computed:            true,
									Default:             int64default.StaticInt64(2),
									MarkdownDescription: "SNMP version (2 supported).",
								},
								"community": schema.StringAttribute{
									Optional:            true,
									Computed:            true,
									Default:             stringdefault.StaticString("{$SNMP_COMMUNITY}"),
									MarkdownDescription: "SNMP v1/v2c community.",
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *hostResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ValidateConfig requires at least one host group. Unknown values (e.g. the ID of a host
// group created in the same apply) are accepted: they are only known at apply time.
func (r *hostResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var ids, names types.Set
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("host_group_ids"), &ids)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("host_group_names"), &names)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !mayHaveElements(ids) && !mayHaveElements(names) {
		resp.Diagnostics.AddAttributeError(
			path.Root("host_group_ids"),
			"Missing value",
			"Provide at least `host_group_ids` or `host_group_names`.",
		)
	}
}

// ModifyPlan plans the host group and template links without calling the API: names are
// resolved to IDs at apply time, so objects created in the same apply can be referenced.
func (r *hostResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var cfg, plan, state hostResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	creating := req.State.Raw.IsNull()
	if !creating {
		resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	plan.HostGroupIDs, plan.HostGroupNames = planLinks(ctx, cfg.HostGroupIDs, cfg.HostGroupNames, state.HostGroupIDs, state.HostGroupNames, creating)
	plan.TemplateIDs, plan.TemplateNames = planLinks(ctx, cfg.TemplateIDs, cfg.TemplateNames, state.TemplateIDs, state.TemplateNames, creating)
	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}

// setLinksAfterApply reads the host back and stores the host group and template links,
// keeping the planned values and filling in the computed ones.
func (r *hostResource) setLinksAfterApply(ctx context.Context, hostID string, plan *hostResourceModel) error {
	host, err := r.client.HostGetByID(ctx, hostID)
	if err != nil {
		return err
	}
	groups, templates := linkedObjects(host)
	plan.HostGroupIDs, plan.HostGroupNames = finalizeLinks(ctx, plan.HostGroupIDs, plan.HostGroupNames, groups)
	plan.TemplateIDs, plan.TemplateNames = finalizeLinks(ctx, plan.TemplateIDs, plan.TemplateNames, templates)
	return nil
}

// clearUnknownLinks replaces unknown link values by null, so a state can be saved even
// when the host could not be read back.
func clearUnknownLinks(plan *hostResourceModel) {
	for _, s := range []*types.Set{&plan.HostGroupIDs, &plan.HostGroupNames, &plan.TemplateIDs, &plan.TemplateNames} {
		if s.IsUnknown() {
			*s = types.SetNull(types.StringType)
		}
	}
}

func linkedObjects(host *zabbix.Host) (groups, templates []linkedObject) {
	for _, g := range host.Groups {
		groups = append(groups, linkedObject{ID: g.GroupID, Name: g.Name, Aliases: []string{g.Name}})
	}
	for _, t := range host.ParentTemplates {
		name := t.Name
		if name == "" {
			name = t.Host
		}
		templates = append(templates, linkedObject{ID: t.TemplateID, Name: name, Aliases: []string{t.Host, t.Name}})
	}
	return groups, templates
}

func (r *hostResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, cfg hostResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupIDs, d := resolveHostGroupIDs(ctx, r.client, cfg)
	resp.Diagnostics.Append(d...)
	templateIDs, d := resolveTemplateIDs(ctx, r.client, cfg)
	resp.Diagnostics.Append(d...)
	tags, d := mapToTags(ctx, plan.Tags)
	resp.Diagnostics.Append(d...)
	interfaces, d := expandInterfaces(plan.Interfaces)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	hostID, err := r.client.HostCreate(ctx, zabbix.HostCreateRequest{
		Host:        plan.Name.ValueString(),
		Name:        nullableString(plan.VisibleName),
		Description: knownStringPtr(plan.Description),
		Status:      boolToHostStatus(plan.Enabled),
		Interfaces:  interfaces,
		GroupIDs:    groupIDs,
		TemplateIDs: templateIDs,
		Tags:        tags,
	})
	if err != nil {
		resp.Diagnostics.AddError("host.create error", err.Error())
		return
	}

	plan.ID = types.StringValue(hostID)
	if plan.Description.IsUnknown() || plan.Description.IsNull() {
		plan.Description = types.StringValue("")
	}
	if err := r.setLinksAfterApply(ctx, hostID, &plan); err != nil {
		clearUnknownLinks(&plan)
		resp.Diagnostics.AddError("host.get error after create", err.Error())
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *hostResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state hostResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	host, err := r.client.HostGetByID(ctx, state.ID.ValueString())
	if err != nil {
		if zabbix.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("host.get error", err.Error())
		return
	}

	state.Name = types.StringValue(host.Host)
	// Without a visible name, Zabbix reports the technical name: keep visible_name unset then.
	if state.VisibleName.IsNull() && host.Name == host.Host {
		state.VisibleName = types.StringNull()
	} else {
		state.VisibleName = nullOrString(host.Name)
	}
	state.Description = types.StringValue(host.Description)
	state.Enabled = types.BoolValue(zabbix.StatusToEnabled(host.Status))
	state.Interfaces = flattenInterfaces(host.Interfaces)

	groups, templates := linkedObjects(host)
	state.HostGroupIDs, state.HostGroupNames = refreshLinks(ctx, state.HostGroupIDs, state.HostGroupNames, groups)
	state.TemplateIDs, state.TemplateNames = refreshLinks(ctx, state.TemplateIDs, state.TemplateNames, templates)

	// Tags are only managed when configured: an unset tags attribute leaves the host's tags
	// (added in the UI or by discovery) alone.
	if !state.Tags.IsNull() {
		state.Tags, _ = tagsToMap(ctx, host.Tags)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *hostResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, cfg, state hostResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupIDs, d := resolveHostGroupIDs(ctx, r.client, cfg)
	resp.Diagnostics.Append(d...)
	templateIDs, d := resolveTemplateIDs(ctx, r.client, cfg)
	resp.Diagnostics.Append(d...)
	tags, d := mapToTags(ctx, plan.Tags)
	resp.Diagnostics.Append(d...)
	interfaces, d := expandInterfaces(plan.Interfaces)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.HostUpdate(ctx, state.ID.ValueString(), zabbix.HostUpdateRequest{
		Host:        plan.Name.ValueString(),
		Name:        nullableString(plan.VisibleName),
		Description: knownStringPtr(plan.Description),
		Status:      boolToHostStatus(plan.Enabled),
		Interfaces:  interfaces,
		GroupIDs:    groupIDs,
		TemplateIDs: templateIDs,
		Tags:        tags,
	})
	if err != nil {
		resp.Diagnostics.AddError("host.update error", err.Error())
		return
	}

	plan.ID = state.ID
	if plan.Description.IsUnknown() || plan.Description.IsNull() {
		plan.Description = state.Description
	}
	if err := r.setLinksAfterApply(ctx, state.ID.ValueString(), &plan); err != nil {
		clearUnknownLinks(&plan)
		resp.Diagnostics.AddError("host.get error after update", err.Error())
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *hostResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state hostResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.HostDelete(ctx, state.ID.ValueString())
	if err != nil && !zabbix.IsNotFound(err) {
		resp.Diagnostics.AddError("host.delete error", err.Error())
	}
}

func (r *hostResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func boolToHostStatus(value types.Bool) int {
	if !value.IsNull() && value.ValueBool() {
		return 0
	}
	return 1
}

func nullableString(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	return value.ValueString()
}

// knownStringPtr returns nil for a null or unknown value, so the attribute is not sent to the API.
func knownStringPtr(value types.String) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	v := value.ValueString()
	return &v
}

func nullOrString(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

func setToStrings(ctx context.Context, value types.Set) ([]string, diag.Diagnostics) {
	var elems []types.String
	diags := value.ElementsAs(ctx, &elems, true)
	if diags.HasError() {
		return nil, diags
	}
	out := make([]string, 0, len(elems))
	for _, e := range elems {
		if e.IsNull() || e.IsUnknown() {
			continue
		}
		out = append(out, e.ValueString())
	}
	return out, diags
}

func setToStringsOptional(ctx context.Context, value types.Set) ([]string, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return []string{}, nil
	}
	return setToStrings(ctx, value)
}

// resolveHostGroupIDs returns the host group IDs to link, from the configuration (known at
// apply time): configured IDs plus configured names resolved through the API.
func resolveHostGroupIDs(ctx context.Context, client *zabbix.Client, cfg hostResourceModel) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics

	ids, d := setToStringsOptional(ctx, cfg.HostGroupIDs)
	diags.Append(d...)

	names, d := setToStringsOptional(ctx, cfg.HostGroupNames)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	if len(ids) == 0 && len(names) == 0 {
		diags.AddAttributeError(
			path.Root("host_group_ids"),
			"Missing value",
			"Provide at least `host_group_ids` or `host_group_names`.",
		)
		return nil, diags
	}

	resolvedIDs := make([]string, 0, len(ids)+len(names))
	seen := map[string]struct{}{}

	for _, id := range ids {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			resolvedIDs = append(resolvedIDs, id)
		}
	}

	if len(names) > 0 {
		byName, err := client.HostGroupIDsByNames(ctx, names)
		if err != nil {
			diags.AddError("Cannot resolve host groups", err.Error())
			return nil, diags
		}
		for _, id := range byName {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				resolvedIDs = append(resolvedIDs, id)
			}
		}
	}

	return resolvedIDs, diags
}

// resolveTemplateIDs returns the template IDs to link, from the configuration. It returns
// nil when neither template_ids nor template_names is configured: the linked templates are
// then left as they are. A configured empty set unlinks all templates.
func resolveTemplateIDs(ctx context.Context, client *zabbix.Client, cfg hostResourceModel) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics

	if cfg.TemplateIDs.IsNull() && cfg.TemplateNames.IsNull() {
		return nil, diags
	}

	ids, d := setToStringsOptional(ctx, cfg.TemplateIDs)
	diags.Append(d...)

	names, d := setToStringsOptional(ctx, cfg.TemplateNames)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	resolvedIDs := make([]string, 0, len(ids)+len(names))
	seen := map[string]struct{}{}

	for _, id := range ids {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			resolvedIDs = append(resolvedIDs, id)
		}
	}

	if len(names) > 0 {
		byName, err := client.TemplateIDsByNames(ctx, names)
		if err != nil {
			diags.AddError("Cannot resolve templates", err.Error())
			return nil, diags
		}
		for _, id := range byName {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				resolvedIDs = append(resolvedIDs, id)
			}
		}
	}

	return resolvedIDs, diags
}

func mapToTags(ctx context.Context, value types.Map) ([]zabbix.Tag, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}
	raw := map[string]string{}
	diags := value.ElementsAs(ctx, &raw, false)
	if diags.HasError() {
		return nil, diags
	}

	out := make([]zabbix.Tag, 0, len(raw))
	for tag, val := range raw {
		out = append(out, zabbix.Tag{Tag: tag, Value: val})
	}
	return out, diags
}

func tagsToMap(ctx context.Context, tags []zabbix.Tag) (types.Map, diag.Diagnostics) {
	raw := make(map[string]string, len(tags))
	for _, t := range tags {
		raw[t.Tag] = t.Value
	}
	return types.MapValueFrom(ctx, types.StringType, raw)
}

func expandInterfaces(interfaces []hostInterfaceModel) ([]zabbix.HostInterface, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := make([]zabbix.HostInterface, 0, len(interfaces))

	for index, it := range interfaces {
		useIP := !it.UseIP.IsNull() && it.UseIP.ValueBool()

		if useIP && (it.IP.IsNull() || it.IP.ValueString() == "") {
			diags.AddAttributeError(
				path.Root("interfaces").AtListIndex(index).AtName("ip"),
				"Missing value",
				"`ip` is required when `use_ip=true`.",
			)
			continue
		}
		if !useIP && (it.DNS.IsNull() || it.DNS.ValueString() == "") {
			diags.AddAttributeError(
				path.Root("interfaces").AtListIndex(index).AtName("dns"),
				"Missing value",
				"`dns` is required when `use_ip=false`.",
			)
			continue
		}

		port := "10050"
		if !it.Port.IsNull() && !it.Port.IsUnknown() && it.Port.ValueString() != "" {
			port = it.Port.ValueString()
			if !isValidPort(port) {
				diags.AddAttributeError(
					path.Root("interfaces").AtListIndex(index).AtName("port"),
					"Invalid port",
					"`port` must be a number or a user macro such as `{$AGENT.PORT}`.",
				)
				continue
			}
		}

		iface := zabbix.HostInterface{
			Type:  int(it.Type.ValueInt64()),
			Main:  boolToInt(!it.Main.IsNull() && it.Main.ValueBool()),
			UseIP: boolToInt(useIP),
			IP:    nullableString(it.IP),
			DNS:   nullableString(it.DNS),
			Port:  port,
		}

		if iface.Type == 2 {
			details := expandSNMPDetails(it.SNMPDetails)
			if details.Version != 2 {
				diags.AddAttributeError(
					path.Root("interfaces").AtListIndex(index).AtName("snmp_details").AtName("version"),
"Unsupported SNMP version",
				"This provider currently only supports SNMP v2 for hosts.",
				)
				continue
			}
			iface.Details = details
		}

		out = append(out, iface)
	}

	return out, diags
}

func flattenInterfaces(interfaces []zabbix.HostInterface) []hostInterfaceModel {
	out := make([]hostInterfaceModel, 0, len(interfaces))
	for _, it := range interfaces {
		out = append(out, hostInterfaceModel{
			Type:  types.Int64Value(int64(it.Type)),
			Main:  types.BoolValue(it.Main == 1),
			UseIP: types.BoolValue(it.UseIP == 1),
			IP:    nullOrString(it.IP),
			DNS:   nullOrString(it.DNS),
			Port:  types.StringValue(it.Port),
		})
		if it.Type == 2 && it.Details != nil && int(it.Details.Version) != 0 {
			out[len(out)-1].SNMPDetails = &hostSNMPDetailsModel{
				Version:   types.Int64Value(int64(it.Details.Version)),
				Community: nullOrString(it.Details.Community),
			}
		}
	}
	return out
}

func expandSNMPDetails(in *hostSNMPDetailsModel) *zabbix.SNMPDetails {
	if in == nil {
		return &zabbix.SNMPDetails{
			Version:   2,
			Community: "{$SNMP_COMMUNITY}",
		}
	}

	version := int(in.Version.ValueInt64())
	if in.Version.IsNull() || in.Version.IsUnknown() || version == 0 {
		version = 2
	}

	community := nullableString(in.Community)
	if community == "" {
		community = "{$SNMP_COMMUNITY}"
	}

	return &zabbix.SNMPDetails{
		Version:   zabbix.FlexIntFrom(version),
		Community: community,
	}
}

// isValidPort accepts a port number or a user macro ({$MACRO}), as Zabbix does.
func isValidPort(port string) bool {
	if strings.HasPrefix(port, "{$") && strings.HasSuffix(port, "}") {
		return true
	}
	n, err := strconv.Atoi(port)
	return err == nil && n >= 0 && n <= 65535
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
