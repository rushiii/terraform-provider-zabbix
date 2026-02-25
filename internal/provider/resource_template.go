package provider

import (
	"context"

	"github.com/RuShIII/terraform-provider-zabbix/internal/zabbix"

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
}

func NewTemplateResource() resource.Resource {
	return &templateResource{}
}

func (r *templateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_template"
}

func (r *templateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ressource Zabbix template.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"host": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Nom interne du template.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Nom visible du template.",
			},
			"host_group_ids": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "IDs de groupes auxquels rattacher le template.",
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
		resp.Diagnostics.AddError("Provider invalide", "Client Zabbix indisponible.")
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

	name := plan.Host.ValueString()
	if !plan.Name.IsNull() && plan.Name.ValueString() != "" {
		name = plan.Name.ValueString()
	}

	id, err := r.client.TemplateCreate(ctx, plan.Host.ValueString(), name, groupIDs)
	if err != nil {
		resp.Diagnostics.AddError("Erreur template.create", err.Error())
		return
	}

	plan.ID = types.StringValue(id)
	plan.Name = types.StringValue(name)
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
		resp.Diagnostics.AddError("Erreur template.get", err.Error())
		return
	}

	groupIDs := make([]string, 0, len(template.Groups))
	for _, g := range template.Groups {
		groupIDs = append(groupIDs, g.GroupID)
	}
	state.HostGroupIDs, _ = types.SetValueFrom(ctx, types.StringType, groupIDs)
	state.Host = types.StringValue(template.Host)
	state.Name = types.StringValue(template.Name)

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

	name := plan.Host.ValueString()
	if !plan.Name.IsNull() && plan.Name.ValueString() != "" {
		name = plan.Name.ValueString()
	}

	if err := r.client.TemplateUpdate(ctx, state.ID.ValueString(), plan.Host.ValueString(), name, groupIDs); err != nil {
		resp.Diagnostics.AddError("Erreur template.update", err.Error())
		return
	}

	plan.ID = state.ID
	plan.Name = types.StringValue(name)
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
		resp.Diagnostics.AddError("Erreur template.delete", err.Error())
	}
}

func (r *templateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
