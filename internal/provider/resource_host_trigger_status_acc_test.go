package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/rushiii/terraform-provider-zabbix/internal/zabbix"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const testAccICMPTrigger = "ICMP Ping: Unavailable by ICMP ping"

func testAccClient(t *testing.T) *zabbix.Client {
	t.Helper()
	c, err := zabbix.NewClient(zabbix.ClientConfig{
		URL:     os.Getenv("ZABBIX_URL"),
		Timeout: 30 * time.Second,
		Auth: zabbix.Auth{
			Method:   zabbix.AuthUserPassword,
			Username: os.Getenv("ZABBIX_USERNAME"),
			Password: os.Getenv("ZABBIX_PASSWORD"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// testAccCheckTriggerStatus checks the status of the trigger in Zabbix itself.
func testAccCheckTriggerStatus(t *testing.T, resourceName string, enabled bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("%s not found in state", resourceName)
		}
		trigger, err := testAccClient(t).HostTriggerGetByID(context.Background(), rs.Primary.ID)
		if err != nil {
			return err
		}
		if got := zabbix.StatusToEnabled(trigger.Status); got != enabled {
			return fmt.Errorf("trigger %s enabled = %v in Zabbix, want %v", rs.Primary.ID, got, enabled)
		}
		return nil
	}
}

func testAccHostTriggerStatusConfig(suffix, status string) string {
	cfg := testAccProviderConfig() + fmt.Sprintf(`
resource "zabbix_host" "test" {
  name             = "tf-acc-trg-%[1]s"
  visible_name     = "tf-acc-trg-%[1]s"
  host_group_names = ["Linux servers"]
  template_names   = ["ICMP Ping"]

  interfaces {
    type = 1
    ip   = "10.0.0.1"
  }
}
`, suffix)
	if status != "" {
		cfg += fmt.Sprintf(`
resource "zabbix_host_trigger_status" "icmp" {
  host_id = zabbix_host.test.id
  name    = %q
  enabled = %s
}
`, testAccICMPTrigger, status)
	}
	return cfg
}

func TestAccHostTriggerStatus_basic(t *testing.T) {
	s := acctest.RandString(8)
	var triggerID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Host and trigger status in the same apply.
				Config: testAccHostTriggerStatusConfig(s, "false"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("zabbix_host_trigger_status.icmp", "enabled", "false"),
					resource.TestCheckResourceAttrPair("zabbix_host_trigger_status.icmp", "host_id", "zabbix_host.test", "id"),
					testAccCheckTriggerStatus(t, "zabbix_host_trigger_status.icmp", false),
					func(st *terraform.State) error {
						triggerID = st.RootModule().Resources["zabbix_host_trigger_status.icmp"].Primary.ID
						return nil
					},
				),
			},
			{
				Config: testAccHostTriggerStatusConfig(s, "true"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("zabbix_host_trigger_status.icmp", "enabled", "true"),
					testAccCheckTriggerStatus(t, "zabbix_host_trigger_status.icmp", true),
				),
			},
			{
				Config: testAccHostTriggerStatusConfig(s, "false"),
				Check:  testAccCheckTriggerStatus(t, "zabbix_host_trigger_status.icmp", false),
			},
			{
				ResourceName:      "zabbix_host_trigger_status.icmp",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Removing the resource restores the status of the template trigger (enabled).
				Config: testAccHostTriggerStatusConfig(s, ""),
				Check: func(*terraform.State) error {
					trigger, err := testAccClient(t).HostTriggerGetByID(context.Background(), triggerID)
					if err != nil {
						return err
					}
					if !zabbix.StatusToEnabled(trigger.Status) {
						return fmt.Errorf("trigger %s still disabled after destroy", triggerID)
					}
					return nil
				},
			},
		},
	})
}

func TestAccHostTriggerStatus_unknownTrigger(t *testing.T) {
	s := acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccHostTriggerStatusConfig(s, "") + `
resource "zabbix_host_trigger_status" "missing" {
  host_id = zabbix_host.test.id
  name    = "No such trigger"
  enabled = false
}
`,
				ExpectError: regexp.MustCompile(`trigger "No such trigger" not found`),
			},
		},
	})
}
