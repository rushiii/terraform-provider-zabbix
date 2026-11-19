package zabbix

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// scriptedServer answers JSON-RPC calls with a handler and records them.
type scriptedServer struct {
	mu    sync.Mutex
	calls []recordedCall
}

func newScriptedServer(t *testing.T, handle func(call recordedCall) (result any, errData string)) (*httptest.Server, *scriptedServer) {
	t.Helper()
	rec := &scriptedServer{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Auth   string          `json:"auth"`
			ID     int64           `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		call := recordedCall{Method: req.Method, Auth: req.Auth, Bearer: strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")}
		_ = json.Unmarshal(req.Params, &call.Params)
		rec.mu.Lock()
		rec.calls = append(rec.calls, call)
		rec.mu.Unlock()

		if req.Method == "apiinfo.version" {
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "result": "7.4.0", "id": req.ID})
			return
		}
		result, errData := handle(call)
		if errData != "" {
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "error": map[string]any{"code": -32602, "message": "Invalid params.", "data": errData}, "id": req.ID})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "result": result, "id": req.ID})
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

func (s *scriptedServer) methods() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.calls))
	for _, c := range s.calls {
		out = append(out, c.Method)
	}
	return out
}

func (s *scriptedServer) last(method string) recordedCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.calls) - 1; i >= 0; i-- {
		if s.calls[i].Method == method {
			return s.calls[i]
		}
	}
	return recordedCall{}
}

// An expired session is renewed once with a new user.login, then the call is retried.
func TestSessionRenewal(t *testing.T) {
	logins := 0
	srv, rec := newScriptedServer(t, func(call recordedCall) (any, string) {
		switch call.Method {
		case "user.login":
			logins++
			if logins == 1 {
				return "session-1", ""
			}
			return "session-2", ""
		case "hostgroup.get":
			if call.Bearer == "session-1" && logins == 1 {
				return nil, "Session terminated, re-login, please."
			}
			return []any{map[string]any{"groupid": "2", "name": "Linux servers"}}, ""
		}
		return map[string]any{}, ""
	})
	c := newTestClient(t, srv.URL, Auth{Method: AuthUserPassword, Username: "Admin", Password: "zabbix"})
	g, err := c.HostGroupGetByID(context.Background(), "2")
	if err != nil {
		t.Fatalf("expected the call to succeed after re-login, got %v (calls: %v)", err, rec.methods())
	}
	if g.Name != "Linux servers" || logins != 2 {
		t.Fatalf("group %+v, logins %d", g, logins)
	}
	if last := rec.last("hostgroup.get"); last.Bearer != "session-2" {
		t.Fatalf("retried with token %q, want session-2", last.Bearer)
	}
}

// With an API token there is nothing to renew: the error is returned as is.
func TestSessionErrorWithTokenIsReturned(t *testing.T) {
	srv, rec := newScriptedServer(t, func(call recordedCall) (any, string) {
		return nil, "Not authorized."
	})
	c := newTestClient(t, srv.URL, Auth{Method: AuthToken, Token: "t"})
	if _, err := c.HostGroupGetByID(context.Background(), "2"); err == nil {
		t.Fatal("expected an error")
	}
	if n := len(rec.methods()); n != 2 { // apiinfo.version + one hostgroup.get
		t.Fatalf("expected no retry, calls: %v", rec.methods())
	}
}

// Several interfaces with the same type and main flag must not share an existing interface ID.
func TestInterfacesForHostUpdateDistinctIDs(t *testing.T) {
	current := map[string][]string{"1,0": {"11", "12"}, "1,1": {"10"}}
	ifaces := []HostInterface{
		{Type: 1, Main: 1, IP: "10.0.0.1"},
		{Type: 1, Main: 0, IP: "10.0.0.2"},
		{Type: 1, Main: 0, IP: "10.0.0.3"},
		{Type: 1, Main: 0, IP: "10.0.0.4"}, // new interface
	}
	out := interfacesForHostUpdate(ifaces, current)
	want := []any{"10", "11", "12", nil}
	for i, m := range out {
		if got := m["interfaceid"]; got != want[i] {
			t.Errorf("interface %d: interfaceid = %v, want %v", i, got, want[i])
		}
	}
	if len(current["1,0"]) != 2 {
		t.Error("input map must not be modified")
	}
}

// A host created with only SNMP interfaces is deleted again if setting up its interfaces fails.
func TestSNMPHostCreateRollback(t *testing.T) {
	srv, rec := newScriptedServer(t, func(call recordedCall) (any, string) {
		switch call.Method {
		case "host.create":
			return map[string]any{"hostids": []string{"42"}}, ""
		case "host.get":
			return []any{map[string]any{"hostid": "42", "host": "h", "interfaces": []any{map[string]any{"interfaceid": "7", "type": "1", "main": "1", "useip": "1"}}}}, ""
		case "hostinterface.create":
			return nil, "Incorrect value for field \"community\"."
		}
		return map[string]any{}, ""
	})
	c := newTestClient(t, srv.URL, Auth{Method: AuthToken, Token: "t"})
	_, err := c.HostCreate(context.Background(), HostCreateRequest{
		Host:       "h",
		GroupIDs:   []string{"2"},
		Interfaces: []HostInterface{{Type: 2, Main: 1, UseIP: 1, IP: "10.0.0.1", Port: "161", Details: &SNMPDetails{Version: 2, Community: "x"}}},
	})
	if err == nil || !strings.Contains(err.Error(), "hostinterface.create") {
		t.Fatalf("expected hostinterface.create error, got %v", err)
	}
	del := rec.last("host.delete")
	if del.Method == "" {
		t.Fatalf("host was not deleted after the failure: %v", rec.methods())
	}
}

// Host tags: nil leaves them unchanged, an empty list removes them.
func TestHostUpdateTags(t *testing.T) {
	srv, rec := newScriptedServer(t, func(call recordedCall) (any, string) {
		if call.Method == "host.get" {
			return []any{map[string]any{"hostid": "10", "host": "h", "interfaces": []any{}}}, ""
		}
		return map[string]any{}, ""
	})
	c := newTestClient(t, srv.URL, Auth{Method: AuthToken, Token: "t"})
	for _, tc := range []struct {
		name string
		tags []Tag
		sent bool
	}{
		{"not configured", nil, false},
		{"empty", []Tag{}, true},
		{"set", []Tag{{Tag: "env", Value: "prod"}}, true},
	} {
		if err := c.HostUpdate(context.Background(), "10", HostUpdateRequest{Host: "h", GroupIDs: []string{"2"}, Tags: tc.tags}); err != nil {
			t.Fatal(err)
		}
		_, sent := rec.last("host.update").Params["tags"]
		if sent != tc.sent {
			t.Errorf("%s: tags sent = %v, want %v", tc.name, sent, tc.sent)
		}
	}
}

// Template macros: nil leaves them unchanged, an empty map removes them.
func TestTemplateUpdateMacros(t *testing.T) {
	srv, rec := newScriptedServer(t, func(call recordedCall) (any, string) { return map[string]any{}, "" })
	c := newTestClient(t, srv.URL, Auth{Method: AuthToken, Token: "t"})
	for _, tc := range []struct {
		name   string
		macros map[string]string
		sent   bool
	}{
		{"not configured", nil, false},
		{"empty", map[string]string{}, true},
		{"set", map[string]string{"{$A}": "1"}, true},
	} {
		if err := c.TemplateUpdate(context.Background(), "20", "t", "t", []string{"1"}, tc.macros); err != nil {
			t.Fatal(err)
		}
		v, sent := rec.last("template.update").Params["macros"]
		if sent != tc.sent {
			t.Errorf("%s: macros sent = %v, want %v", tc.name, sent, tc.sent)
		}
		if tc.name == "empty" && len(v.([]any)) != 0 {
			t.Errorf("empty: macros = %v, want []", v)
		}
	}
}

func TestItemDelay(t *testing.T) {
	for _, tc := range []struct {
		delay, flex string
		want        any
	}{
		{"10m", "", "10m"},
		{"0", "", 0},
		{"10m", "50s/1-5,09:00-18:00", "10m;50s/1-5,09:00-18:00"},
		{"10m", "50s;1-7,00:00-24:00", "10m;50s/1-7,00:00-24:00"}, // older documented form
		{"10m", "50s/1-5,09:00-18:00;md1-5", "10m;50s/1-5,09:00-18:00;md1-5"},
		{"0", "md1-5h9", "0;md1-5h9"},
	} {
		if got := itemDelayParam(tc.delay, tc.flex); got != tc.want {
			t.Errorf("itemDelayParam(%q, %q) = %v, want %v", tc.delay, tc.flex, got, tc.want)
		}
	}
	if d, f := SplitItemDelay("10m;50s/1-5,09:00-18:00;md1-5"); d != "10m" || f != "50s/1-5,09:00-18:00;md1-5" {
		t.Errorf("SplitItemDelay = %q, %q", d, f)
	}
	if d, f := SplitItemDelay("60s"); d != "60s" || f != "" {
		t.Errorf("SplitItemDelay = %q, %q", d, f)
	}
}

func TestActionParams(t *testing.T) {
	p := actionParams(ActionCreateRequest{
		Name: "a", UserGroupIDs: []string{"7"}, Subject: "Problème {HOST.NAME}", Message: "Corps\nligne 2",
		HostGroupIDs: []string{"10"}, TriggerNameLike: []string{"Lampe"}, Enabled: true,
	})
	op := p["operations"].([]map[string]any)[0]
	msg := op["opmessage"].(map[string]string)
	if msg["default_msg"] != "0" || msg["subject"] != "Problème {HOST.NAME}" || msg["message"] != "Corps\nligne 2" {
		t.Errorf("opmessage = %v", msg)
	}
	filter := p["filter"].(map[string]any)
	if filter["evaltype"] != 2 || len(filter["conditions"].([]map[string]any)) != 2 {
		t.Errorf("filter = %v", filter)
	}
	if p["status"] != "0" || p["esc_period"] != "1h" {
		t.Errorf("status/esc_period = %v/%v", p["status"], p["esc_period"])
	}
}

// A transient database error (concurrent requests) is retried; other errors are not.
func TestTransientDBErrorRetried(t *testing.T) {
	saved := dbRetryDelays
	dbRetryDelays = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	t.Cleanup(func() { dbRetryDelays = saved })

	creates := 0
	srv, _ := newScriptedServer(t, func(call recordedCall) (any, string) {
		if call.Method == "template.create" {
			creates++
			if creates < 3 {
				return nil, "Database error occurred."
			}
			return map[string]any{"templateids": []string{"30"}}, ""
		}
		if call.Method == "hostgroup.create" {
			return nil, "Host group \"x\" already exists."
		}
		return map[string]any{}, ""
	})
	c := newTestClient(t, srv.URL, Auth{Method: AuthToken, Token: "t"})
	id, err := c.TemplateCreate(context.Background(), "t", "t", []string{"1"}, nil)
	if err != nil || id != "30" || creates != 3 {
		t.Fatalf("id=%q err=%v creates=%d", id, err, creates)
	}

	if _, err := c.HostGroupCreate(context.Background(), "x"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected the error to be returned as is, got %v", err)
	}

	// Gives up after the last retry.
	creates = -10
	if _, err := c.TemplateCreate(context.Background(), "t", "t", []string{"1"}, nil); err == nil || creates != -6 {
		t.Fatalf("expected failure after %d attempts, err=%v creates=%d", len(dbRetryDelays)+1, err, creates)
	}
}
