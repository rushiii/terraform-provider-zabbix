package zabbix

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type recordedCall struct {
	Method string
	Params map[string]any
	Auth   string // "auth" field of the JSON-RPC body
	Bearer string // Authorization header
}

// fakeZabbix emulates the API-level differences between Zabbix versions that matter to the client.
func fakeZabbix(t *testing.T, version string, calls *[]recordedCall) *httptest.Server {
	t.Helper()
	v, err := parseAPIVersion(version)
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Auth   string          `json:"auth"`
			ID     int64           `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		call := recordedCall{Method: req.Method, Auth: req.Auth, Bearer: strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")}
		_ = json.Unmarshal(req.Params, &call.Params)
		*calls = append(*calls, call)

		reply := func(result any) {
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "result": result, "id": req.ID})
		}
		fail := func(msg string) {
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "error": map[string]any{"code": -32602, "message": "Invalid params.", "data": msg}, "id": req.ID})
		}

		switch req.Method {
		case "apiinfo.version":
			reply(version)
			return
		case "user.login":
			reply("session-token")
			return
		}

		// Authentication checks.
		if v.AtLeast(7, 2) && req.Auth != "" {
			fail(`Invalid parameter "/": unexpected parameter "auth".`)
			return
		}
		if !v.AtLeast(6, 4) && call.Bearer != "" {
			fail("Not authorized.")
			return
		}
		if req.Auth == "" && call.Bearer == "" {
			fail("Not authorized.")
			return
		}

		switch req.Method {
		case "host.get":
			if v.AtLeast(7, 2) {
				if _, ok := call.Params["selectGroups"]; ok {
					fail(`Invalid parameter "/": unexpected parameter "selectGroups".`)
					return
				}
			}
			host := map[string]any{"hostid": "10", "host": "h", "name": "h", "status": "0", "interfaces": []any{}, "parentTemplates": []any{}, "tags": []any{}}
			group := []any{map[string]any{"groupid": "2", "name": "Linux servers"}}
			if _, ok := call.Params["selectHostGroups"]; ok {
				host["hostgroups"] = group
			} else {
				host["groups"] = group
			}
			reply([]any{host})
		case "template.get":
			tpl := map[string]any{"templateid": "20", "host": "t", "name": "t", "macros": []any{}}
			group := []any{map[string]any{"groupid": "12"}}
			if _, ok := call.Params["selectTemplateGroups"]; ok {
				tpl["templategroups"] = group
			} else if !v.AtLeast(7, 2) {
				tpl["groups"] = group
			}
			reply([]any{tpl})
		case "usergroup.create", "usergroup.update":
			if v.AtLeast(7, 2) {
				if _, ok := call.Params["rights"]; ok {
					fail(`Invalid parameter "/1": unexpected parameter "rights".`)
					return
				}
			}
			reply(map[string]any{"usrgrpids": []string{"30"}})
		default:
			reply(map[string]any{})
		}
	}))
}

func newTestClient(t *testing.T, url string, auth Auth) *Client {
	t.Helper()
	c, err := NewClient(ClientConfig{URL: url, Auth: auth})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestClientAcrossVersions(t *testing.T) {
	cases := []struct {
		version        string
		wantHeader     bool
		wantNewGroups  bool
		wantLoginField string
	}{
		{"5.0.40", false, false, "user"},
		{"6.0.30", false, false, "username"},
		{"6.4.15", true, true, "username"},
		{"7.0.5", true, true, "username"},
		{"7.2.3", true, true, "username"},
		{"7.4.15", true, true, "username"},
		{"8.0.0", true, true, "username"},
	}
	for _, tc := range cases {
		for _, auth := range []Auth{
			{Method: AuthToken, Token: "api-token"},
			{Method: AuthUserPassword, Username: "Admin", Password: "zabbix"},
		} {
			t.Run(tc.version+"/"+string(auth.Method), func(t *testing.T) {
				var calls []recordedCall
				srv := fakeZabbix(t, tc.version, &calls)
				defer srv.Close()
				c := newTestClient(t, srv.URL, auth)
				ctx := context.Background()

				if err := c.Ping(ctx); err != nil {
					t.Fatalf("ping: %v", err)
				}
				host, err := c.HostGetByID(ctx, "10")
				if err != nil {
					t.Fatalf("host.get: %v", err)
				}
				if len(host.Groups) != 1 || host.Groups[0].GroupID != "2" || host.Groups[0].Name != "Linux servers" {
					t.Fatalf("host groups not parsed: %+v", host.Groups)
				}
				tpl, err := c.TemplateGetByID(ctx, "20")
				if err != nil {
					t.Fatalf("template.get: %v", err)
				}
				if len(tpl.Groups) != 1 || tpl.Groups[0].GroupID != "12" {
					t.Fatalf("template groups not parsed: %+v", tpl.Groups)
				}
				if _, err := c.UserGroupCreate(ctx, "g", []string{"2"}); err != nil {
					t.Fatalf("usergroup.create: %v", err)
				}
				if err := c.UserGroupUpdate(ctx, "30", "g", []string{"2"}); err != nil {
					t.Fatalf("usergroup.update: %v", err)
				}

				wantToken := "api-token"
				if auth.Method == AuthUserPassword {
					wantToken = "session-token"
				}
				for _, call := range calls {
					switch call.Method {
					case "apiinfo.version":
						if call.Auth != "" || call.Bearer != "" {
							t.Errorf("apiinfo.version must be sent without authentication")
						}
					case "user.login":
						if call.Auth != "" || call.Bearer != "" {
							t.Errorf("user.login must be sent without authentication")
						}
						if _, ok := call.Params[tc.wantLoginField]; !ok {
							t.Errorf("user.login: expected %q parameter, got %v", tc.wantLoginField, call.Params)
						}
					default:
						if tc.wantHeader && (call.Bearer != wantToken || call.Auth != "") {
							t.Errorf("%s: expected Authorization header only, got auth=%q bearer=%q", call.Method, call.Auth, call.Bearer)
						}
						if !tc.wantHeader && (call.Auth != wantToken || call.Bearer != "") {
							t.Errorf("%s: expected auth field only, got auth=%q bearer=%q", call.Method, call.Auth, call.Bearer)
						}
					}
					if call.Method == "usergroup.create" || call.Method == "usergroup.update" {
						_, hasNew := call.Params["hostgroup_rights"]
						_, hasOld := call.Params["rights"]
						if hasNew != tc.wantNewGroups || hasOld == tc.wantNewGroups {
							t.Errorf("%s: unexpected rights parameter: %v", call.Method, call.Params)
						}
					}
				}
			})
		}
	}
}

func TestParseAPIVersion(t *testing.T) {
	v, err := parseAPIVersion("7.4.2")
	if err != nil || v.Major != 7 || v.Minor != 4 {
		t.Fatalf("got %v, %v", v, err)
	}
	if !v.AtLeast(6, 4) || !v.AtLeast(7, 4) || v.AtLeast(7, 5) || v.AtLeast(8, 0) {
		t.Fatalf("AtLeast mismatch for %v", v)
	}
	if _, err := parseAPIVersion("garbage"); err == nil {
		t.Fatal("expected error")
	}
}
