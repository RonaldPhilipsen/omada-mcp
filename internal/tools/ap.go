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
