package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func set(values ...string) types.Set {
	return stringSetValue(context.Background(), values)
}

var (
	null    = types.SetNull(types.StringType)
	unknown = types.SetUnknown(types.StringType)
	// A set whose element is not known yet, e.g. [zabbix_host_group.x.id] before the group exists.
	unknownElement = types.SetValueMust(types.StringType, []attr.Value{types.StringUnknown()})
)

func assertSet(t *testing.T, label string, got, want types.Set) {
	t.Helper()
	if !got.Equal(want) {
		t.Errorf("%s = %s, want %s", label, got, want)
	}
}

func TestPlanLinks(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name                 string
		cfgIDs, cfgNames     types.Set
		stateIDs, stateNames types.Set
		creating             bool
		wantIDs, wantNames   types.Set
	}{
		{
			name:   "create with an ID unknown at plan time (group created in the same apply)",
			cfgIDs: unknownElement, cfgNames: null, stateIDs: null, stateNames: null, creating: true,
			wantIDs: unknownElement, wantNames: unknown,
		},
		{
			name:   "create with names: resolved at apply time, not at plan time",
			cfgIDs: null, cfgNames: set("Visiosense"), stateIDs: null, stateNames: null, creating: true,
			wantIDs: unknown, wantNames: set("Visiosense"),
		},
		{
			name:   "IDs unchanged: computed names kept from state (no perpetual diff)",
			cfgIDs: set("34"), cfgNames: null, stateIDs: set("34"), stateNames: set("Visiosense"),
			wantIDs: set("34"), wantNames: set("Visiosense"),
		},
		{
			name:   "IDs changed: computed names become unknown",
			cfgIDs: set("35"), cfgNames: null, stateIDs: set("34"), stateNames: set("Visiosense"),
			wantIDs: set("35"), wantNames: unknown,
		},
		{
			name:   "names unchanged: computed IDs kept from state",
			cfgIDs: null, cfgNames: set("Visiosense"), stateIDs: set("34"), stateNames: set("Visiosense"),
			wantIDs: set("34"), wantNames: set("Visiosense"),
		},
		{
			name:   "names changed: computed IDs become unknown",
			cfgIDs: null, cfgNames: set("Autre"), stateIDs: set("34"), stateNames: set("Visiosense"),
			wantIDs: unknown, wantNames: set("Autre"),
		},
		{
			name:   "both configured: plan is the configuration",
			cfgIDs: set("34"), cfgNames: set("Autre"), stateIDs: set("34", "35"), stateNames: set("Visiosense", "Autre"),
			wantIDs: set("34"), wantNames: set("Autre"),
		},
		{
			name:   "none configured on create: no links",
			cfgIDs: null, cfgNames: null, stateIDs: null, stateNames: null, creating: true,
			wantIDs: set(), wantNames: set(),
		},
		{
			name:   "none configured on update: links left as they are",
			cfgIDs: null, cfgNames: null, stateIDs: set("10"), stateNames: set("ICMP Ping"),
			wantIDs: set("10"), wantNames: set("ICMP Ping"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ids, names := planLinks(ctx, tc.cfgIDs, tc.cfgNames, tc.stateIDs, tc.stateNames, tc.creating)
			assertSet(t, "ids", ids, tc.wantIDs)
			assertSet(t, "names", names, tc.wantNames)
		})
	}
}

func TestFinalizeLinks(t *testing.T) {
	ctx := context.Background()
	linked := []linkedObject{{ID: "34", Name: "Visiosense", Aliases: []string{"Visiosense"}}}

	ids, names := finalizeLinks(ctx, set("34"), unknown, linked)
	assertSet(t, "ids", ids, set("34"))
	assertSet(t, "names", names, set("Visiosense"))

	ids, names = finalizeLinks(ctx, unknown, set("Visiosense"), linked)
	assertSet(t, "ids", ids, set("34"))
	assertSet(t, "names", names, set("Visiosense"))

	// Planned values are kept as they are, so the state matches the plan.
	ids, names = finalizeLinks(ctx, set("34"), set("Visiosense"), linked)
	assertSet(t, "ids", ids, set("34"))
	assertSet(t, "names", names, set("Visiosense"))
}

func TestRefreshLinks(t *testing.T) {
	ctx := context.Background()
	tpl := []linkedObject{{ID: "10", Name: "Template Custom ICMP (visible)", Aliases: []string{"tpl-custom-icmp", "Template Custom ICMP (visible)"}}}

	// A template configured by its technical name keeps that name (no perpetual diff).
	ids, names := refreshLinks(ctx, set("10"), set("tpl-custom-icmp"), tpl)
	assertSet(t, "ids", ids, set("10"))
	assertSet(t, "names", names, set("tpl-custom-icmp"))

	// Import: no previous values, everything comes from Zabbix.
	ids, names = refreshLinks(ctx, null, null, tpl)
	assertSet(t, "ids", ids, set("10"))
	assertSet(t, "names", names, set("Template Custom ICMP (visible)"))

	groups := []linkedObject{
		{ID: "34", Name: "Visiosense", Aliases: []string{"Visiosense"}},
		{ID: "35", Name: "Autre", Aliases: []string{"Autre"}},
	}
	// IDs and names configured together are kept while they cover what is linked.
	ids, names = refreshLinks(ctx, set("34"), set("Autre"), groups)
	assertSet(t, "ids", ids, set("34"))
	assertSet(t, "names", names, set("Autre"))

	// A group added outside Terraform: rebuilt from Zabbix so the drift is visible.
	ids, names = refreshLinks(ctx, set("34"), set("Visiosense"), groups)
	assertSet(t, "ids", ids, set("34", "35"))
	assertSet(t, "names", names, set("Visiosense", "Autre"))

	// A group renamed outside Terraform: names updated.
	renamed := []linkedObject{{ID: "34", Name: "Visiosense 2", Aliases: []string{"Visiosense 2"}}}
	ids, names = refreshLinks(ctx, set("34"), set("Visiosense"), renamed)
	assertSet(t, "ids", ids, set("34"))
	assertSet(t, "names", names, set("Visiosense 2"))
}

func TestMayHaveElements(t *testing.T) {
	for _, tc := range []struct {
		name string
		s    types.Set
		want bool
	}{
		{"null", null, false},
		{"empty", set(), false},
		{"unknown", unknown, true},
		{"unknown element", unknownElement, true},
		{"value", set("34"), true},
	} {
		if got := mayHaveElements(tc.s); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
