# omada-mcp — Design

A configurable MCP (Model Context Protocol) server exposing the TP-Link Omada
OpenAPI for use with [ToolHive](https://github.com/stacklok/toolhive), shipped
as a container image to ghcr.

Companion to the `github.com/RonaldPhilipsen/omada` Go module, which provides
the client packages this server wraps.

## Intent and why

Operators run Omada controllers and want an AI agent (via MCP-capable clients
such as Claude, Keeb, or other ToolHive-registered client stacks) to inspect
and manage their network: sites, clients, devices, VLANs, VPN, alerts, logs.
The `omada` module already speaks the API; this project is the MCP surface in
front of it, packaged so ToolHive can deploy, configure, and audit it.

## Goals, non-goals, anti-goals

**Goals**

- One server container, fully proxyless — no vendor SDKs at runtime.
- All configuration via environment variables.
- Read-only by default; writes require explicitly opting out.
- Work with ToolHive out of the box: streamable HTTP transport, container
  image on ghcr, secrets passed as env vars.
- Client-credential authentication only (Omada OpenAPI style).

**Non-goals**

- Hosted Omada cloud mode (username/password token at `/v1/token`).
- The hand-written `omada/controller` package (username/password session
  login).
- Multi-tenant seat-key management, TLS termination, or custom auth layers in
  the server (ToolHive handles upstream authn/authz).

**Anti-goals — would make it a failure even if it worked**

- Registering thousands of tools directly into the MCP client context.
- Performing destructive device actions (rebooting, blocking, resetting,
  reconfiguring VPNs) without the operator having explicitly enabled writes.

## Constraints

- Credentials are exactly `CLIENT_ID` + `CLIENT_SECRET` as the Omada OpenAPI
  defines; the server never handles usernames/passwords.
- Authentication is the on-prem northbound client-credentials flow
  (`POST /openapi/authorize/token?grant_type=client_credentials`) that
  `cloud.New(cloud.Options{BaseURL, OmadacID, ClientID, ClientSecret})` in the
  `omada` module already implements.
- The server depends on `github.com/RonaldPhilipsen/omada` as a module
  dependency; no code vendoring. Renovate keeps that dependency current.

## Architecture

Single Go binary built with the official
`github.com/modelcontextprotocol/go-sdk` MCP SDK.

There is one backend: the generated northbound client from the `omada`
module, pointed at an on-prem controller (`OMADA_BASE_URL` *is* the
controller address). The choice to go "Both, from the start" for API surface
is realized by the *tool groups* below covering controller-relevant domains
(site, client, device, lan, vpn, insight) alongside cloud broad domains from
the same client; since the generated client targets the same controller,
there is no second backend to wire up. This collapse from an earlier sketch
(two backends) to one was the user's decision — credentials are
client-credential only, so the legacy `controller/` package has no auth path
and is intentionally not used.

### Tool surface

Two strategies live side by side:

1. **Curated tool groups.** A hand-written set of ~35–45 MCP tools that wrap
   named operations from the generated packages, made resilient and
   ergonomic (defaults, envelope decoding, error mapping to MCP tool
   errors).

   | Group | Read tools | Write tools (only when not read-only) |
   |---|---|---|
   | `site` | list/get sites | update site settings |
   | `client` | list/get clients | block, unblock, reconnect |
   | `device` | list/get devices, radios, ports, LED state | LED, reboot, firmware, reset |
   | `lan` | networks/VLANs, DHCP reservations | create/delete network |
   | `vpn` | policies + status | policy changes |
   | `insight` | alerts, incidents | — |
   | `log` | query omada logs | — |

2. **Generic dispatch escape hatch** (behind `OMADA_ENABLE_DISPATCH=true`):
   Two tools:
   - `ops_lookup(keyword | tag)` — search the operation index.
   - `ops_call(operation_id, params)` — issue the operation via the same
     client-credential token transport, using a small checked-in index of
     `{operationId → tag, summary, method, path, params}`.

   The index is generated from the northbound spec: generation fetches the
   spec live, asserts its SHA-256 equals `omada`'s pinned `gen/spec.sha256`,
   and fails loudly on mismatch, so drift cannot sneak in invisibly.
   Regeneration follows the same `make -C gen gen` pattern, no `omada`
   module change needed for this.

   **Read-only gates dispatch too.** `ops_call` reaches every indexed
   operation, including destructive ones — so it is subject to
   `OMADA_READ_ONLY` exactly like curated tools: when read-only, only
   non-mutating operations are available through dispatch (the index
   generation classifies each operation's mutability alongside
   method/path/params: GET and body-carrying search-style reads vs actual
   mutations — the precise classification is a builder call).

### Configuration

All env, no config files. Prefix `OMADA_`.

| Var | Required? | Meaning |
|---|---|---|
| `OMADA_BASE_URL` | yes | Controller address, e.g. `https://omada.example.com:8043` |
| `OMADA_CLIENT_ID` | yes | OpenAPI application client ID |
| `OMADA_CLIENT_SECRET` | yes | OpenAPI application client secret |
| `OMADA_OMADAC_ID` | yes | Controller ID; `cloud.New` on-prem mode requires it non-empty |
| `OMADA_INSECURE_TLS` | no, defaults `false` | Maps to `cloud.Options.InsecureTLS` — stock on-prem controllers serve a self-signed certificate (`CN=localhost`), so self-hosted deployments typically need `true` here; opt-in like `READ_ONLY` |
| `OMADA_READ_ONLY` | no, defaults `true` | `false` explicitly required to unlock write tools; anything else stays read-only |
| `OMADA_TOOL_GROUPS` | no | Comma-separated allowlist; default all groups |
| `OMADA_ENABLE_DISPATCH` | no, defaults `false` | Turn on `ops_lookup`/`ops_call` |
| `OMADA_TRANSPORT` | no, defaults `streamable-http` | `streamable-http` or `stdio` |
| `OMADA_PORT` | no, defaults `8080` | Listen port for streamable-http |

Read-only gating happens at *registration* time: write tools are not even
listed to the client when `OMADA_READ_ONLY != "false"` (so a hallucinating
agent can't try to call them and get an error — they don't exist).

### Dependencies

- `github.com/modelcontextprotocol/go-sdk` — the official MCP SDK.
- `github.com/RonaldPhilipsen/omada` `v0.1.0`+ — the generated northbound
  client and its auth transport.
- If a needed helper is unexported in `omada` (e.g. exposing the authed
  `*http.Client` for `ops_call`), the fix is a tiny addition to the `omada`
  module and a tag bump there, not reimplementing auth here.

### Packaging

- Multi-stage Dockerfile: build stage compiles a static binary
  (`CGO_ENABLED=0`); final stage is minimal (`gcr.io/distroless/static` or
  `FROM scratch`).
- GitHub Actions workflow on tag pushes `ghcr.io/ronaldphilipsen/omada-mcp`
  with the semantic version tag and `latest`, multi-arch (`linux/amd64`,
  `linux/arm64`).
- ToolHive users run it with something like:
  `thv run transport=streamable-http --port ... ghcr.io/ronaldphilipsen/omada-mcp`
  passing the `OMADA_*` env vars as ToolHive secrets/env.

## What's left to the builder

- Exact curated-tool inventory and signatures per group (the table above is
  intent, not a complete list).
- The precise fuzz-free decoding of envelope shapes in curated tools
  (some spec operations have bare payloads, not envelopes).
- Choice between `scratch` and `distroless` final stage.
- Index format details for `ops_lookup`/`ops_call`.
- Test strategy details — httptest fixtures mirroring the `omada` module's
  test conventions are a strong prior.
