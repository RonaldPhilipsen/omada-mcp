# omada-mcp

omada-mcp is an MCP server that exposes the TP-Link Omada OpenAPI. An AI client uses the server to inspect and manage an Omada controller. The server is written in Go and built with the official MCP Go SDK. It wraps the client in the [github.com/RonaldPhilipsen/omada](https://github.com/RonaldPhilipsen/omada) module. The project ships a container image to [GHCR](https://ghcr.io/ronaldphilipsen/omada-mcp) for use with [ToolHive](https://github.com/stacklok/toolhive) and other MCP clients.

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

Read-only mode hides every write tool from the client. Only the literal value `false` for `OMADA_READ_ONLY` unlocks the write tools. Do not set `OMADA_READ_ONLY=false` on a controller you do not control. The server can then block clients, delete networks, and reboot devices.

## Tools

A curated tool wraps one named operation from the Omada client. The server ships 35 curated tools in nine groups. The table lists every group and tool. The label (write) marks a tool that changes controller state.

| Group | Tools |
|---|---|
| `site` | `omada_site_list`, `omada_site_get`, `omada_site_create` (write), `omada_site_delete` (write) |
| `client` | `omada_client_list`, `omada_client_get`, `omada_client_block` (write), `omada_client_unblock` (write), `omada_client_reconnect` (write) |
| `device` | `omada_device_list`, `omada_devices_by_site`, `omada_device_reboot` (write), `omada_topology_get` |
| `lan` | `omada_lan_networks_list`, `omada_lan_network_get`, `omada_lan_network_create` (write), `omada_lan_network_delete` (write), `omada_dhcp_list` |
| `vpn` | `omada_vpn_list`, `omada_vpn_s2s_list`, `omada_vpn_c2s_servers_list`, `omada_vpn_s2s_modify` (write) |
| `insight` | `omada_insight_overview`, `omada_insight_unresolved_count`, `omada_insight_incidents` |
| `log` | `omada_log_alerts`, `omada_log_events` |
| `ap` | `omada_ap_radio_config_get`, `omada_ap_radio_config_modify` (write), `omada_ap_radios_get`, `omada_ap_available_channel_get` |
| `wireless` | `omada_wireless_wlans_list`, `omada_wireless_ssids_list`, `omada_wireless_ssid_get`, `omada_wireless_ssid_band_steer_modify` (write) |

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

Add `OMADA_INSECURE_TLS=true` when the controller uses a self-signed certificate. Point your MCP client at the `/mcp` endpoint on port 8080. Register the same variables as ToolHive secrets when you run the server in ToolHive.

## Building from source

You need Go 1.27.3. Run `go build ./cmd/omada-mcp` to build the binary. Run `go test ./...` to run the test suite.

## Development

Install the toolchain with mise. The project pins Go 1.27.3 and prek in mise.toml. The command `mise install` also installs the pre-commit hooks.

Regenerate the operation index with `make gen`. The generator fetches the Omada spec and checks its SHA-256 against gen/data/spec.sha256. The generator fails when the values differ.

## Releases

GitHub Actions creates a release when new commits on main produce a new version. The workflow computes the version from the commit history with semVersie. It builds the image for linux/amd64 and linux/arm64. It pushes only the version tag to GHCR, without a latest tag. It creates a GitHub release with the release notes.
