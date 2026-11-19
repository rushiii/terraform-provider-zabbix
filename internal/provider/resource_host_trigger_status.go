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
	_ resource.Resource                = &hostTriggerStatusResource{}
	_ resource.ResourceWithConfigure   = &hostTriggerStatusResource{}
	_ resource.ResourceWithImportState = &hostTriggerStatusResource{}
)

// hostTriggerStatusResource enables or disables an existing trigger of a host, typically one
// inherited from a linked template. It never creates nor deletes the trigger itself.
type hostTriggerStatusResource struct {
	client *zabbix.Client
}

type hostTriggerStatusResourceModel struct {
	ID      types.String `tfsdk:"id"`
	HostID  types.String `tfsdk:"host_id"`
	Name    types.String `tfsdk:"name"`
	Enabled types.Bool   `tfsdk:"enabled"`
}

func NewHostTriggerStatusResource() resource.Resource {
	return &hostTriggerStatusResource{}
}

func (r *hostTriggerStatusResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host_trigger_status"
}

func (r *hostTriggerStatusResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Enables or disables an existing trigger of a host, such as a trigger inherited from a linked template. " +
			"The trigger is neither created nor deleted; on destroy, an inherited trigger gets back the status of its template trigger.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Trigger ID on the host.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"host_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the host the trigger belongs to, e.g. `zabbix_host.web.id`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Trigger name, as defined in the template (e.g. `ICMP Ping: Unavailable by ICMP ping`) " +
					"or as displayed on the host with macros expanded.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enabled": schema.BoolAttribute{
				Required:            true,
				MarkdownDescription: "Whether the trigger is enabled on this host.",
			},
		},
	}
}

func (r *hostTriggerStatusResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *hostTriggerStatusResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan hostTriggerStatusResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	trigger, err := r.client.HostTriggerFind(ctx, plan.HostID.ValueString(), plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("name"), "Trigger not found", err.Error())
		return
	}
	if err := r.client.TriggerSetStatus(ctx, trigger.TriggerID, plan.Enabled.ValueBool()); err != nil {
		resp.Diagnostics.AddError("trigger.update error", err.Error())
		return
	}

	plan.ID = types.StringValue(trigger.TriggerID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *hostTriggerStatusResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state hostTriggerStatusResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	trigger, err := r.client.HostTriggerGetByID(ctx, state.ID.ValueString())
	if err != nil {
		if zabbix.IsNotFound(err) {
			// Host deleted or template unlinked: the trigger no longer exists.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("trigger.get error", err.Error())
		return
	}

	state.Enabled = types.BoolValue(zabbix.StatusToEnabled(trigger.Status))
	if len(trigger.Hosts) > 0 {
		state.HostID = types.StringValue(trigger.Hosts[0].HostID)
	}
	// The configured name may be the expanded one: only fill it in on import.
	if state.Name.IsNull() || state.Name.ValueString() == "" {
		state.Name = types.StringValue(trigger.Description)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *hostTriggerStatusResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state hostTriggerStatusResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.TriggerSetStatus(ctx, state.ID.ValueString(), plan.Enabled.ValueBool()); err != nil {
		resp.Diagnostics.AddError("trigger.update error", err.Error())
		return
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete stops managing the trigger. An inherited trigger gets back the status of its
// template trigger; other triggers are left as they are.
func (r *hostTriggerStatusResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state hostTriggerStatusResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	trigger, err := r.client.HostTriggerGetByID(ctx, state.ID.ValueString())
	if err != nil {
		if !zabbix.IsNotFound(err) {
			resp.Diagnostics.AddError("trigger.get error", err.Error())
		}
		return
	}
	if trigger.TemplateID == "" || trigger.TemplateID == "0" {
		return
	}
	parent, err := r.client.HostTriggerGetByID(ctx, trigger.TemplateID)
	if err != nil {
		if !zabbix.IsNotFound(err) {
			resp.Diagnostics.AddError("trigger.get error (template trigger)", err.Error())
		}
		return
	}
	if parent.Status == trigger.Status {
		return
	}
	if err := r.client.TriggerSetStatus(ctx, trigger.TriggerID, zabbix.StatusToEnabled(parent.Status)); err != nil {
		resp.Diagnostics.AddError("trigger.update error", fmt.Sprintf("restoring the template status: %v", err))
	}
}

func (r *hostTriggerStatusResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
