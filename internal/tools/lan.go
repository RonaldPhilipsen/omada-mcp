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

// lanNetworkBodyDesc documents the lan.LanNetworkOpenApiVO fields that
// create and modify send. Its JSON names differ from what
// omada_lan_network_get returns, and the controller drops unknown fields
// without an error.
const lanNetworkBodyDesc = "body mirrors the Omada request type lan.LanNetworkOpenApiVO. " +
	"Always send name (1-128 characters), purpose (0=VLAN, 1=interface) and igmpSnoopEnable: the body type always sends these three, so a missing igmpSnoopEnable turns IGMP snooping off. " +
	"Common fields: vlan (1-4090), vlanType (0=single, 1=multiple), vlans, gatewaySubnet (IP/mask, e.g. 192.168.30.1/24, for purpose 1), interfaceIds (gateway LAN port IDs), domain, isolation, application (0=gateway and switch, 1=switch only), mldSnoopEnable, dhcpL2RelayEnable, dhcpGuard, dhcpv6Guard, allLan. " +
	"DHCP goes in dhcpSettingsVO, NOT dhcpSettings (the name that omada_lan_network_get returns): enable (true turns the DHCP server on), dhcpns (\"auto\" or \"manual\"), gateway, ipaddrStart, ipaddrEnd, priDns and sndDns (used when dhcpns is manual), leasetime (2-2880 minutes), options (custom DHCP options: code, name, type 0=string 1=IP 2=hex, value), and dhcpNextServer, option60, option66, option138. " +
	"IPv6 goes in lanNeworkIpv6Config (the spec's spelling, with no t), NOT lanNetworkIpv6Config. " +
	"The tool rejects a body that has dhcpSettings or lanNetworkIpv6Config, because the controller would drop them without an error. If you copy the output of omada_lan_network_get, rename those two keys first."

// lanNetworkBodyAliases maps the field names that omada_lan_network_get
// returns to the names that lan.LanNetworkOpenApiVO expects.
var lanNetworkBodyAliases = map[string]string{
	"dhcpSettings":         "dhcpSettingsVO",
	"lanNetworkIpv6Config": "lanNeworkIpv6Config",
}

// rejectLanNetworkBodyAliases fails when the raw body uses a field name from
// the get output instead of the request type: json.Unmarshal into
// lan.LanNetworkOpenApiVO drops such a field, and the network would be
// created or changed without DHCP or IPv6 settings.
func rejectLanNetworkBodyAliases(args *mcpkit.Args) error {
	var raw struct {
		Body map[string]json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(*args, &raw); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	for _, got := range []string{"dhcpSettings", "lanNetworkIpv6Config"} {
		if _, ok := raw.Body[got]; ok {
			return fmt.Errorf("body.%s is not a field of lan.LanNetworkOpenApiVO and the controller would drop it; use body.%s", got, lanNetworkBodyAliases[got])
		}
	}
	return nil
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
		Description: "Create a new Omada LAN network. " + lanNetworkBodyDesc,
		Group:       "lan",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID to create the network in (required)."),
				"body":      prop("object", "LAN network entity; mirrors the Omada request type lan.LanNetworkOpenApiVO. DHCP goes in dhcpSettingsVO, IPv6 in lanNeworkIpv6Config."),
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
			if err := rejectLanNetworkBodyAliases(args); err != nil {
				return nil, err
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

// lanNetworkModifyIn's body mirrors the Omada request type
// lan.LanNetworkOpenApiVO (the generated lan.ModifyLanNetworkJSONRequestBody).
type lanNetworkModifyIn struct {
	networkIDIn
	Body lan.LanNetworkOpenApiVO `json:"body"`
}

// LanNetworkModify returns the omada_lan_network_modify tool: PATCH one LAN
// network. The body mirrors the Omada request type lan.LanNetworkOpenApiVO,
// the same type as create.
func (b *Base) LanNetworkModify() mcpkit.Tool {
	return mcpkit.Tool{
		Name: "omada_lan_network_modify",
		Description: "Modify an existing Omada LAN network (PATCH). " + lanNetworkBodyDesc + " " +
			"The body type always sends name, purpose and igmpSnoopEnable, so this is not a safe partial update: start from omada_lan_network_get, rename the two keys, change what you need and send the whole entity. " +
			"A change to the subnet, VLAN or DHCP range can cut off the clients on that network.",
		Group:    "lan",
		Mutating: true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id":  prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":    prop("string", "The site ID the network belongs to (required)."),
				"network_id": prop("string", "The LAN network ID to modify, from omada_lan_networks_list (required)."),
				"body":       prop("object", "LAN network entity; mirrors the Omada request type lan.LanNetworkOpenApiVO. DHCP goes in dhcpSettingsVO, IPv6 in lanNeworkIpv6Config."),
			}, "site_id", "network_id", "body")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[lanNetworkModifyIn](args)
			if err != nil {
				return nil, err
			}
			if err := validateNetworkID(&in.networkIDIn); err != nil {
				return nil, err
			}
			if err := rejectLanNetworkBodyAliases(args); err != nil {
				return nil, err
			}
			if in.Body.Name == "" {
				return nil, fmt.Errorf("body.name is required: the body type always sends it, so send the full network entity")
			}
			omadacID, err := b.ResolveOmac(in.OmacIDArg)
			if err != nil {
				return nil, err
			}
			resp, err := b.Omc.CloudAPI().Lan().ModifyLanNetwork(ctx, omadacID, in.SiteID, in.NetworkID, in.Body)
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
