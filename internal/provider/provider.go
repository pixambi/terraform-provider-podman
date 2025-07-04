package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ provider.Provider = &podmanProvider{}
)

type podmanProviderModel struct {
	Connection types.String `tfsdk:"connection"`
	Identity   types.String `tfsdk:"identity"`
	Host       types.String `tfsdk:"host"`
	Username   types.String `tfsdk:"username"`
	URI        types.String `tfsdk:"uri"`
	SocketPath types.String `tfsdk:"socket_path"`
}

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &podmanProvider{
			version: version,
		}
	}
}

// hashicupsProvider is the provider implementation.
type podmanProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// Metadata returns the provider type name.
func (p *podmanProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "podman"
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data.
func (p *podmanProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"connection": schema.StringAttribute{
				Optional:    true,
				Description: "Name of the Podman connection to use. If not specified, uses the default connection or local socket.",
			},
			"identity": schema.StringAttribute{
				Optional:    true,
				Description: "Path to SSH private key file for remote connections (e.g., ~/.ssh/id_ed25519).",
			},
			"host": schema.StringAttribute{
				Optional:    true,
				Description: "Remote host for SSH connection (e.g., 192.168.122.1). Used when creating ad-hoc connections.",
			},
			"username": schema.StringAttribute{
				Optional:    true,
				Description: "Username for SSH connection to remote Podman host.",
			},
			"uri": schema.StringAttribute{
				Optional:    true,
				Description: "Full URI for Podman connection (e.g., ssh://user@host/run/user/1000/podman/podman.sock).",
			},
			"socket_path": schema.StringAttribute{
				Optional:    true,
				Description: "Path to Podman socket. Defaults to /run/user/${UID}/podman/podman.sock for rootless or /run/podman/podman.sock for root.",
			},
		},
	}
}

// Configure prepares a HashiCups API client for data sources and resources.
func (p *podmanProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {

	var config podmanProviderModel

	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	if config.Connection.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("connection"),
			"Unknown Connection",
			"The provider cannot determine the connection to use. Please specify a valid connection.",
		)
		return
	}

	if config.Identity.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("identity"),
			"Unknown Identity",
			"The provider cannot determine the identity to use. Please specify a valid identity.",
		)
		return
	}

	if config.Host.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Unknown Host",
			"The provider cannot determine the host to use. Please specify a valid host.",
		)
		return
	}

	if config.Username.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("username"),
			"Unknown Username",
			"The provider cannot determine the username to use. Please specify a valid username.",
		)
		return
	}

	if config.URI.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("uri"),
			"Unknown URI",
			"The provider cannot determine the URI to use. Please specify a valid URI.",
		)
		return
	}

	if config.SocketPath.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("socket_path"),
			"Unknown Socket Path",
			"The provider cannot determine the socket path to use. Please specify a valid socket path.",
		)
		return
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Default values to environment variables, but override
	// with Terraform configuration value if set.
	connection := os.Getenv("PODMAN_CONNECTION")
	identity := os.Getenv("PODMAN_IDENTITY")
	host := os.Getenv("PODMAN_HOST")
	username := os.Getenv("PODMAN_USERNAME")
	uri := os.Getenv("PODMAN_URI")
	socketPath := os.Getenv("PODMAN_SOCKET_PATH")

	if !config.Connection.IsNull() {
		connection = config.Connection.ValueString()
	}

	if !config.Identity.IsNull() {
		identity = config.Identity.ValueString()
	}

	if !config.Host.IsNull() {
		host = config.Host.ValueString()
	}

	if !config.Username.IsNull() {
		username = config.Username.ValueString()
	}

	if !config.URI.IsNull() {
		uri = config.URI.ValueString()
	}

	if !config.SocketPath.IsNull() {
		socketPath = config.SocketPath.ValueString()
	}

	var podmanCtx context.Context
	var err error

	switch {

	//Direct URI connection
	case uri != "":
		podmanCtx, err = bindings.NewConnection(ctx, uri)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to Create Podman Connection from URI",
				fmt.Sprintf("Failed to connect using URI '%s': %s", uri, err.Error()),
			)
			return
		}

	//Named connection
	case connection != "":
		connectionURI, err := getPodmanConnectionURI(connection)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to Resolve Named Connection",
				fmt.Sprintf("Failed to resolve connection '%s': %s", connection, err.Error()),
			)
			return
		}
		podmanCtx, err = bindings.NewConnection(ctx, connectionURI)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to Create Podman Connection",
				fmt.Sprintf("Failed to connect using named connection '%s': %s", connection, err.Error()),
			)
			return
		}

	//Ad-hoc remote connection
	case host != "" && username != "":
		if identity == "" {
			resp.Diagnostics.AddError(
				"Missing SSH Identity for Remote Connection",
				"When using host and username for remote connection, an SSH identity (private key) must be provided.",
			)
			return
		}

		// Default socket path for remote if not specified
		if socketPath == "" {
			socketPath = "/run/user/1000/podman/podman.sock"
		}

		// Build SSH URI for remote connection
		remoteURI := fmt.Sprintf("ssh://%s@%s%s", username, host, socketPath)
		if identity != "" {

		}

		podmanCtx, err = bindings.NewConnection(ctx, remoteURI)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to Create Remote Podman Connection",
				fmt.Sprintf("Failed to connect to %s@%s: %s", username, host, err.Error()),
			)
			return
		}

	default:
		// Default to local connection
		if socketPath == "" {
			socketPath, err = getDefaultSocketPath()
			if err != nil {
				resp.Diagnostics.AddError(
					"Unable to Determine Default Socket Path",
					fmt.Sprintf("Failed to determine default socket path: %s", err.Error()),
				)
				return
			}
		}

		socketURI := fmt.Sprintf("unix://%s", socketPath)
		podmanCtx, err = bindings.NewConnection(ctx, socketURI)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to Create Local Podman Connection",
				fmt.Sprintf("Failed to connect to local socket '%s': %s", socketPath, err.Error()),
			)
			return
		}
	}

	resp.DataSourceData = podmanCtx
	resp.ResourceData = podmanCtx

}

// DataSources defines the data sources implemented in the provider.
func (p *podmanProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

// Resources defines the resources implemented in the provider.
func (p *podmanProvider) Resources(_ context.Context) []func() resource.Resource {
	return nil
}
