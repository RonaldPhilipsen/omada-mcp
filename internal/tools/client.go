package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/RonaldPhilipsen/omada/cloud/client"

	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
	"github.com/RonaldPhilipsen/omada-mcp/internal/omadaclient"
)

// clientListIn is the omada_client_list argument object. The body mirrors
// the Omada query type client.ClientQueryDataOpenApiVO (the generated
// client.GetGridAllClientsJSONRequestBody); page and page_size default to
// 1 and 20.
type clientListIn struct {
	OmacIDArg
	SiteID string                          `json:"site_id"`
	Body   client.ClientQueryDataOpenApiVO `json:"body,omitempty"`
}

// ClientList returns the omada_client_list tool: the paged grid of all
// clients in a site. page defaults to 1 and page_size to 20.
func (b *Base) ClientList() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_client_list",
		Description: "List all clients in an Omada site with paging. page starts at 1 (default 1); page_size is the number of entries per page (default 20).",
		Group:       "client",
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID to list clients for (required)."),
				"body":      prop("object", "Client query; mirrors the Omada request type client.ClientQueryDataOpenApiVO (page, page_size, scope, searchKey, filters, sorts)."),
			}, "site_id")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[clientListIn](args)
			if err != nil {
				return nil, err
			}
			if in.SiteID == "" {
				return nil, fmt.Errorf("site_id is required")
			}
			if in.Body.Page == 0 {
				in.Body.Page = 1
			}
			if in.Body.PageSize == 0 {
				in.Body.PageSize = 20
			}
			omadacID, err := b.ResolveOmac(in.OmacIDArg)
			if err != nil {
				return nil, err
			}
			resp, err := b.Omc.CloudAPI().Clients().GetGridAllClients(ctx, omadacID, in.SiteID, in.Body)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			var result json.RawMessage
			if err := omadaclient.Envelope(resp, &result); err != nil {
				return nil, err
			}
			return mcpkit.JSONResult(resultPayload(result))
		},
	}
}

// clientMacIn is the argument object shared by the per-client-mac client
// tools (get, block, unblock, reconnect).
type clientMacIn struct {
	OmacIDArg
	SiteID    string `json:"site_id"`
	ClientMac string `json:"client_mac"`
}

// clientMacSchema is the input schema shared by the per-client-mac client
// tools.
func clientMacSchema() map[string]any {
	return objectSchema(map[string]any{
		"omadac_id":  prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
		"site_id":    prop("string", "The site ID the client belongs to (required)."),
		"client_mac": prop("string", "The client MAC address to act on (required)."),
	}, "site_id", "client_mac")
}

// validateClientMac validates the per-client-mac required args before any
// HTTP.
func validateClientMac(in *clientMacIn) error {
	if in.SiteID == "" {
		return fmt.Errorf("site_id is required")
	}
	if in.ClientMac == "" {
		return fmt.Errorf("client_mac is required")
	}
	return nil
}

// ClientGet returns the omada_client_get tool: one client's full entity by
// MAC address.
func (b *Base) ClientGet() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_client_get",
		Description: "Get one Omada client's full entity by site and client MAC address.",
		Group:       "client",
		Register: func(t *mcp.Tool) {
			t.InputSchema = clientMacSchema()
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[clientMacIn](args)
			if err != nil {
				return nil, err
			}
			if err := validateClientMac(in); err != nil {
				return nil, err
			}
			omadacID, err := b.ResolveOmac(in.OmacIDArg)
			if err != nil {
				return nil, err
			}
			resp, err := b.Omc.CloudAPI().Clients().GetClientDetail(ctx, omadacID, in.SiteID, in.ClientMac)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			var result json.RawMessage
			if err := omadaclient.Envelope(resp, &result); err != nil {
				return nil, err
			}
			return mcpkit.JSONResult(resultPayload(result))
		},
	}
}

// ClientBlock returns the omada_client_block tool: block a client by MAC
// address in a site.
func (b *Base) ClientBlock() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_client_block",
		Description: "Block an Omada client by site and client MAC address.",
		Group:       "client",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = clientMacSchema()
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[clientMacIn](args)
			if err != nil {
				return nil, err
			}
			if err := validateClientMac(in); err != nil {
				return nil, err
			}
			omadacID, err := b.ResolveOmac(in.OmacIDArg)
			if err != nil {
				return nil, err
			}
			resp, err := b.Omc.CloudAPI().Clients().BlockClient(ctx, omadacID, in.SiteID, in.ClientMac)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			var result json.RawMessage
			if err := omadaclient.Envelope(resp, &result); err != nil {
				return nil, err
			}
			return mcpkit.JSONResult(resultPayload(result))
		},
	}
}

// ClientUnblock returns the omada_client_unblock tool: unblock a client by
// MAC address in a site.
func (b *Base) ClientUnblock() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_client_unblock",
		Description: "Unblock an Omada client by site and client MAC address.",
		Group:       "client",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = clientMacSchema()
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[clientMacIn](args)
			if err != nil {
				return nil, err
			}
			if err := validateClientMac(in); err != nil {
				return nil, err
			}
			omadacID, err := b.ResolveOmac(in.OmacIDArg)
			if err != nil {
				return nil, err
			}
			resp, err := b.Omc.CloudAPI().Clients().UnblockClient(ctx, omadacID, in.SiteID, in.ClientMac)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			var result json.RawMessage
			if err := omadaclient.Envelope(resp, &result); err != nil {
				return nil, err
			}
			return mcpkit.JSONResult(resultPayload(result))
		},
	}
}

// ClientReconnect returns the omada_client_reconnect tool: reconnect a
// client by MAC address in a site.
func (b *Base) ClientReconnect() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_client_reconnect",
		Description: "Reconnect an Omada client by site and client MAC address.",
		Group:       "client",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = clientMacSchema()
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[clientMacIn](args)
			if err != nil {
				return nil, err
			}
			if err := validateClientMac(in); err != nil {
				return nil, err
			}
			omadacID, err := b.ResolveOmac(in.OmacIDArg)
			if err != nil {
				return nil, err
			}
			resp, err := b.Omc.CloudAPI().Clients().ReconnectClient(ctx, omadacID, in.SiteID, in.ClientMac)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			var result json.RawMessage
			if err := omadaclient.Envelope(resp, &result); err != nil {
				return nil, err
			}
			return mcpkit.JSONResult(resultPayload(result))
		},
	}
}
