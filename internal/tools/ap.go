// Package tools is the omada-mcp MCP tool catalog. Each curated group
// (site, client, device, ...) is one file that exports one mcpkit.Tool
// constructor per operation, all built on the shared Base.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/RonaldPhilipsen/omada/cloud/ap"

	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
	"github.com/RonaldPhilipsen/omada-mcp/internal/omadaclient"
)

type apIDIn struct {
	OmacIDArg
	SiteID string `json:"site_id"`
	ApMac  string `json:"ap_mac"`
}

func apIDSchema() map[string]any {
	return objectSchema(map[string]any{
		"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
		"site_id":   prop("string", "The site ID the AP belongs to (required)."),
		"ap_mac":    prop("string", "The AP MAC address to act on, e.g. 20-36-26-D4-AE-A0 (required)."),
	}, "site_id", "ap_mac")
}

// validateApID validates the site_id and ap_mac required args before any
// HTTP.
func validateApID(in *apIDIn) error {
	if in.SiteID == "" {
		return fmt.Errorf("site_id is required")
	}
	if in.ApMac == "" {
		return fmt.Errorf("ap_mac is required")
	}
	return nil
}

func apIDBody(b *Base, in *apIDIn, call func(omadacID string) (*http.Response, error)) (*mcp.CallToolResult, error) {
	omadacID, err := b.ResolveOmac(in.OmacIDArg)
	if err != nil {
		return nil, err
	}
	resp, err := call(omadacID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result json.RawMessage
	if err := omadaclient.Envelope(resp, &result); err != nil {
		return nil, err
	}
	return mcpkit.JSONResult(resultPayload(result))
}

// ApRadioConfigGet returns the omada_ap_radio_config_get tool: the
// configured radio settings (channel, width, tx power) per band of an AP.
func (b *Base) ApRadioConfigGet() mcpkit.Tool {
	return mcpkit.Tool{
		Name: "omada_ap_radio_config_get",
		Description: "Get the configured radio settings of an Omada AP: per-band radioSetting2g/5g/6g with channel, channelWidth, txPower and txPowerLevel. " +
			"Note: channel is a channel INDEX, not a channel number; use omada_ap_available_channel_get to map index to channel and frequency.",
		Group:    "ap",
		Register: func(t *mcp.Tool) { t.InputSchema = apIDSchema() },
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[apIDIn](args)
			if err != nil {
				return nil, err
			}
			if err := validateApID(in); err != nil {
				return nil, err
			}
			return apIDBody(b, in, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Ap().GetRadiosConfig(ctx, omadacID, in.SiteID, in.ApMac)
			})
		},
	}
}

// ApRadioConfigModifyIn's body mirrors the Omada request type
// ap.ApRadiosConfig (the generated ap.ModifyRadiosConfigJSONRequestBody).
type ApRadioConfigModifyIn struct {
	apIDIn
	Body ap.ApRadiosConfig `json:"body"`
}

func apRadioConfigModifySchema() map[string]any {
	return objectSchema(map[string]any{
		"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
		"site_id":   prop("string", "The site ID the AP belongs to (required)."),
		"ap_mac":    prop("string", "The AP MAC address to act on, e.g. 20-36-26-D4-AE-A0 (required)."),
		"body": prop("object", "Radio settings; mirrors the Omada request type ap.ApRadiosConfig with radioSetting2g/5g/6g sections. "+
			"channel is a channel INDEX (from omada_ap_available_channel_get), not a channel number. "+
			"channelWidth: 2=20MHz, 3=40MHz, 5=80MHz, 7=160MHz, 10=320MHz. "+
			"txPowerLevel: 0=Low, 1=Medium, 2=High, 3=Custom (txPower in dBm), 4=Auto. "+
			"channelRange (list of frequencies) pins the channel when firmware auto-selection would move it."),
	}, "site_id", "ap_mac", "body")
}

