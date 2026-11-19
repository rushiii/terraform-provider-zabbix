package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Acceptance tests run against a real Zabbix server. They are skipped unless TF_ACC is set:
//
//	TF_ACC=1 ZABBIX_URL=http://localhost:8080/api_jsonrpc.php ZABBIX_USERNAME=Admin ZABBIX_PASSWORD=zabbix \
//	  go test ./internal/provider/ -run TestAcc -v
//
// With OpenTofu instead of Terraform, also set TF_ACC_TERRAFORM_PATH=/path/to/tofu and
// TF_ACC_PROVIDER_HOST=registry.opentofu.org.
//
// Every step is followed by a plan that must be empty (no perpetual diff).

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"zabbix": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	for _, v := range []string{"ZABBIX_URL", "ZABBIX_USERNAME", "ZABBIX_PASSWORD"} {
		if os.Getenv(v) == "" {
			t.Fatalf("%s must be set for acceptance tests", v)
		}
	}
}

func testAccProviderConfig() string {
	return fmt.Sprintf(`
provider "zabbix" {
  url      = %q
  username = %q
  password = %q
}
`, os.Getenv("ZABBIX_URL"), os.Getenv("ZABBIX_USERNAME"), os.Getenv("ZABBIX_PASSWORD"))
}

// testAccHostConfig returns a configuration with two host groups and one host; links holds
// the host's group/template attributes.
func testAccHostConfig(suffix, links string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "zabbix_host_group" "a" {
  name = "tf-acc-a-%[1]s"
}

resource "zabbix_host_group" "b" {
  name = "tf-acc-b-%[1]s"
}

resource "zabbix_host" "test" {
  name         = "tf-acc-host-%[1]s"
  visible_name = "tf-acc-host-%[1]s"
  enabled      = true

%[2]s

  interfaces {
    type   = 3
    ip     = "172.17.32.22"
    main   = true
    port   = "623"
    use_ip = true
  }
}
`, suffix, links)
}

// Host group created in the same apply and referenced by ID (bug: "Missing value" at plan,
// then host_group_names unknown after apply).
func TestAccHost_hostGroupIDsSameApply(t *testing.T) {
	s := acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccHostConfig(s, `host_group_ids = [zabbix_host_group.a.id]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("zabbix_host.test", "host_group_ids.0", "zabbix_host_group.a", "id"),
					resource.TestCheckResourceAttr("zabbix_host.test", "host_group_names.#", "1"),
					resource.TestCheckTypeSetElemAttr("zabbix_host.test", "host_group_names.*", "tf-acc-a-"+s),
					resource.TestCheckResourceAttr("zabbix_host.test", "template_ids.#", "0"),
					resource.TestCheckResourceAttr("zabbix_host.test", "template_names.#", "0"),
				),
			},
		},
	})
}

// Host group created in the same apply and referenced by name (bug: name resolved at plan
// time, before the group exists).
func TestAccHost_hostGroupNamesSameApply(t *testing.T) {
	s := acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccHostConfig(s, fmt.Sprintf(`host_group_names = ["tf-acc-a-%s"]
  depends_on       = [zabbix_host_group.a]`, s)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("zabbix_host.test", "host_group_ids.#", "1"),
					resource.TestCheckResourceAttrPair("zabbix_host.test", "host_group_ids.0", "zabbix_host_group.a", "id"),
					resource.TestCheckTypeSetElemAttr("zabbix_host.test", "host_group_names.*", "tf-acc-a-"+s),
				),
			},
		},
	})
}

// Moving the host to another group by ID updates the computed names.
func TestAccHost_switchHostGroupByID(t *testing.T) {
	s := acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccHostConfig(s, `host_group_ids = [zabbix_host_group.a.id]`),
				Check:  resource.TestCheckTypeSetElemAttr("zabbix_host.test", "host_group_names.*", "tf-acc-a-"+s),
			},
			{
				Config: testAccHostConfig(s, `host_group_ids = [zabbix_host_group.b.id]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("zabbix_host.test", "host_group_ids.0", "zabbix_host_group.b", "id"),
					resource.TestCheckResourceAttr("zabbix_host.test", "host_group_names.#", "1"),
					resource.TestCheckTypeSetElemAttr("zabbix_host.test", "host_group_names.*", "tf-acc-b-"+s),
				),
			},
			{
				// Switching to names for the same group: IDs follow. A name does not create a
				// dependency, hence depends_on (otherwise the group may be destroyed first).
				Config: testAccHostConfig(s, fmt.Sprintf(`host_group_names = ["tf-acc-a-%s"]
  depends_on       = [zabbix_host_group.a]`, s)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("zabbix_host.test", "host_group_ids.0", "zabbix_host_group.a", "id"),
					resource.TestCheckResourceAttr("zabbix_host.test", "host_group_ids.#", "1"),
				),
			},
			{
				ResourceName:      "zabbix_host.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// Without any host group the configuration is still rejected.
func TestAccHost_hostGroupRequired(t *testing.T) {
	s := acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccHostConfig(s, ``),
				ExpectError: regexp.MustCompile("Provide at least `host_group_ids` or `host_group_names`"),
			},
			{
				Config:      testAccHostConfig(s, `host_group_ids = []`),
				ExpectError: regexp.MustCompile("Provide at least `host_group_ids` or `host_group_names`"),
			},
		},
	})
}

// Same rules for templates: a template created in the same apply, referenced by its
// technical name, then by ID, then unlinked.
func TestAccHost_templates(t *testing.T) {
	s := acctest.RandString(8)
	tpl := fmt.Sprintf(`
resource "zabbix_template" "t" {
  host           = "tf-acc-tpl-%[1]s"
  name           = "TF acc template %[1]s"
  host_group_ids = ["1"]
}
`, s)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Technical name: no perpetual diff to the visible name.
				Config: tpl + testAccHostConfig(s, fmt.Sprintf(`host_group_ids = [zabbix_host_group.a.id]
  template_names = ["tf-acc-tpl-%s"]
  depends_on     = [zabbix_template.t]`, s)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("zabbix_host.test", "template_ids.0", "zabbix_template.t", "id"),
					resource.TestCheckTypeSetElemAttr("zabbix_host.test", "template_names.*", "tf-acc-tpl-"+s),
				),
			},
			{
				Config: tpl + testAccHostConfig(s, `host_group_ids = [zabbix_host_group.a.id]
  template_ids   = [zabbix_template.t.id]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("zabbix_host.test", "template_ids.0", "zabbix_template.t", "id"),
					resource.TestCheckResourceAttr("zabbix_host.test", "template_names.#", "1"),
				),
			},
			{
				// An empty set unlinks the templates.
				Config: tpl + testAccHostConfig(s, `host_group_ids = [zabbix_host_group.a.id]
  template_ids   = []`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("zabbix_host.test", "template_ids.#", "0"),
					resource.TestCheckResourceAttr("zabbix_host.test", "template_names.#", "0"),
				),
			},
		},
	})
}
