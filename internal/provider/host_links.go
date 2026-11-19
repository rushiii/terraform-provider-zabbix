package provider

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Host groups and templates are linked to a host through a pair of attributes:
// "*_ids" and "*_names". The user configures either or both; whichever is not
// configured is computed. The helpers below keep the pair consistent:
//
//   - planLinks decides the planned values without calling the API, so objects
//     created in the same apply (unknown IDs, names not yet in Zabbix) can be used;
//   - finalizeLinks fills in what the plan left unknown, after create/update;
//   - refreshLinks updates the state on read without producing a perpetual diff.

// linkedObject is a host group or template linked to a host, as returned by host.get.
type linkedObject struct {
	ID string
	// Name is the value written to the "*_names" attribute when it is computed.
	Name string
	// Aliases are the names that resolve to this object (a template resolves by
	// technical name or by visible name).
	Aliases []string
}

// knownStrings returns the elements of a set, and false when the set is null,
// unknown or has unknown elements.
func knownStrings(ctx context.Context, s types.Set) ([]string, bool) {
	if s.IsNull() || s.IsUnknown() {
		return nil, false
	}
	var elems []types.String
	if diags := s.ElementsAs(ctx, &elems, false); diags.HasError() {
		return nil, false
	}
	out := make([]string, 0, len(elems))
	for _, e := range elems {
		if e.IsUnknown() {
			return nil, false
		}
		if !e.IsNull() {
			out = append(out, e.ValueString())
		}
	}
	return out, true
}

func stringSetValue(ctx context.Context, values []string) types.Set {
	if values == nil {
		values = []string{}
	}
	v, _ := types.SetValueFrom(ctx, types.StringType, values)
	return v
}

func sameElements(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x := append([]string(nil), a...)
	y := append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

// mayHaveElements reports whether a configured set is, or may turn out to be, non-empty.
// Unknown values are given the benefit of the doubt: they are only known at apply time.
func mayHaveElements(s types.Set) bool {
	return s.IsUnknown() || (!s.IsNull() && len(s.Elements()) > 0)
}

// sameKnownSet reports whether two sets are both fully known and hold the same elements.
func sameKnownSet(ctx context.Context, a, b types.Set) bool {
	x, okX := knownStrings(ctx, a)
	y, okY := knownStrings(ctx, b)
	return okX && okY && sameElements(x, y)
}

// planLinks returns the planned "*_ids" and "*_names" values.
//
// A configured attribute keeps its configured value. The other one is computed: it keeps
// its value from the state when the configured attribute did not change, and is unknown
// (resolved at apply time) otherwise. When neither is configured, the links in the state
// are kept as they are (none on creation).
func planLinks(ctx context.Context, cfgIDs, cfgNames, stateIDs, stateNames types.Set, creating bool) (types.Set, types.Set) {
	unknown := types.SetUnknown(types.StringType)
	idsSet, namesSet := !cfgIDs.IsNull(), !cfgNames.IsNull()

	switch {
	case idsSet && namesSet:
		return cfgIDs, cfgNames
	case idsSet:
		if !creating && sameKnownSet(ctx, cfgIDs, stateIDs) && !stateNames.IsNull() && !stateNames.IsUnknown() {
			return cfgIDs, stateNames
		}
		return cfgIDs, unknown
	case namesSet:
		if !creating && sameKnownSet(ctx, cfgNames, stateNames) && !stateIDs.IsNull() && !stateIDs.IsUnknown() {
			return stateIDs, cfgNames
		}
		return unknown, cfgNames
	default:
		if creating || stateIDs.IsNull() || stateNames.IsNull() {
			return stringSetValue(ctx, nil), stringSetValue(ctx, nil)
		}
		return stateIDs, stateNames
	}
}

func linkedIDsAndNames(linked []linkedObject) ([]string, []string) {
	ids := make([]string, 0, len(linked))
	names := make([]string, 0, len(linked))
	for _, o := range linked {
		ids = append(ids, o.ID)
		names = append(names, o.Name)
	}
	return ids, names
}

// finalizeLinks returns the values to store after create/update: the planned values when
// known, otherwise what Zabbix reports.
func finalizeLinks(ctx context.Context, planIDs, planNames types.Set, linked []linkedObject) (types.Set, types.Set) {
	ids, names := linkedIDsAndNames(linked)
	outIDs, outNames := planIDs, planNames
	if _, ok := knownStrings(ctx, planIDs); !ok {
		outIDs = stringSetValue(ctx, ids)
	}
	if _, ok := knownStrings(ctx, planNames); !ok {
		outNames = stringSetValue(ctx, names)
	}
	return outIDs, outNames
}

// refreshLinks returns the values to store on read. The previous values are kept while
// they still describe exactly the objects linked in Zabbix (so a template configured by
// its technical name does not turn into its visible name); otherwise both attributes are
// rebuilt from Zabbix, which shows the drift in the next plan.
func refreshLinks(ctx context.Context, priorIDs, priorNames types.Set, linked []linkedObject) (types.Set, types.Set) {
	ids, names := linkedIDsAndNames(linked)
	pIDs, okIDs := knownStrings(ctx, priorIDs)
	pNames, okNames := knownStrings(ctx, priorNames)
	if okIDs && okNames {
		expected := map[string]struct{}{}
		for _, id := range pIDs {
			expected[id] = struct{}{}
		}
		allFound := true
		for _, n := range pNames {
			found := false
			for _, o := range linked {
				for _, a := range o.Aliases {
					if a == n {
						expected[o.ID] = struct{}{}
						found = true
					}
				}
			}
			if !found {
				allFound = false
				break
			}
		}
		if allFound {
			exp := make([]string, 0, len(expected))
			for id := range expected {
				exp = append(exp, id)
			}
			if sameElements(exp, ids) {
				return priorIDs, priorNames
			}
		}
	}
	return stringSetValue(ctx, ids), stringSetValue(ctx, names)
}
