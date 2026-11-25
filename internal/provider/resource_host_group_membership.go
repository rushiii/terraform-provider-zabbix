package provider

import (
	"context"
	"fmt"

	"github.com/rushiii/terraform-provider-zabbix/internal/zabbix"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &hostGroupMembershipResource{}
	_ resource.ResourceWithConfigure      = &hostGroupMembershipResource{}
	_ resource.ResourceWithImportState    = &hostGroupMembershipResource{}
	_ resource.ResourceWithValidateConfig = &hostGroupMembershipResource{}
)

// hostGroupMembershipResource adds existing hosts to a host group without managing anything else
// about them: their other groups, interfaces, templates, etc. are left as they are.
type hostGroupMembershipResource struct {
	client *zabbix.Client
}

type hostGroupMembershipResourceModel struct {
	ID          types.String `tfsdk:"id"`
	HostGroupID types.String `tfsdk:"host_group_id"`
	HostIDs     types.Set    `tfsdk:"host_ids"`
}

func NewHostGroupMembershipResource() resource.Resource {
	return &hostGroupMembershipResource{}
}

func (r *hostGroupMembershipResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host_group_membership"
}

func (r *hostGroupMembershipResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Adds existing hosts to a host group, e.g. hosts created in the Zabbix UI or by discovery. " +
			"Only the membership of the listed hosts in this group is managed: their other groups and settings are left as they are.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Same as `host_group_id`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"host_group_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the host group, e.g. `zabbix_host_group.x.id`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"host_ids": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "IDs of the hosts to put in the group, e.g. `data.zabbix_host.x.id`. At least one.",
			},
		},
	}
}

func (r *hostGroupMembershipResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var ids types.Set
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("host_ids"), &ids)...)
	if !resp.Diagnostics.HasError() && !ids.IsNull() && !mayHaveElements(ids) {
		resp.Diagnostics.AddAttributeError(path.Root("host_ids"), "Missing value", "Provide at least one host ID.")
	}
}

func (r *hostGroupMembershipResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *hostGroupMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan hostGroupMembershipResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	hostIDs, d := setToStrings(ctx, plan.HostIDs)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.HostGroupAddHosts(ctx, plan.HostGroupID.ValueString(), hostIDs); err != nil {
		resp.Diagnostics.AddError("hostgroup.massadd error", err.Error())
		return
	}
	plan.ID = plan.HostGroupID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *hostGroupMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state hostGroupMembershipResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	groupID := state.ID.ValueString()

	if _, err := r.client.HostGroupGetByID(ctx, groupID); err != nil {
		if zabbix.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("hostgroup.get error", err.Error())
		return
	}
	inGroup, err := r.client.HostIDsInGroup(ctx, groupID)
	if err != nil {
		resp.Diagnostics.AddError("host.get error", err.Error())
		return
	}

	// Only the managed hosts are tracked: other hosts of the group are none of this resource's
	// business. On import (no hosts in state yet), all hosts of the group are taken.
	managed, _ := knownStrings(ctx, state.HostIDs)
	present := map[string]bool{}
	for _, id := range inGroup {
		present[id] = true
	}
	hostIDs := inGroup
	if len(managed) > 0 {
		hostIDs = make([]string, 0, len(managed))
		for _, id := range managed {
			if present[id] {
				hostIDs = append(hostIDs, id)
			}
		}
	}

	state.HostGroupID = types.StringValue(groupID)
	state.HostIDs = stringSetValue(ctx, hostIDs)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *hostGroupMembershipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state hostGroupMembershipResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	want, d := setToStrings(ctx, plan.HostIDs)
	resp.Diagnostics.Append(d...)
	have, d := setToStrings(ctx, state.HostIDs)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	added, removed := diffStrings(have, want)
	groupID := state.ID.ValueString()
	if err := r.client.HostGroupAddHosts(ctx, groupID, added); err != nil {
		resp.Diagnostics.AddError("hostgroup.massadd error", err.Error())
		return
	}
	if err := r.client.HostGroupRemoveHosts(ctx, groupID, removed); err != nil {
		resp.Diagnostics.AddError("hostgroup.massremove error", removeHint(err))
		return
	}
	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *hostGroupMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state hostGroupMembershipResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	hostIDs, d := setToStrings(ctx, state.HostIDs)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Only remove the hosts still in the group (a host may have been deleted meanwhile).
	inGroup, err := r.client.HostIDsInGroup(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("host.get error", err.Error())
		return
	}
	present := map[string]bool{}
	for _, id := range inGroup {
		present[id] = true
	}
	toRemove := make([]string, 0, len(hostIDs))
	for _, id := range hostIDs {
		if present[id] {
			toRemove = append(toRemove, id)
		}
	}
	if err := r.client.HostGroupRemoveHosts(ctx, state.ID.ValueString(), toRemove); err != nil {
		resp.Diagnostics.AddError("hostgroup.massremove error", removeHint(err))
	}
}

// ImportState imports the membership of all hosts of a group; the import ID is the host group ID.
func (r *hostGroupMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// diffStrings returns the elements of want missing from have, and those of have missing from want.
func diffStrings(have, want []string) (added, removed []string) {
	inHave := map[string]bool{}
	for _, v := range have {
		inHave[v] = true
	}
	inWant := map[string]bool{}
	for _, v := range want {
		inWant[v] = true
		if !inHave[v] {
			added = append(added, v)
		}
	}
	for _, v := range have {
		if !inWant[v] {
			removed = append(removed, v)
		}
	}
	return added, removed
}

func removeHint(err error) string {
	return fmt.Sprintf("%v\n\nZabbix refuses to remove a host from its last host group: add the host to another group first.", err)
}
