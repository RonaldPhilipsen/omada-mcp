# omada-mcp

omada-mcp is an MCP server that exposes the TP-Link Omada OpenAPI. An AI client uses the server to inspect and manage an Omada controller. The server is a Go program that uses the official MCP Go SDK. It wraps the client in the [github.com/RonaldPhilipsen/omada](https://github.com/RonaldPhilipsen/omada) module. The project ships a container image to [GHCR](https://ghcr.io/ronaldphilipsen/omada-mcp) for use with [ToolHive](https://github.com/stacklok/toolhive) and other MCP clients.

## Authentication

The server uses client credentials, an ID and a secret that identify the application. You create the application in the Omada controller. The server never handles usernames and passwords. It does not support the hosted Omada cloud mode.

## Configuration

All configuration uses OMADA_ environment variables, and the server reads no configuration files. The four required variables name the controller and the OpenAPI application. The optional variables set TLS, read-only mode, tool groups, dispatch, the transport, and the port.

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `OMADA_BASE_URL` | yes | | Controller address, for example `https://omada.example.com:8043` |
| `OMADA_OMADAC_ID` | yes | | Controller ID |
| `OMADA_CLIENT_ID` | yes | | OpenAPI application client ID |
| `OMADA_CLIENT_SECRET` | yes | | OpenAPI application client secret |
| `OMADA_INSECURE_TLS` | no | `false` | Set to `true` to accept a self-signed controller certificate |
| `OMADA_READ_ONLY` | no | `true` | Set to `false` to expose the write tools |
| `OMADA_TOOL_GROUPS` | no | all | Comma-separated list of tool groups to expose |
| `OMADA_ENABLE_DISPATCH` | no | `false` | Set to `true` to expose the dispatch tools |
| `OMADA_TRANSPORT` | no | `streamable-http` | `streamable-http` or `stdio` |
| `OMADA_PORT` | no | `8080` | Listen port for streamable HTTP |

Read-only mode hides every write tool from the client. The write tools appear only when `OMADA_READ_ONLY` has the exact value `false`. Do not set `OMADA_READ_ONLY=false` on a controller that you do not control. With write tools, a client can block clients, delete networks, and reboot devices.

## Tools

A curated tool is a tool that wraps one named operation from the Omada client. The table lists every tool group and its tools. The label (write) marks a tool that changes controller state.

| Group | Tools |
|---|---|
| `site` | `omada_site_list`, `omada_site_get`, `omada_site_create` (write), `omada_site_delete` (write) |
| `client` | `omada_client_list`, `omada_client_get`, `omada_client_block` (write), `omada_client_unblock` (write), `omada_client_reconnect` (write), `omada_client_ip_setting_modify` (write) |
| `device` | `omada_device_list`, `omada_devices_by_site`, `omada_device_reboot` (write), `omada_topology_get` |
| `switch` | `omada_switch_ports_overview`, `omada_lan_profiles_list`, `omada_switch_port_modify` (write), `omada_switch_port_enable` (write) |
| `lan` | `omada_lan_networks_list`, `omada_lan_network_get`, `omada_lan_network_create` (write), `omada_lan_network_modify` (write), `omada_lan_network_delete` (write), `omada_dhcp_list` |
| `vpn` | `omada_vpn_list`, `omada_vpn_s2s_list`, `omada_vpn_c2s_servers_list`, `omada_vpn_s2s_modify` (write) |
| `insight` | `omada_insight_overview`, `omada_insight_unresolved_count`, `omada_insight_incidents` |
| `log` | `omada_log_alerts`, `omada_log_events` |
| `ap` | `omada_ap_radio_config_get`, `omada_ap_radio_config_modify` (write), `omada_ap_radios_get`, `omada_ap_available_channel_get`, `omada_ap_ports_list`, `omada_ap_port_modify` (write), `omada_ap_port_vlans_list` |
| `wireless` | `omada_wireless_wlans_list`, `omada_wireless_ssids_list`, `omada_wireless_ssid_get`, `omada_wireless_ssid_band_steer_modify` (write) |
| `security` | `omada_acl_gateway_list`, `omada_acl_gateway_create` (write), `omada_acl_gateway_modify` (write), `omada_acl_ap_list`, `omada_acl_ap_create` (write), `omada_acl_ap_modify` (write), `omada_acl_switch_list`, `omada_acl_switch_create` (write), `omada_acl_switch_modify` (write), `omada_acl_delete` (write), `omada_acl_modify_index` (write), `omada_mac_filter_allow_list`, `omada_mac_filter_deny_list`, `omada_mac_filter_add` (write), `omada_mac_filter_remove` (write), `omada_ssid_mac_filter_modify` (write), `omada_group_profile_list`, `omada_group_profile_create` (write), `omada_group_profile_modify` (write), `omada_group_profile_delete` (write) |
| `bonjour` | `omada_bonjour_service_list`, `omada_bonjour_service_create` (write), `omada_bonjour_service_modify` (write), `omada_bonjour_service_delete` (write), `omada_bonjour_rule_list`, `omada_bonjour_rule_create` (write), `omada_bonjour_rule_modify` (write), `omada_bonjour_rule_delete` (write) |

The server hides the write tools in read-only mode. Set `OMADA_TOOL_GROUPS` to expose only the groups that your client needs.

## Dispatch

Dispatch is a pair of tools that call any operation in the Omada spec. You enable it with `OMADA_ENABLE_DISPATCH=true`. The tools read an operation index, a list of all operations in the Omada spec.

The tool `ops_lookup` searches the index by keyword or tag. The tool `ops_call` runs one operation by its ID. In read-only mode, `ops_call` rejects operations that change state.

## Transport

The default transport is streamable HTTP. The server serves the MCP endpoint at `/mcp` on port 8080. Set `OMADA_TRANSPORT=stdio` to speak MCP over stdin and stdout. The stdio transport rejects `OMADA_PORT`. The server writes logs to stderr and keeps stdout free for the stdio transport.

## Running the server

Run the image from GHCR. The example starts the server in read-only mode on port 8080.

```bash
docker run --rm -p 8080:8080 \
  -e OMADA_BASE_URL=https://omada.example.com:8043 \
  -e OMADA_OMADAC_ID=abc123 \
  -e OMADA_CLIENT_ID=your-client-id \
  -e OMADA_CLIENT_SECRET=your-client-secret \
  ghcr.io/ronaldphilipsen/omada-mcp:v0.0.2
```

If the controller uses a self-signed certificate, add `OMADA_INSECURE_TLS=true`. Point your MCP client at the `/mcp` endpoint on port 8080. If you run the server in ToolHive, register the same variables as ToolHive secrets.

## Building from source

You need Go 1.27.3. Run `go build ./cmd/omada-mcp` to build the binary. Run `go test ./...` to run the test suite.

## Development

Install the toolchain with mise. The project pins Go 1.27.3 and prek in mise.toml. The command `mise install` also installs the pre-commit hooks.

Regenerate the operation index with `make gen`. The generator downloads the Omada spec and compares its SHA-256 with the value in gen/data/spec.sha256. If the values differ, the generator fails.

## Releases

GitHub Actions creates a release when new commits on main produce a new version. The workflow computes the version from the commit history with semVersie. It builds the image for linux/amd64 and linux/arm64. It pushes only the version tag to GHCR, without a latest tag. It creates a GitHub release with the release notes.
