package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// testAccAPI calls the Zabbix API directly (to check or change objects outside Terraform).
func testAccAPI(t *testing.T, method string, params any, out any) {
	t.Helper()
	call := func(m string, p any, auth string) json.RawMessage {
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": m, "params": p, "id": 1})
		req, _ := http.NewRequest(http.MethodPost, os.Getenv("ZABBIX_URL"), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json-rpc")
		if auth != "" {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var r struct {
			Result json.RawMessage `json:"result"`
			Error  any             `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			t.Fatal(err)
		}
		if r.Error != nil {
			t.Fatalf("%s: %v", m, r.Error)
		}
		return r.Result
	}
	var token string
	_ = json.Unmarshal(call("user.login", map[string]string{"username": os.Getenv("ZABBIX_USERNAME"), "password": os.Getenv("ZABBIX_PASSWORD")}, ""), &token)
	if err := json.Unmarshal(call(method, params, token), out); err != nil {
		t.Fatal(err)
	}
}

type accHost struct {
	HostID     string `json:"hostid"`
	Name       string `json:"name"`
	Interfaces []struct {
		IP   string `json:"ip"`
		Main string `json:"main"`
		Port string `json:"port"`
	} `json:"interfaces"`
	Tags []struct {
		Tag   string `json:"tag"`
		Value string `json:"value"`
	} `json:"tags"`
}

func testAccGetHost(t *testing.T, host string) accHost {
	var hosts []accHost
	testAccAPI(t, "host.get", map[string]any{
		"output": []string{"hostid", "name"}, "filter": map[string]any{"host": []string{host}},
		"selectInterfaces": []string{"ip", "main", "port"}, "selectTags": "extend",
	}, &hosts)
	if len(hosts) != 1 {
		t.Fatalf("host %s not found", host)
	}
	return hosts[0]
}

func testAccHostBlock(host, extra, interfaces string) string {
	if interfaces == "" {
		interfaces = `
  interfaces {
    type = 1
    ip   = "10.0.0.1"
  }`
	}
	return fmt.Sprintf(`
resource "zabbix_host" "test" {
  name             = %q
  host_group_names = ["Linux servers"]
  template_names   = ["ICMP Ping"]
%s
%s
}
`, host, extra, interfaces)
}

// A host without visible_name: no perpetual diff (Zabbix reports the technical name).
func TestAccHost_noVisibleName(t *testing.T) {
	host := "tf-acc-novis-" + acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + testAccHostBlock(host, "", ""),
				Check:  resource.TestCheckNoResourceAttr("zabbix_host.test", "visible_name"),
			},
			{
				Config: testAccProviderConfig() + testAccHostBlock(host, `  visible_name = "Visible"`, ""),
				Check:  resource.TestCheckResourceAttr("zabbix_host.test", "visible_name", "Visible"),
			},
		},
	})
}

// Tags: unmanaged when not configured, managed when set, removed with {}.
func TestAccHost_tags(t *testing.T) {
	host := "tf-acc-tags-" + acctest.RandString(8)
	cfg := func(extra string) string {
		return testAccProviderConfig() + testAccHostBlock(host, `  visible_name = "v"
`+extra, "")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg("")},
			{
				// A tag added outside Terraform is kept: no diff, and an apply does not remove it.
				PreConfig: func() {
					h := testAccGetHost(t, host)
					var ignored any
					testAccAPI(t, "host.update", map[string]any{"hostid": h.HostID, "tags": []map[string]string{{"tag": "external", "value": "yes"}}}, &ignored)
				},
				Config: cfg(`  enabled = false`),
				Check: func(*terraform.State) error {
					if tags := testAccGetHost(t, host).Tags; len(tags) != 1 || tags[0].Tag != "external" {
						return fmt.Errorf("external tag removed: %v", tags)
					}
					return nil
				},
			},
			{
				Config: cfg(`  tags = { env = "prod" }`),
				Check: func(*terraform.State) error {
					if tags := testAccGetHost(t, host).Tags; len(tags) != 1 || tags[0].Tag != "env" || tags[0].Value != "prod" {
						return fmt.Errorf("tags = %v, want env=prod", tags)
					}
					return nil
				},
			},
			{
				Config: cfg(`  tags = {}`),
				Check: func(*terraform.State) error {
					if tags := testAccGetHost(t, host).Tags; len(tags) != 0 {
						return fmt.Errorf("tags = %v, want none", tags)
					}
					return nil
				},
			},
		},
	})
}

// A user macro as port is kept as is; an invalid port is rejected.
func TestAccHost_portMacro(t *testing.T) {
	host := "tf-acc-port-" + acctest.RandString(8)
	iface := func(port string) string {
		return fmt.Sprintf(`
  interfaces {
    type = 1
    ip   = "10.0.0.1"
    port = %q
  }`, port)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + testAccHostBlock(host, `  visible_name = "v"`, iface("{$AGENT.PORT}")),
				Check: func(*terraform.State) error {
					if p := testAccGetHost(t, host).Interfaces[0].Port; p != "{$AGENT.PORT}" {
						return fmt.Errorf("port = %q in Zabbix", p)
					}
					return nil
				},
			},
			{
				Config:      testAccProviderConfig() + testAccHostBlock(host, `  visible_name = "v"`, iface("agent")),
				ExpectError: regexp.MustCompile("Invalid port"),
			},
		},
	})
}

// Several non-main interfaces of the same type can be updated.
func TestAccHost_interfacesSameType(t *testing.T) {
	host := "tf-acc-ifaces-" + acctest.RandString(8)
	ifaces := func(a, b string) string {
		return fmt.Sprintf(`
  interfaces {
    type = 1
    ip   = "10.0.0.1"
  }
  interfaces {
    type = 1
    main = false
    ip   = %q
  }
  interfaces {
    type = 1
    main = false
    ip   = %q
  }`, a, b)
	}
	check := func(want ...string) resource.TestCheckFunc {
		return func(*terraform.State) error {
			var got []string
			for _, i := range testAccGetHost(t, host).Interfaces {
				got = append(got, i.IP)
			}
			sort.Strings(got)
			sort.Strings(want)
			if strings.Join(got, ",") != strings.Join(want, ",") {
				return fmt.Errorf("interfaces = %v, want %v", got, want)
			}
			return nil
		}
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + testAccHostBlock(host, `  visible_name = "v"`, ifaces("10.0.0.2", "10.0.0.3")),
				Check:  check("10.0.0.1", "10.0.0.2", "10.0.0.3"),
			},
			{
				Config: testAccProviderConfig() + testAccHostBlock(host, `  visible_name = "v"`, ifaces("10.0.0.4", "10.0.0.5")),
				Check:  check("10.0.0.1", "10.0.0.4", "10.0.0.5"),
			},
		},
	})
}

// The configured subject and message are the ones Zabbix sends.
func TestAccAction_message(t *testing.T) {
	name := "tf-acc-action-" + acctest.RandString(8)
	cfg := func(subject, message string) string {
		return testAccProviderConfig() + fmt.Sprintf(`
data "zabbix_user_group" "admins" {
  name = "Zabbix administrators"
}

resource "zabbix_action" "test" {
  name              = %q
  user_group_ids    = [data.zabbix_user_group.admins.id]
  trigger_name_like = ["Lampe"]
  subject           = %q
  message           = <<-EOT
    %s
  EOT
}
`, name, subject, message)
	}
	check := func(subject, message string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			var actions []struct {
				Operations []struct {
					Opmessage struct {
						DefaultMsg string `json:"default_msg"`
						Subject    string `json:"subject"`
						Message    string `json:"message"`
					} `json:"opmessage"`
				} `json:"operations"`
			}
			testAccAPI(t, "action.get", map[string]any{
				"actionids": []string{s.RootModule().Resources["zabbix_action.test"].Primary.ID}, "selectOperations": "extend",
			}, &actions)
			m := actions[0].Operations[0].Opmessage
			if m.DefaultMsg != "0" || m.Subject != subject || strings.TrimSpace(m.Message) != message {
				return fmt.Errorf("opmessage = %+v", m)
			}
			return nil
		}
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg("Alerte {HOST.NAME}", "Trigger: {TRIGGER.NAME}"), Check: check("Alerte {HOST.NAME}", "Trigger: {TRIGGER.NAME}")},
			{Config: cfg("Lampe HS", "Changer la lampe"), Check: check("Lampe HS", "Changer la lampe")},
		},
	})
}

// Flexible intervals are applied (older and Zabbix formats) without perpetual diff.
func TestAccItem_delayFlex(t *testing.T) {
	s := acctest.RandString(8)
	cfg := func(flex string) string {
		return testAccProviderConfig() + fmt.Sprintf(`
resource "zabbix_template" "t" {
  host           = "tf-acc-item-tpl-%s"
  host_group_ids = ["1"]
}

resource "zabbix_item" "test" {
  host_id    = zabbix_template.t.id
  name       = "Agent ping"
  key        = "agent.ping"
  type       = 0
  value_type = 3
  delay      = "10m"
  delay_flex = %q
}
`, s, flex)
	}
	check := func(want string) resource.TestCheckFunc {
		return func(st *terraform.State) error {
			var items []struct {
				Delay string `json:"delay"`
			}
			testAccAPI(t, "item.get", map[string]any{"itemids": []string{st.RootModule().Resources["zabbix_item.test"].Primary.ID}, "output": []string{"delay"}}, &items)
			if items[0].Delay != want {
				return fmt.Errorf("delay = %q in Zabbix, want %q", items[0].Delay, want)
			}
			return nil
		}
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg("50s;1-7,00:00-24:00"), Check: check("10m;50s/1-7,00:00-24:00")},
			{Config: cfg("30s/1-5,09:00-18:00;md1-5"), Check: check("10m;30s/1-5,09:00-18:00;md1-5")},
		},
	})
}

// The last macro of a template can be removed with macros = {}.
func TestAccTemplate_removeMacros(t *testing.T) {
	s := acctest.RandString(8)
	cfg := func(macros string) string {
		return testAccProviderConfig() + fmt.Sprintf(`
resource "zabbix_template" "t" {
  host           = "tf-acc-macros-%s"
  host_group_ids = ["1"]
  macros         = %s
}
`, s, macros)
	}
	count := func(want int) resource.TestCheckFunc {
		return func(st *terraform.State) error {
			var macros []any
			testAccAPI(t, "usermacro.get", map[string]any{"hostids": []string{st.RootModule().Resources["zabbix_template.t"].Primary.ID}}, &macros)
			if len(macros) != want {
				return fmt.Errorf("%d macros in Zabbix, want %d", len(macros), want)
			}
			return nil
		}
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(`{ "{$SNMP_COMMUNITY}" = "public" }`), Check: count(1)},
			{Config: cfg(`{}`), Check: count(0)},
		},
	})
}
