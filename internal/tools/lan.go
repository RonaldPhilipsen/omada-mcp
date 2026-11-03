package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/RonaldPhilipsen/omada/cloud/lan"

	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
	"github.com/RonaldPhilipsen/omada-mcp/internal/omadaclient"
)

func (b *Base) LanNetworksList() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_lan_networks_list",
		Description: "List every LAN network in an Omada site.",
		Group:       "lan",
		Register: func(t *mcp.Tool) {
			t.InputSchema = siteOnlySchema()
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[siteOnlyIn](args)
			if err != nil {
				return nil, err
			}
			if in.SiteID == "" {
				return nil, fmt.Errorf("site_id is required")
			}
			omadacID, err := b.ResolveOmac(in.OmacIDArg)
			if err != nil {
				return nil, err
			}
			resp, err := b.Omc.CloudAPI().Lan().GetAllLanNetworks(ctx, omadacID, in.SiteID)
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

type networkIDIn struct {
	OmacIDArg
	SiteID    string `json:"site_id"`
	NetworkID string `json:"network_id"`
}

func networkIDSchema() map[string]any {
	return objectSchema(map[string]any{
		"omadac_id":  prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
		"site_id":    prop("string", "The site ID the network belongs to (required)."),
		"network_id": prop("string", "The LAN network ID to act on (required)."),
	}, "site_id", "network_id")
}

// validateNetworkID validates the site_id and network_id required args
// before any HTTP.
func validateNetworkID(in *networkIDIn) error {
	if in.SiteID == "" {
		return fmt.Errorf("site_id is required")
	}
	if in.NetworkID == "" {
		return fmt.Errorf("network_id is required")
	}
	return nil
}

func (b *Base) LanNetworkGet() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_lan_network_get",
		Description: "Get one Omada LAN network's full entity by site and network ID.",
		Group:       "lan",
		Register: func(t *mcp.Tool) {
			t.InputSchema = networkIDSchema()
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[networkIDIn](args)
			if err != nil {
				return nil, err
			}
			if err := validateNetworkID(in); err != nil {
				return nil, err
			}
			omadacID, err := b.ResolveOmac(in.OmacIDArg)
			if err != nil {
				return nil, err
			}
			resp, err := b.Omc.CloudAPI().Lan().GetLanNetwork(ctx, omadacID, in.SiteID, in.NetworkID)
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

// lanNetworkCreateIn's body mirrors the Omada request type
// lan.LanNetworkOpenApiVO (the generated lan.CreateLanNetworkJSONRequestBody).
type lanNetworkCreateIn struct {
	OmacIDArg
	SiteID string                  `json:"site_id"`
	Body   lan.LanNetworkOpenApiVO `json:"body"`
}

// LanNetworkCreate returns the omada_lan_network_create tool. The body
// mirrors the Omada request type; name and purpose are the required
// minimum set.
func (b *Base) LanNetworkCreate() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_lan_network_create",
		Description: "Create a new Omada LAN network. body mirrors the Omada request type lan.LanNetworkOpenApiVO; name and purpose are the required minimum set.",
		Group:       "lan",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID to create the network in (required)."),
				"body":      prop("object", "LAN network entity; mirrors the Omada request type lan.LanNetworkOpenApiVO."),
			}, "site_id", "body")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[lanNetworkCreateIn](args)
			if err != nil {
				return nil, err
			}
			if in.SiteID == "" {
				return nil, fmt.Errorf("site_id is required")
			}
			omadacID, err := b.ResolveOmac(in.OmacIDArg)
			if err != nil {
				return nil, err
			}
			resp, err := b.Omc.CloudAPI().Lan().CreateLanNetwork(ctx, omadacID, in.SiteID, in.Body)
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

func (b *Base) LanNetworkDelete() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_lan_network_delete",
		Description: "Delete an Omada LAN network by site and network ID.",
		Group:       "lan",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = networkIDSchema()
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[networkIDIn](args)
			if err != nil {
				return nil, err
			}
			if err := validateNetworkID(in); err != nil {
				return nil, err
			}
			omadacID, err := b.ResolveOmac(in.OmacIDArg)
			if err != nil {
				return nil, err
			}
			resp, err := b.Omc.CloudAPI().Lan().DeleteLanNetwork(ctx, omadacID, in.SiteID, in.NetworkID)
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

// dhcpListIn's page defaults to 1 and page_size to 20.
type dhcpListIn struct {
	OmacIDArg
	SiteID   string `json:"site_id"`
	Page     int    `json:"page,omitempty"`
	PageSize int    `json:"page_size,omitempty"`
}

// DhcpList returns the omada_dhcp_list tool. page defaults to 1 and
// page_size to 20.
func (b *Base) DhcpList() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_dhcp_list",
		Description: "List DHCP reservations in an Omada site with paging. page starts at 1 (default 1); page_size is the number of entries per page (default 20).",
		Group:       "lan",
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID to list DHCP reservations for (required)."),
				"page":      prop("integer", "Page number, starting at 1 (default 1)."),
				"page_size": prop("integer", "Entries per page (default 20)."),
			}, "site_id")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[dhcpListIn](args)
			if err != nil {
				return nil, err
			}
			if in.SiteID == "" {
				return nil, fmt.Errorf("site_id is required")
			}
			if in.Page == 0 {
				in.Page = 1
			}
			if in.PageSize == 0 {
				in.PageSize = 20
			}
			omadacID, err := b.ResolveOmac(in.OmacIDArg)
			if err != nil {
				return nil, err
			}
			resp, err := b.Omc.CloudAPI().Lan().GetDhcpReservationGrid(ctx, omadacID, in.SiteID, &lan.GetDhcpReservationGridParams{
				Page:     int32(in.Page),
				PageSize: int32(in.PageSize),
			})
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