// ApRadioConfigModify returns the omada_ap_radio_config_modify tool: PATCH
// the per-band radio settings of an AP.
func (b *Base) ApRadioConfigModify() mcpkit.Tool {
	return mcpkit.Tool{
		Name: "omada_ap_radio_config_modify",
		Description: "Modify the radio settings of an Omada AP (channel index, channel width, tx power level per band). body mirrors the Omada request type ap.ApRadiosConfig. " +
			"channel is a channel INDEX (from omada_ap_available_channel_get), not a channel number. channelWidth: 2=20MHz, 3=40MHz, 5=80MHz, 7=160MHz, 10=320MHz. " +
			"txPowerLevel: 0=Low, 1=Medium, 2=High, 3=Custom (txPower in dBm), 4=Auto. channelRange (list of frequencies) pins the channel when firmware auto-selection would move it. " +
			"After modifying, verify the actual operating state with omada_ap_radios_get; some firmwares re-select channel or width on reboot.",
		Group:    "ap",
		Mutating: true,
		Register: func(t *mcp.Tool) { t.InputSchema = apRadioConfigModifySchema() },
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[ApRadioConfigModifyIn](args)
			if err != nil {
				return nil, err
			}
			if err := validateApID(&in.apIDIn); err != nil {
				return nil, err
			}
			return apIDBody(b, &in.apIDIn, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Ap().ModifyRadiosConfig(ctx, omadacID, in.SiteID, in.ApMac, in.Body)
			})
		},
	}
}

// ApRadiosGet returns the omada_ap_radios_get tool: the operational radio
// state (actual operating channel, width, tx power, traffic) of an AP.
func (b *Base) ApRadiosGet() mcpkit.Tool {
	return mcpkit.Tool{
		Name: "omada_ap_radios_get",
		Description: "Get the OPERATIONAL radio state of an Omada AP: per-band actual operating channel and frequency (wp2g/wp5g/wp6g), band width, tx power and traffic counters. " +
			"Use this after omada_ap_radio_config_modify to verify what the radio is actually doing; firmware may auto-adjust a configured channel or width.",
		Group:    "ap",
		Register: func(t *mcp.Tool) { t.InputSchema = apIDSchema() },
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[apIDIn](args)
			if err != nil {
				return nil, err
			}
			if err := validateApID(in); err != nil {
				return nil, err
			}
			return apIDBody(b, in, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Ap().GetRadiosDetail(ctx, omadacID, in.SiteID, in.ApMac)
			})
		},
	}
}

// ApAvailableChannelGet returns the omada_ap_available_channel_get tool:
// the per-radio list of available channels (index, channel, frequency,
// supported widths) of an AP.
func (b *Base) ApAvailableChannelGet() mcpkit.Tool {
	return mcpkit.Tool{
		Name: "omada_ap_available_channel_get",
		Description: "Get the per-radio list of channels available on an Omada AP: channel index, channel number, frequency and supported channel widths per radioId (0=2.4GHz, 1=5GHz, 3=6GHz). " +
			"The channel INDEX is the value omada_ap_radio_config_modify expects in its channel field; supported widths here define what channelWidth values the radio can use on that channel.",
		Group:    "ap",
		Register: func(t *mcp.Tool) { t.InputSchema = apIDSchema() },
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[apIDIn](args)
			if err != nil {
				return nil, err
			}
			if err := validateApID(in); err != nil {
				return nil, err
			}
			return apIDBody(b, in, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Ap().GetAvailableChannelOfAp(ctx, omadacID, in.SiteID, in.ApMac)
			})
		},
	}
}

// ApPortsList returns the omada_ap_ports_list tool: the wired LAN ports of
// an AP and their current settings.
func (b *Base) ApPortsList() mcpkit.Tool {
	return mcpkit.Tool{
		Name: "omada_ap_ports_list",
		Description: "List the wired LAN ports of an Omada AP with their current settings: port number, name, status, link status and speed, and the local VLAN fields (localVlanEnable, localVlanId, localVlanNetworkId). " +
			"Use the port number for omada_ap_port_modify. Use omada_ap_port_vlans_list to see which ports sit on each network.",
		Group:    "ap",
		Register: func(t *mcp.Tool) { t.InputSchema = apIDSchema() },
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[apIDIn](args)
			if err != nil {
				return nil, err
			}
			if err := validateApID(in); err != nil {
				return nil, err
			}
			return apIDBody(b, in, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Ap().GetApPortList(ctx, omadacID, in.SiteID, in.ApMac)
			})
		},
	}
}

