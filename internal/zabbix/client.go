package zabbix

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type AuthMethod string

const (
	AuthToken        AuthMethod = "token"
	AuthUserPassword AuthMethod = "userpass"
)

var ErrNotFound = errors.New("zabbix object not found")

func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

type Auth struct {
	Method   AuthMethod
	Token    string
	Username string
	Password string
}

type ClientConfig struct {
	URL             string
	Timeout         time.Duration
	InsecureSkipTLS bool
	Auth            Auth
}

type Client struct {
	url        string
	httpClient *http.Client
	auth       Auth

	mu          sync.Mutex
	sessionAuth string
	rpcID       int64
}

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	Auth    string      `json:"auth,omitempty"`
	ID      int64       `json:"id"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
	ID      int64           `json:"id"`
}

func NewClient(cfg ClientConfig) (*Client, error) {
	if cfg.URL == "" {
		return nil, errors.New("url Zabbix obligatoire")
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.InsecureSkipTLS}, //nolint:gosec
	}

	return &Client{
		url: cfg.URL,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		auth:  cfg.Auth,
		rpcID: 1,
	}, nil
}

func (c *Client) nextID() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rpcID++
	return c.rpcID
}

func (c *Client) ensureAuth(ctx context.Context) (string, error) {
	if c.auth.Method == AuthToken {
		if c.auth.Token == "" {
			return "", errors.New("api_token vide")
		}
		return c.auth.Token, nil
	}

	c.mu.Lock()
	if c.sessionAuth != "" {
		token := c.sessionAuth
		c.mu.Unlock()
		return token, nil
	}
	c.mu.Unlock()

	params := map[string]any{
		"username": c.auth.Username,
		"password": c.auth.Password,
	}

	var token string
	if err := c.callNoAuth(ctx, "user.login", params, &token); err != nil {
		return "", err
	}

	c.mu.Lock()
	c.sessionAuth = token
	c.mu.Unlock()

	return token, nil
}

func (c *Client) Ping(ctx context.Context) error {
	var version string
	return c.callNoAuth(ctx, "apiinfo.version", map[string]any{}, &version)
}

func (c *Client) callNoAuth(ctx context.Context, method string, params interface{}, out interface{}) error {
	return c.call(ctx, method, params, false, out)
}

func (c *Client) callAuth(ctx context.Context, method string, params interface{}, out interface{}) error {
	return c.call(ctx, method, params, true, out)
}

func (c *Client) call(ctx context.Context, method string, params interface{}, withAuth bool, out interface{}) error {
	if method == "host.update" {
		if b, err := json.Marshal(params); err == nil {
			log.Printf("zabbix client debug: host.update params=%s", string(b))
		}
	}

	requestBody := rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      c.nextID(),
	}
	if withAuth {
		token, err := c.ensureAuth(ctx)
		if err != nil {
			return err
		}
		requestBody.Auth = token
	}

	rawReq, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(rawReq))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json-rpc")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	rawResp, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return err
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode > 299 {
		return fmt.Errorf("http status %d: %s", httpResp.StatusCode, string(rawResp))
	}

	var payload rpcResponse
	if err := json.Unmarshal(rawResp, &payload); err != nil {
		return err
	}
	if payload.Error != nil {
		return fmt.Errorf("zabbix api error (%d) %s: %s", payload.Error.Code, payload.Error.Message, payload.Error.Data)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(payload.Result, out)
}

type Tag struct {
	Tag   string `json:"tag"`
	Value string `json:"value"`
}

type SNMPDetails struct {
	Version   int    `json:"version,omitempty"`
	Community string `json:"community,omitempty"`
	Security  string `json:"securityname,omitempty"`
	AuthProto string `json:"authprotocol,omitempty"`
	AuthPass  string `json:"authpassphrase,omitempty"`
	PrivProto string `json:"privprotocol,omitempty"`
	PrivPass  string `json:"privpassphrase,omitempty"`
	Context   string `json:"contextname,omitempty"`
}

// parseInt accepts JSON number or string for Zabbix API compatibility (some versions return main/type/useip as string).
func parseInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return int(x)
	case string:
		i, _ := strconv.Atoi(x)
		return i
	case int:
		return x
	default:
		return 0
	}
}

// parseString accepts JSON string or number for Zabbix API compatibility (e.g. port, status as number).
func parseString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	default:
		return ""
	}
}

type hostInterfaceJSON struct {
	InterfaceID string          `json:"interfaceid,omitempty"`
	Type        interface{}     `json:"type"`
	Main        interface{}     `json:"main"`
	UseIP       interface{}     `json:"useip"`
	IP          string          `json:"ip,omitempty"`
	DNS         string          `json:"dns,omitempty"`
	Port        interface{}     `json:"port,omitempty"` // API can return number or string
	Details     json.RawMessage `json:"details,omitempty"`
}

func (hi *HostInterface) UnmarshalJSON(data []byte) error {
	var raw hostInterfaceJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	hi.InterfaceID = raw.InterfaceID
	hi.Type = parseInt(raw.Type)
	hi.Main = parseInt(raw.Main)
	hi.UseIP = parseInt(raw.UseIP)
	hi.IP = raw.IP
	hi.DNS = raw.DNS
	hi.Port = parseString(raw.Port)
	hi.Details = parseSNMPDetails(raw.Details)
	return nil
}

// parseSNMPDetails accepts details as JSON object or array (Zabbix API can return either).
func parseSNMPDetails(data json.RawMessage) *SNMPDetails {
	if len(data) == 0 {
		return nil
	}
	var single SNMPDetails
	if err := json.Unmarshal(data, &single); err == nil {
		return &single
	}
	var arr []SNMPDetails
	if err := json.Unmarshal(data, &arr); err == nil && len(arr) > 0 {
		return &arr[0]
	}
	return nil
}

type HostInterface struct {
	InterfaceID string       `json:"interfaceid,omitempty"`
	Type        int          `json:"type"`
	Main        int          `json:"main"`
	UseIP       int          `json:"useip"`
	IP          string       `json:"ip,omitempty"`
	DNS         string       `json:"dns,omitempty"`
	Port        string       `json:"port,omitempty"`
	Details     *SNMPDetails `json:"details,omitempty"`
}

type Host struct {
	HostID string `json:"hostid"`
	Host   string `json:"host"`
	Name   string `json:"name"`
	Status string `json:"status"`

	Interfaces []HostInterface `json:"interfaces"`
	Groups     []struct {
		GroupID string `json:"groupid"`
		Name    string `json:"name"`
	} `json:"groups"`
	ParentTemplates []struct {
		TemplateID string `json:"templateid"`
		Host       string `json:"host"`
		Name       string `json:"name"`
	} `json:"parentTemplates"`
	Tags []Tag `json:"tags"`
}

type hostJSON struct {
	HostID   interface{} `json:"hostid"`   // API can return string or number
	Host     string      `json:"host"`
	Name     string      `json:"name"`
	Status   interface{} `json:"status"`  // API can return "0"/"1" or 0/1
	Interfaces []HostInterface `json:"interfaces"`
	Groups   []struct {
		GroupID string `json:"groupid"`
		Name    string `json:"name"`
	} `json:"groups"`
	ParentTemplates []struct {
		TemplateID string `json:"templateid"`
		Host       string `json:"host"`
		Name       string `json:"name"`
	} `json:"parentTemplates"`
	Tags []Tag `json:"tags"`
}

func (h *Host) UnmarshalJSON(data []byte) error {
	var raw hostJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	h.HostID = parseString(raw.HostID)
	h.Host = raw.Host
	h.Name = raw.Name
	h.Status = parseString(raw.Status)
	h.Interfaces = raw.Interfaces
	h.Groups = raw.Groups
	h.ParentTemplates = raw.ParentTemplates
	h.Tags = raw.Tags
	return nil
}

type HostCreateRequest struct {
	Host        string
	Name        string
	Status      int
	Interfaces  []HostInterface
	GroupIDs    []string
	TemplateIDs []string
	Tags        []Tag
}

type HostUpdateRequest = HostCreateRequest

func (c *Client) HostCreate(ctx context.Context, req HostCreateRequest) (string, error) {
	groups := make([]map[string]string, 0, len(req.GroupIDs))
	for _, g := range req.GroupIDs {
		groups = append(groups, map[string]string{"groupid": g})
	}

	templates := make([]map[string]string, 0, len(req.TemplateIDs))
	for _, t := range req.TemplateIDs {
		templates = append(templates, map[string]string{"templateid": t})
	}

	tags := req.Tags
	if tags == nil {
		tags = []Tag{}
	}
	params := map[string]any{
		"host":       req.Host,
		"name":       req.Name,
		"status":     req.Status,
		"interfaces": req.Interfaces,
		"groups":     groups,
		"tags":       tags,
	}
	if len(templates) > 0 {
		params["templates"] = templates
	}

	var result struct {
		HostIDs []string `json:"hostids"`
	}
	if err := c.callAuth(ctx, "host.create", params, &result); err != nil {
		return "", err
	}
	if len(result.HostIDs) == 0 {
		return "", errors.New("host.create n'a retourné aucun hostid")
	}
	return result.HostIDs[0], nil
}

func (c *Client) HostGetByID(ctx context.Context, hostID string) (*Host, error) {
	params := map[string]any{
		"hostids":               []string{hostID},
		"output":                []string{"hostid", "host", "name", "status"},
		"selectInterfaces":      "extend",
		"selectGroups":          []string{"groupid", "name"},
		"selectParentTemplates": []string{"templateid", "host", "name"},
		"selectTags":            "extend",
	}

	var hosts []Host
	if err := c.callAuth(ctx, "host.get", params, &hosts); err != nil {
		return nil, err
	}
	if len(hosts) == 0 {
		return nil, ErrNotFound
	}
	return &hosts[0], nil
}

func (c *Client) HostUpdate(ctx context.Context, hostID string, req HostUpdateRequest) error {
	groups := make([]map[string]string, 0, len(req.GroupIDs))
	for _, g := range req.GroupIDs {
		groups = append(groups, map[string]string{"groupid": g})
	}

	templates := make([]map[string]string, 0, len(req.TemplateIDs))
	for _, t := range req.TemplateIDs {
		templates = append(templates, map[string]string{"templateid": t})
	}

	tags := req.Tags
	if tags == nil {
		tags = []Tag{}
	}
	params := map[string]any{
		"hostid":     hostID,
		"host":       req.Host,
		"status":     req.Status, // 0=monitored, 1=not monitored (integer pour Zabbix 6.x)
		"interfaces": req.Interfaces,
		"groups":     groups,
		"tags":       tags,
	}
	if req.Name != "" {
		params["name"] = req.Name
	}
	if len(templates) > 0 {
		params["templates"] = templates
	}

	// Zabbix 6.x host.update attend un tableau d'objets host.
	var ignored any
	if err := c.callAuth(ctx, "host.update", params, &ignored); err != nil {
		return err
	}

	return nil
}

func (c *Client) HostDelete(ctx context.Context, hostID string) error {
	var ignored any
	return c.callAuth(ctx, "host.delete", []string{hostID}, &ignored)
}

type HostGroup struct {
	GroupID string `json:"groupid"`
	Name    string `json:"name"`
}

func (c *Client) HostGroupCreate(ctx context.Context, name string) (string, error) {
	var result struct {
		GroupIDs []string `json:"groupids"`
	}
	if err := c.callAuth(ctx, "hostgroup.create", map[string]string{"name": name}, &result); err != nil {
		return "", err
	}
	if len(result.GroupIDs) == 0 {
		return "", errors.New("hostgroup.create n'a retourné aucun groupid")
	}
	return result.GroupIDs[0], nil
}

func (c *Client) HostGroupGetByID(ctx context.Context, id string) (*HostGroup, error) {
	params := map[string]any{
		"groupids": []string{id},
		"output":   []string{"groupid", "name"},
	}
	var groups []HostGroup
	if err := c.callAuth(ctx, "hostgroup.get", params, &groups); err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, ErrNotFound
	}
	return &groups[0], nil
}

func (c *Client) HostGroupIDsByNames(ctx context.Context, names []string) ([]string, error) {
	out := make([]string, 0, len(names))
	for _, name := range names {
		params := map[string]any{
			"output": []string{"groupid", "name"},
			"filter": map[string]any{
				"name": []string{name},
			},
		}
		var groups []HostGroup
		if err := c.callAuth(ctx, "hostgroup.get", params, &groups); err != nil {
			return nil, err
		}
		if len(groups) == 0 {
			return nil, fmt.Errorf("groupe d'hotes introuvable: %s", name)
		}
		if len(groups) > 1 {
			return nil, fmt.Errorf("groupe d'hotes ambigu: %s", name)
		}
		out = append(out, groups[0].GroupID)
	}
	return out, nil
}

func (c *Client) HostGroupUpdate(ctx context.Context, id, name string) error {
	var ignored any
	return c.callAuth(ctx, "hostgroup.update", map[string]string{"groupid": id, "name": name}, &ignored)
}

func (c *Client) HostGroupDelete(ctx context.Context, id string) error {
	var ignored any
	return c.callAuth(ctx, "hostgroup.delete", []string{id}, &ignored)
}

type Template struct {
	TemplateID string `json:"templateid"`
	Host       string `json:"host"`
	Name       string `json:"name"`
	Groups     []struct {
		GroupID string `json:"groupid"`
	} `json:"groups"`
}

func (c *Client) TemplateCreate(ctx context.Context, host, name string, groupIDs []string) (string, error) {
	groups := make([]map[string]string, 0, len(groupIDs))
	for _, g := range groupIDs {
		groups = append(groups, map[string]string{"groupid": g})
	}
	params := map[string]any{
		"host":   host,
		"name":   name,
		"groups": groups,
	}
	var result struct {
		TemplateIDs []string `json:"templateids"`
	}
	if err := c.callAuth(ctx, "template.create", params, &result); err != nil {
		return "", err
	}
	if len(result.TemplateIDs) == 0 {
		return "", errors.New("template.create n'a retourné aucun templateid")
	}
	return result.TemplateIDs[0], nil
}

func (c *Client) TemplateGetByID(ctx context.Context, id string) (*Template, error) {
	params := map[string]any{
		"templateids":  []string{id},
		"output":       []string{"templateid", "host", "name"},
		"selectGroups": []string{"groupid"},
	}
	var templates []Template
	if err := c.callAuth(ctx, "template.get", params, &templates); err != nil {
		return nil, err
	}
	if len(templates) == 0 {
		return nil, ErrNotFound
	}
	return &templates[0], nil
}

func (c *Client) TemplateIDsByNames(ctx context.Context, names []string) ([]string, error) {
	out := make([]string, 0, len(names))
	for _, name := range names {
		id, err := c.templateIDByName(ctx, name)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

func (c *Client) templateIDByName(ctx context.Context, name string) (string, error) {
	// Priorite au nom interne "host" (cas le plus frequent dans Zabbix).
	paramsByHost := map[string]any{
		"output": []string{"templateid", "host", "name"},
		"filter": map[string]any{
			"host": []string{name},
		},
	}
	var hostMatches []Template
	if err := c.callAuth(ctx, "template.get", paramsByHost, &hostMatches); err != nil {
		return "", err
	}
	if len(hostMatches) == 1 {
		return hostMatches[0].TemplateID, nil
	}
	if len(hostMatches) > 1 {
		return "", fmt.Errorf("template ambigu (host): %s", name)
	}

	paramsByName := map[string]any{
		"output": []string{"templateid", "host", "name"},
		"filter": map[string]any{
			"name": []string{name},
		},
	}
	var nameMatches []Template
	if err := c.callAuth(ctx, "template.get", paramsByName, &nameMatches); err != nil {
		return "", err
	}
	if len(nameMatches) == 0 {
		return "", fmt.Errorf("template introuvable: %s", name)
	}
	if len(nameMatches) > 1 {
		return "", fmt.Errorf("template ambigu (name): %s", name)
	}
	return nameMatches[0].TemplateID, nil
}

func (c *Client) TemplateUpdate(ctx context.Context, id, host, name string, groupIDs []string) error {
	groups := make([]map[string]string, 0, len(groupIDs))
	for _, g := range groupIDs {
		groups = append(groups, map[string]string{"groupid": g})
	}
	params := map[string]any{
		"templateid": id,
		"host":       host,
		"name":       name,
		"groups":     groups,
	}
	var ignored any
	return c.callAuth(ctx, "template.update", params, &ignored)
}

func (c *Client) TemplateDelete(ctx context.Context, id string) error {
	var ignored any
	return c.callAuth(ctx, "template.delete", []string{id}, &ignored)
}

type Trigger struct {
	TriggerID   string `json:"triggerid"`
	Description string `json:"description"`
	Expression  string `json:"expression"`
	Priority    string `json:"priority"`
	Status      string `json:"status"`
}

func (c *Client) TriggerCreate(ctx context.Context, description, expression, priority string, enabled bool) (string, error) {
	params := map[string]any{
		"description": description,
		"expression":  expression,
		"priority":    priority,
		"status":      boolToStatus(enabled),
	}
	var result struct {
		TriggerIDs []string `json:"triggerids"`
	}
	if err := c.callAuth(ctx, "trigger.create", params, &result); err != nil {
		return "", err
	}
	if len(result.TriggerIDs) == 0 {
		return "", errors.New("trigger.create n'a retourné aucun triggerid")
	}
	return result.TriggerIDs[0], nil
}

func (c *Client) TriggerGetByID(ctx context.Context, id string) (*Trigger, error) {
	params := map[string]any{
		"triggerids": []string{id},
		"output":     []string{"triggerid", "description", "expression", "priority", "status"},
	}
	var triggers []Trigger
	if err := c.callAuth(ctx, "trigger.get", params, &triggers); err != nil {
		return nil, err
	}
	if len(triggers) == 0 {
		return nil, ErrNotFound
	}
	return &triggers[0], nil
}

func (c *Client) TriggerUpdate(ctx context.Context, id, description, expression, priority string, enabled bool) error {
	params := map[string]any{
		"triggerid":   id,
		"description": description,
		"expression":  expression,
		"priority":    priority,
		"status":      boolToStatus(enabled),
	}
	var ignored any
	return c.callAuth(ctx, "trigger.update", params, &ignored)
}

func (c *Client) TriggerDelete(ctx context.Context, id string) error {
	var ignored any
	return c.callAuth(ctx, "trigger.delete", []string{id}, &ignored)
}

func StatusToEnabled(status string) bool {
	value, err := strconv.Atoi(status)
	if err != nil {
		return false
	}
	return value == 0
}

func boolToStatus(enabled bool) int {
	if enabled {
		return 0
	}
	return 1
}
