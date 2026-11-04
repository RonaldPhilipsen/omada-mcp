// Package tools is the omada-mcp MCP tool catalog. Each curated group
// (site, client, device, ...) is one file that exports one mcpkit.Tool
// constructor per operation, all built on the shared Base.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/RonaldPhilipsen/omada/cloud/wireless"

	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
	"github.com/RonaldPhilipsen/omada-mcp/internal/omadaclient"
)

func (b *Base) WirelessWlansList() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_wireless_wlans_list",
		Description: "List every WLAN in an Omada site. WLANs group SSIDs; the wlan_id is needed for omada_wireless_ssid_get.",
		Group:       "wireless",
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
			resp, err := b.Omc.CloudAPI().Wireless().GetWlanGroupList(ctx, omadacID, in.SiteID)
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

func (b *Base) WirelessSsidsList() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_wireless_ssids_list",
		Description: "List every SSID in an Omada site, grouped per WLAN, with the ssid_id values used by omada_wireless_ssid_get and omada_wireless_ssid_band_steer_modify.",
		Group:       "wireless",
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
			resp, err := b.Omc.CloudAPI().Wireless().GetSsidsBySite(ctx, omadacID, in.SiteID, nil)
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

type ssidIDIn struct {
	OmacIDArg
	SiteID string `json:"site_id"`
	WlanID string `json:"wlan_id"`
	SsidID string `json:"ssid_id"`
}

func ssidIDSchema() map[string]any {
	return objectSchema(map[string]any{
		"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
		"site_id":   prop("string", "The site ID the SSID belongs to (required)."),
		"wlan_id":   prop("string", "The WLAN ID the SSID belongs to, from omada_wireless_wlans_list (required)."),
		"ssid_id":   prop("string", "The SSID ID to act on, from omada_wireless_ssids_list (required)."),
	}, "site_id", "wlan_id", "ssid_id")
}

// validateSSIDID validates the site_id, wlan_id and ssid_id required args
// before any HTTP.
func validateSSIDID(in *ssidIDIn) error {
	if in.SiteID == "" {
		return fmt.Errorf("site_id is required")
	}
	if in.WlanID == "" {
		return fmt.Errorf("wlan_id is required")
	}
	if in.SsidID == "" {
		return fmt.Errorf("ssid_id is required")
	}
	return nil
}

func (b *Base) WirelessSsidGet() mcpkit.Tool {
	return mcpkit.Tool{
		Name: "omada_wireless_ssid_get",
		Description: "Get one Omada SSID's full entity by site, WLAN and SSID ID: security, band, VLAN, broadcast, " +
			"band-steer mode, 802.11r (fast roam) and rate-limit settings.",
		Group:    "wireless",
		Register: func(t *mcp.Tool) { t.InputSchema = ssidIDSchema() },
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[ssidIDIn](args)
			if err != nil {
				return nil, err
			}
			if err := validateSSIDID(in); err != nil {
				return nil, err
			}
			omadacID, err := b.ResolveOmac(in.OmacIDArg)
			if err != nil {
				return nil, err
			}
			resp, err := b.Omc.CloudAPI().Wireless().GetSsidDetail(ctx, omadacID, in.SiteID, in.WlanID, in.SsidID)
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

type ssidBandSteerIn struct {
	OmacIDArg
	SiteID string `json:"site_id"`
	SsidID string `json:"ssid_id"`
	Mode   int    `json:"mode"`
}

func ssidBandSteerSchema() map[string]any {
	return objectSchema(map[string]any{
		"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
		"site_id":   prop("string", "The site ID the SSID belongs to (required)."),
		"ssid_id":   prop("string", "The SSID ID to act on, from omada_wireless_ssids_list (required)."),
		"mode":      prop("integer", "Band-steer mode: 0=Disable, 1=Prefer 5GHz/6GHz, 2=Balance, 3=Use site setting (required)."),
	}, "site_id", "ssid_id", "mode")
}

// WirelessSsidBandSteerModify returns the omada_wireless_ssid_band_steer_modify
// tool: PATCH the band-steer mode of one SSID.
func (b *Base) WirelessSsidBandSteerModify() mcpkit.Tool {
	return mcpkit.Tool{
		Name: "omada_wireless_ssid_band_steer_modify",
		Description: "Modify the band-steer mode of an Omada SSID. mode: 0=Disable, 1=Prefer 5GHz/6GHz, 2=Balance, 3=Use site setting. " +
			"Band steering pushes capable clients off 2.4 GHz toward 5/6 GHz; pair it with non-overlapping 5 GHz channels per AP (omada_ap_radio_config_modify).",
		Group:    "wireless",
		Mutating: true,
		Register: func(t *mcp.Tool) { t.InputSchema = ssidBandSteerSchema() },
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[ssidBandSteerIn](args)
			if err != nil {
				return nil, err
			}
			if in.SiteID == "" {
				return nil, fmt.Errorf("site_id is required")
			}
			if in.SsidID == "" {
				return nil, fmt.Errorf("ssid_id is required")
			}
			if in.Mode < 0 || in.Mode > 3 {
				return nil, fmt.Errorf("mode must be 0 (disable), 1 (prefer 5/6 GHz), 2 (balance) or 3 (site setting); got %d", in.Mode)
			}
			omadacID, err := b.ResolveOmac(in.OmacIDArg)
			if err != nil {
				return nil, err
			}
			resp, err := b.Omc.CloudAPI().Wireless().UpdateSsidBandSteerConfigBySite(ctx, omadacID, in.SiteID, in.SsidID,
				wireless.UpdateSsidBandSteerOpenApiVO{Mode: int32(in.Mode)})
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