// apPortModifyDesc documents the ap.ModifyAPLANPort fields that matter for
// putting a wired AP port on a network.
const apPortModifyDesc = "body mirrors the Omada request type ap.ModifyAPLANPort; send only the fields to change. " +
	"To set the network of the port (for example to put the port that feeds a TV on a given LAN): localVlanEnable true and localVlanNetworkId (a LAN network ID from omada_lan_networks_list), with localVlanId as the VLAN ID. " +
	"untaggedNetworkId and taggedNetworkId are lists of network IDs; untagged and tagged are string fields. The spec does not explain how these relate to the localVlan fields, so check the result with omada_ap_port_vlans_list or omada_ap_ports_list after the change. " +
	"Other fields: name, status (true=enabled), poeOutEnable, bandwidthControlEnable, ingressRateLimit and egressRateLimit."

// ApPortModifyIn is the input of omada_ap_port_modify.
type ApPortModifyIn struct {
	apIDIn
	Port string             `json:"port"`
	Body ap.ModifyAPLANPort `json:"body"`
}

// ApPortModify returns the omada_ap_port_modify tool: PATCH one wired LAN
// port of an AP. The body mirrors the Omada request type ap.ModifyAPLANPort.
func (b *Base) ApPortModify() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_ap_port_modify",
		Description: "Modify one wired LAN port of an Omada AP. " + apPortModifyDesc,
		Group:       "ap",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID the AP belongs to (required)."),
				"ap_mac":    prop("string", "The AP MAC address, e.g. 20-36-26-D4-AE-A0 (required)."),
				"port":      prop("string", "The LAN port number on the AP, e.g. \"1\", from omada_ap_ports_list (required)."),
				"body":      prop("object", "Port settings; mirrors the Omada request type ap.ModifyAPLANPort. "+apPortModifyDesc),
			}, "site_id", "ap_mac", "port", "body")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[ApPortModifyIn](args)
			if err != nil {
				return nil, err
			}
			if err := validateApID(&in.apIDIn); err != nil {
				return nil, err
			}
			if in.Port == "" {
				return nil, fmt.Errorf("port is required")
			}
			return apIDBody(b, &in.apIDIn, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Ap().ModifyApPort(ctx, omadacID, in.SiteID, in.ApMac, in.Port, in.Body)
			})
		},
	}
}

type apPortVlansIn struct {
	apIDIn
	Page     int `json:"page,omitempty"`
	PageSize int `json:"page_size,omitempty"`
}

// ApPortVlansList returns the omada_ap_port_vlans_list tool: the current
// VLAN membership of an AP's ports, per VLAN network (GET port-vlans).
func (b *Base) ApPortVlansList() mcpkit.Tool {
	return mcpkit.Tool{
		Name: "omada_ap_port_vlans_list",
		Description: "List the current VLAN configuration of an Omada AP's ports, with paging: per VLAN network, its localVlanId, localVlanNetworkId, name, and the port lists nativePort, untagPort and tagPort. " +
			"Use it to confirm which wired port is on which network after omada_ap_port_modify.",
		Group: "ap",
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID the AP belongs to (required)."),
				"ap_mac":    prop("string", "The AP MAC address, e.g. 20-36-26-D4-AE-A0 (required)."),
				"page":      prop("integer", "Page number, starting at 1 (default 1)."),
				"page_size": prop("integer", "Entries per page, 1-1000 (default 20)."),
			}, "site_id", "ap_mac")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[apPortVlansIn](args)
			if err != nil {
				return nil, err
			}
			if err := validateApID(&in.apIDIn); err != nil {
				return nil, err
			}
			page, pageSize := in.Page, in.PageSize
			if page == 0 {
				page = 1
			}
			if pageSize == 0 {
				pageSize = 20
			}
			return apIDBody(b, &in.apIDIn, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Ap().GetApPortVlans(ctx, omadacID, in.SiteID, in.ApMac, &ap.GetApPortVlansParams{
					Page:     int32(page),
					PageSize: int32(pageSize),
				})
			})
		},
	}
}
