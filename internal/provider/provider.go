package provider

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &NxsAnomalyProvider{}

type NxsAnomalyProvider struct {
	version string
}

type providerModel struct {
	URL                   types.String `tfsdk:"url"`
	APIKey                types.String `tfsdk:"api_key"`
	TLSInsecureSkipVerify types.Bool   `tfsdk:"tls_insecure_skip_verify"`
	RequestTimeout        types.Int64  `tfsdk:"request_timeout"`
	MaxRetries            types.Int64  `tfsdk:"max_retries"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &NxsAnomalyProvider{version: version}
	}
}

func (p *NxsAnomalyProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "anomaly"
	resp.Version = p.version
}

func (p *NxsAnomalyProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for nxs-anomaly alerting and on-call management.",
		Attributes: map[string]schema.Attribute{
			"url": schema.StringAttribute{
				Optional:    true,
				Description: "Base URL of the nxs-anomaly API (e.g. http://localhost:8080). Env: NXS_ANOMALY_URL.",
			},
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "API key for authentication. Env: NXS_ANOMALY_API_KEY.",
			},
			"tls_insecure_skip_verify": schema.BoolAttribute{
				Optional:    true,
				Description: "Disable TLS certificate verification. Env: NXS_ANOMALY_TLS_INSECURE.",
			},
			"request_timeout": schema.Int64Attribute{
				Optional:    true,
				Description: "HTTP request timeout in seconds (default 30). Env: NXS_ANOMALY_REQUEST_TIMEOUT.",
			},
			"max_retries": schema.Int64Attribute{
				Optional:    true,
				Description: "Number of retries on 502/503/504 responses (default 0). Env: NXS_ANOMALY_MAX_RETRIES.",
			},
		},
	}
}

func (p *NxsAnomalyProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := os.Getenv("NXS_ANOMALY_URL")
	if !cfg.URL.IsNull() && !cfg.URL.IsUnknown() {
		url = cfg.URL.ValueString()
	}
	if url == "" {
		url = "http://localhost:8080"
	}

	apiKey := os.Getenv("NXS_ANOMALY_API_KEY")
	if !cfg.APIKey.IsNull() && !cfg.APIKey.IsUnknown() {
		apiKey = cfg.APIKey.ValueString()
	}

	tlsInsecure := envBool("NXS_ANOMALY_TLS_INSECURE", false)
	if !cfg.TLSInsecureSkipVerify.IsNull() && !cfg.TLSInsecureSkipVerify.IsUnknown() {
		tlsInsecure = cfg.TLSInsecureSkipVerify.ValueBool()
	}

	timeout := envInt64("NXS_ANOMALY_REQUEST_TIMEOUT", 30)
	if !cfg.RequestTimeout.IsNull() && !cfg.RequestTimeout.IsUnknown() {
		timeout = cfg.RequestTimeout.ValueInt64()
	}

	maxRetries := envInt64("NXS_ANOMALY_MAX_RETRIES", 0)
	if !cfg.MaxRetries.IsNull() && !cfg.MaxRetries.IsUnknown() {
		maxRetries = cfg.MaxRetries.ValueInt64()
	}

	httpClient := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: tlsInsecure, //nolint:gosec
			},
		},
	}

	c := newClientWithOptions(url, apiKey, httpClient, int(maxRetries))
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *NxsAnomalyProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewUserResource,
		NewTeamResource,
		NewScheduleResource,
		NewScheduleOverrideResource,
		NewEscalationChainResource,
		NewIntegrationResource,
		NewChatopsChannelResource,
		NewMaintenanceWindowResource,
	}
}

func (p *NxsAnomalyProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		// Single-item lookups
		NewUserDataSource,
		NewTeamDataSource,
		NewIntegrationDataSource,
		NewEscalationChainDataSource,
		NewScheduleDataSource,
		NewChatopsChannelDataSource,
		NewMaintenanceWindowDataSource,
		// List sources
		NewUsersDataSource,
		NewTeamsDataSource,
		NewIntegrationsDataSource,
		NewSchedulesDataSource,
		NewEscalationChainsDataSource,
		NewChatopsChannelsDataSource,
		NewMaintenanceWindowsDataSource,
		// Operational reports (read-only, no stored object behind them)
		NewOnCallDataSource,
		NewScheduleCoverageDataSource,
		NewSchedulePreviewDataSource,
		NewReadinessDataSource,
	}
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envInt64(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}
