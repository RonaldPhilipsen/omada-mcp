package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/RonaldPhilipsen/omada/cloud/vpn"

	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
	"github.com/RonaldPhilipsen/omada-mcp/internal/omadaclient"
)

func (b *Base) VpnList() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_vpn_list",
		Description: "List every VPN in an Omada site.",
		Group:       "vpn",
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
			resp, err := b.Omc.CloudAPI().Vpn().GetAllVpnList(ctx, omadacID, in.SiteID)
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

func (b *Base) VpnS2sList() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_vpn_s2s_list",
		Description: "List every site-to-site VPN in an Omada site.",
		Group:       "vpn",
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
			resp, err := b.Omc.CloudAPI().Vpn().GetSiteToSiteVpnList(ctx, omadacID, in.SiteID)
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

func (b *Base) VpnC2sServersList() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_vpn_c2s_servers_list",
		Description: "List every client-to-site VPN server in an Omada site.",
		Group:       "vpn",
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
			resp, err := b.Omc.CloudAPI().Vpn().GetClientToSiteVpnServerList(ctx, omadacID, in.SiteID)
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

// vpnS2sModifyIn's body mirrors the Omada request type vpn.SiteToSiteVpn
// (the generated vpn.ModifySiteToSiteVpnJSONRequestBody).
type vpnS2sModifyIn struct {
	OmacIDArg
	SiteID string            `json:"site_id"`
	VpnID  string            `json:"vpn_id"`
	Body   vpn.SiteToSiteVpn `json:"body"`
}

// VpnS2sModify returns the omada_vpn_s2s_modify tool. The body mirrors the
// Omada request type; name, siteVpnType, and status are the required
// minimum set.
func (b *Base) VpnS2sModify() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_vpn_s2s_modify",
		Description: "Modify a site-to-site Omada VPN by site and VPN ID. body mirrors the Omada request type vpn.SiteToSiteVpn; name, siteVpnType, and status are the required minimum set.",
		Group:       "vpn",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID the VPN belongs to (required)."),
				"vpn_id":    prop("string", "The site-to-site VPN ID to modify (required)."),
				"body":      prop("object", "Site-to-site VPN entity; mirrors the Omada request type vpn.SiteToSiteVpn."),
			}, "site_id", "vpn_id", "body")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[vpnS2sModifyIn](args)
			if err != nil {
				return nil, err
			}
			if in.SiteID == "" {
				return nil, fmt.Errorf("site_id is required")
			}
			if in.VpnID == "" {
				return nil, fmt.Errorf("vpn_id is required")
			}
			omadacID, err := b.ResolveOmac(in.OmacIDArg)
			if err != nil {
				return nil, err
			}
			resp, err := b.Omc.CloudAPI().Vpn().ModifySiteToSiteVpn(ctx, omadacID, in.SiteID, in.VpnID, in.Body)
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
