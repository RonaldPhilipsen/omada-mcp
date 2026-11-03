package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/RonaldPhilipsen/omada/cloud/device"

	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
	"github.com/RonaldPhilipsen/omada-mcp/internal/omadaclient"
)

// deviceListIn's page defaults to 1 and page_size to 20.
type deviceListIn struct {
	OmacIDArg
	SiteID   string `json:"site_id"`
	Page     int    `json:"page,omitempty"`
	PageSize int    `json:"page_size,omitempty"`
}

// DeviceList returns the omada_device_list tool. page defaults to 1 and
// page_size to 20.
func (b *Base) DeviceList() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_device_list",
		Description: "List devices in an Omada site with paging. page starts at 1 (default 1); page_size is the number of entries per page (default 20).",
		Group:       "device",
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID to list devices for (required)."),
				"page":      prop("integer", "Page number, starting at 1 (default 1)."),
				"page_size": prop("integer", "Entries per page (default 20)."),
			}, "site_id")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[deviceListIn](args)
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
			resp, err := b.Omc.CloudAPI().Devices().GetDeviceList(ctx, omadacID, in.SiteID, &device.GetDeviceListParams{
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

type siteOnlyIn struct {
	OmacIDArg
	SiteID string `json:"site_id"`
}

func siteOnlySchema() map[string]any {
	return objectSchema(map[string]any{
		"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
		"site_id":   prop("string", "The site ID (required)."),
	}, "site_id")
}

// DevicesBySite returns the omada_devices_by_site tool: every device in a
// site, without paging.
func (b *Base) DevicesBySite() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_devices_by_site",
		Description: "List every device in an Omada site, without paging.",
		Group:       "device",
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
			resp, err := b.Omc.CloudAPI().Devices().GetAllDeviceBySite(ctx, omadacID, in.SiteID)
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

type deviceRebootIn struct {
	OmacIDArg
	SiteID    string `json:"site_id"`
	DeviceMac string `json:"device_mac"`
}

func (b *Base) DeviceReboot() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_device_reboot",
		Description: "Reboot an Omada device by site and device MAC address.",
		Group:       "device",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id":  prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":    prop("string", "The site ID the device belongs to (required)."),
				"device_mac": prop("string", "The device MAC address to reboot (required)."),
			}, "site_id", "device_mac")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[deviceRebootIn](args)
			if err != nil {
				return nil, err
			}
			if in.SiteID == "" {
				return nil, fmt.Errorf("site_id is required")
			}
			if in.DeviceMac == "" {
				return nil, fmt.Errorf("device_mac is required")
			}
			omadacID, err := b.ResolveOmac(in.OmacIDArg)
			if err != nil {
				return nil, err
			}
			resp, err := b.Omc.CloudAPI().Devices().RebootDevice(ctx, omadacID, in.SiteID, in.DeviceMac)
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

func (b *Base) TopologyGet() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_topology_get",
		Description: "Get an Omada site's network topology.",
		Group:       "device",
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
			resp, err := b.Omc.CloudAPI().Topology().GetTopology(ctx, omadacID, in.SiteID)
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
