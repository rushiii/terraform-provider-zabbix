package provider

import (
	"context"

	"github.com/rushiii/terraform-provider-zabbix/internal/zabbix"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &templateResource{}
	_ resource.ResourceWithConfigure   = &templateResource{}
	_ resource.ResourceWithImportState = &templateResource{}
)

type templateResource struct {
	client *zabbix.Client
}

type templateResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Host         types.String `tfsdk:"host"`
	Name         types.String `tfsdk:"name"`
	HostGroupIDs types.Set    `tfsdk:"host_group_ids"`
	Macros       types.Map    `tfsdk:"macros"`
}

func NewTemplateResource() resource.Resource {
	return &templateResource{}
}

func (r *templateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_template"
}

func (r *templateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Zabbix template resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"host": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Internal template name.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Visible template name.",
			},
			"host_group_ids": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Group IDs to attach the template to.",
			},
			"macros": schema.MapAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "User macros for the template (e.g. `{\"{$SNMP_COMMUNITY}\" = \"public\"}`).",
			},
		},
	}
}

func (r *templateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *templateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan templateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupIDs, d := setToStrings(ctx, plan.HostGroupIDs)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	macros := templateMacrosFromPlan(plan.Macros)

	name := plan.Host.ValueString()
	if !plan.Name.IsNull() && plan.Name.ValueString() != "" {
		name = plan.Name.ValueString()
	}

	id, err := r.client.TemplateCreate(ctx, plan.Host.ValueString(), name, groupIDs, macros)
	if err != nil {
		resp.Diagnostics.AddError("template.create error", err.Error())
		return
	}

	plan.ID = types.StringValue(id)
	plan.Name = types.StringValue(name)
	plan.Macros = templateMacrosToStateKnown(ctx, plan.Macros)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *templateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state templateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	template, err := r.client.TemplateGetByID(ctx, state.ID.ValueString())
	if err != nil {
		if zabbix.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("template.get error", err.Error())
		return
	}

	groupIDs := make([]string, 0, len(template.Groups))
	for _, g := range template.Groups {
		groupIDs = append(groupIDs, g.GroupID)
	}
	state.HostGroupIDs, _ = types.SetValueFrom(ctx, types.StringType, groupIDs)
	state.Host = types.StringValue(template.Host)
	state.Name = types.StringValue(template.Name)
	macrosMap := make(map[string]string, len(template.Macros))
	for _, x := range template.Macros {
		macrosMap[x.Macro] = x.Value
	}
	state.Macros = templateMacrosToState(ctx, macrosMap)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *templateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan templateResourceModel
	var state templateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupIDs, d := setToStrings(ctx, plan.HostGroupIDs)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	macros := templateMacrosFromPlan(plan.Macros)

	name := plan.Host.ValueString()
	if !plan.Name.IsNull() && plan.Name.ValueString() != "" {
		name = plan.Name.ValueString()
	}

	if err := r.client.TemplateUpdate(ctx, state.ID.ValueString(), plan.Host.ValueString(), name, groupIDs, macros); err != nil {
		resp.Diagnostics.AddError("template.update error", err.Error())
		return
	}

	plan.ID = state.ID
	plan.Name = types.StringValue(name)
	plan.Macros = templateMacrosToStateKnown(ctx, plan.Macros)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *templateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state templateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.TemplateDelete(ctx, state.ID.ValueString())
	if err != nil && !zabbix.IsNotFound(err) {
		resp.Diagnostics.AddError("template.delete error", err.Error())
	}
}

func (r *templateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// templateMacrosFromPlan returns a map for the API (macro name -> value); nil if plan macros are null or empty.
func templateMacrosFromPlan(m types.Map) map[string]string {
	if m.IsNull() || m.IsUnknown() || len(m.Elements()) == 0 {
		return nil
	}
	out := make(map[string]string, len(m.Elements()))
	for k, v := range m.Elements() {
		key := k
		if s, ok := v.(types.String); ok && !s.IsNull() {
			out[key] = s.ValueString()
		}
	}
	return out
}

// templateMacrosToState converts macro map to a Map attribute for state.
func templateMacrosToState(ctx context.Context, m map[string]string) types.Map {
	if len(m) == 0 {
		return types.MapNull(types.StringType)
	}
	val, _ := types.MapValueFrom(ctx, types.StringType, m)
	return val
}

// templateMacrosToStateKnown returns a known value for macros (never unknown) after Create/Update.
func templateMacrosToStateKnown(ctx context.Context, planMacros types.Map) types.Map {
	if !planMacros.IsNull() && !planMacros.IsUnknown() && len(planMacros.Elements()) > 0 {
		out := make(map[string]string, len(planMacros.Elements()))
		for k, v := range planMacros.Elements() {
			if s, ok := v.(types.String); ok && !s.IsNull() {
				out[k] = s.ValueString()
			}
		}
		val, _ := types.MapValueFrom(ctx, types.StringType, out)
		return val
	}
	// Known empty map (required: no unknown after apply).
	val, _ := types.MapValueFrom(ctx, types.StringType, map[string]string{})
	return val
}
