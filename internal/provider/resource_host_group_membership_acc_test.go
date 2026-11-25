package provider

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// testAccCreateExternalHost creates a host directly through the API (as if created in the Zabbix
// UI), in "Linux servers" (group 2), and deletes it at the end of the test.
func testAccCreateExternalHost(t *testing.T, name, visibleName string) {
	t.Helper()
	var res struct {
		HostIDs []string `json:"hostids"`
	}
	testAccAPI(t, "host.create", map[string]any{
		"host": name, "name": visibleName,
		"groups":     []map[string]string{{"groupid": "2"}},
		"interfaces": []map[string]any{{"type": 1, "main": 1, "useip": 1, "ip": "10.0.0.1", "dns": "", "port": "10050"}},
	}, &res)
	t.Cleanup(func() {
		var ignored any
		testAccAPI(t, "host.delete", res.HostIDs, &ignored)
	})
}

// testAccCheckHostGroups checks the groups of a host in Zabbix.
func testAccCheckHostGroups(t *testing.T, host string, want ...string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		var hosts []struct {
			HostGroups []struct {
				Name string `json:"name"`
			} `json:"hostgroups"`
		}
		testAccAPI(t, "host.get", map[string]any{
			"filter": map[string]any{"host": []string{host}}, "selectHostGroups": []string{"name"},
		}, &hosts)
		if len(hosts) != 1 {
			return fmt.Errorf("host %s not found", host)
		}
		var got []string
		for _, g := range hosts[0].HostGroups {
			got = append(got, g.Name)
		}
		sort.Strings(got)
		sort.Strings(want)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			return fmt.Errorf("groups of %s = %v, want %v", host, got, want)
		}
		return nil
	}
}

// Hosts created outside Terraform are added to and removed from a new group, without touching
// their other groups.
func TestAccHostGroupMembership_existingHosts(t *testing.T) {
	s := acctest.RandString(8)
	h1, h2, group := "tf-acc-ext1-"+s, "tf-acc-ext2-"+s, "tf-acc-group-"+s
	cfg := func(members string) string {
		return testAccProviderConfig() + fmt.Sprintf(`
resource "zabbix_host_group" "new" {
  name = %[1]q
}

data "zabbix_host" "h1" {
  name = %[2]q
}

# Looked up by its visible name.
data "zabbix_host" "h2" {
  name = "Visible %[3]s"
}

resource "zabbix_host_group_membership" "new" {
  host_group_id = zabbix_host_group.new.id
  host_ids      = [%[4]s]
}
`, group, h1, h2, members)
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccCreateExternalHost(t, h1, h1)
			testAccCreateExternalHost(t, h2, "Visible "+h2)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg(`data.zabbix_host.h1.id`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.zabbix_host.h1", "host", h1),
					resource.TestCheckResourceAttr("data.zabbix_host.h2", "host", h2),
					resource.TestCheckResourceAttr("data.zabbix_host.h1", "host_group_ids.#", "1"),
					testAccCheckHostGroups(t, h1, "Linux servers", group),
					testAccCheckHostGroups(t, h2, "Linux servers"),
				),
			},
			{
				Config: cfg(`data.zabbix_host.h1.id, data.zabbix_host.h2.id`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckHostGroups(t, h1, "Linux servers", group),
					testAccCheckHostGroups(t, h2, "Linux servers", group),
				),
			},
			{
				Config: cfg(`data.zabbix_host.h2.id`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckHostGroups(t, h1, "Linux servers"),
					testAccCheckHostGroups(t, h2, "Linux servers", group),
				),
			},
			{
				ResourceName:      "zabbix_host_group_membership.new",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Destroying the membership (and the group) leaves the hosts in their own groups.
				Config: testAccProviderConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckHostGroups(t, h1, "Linux servers"),
					testAccCheckHostGroups(t, h2, "Linux servers"),
				),
			},
		},
	})
}

// An unknown host name is a clear error.
func TestAccHostData_unknownHost(t *testing.T) {
	s := acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + fmt.Sprintf(`
data "zabbix_host" "missing" {
  name = "tf-acc-missing-%s"
}
`, s),
				ExpectError: regexp.MustCompile(`host "tf-acc-missing-.*" not found`),
			},
		},
	})
}
