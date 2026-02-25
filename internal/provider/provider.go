package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/RuShIII/terraform-provider-zabbix/internal/zabbix"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &zabbixProvider{}

type zabbixProvider struct {
	version string
}

type providerModel struct {
	URL             types.String `tfsdk:"url"`
	APIToken        types.String `tfsdk:"api_token"`
	Username        types.String `tfsdk:"username"`
	Password        types.String `tfsdk:"password"`
	TimeoutSeconds  types.Int64  `tfsdk:"timeout_seconds"`
	InsecureSkipTLS types.Bool   `tfsdk:"insecure_skip_tls"`
}

type providerData struct {
	Client *zabbix.Client
}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &zabbixProvider{version: version} }
}

func (p *zabbixProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "zabbix"
	resp.Version = p.version
}

func (p *zabbixProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = pschema.Schema{
		MarkdownDescription: "Provider OpenTofu/Terraform pour l'API Zabbix (JSON-RPC).",
		Attributes: map[string]pschema.Attribute{
			"url": pschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "URL de l'API Zabbix, ex: https://zabbix.example.com/api_jsonrpc.php",
			},
			"api_token": pschema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Token API Zabbix. Prioritaire si défini.",
			},
			"username": pschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Nom d'utilisateur Zabbix (si api_token absent).",
			},
			"password": pschema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Mot de passe Zabbix (si api_token absent).",
			},
			"timeout_seconds": pschema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Timeout HTTP en secondes (défaut: 30).",
			},
			"insecure_skip_tls": pschema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Ignore la validation TLS.",
			},
		},
	}
}

func (p *zabbixProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(checkKnown(cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	timeout := 30 * time.Second
	if !cfg.TimeoutSeconds.IsNull() && cfg.TimeoutSeconds.ValueInt64() > 0 {
		timeout = time.Duration(cfg.TimeoutSeconds.ValueInt64()) * time.Second
	}

	auth, d := buildAuth(cfg)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := zabbix.NewClient(zabbix.ClientConfig{
		URL:             cfg.URL.ValueString(),
		Timeout:         timeout,
		InsecureSkipTLS: !cfg.InsecureSkipTLS.IsNull() && cfg.InsecureSkipTLS.ValueBool(),
		Auth:            auth,
	})
	if err != nil {
		resp.Diagnostics.AddError("Erreur initialisation client Zabbix", err.Error())
		return
	}

	if err := client.Ping(ctx); err != nil {
		resp.Diagnostics.AddError(
			"Connexion Zabbix invalide",
			fmt.Sprintf("Impossible de valider l'accès API: %v", err),
		)
		return
	}

	data := &providerData{Client: client}
	resp.ResourceData = data
	resp.DataSourceData = data
}

func checkKnown(cfg providerModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if cfg.URL.IsUnknown() {
		diags.AddAttributeError(path.Root("url"), "Valeur inconnue", "`url` doit être connue au plan.")
	}
	return diags
}

func buildAuth(cfg providerModel) (zabbix.Auth, diag.Diagnostics) {
	var diags diag.Diagnostics

	token := ""
	user := ""
	pass := ""

	if !cfg.APIToken.IsNull() {
		token = cfg.APIToken.ValueString()
	}
	if !cfg.Username.IsNull() {
		user = cfg.Username.ValueString()
	}
	if !cfg.Password.IsNull() {
		pass = cfg.Password.ValueString()
	}

	if token != "" {
		if user != "" || pass != "" {
			diags.AddWarning("Authentification mixte", "`api_token` est prioritaire sur `username/password`.")
		}
		return zabbix.Auth{Method: zabbix.AuthToken, Token: token}, diags
	}

	if user == "" || pass == "" {
		diags.AddError(
			"Authentification invalide",
			"Renseigne soit `api_token`, soit `username` et `password`.",
		)
		return zabbix.Auth{}, diags
	}
	return zabbix.Auth{Method: zabbix.AuthUserPassword, Username: user, Password: pass}, diags
}

func (p *zabbixProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

func (p *zabbixProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewHostResource,
		NewHostGroupResource,
		NewTemplateResource,
		NewTriggerResource,
	}
}
