package tools

import (
	"context"
	"fmt"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/RonaldPhilipsen/omada/cloud/client"
	"github.com/RonaldPhilipsen/omada/cloud/lan"
	switchpkg "github.com/RonaldPhilipsen/omada/cloud/switch"

	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
)

// switchPortBodyDesc documents the OswPortSettingVO fields that matter
// for the common port changes.
const switchPortBodyDesc = "Port settings; mirrors the Omada request type switch.OswPortSettingVO. " +
	"profileId (required for a single port; IDs from omada_lan_profiles_list). " +
	"To set the untagged (native) network of a port: profileVlanOverrideEnable true and nativeNetworkId (a LAN network ID from omada_lan_networks_list), " +
	"optionally networkTagsSetting (0=allow all, 1=block all, 2=custom) with tagNetworkIds. " +
	"Other common fields: name, poe (0=off, 1=on), profileOverrideEnable, portIsolationEnable, linkSpeed, duplex. " +
	"To turn a port off or on, use omada_switch_port_enable, not disable here: in a live test, disable true and then false through this PATCH did not take the link down, and the cause is not known."

// SwitchPortsOverview returns the omada_switch_ports_overview tool: every
// switch port in the site with its profile, networks and link state.
func (b *Base) SwitchPortsOverview() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_switch_ports_overview",
		Description: "List the ports of all switches in the site with paging: switch MAC, port number, name, profile, native and tagged networks, and link state. Use it to find the switch_mac and port for omada_switch_port_modify.",
		Group:       "switch",
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID to list switch ports for (required)."),
				"page":      prop("integer", "Page number, starting at 1 (default 1)."),
				"page_size": prop("integer", "Entries per page, 1-1000 (default 100)."),
			}, "site_id")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[aclListIn](args)
			if err != nil {
				return nil, err
			}
			if in.SiteID == "" {
				return nil, fmt.Errorf("site_id is required")
			}
			page, pageSize := in.Page, in.PageSize
			if page == 0 {
				page = 1
			}
			if pageSize == 0 {
				pageSize = 100
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Switch().GetGridSwitchesPortsOverview(ctx, omadacID, in.SiteID, &switchpkg.GetGridSwitchesPortsOverviewParams{
					Page:     int32(page),
					PageSize: int32(pageSize),
				})
			})
		},
	}
}

// LanProfilesList returns the omada_lan_profiles_list tool: the switch
// port profiles (Omada: Settings → Wired Networks → LAN → Profiles).
func (b *Base) LanProfilesList() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_lan_profiles_list",
		Description: "List the site's switch port profiles (LAN profiles) with paging. A profile sets a port's native network, tagged networks and other port settings; omada_switch_port_modify needs a profileId from this list.",
		Group:       "switch",
		Register: func(t *mcp.Tool) {
			t.InputSchema = aclListSchema()
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[aclListIn](args)
			if err != nil {
				return nil, err
			}
			page, pageSize, err := validateAclList(in)
			if err != nil {
				return nil, err
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Lan().GetLanProfileList(ctx, omadacID, in.SiteID, &lan.GetLanProfileListParams{
					Page:     int32(page),
					PageSize: int32(pageSize),
				})
			})
		},
	}
}

// SwitchPortModify returns the omada_switch_port_modify tool. The body
// mirrors the Omada request type switch.OswPortSettingVO.
func (b *Base) SwitchPortModify() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_switch_port_modify",
		Description: "Modify one switch port. " + switchPortBodyDesc,
		Group:       "switch",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id":  prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":    prop("string", "The site ID the switch belongs to (required)."),
				"switch_mac": prop("string", "The switch MAC address, e.g. A8-29-48-EE-EB-1D (required)."),
				"port":       prop("string", "The port number on the switch, e.g. \"2\" (required)."),
				"body":       prop("object", "Port settings; mirrors the Omada request type switch.OswPortSettingVO."),
			}, "site_id", "switch_mac", "port", "body")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[struct {
				OmacIDArg
				SiteID    string                     `json:"site_id"`
				SwitchMac string                     `json:"switch_mac"`
				Port      string                     `json:"port"`
				Body      switchpkg.OswPortSettingVO `json:"body"`
			}](args)
			if err != nil {
				return nil, err
			}
			switch {
			case in.SiteID == "":
				return nil, fmt.Errorf("site_id is required")
			case in.SwitchMac == "":
				return nil, fmt.Errorf("switch_mac is required")
			case in.Port == "":
				return nil, fmt.Errorf("port is required")
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Switch().ModifySwitchPort(ctx, omadacID, in.SiteID, in.SwitchMac, in.Port, in.Body)
			})
		},
	}
}

// SwitchPortEnable returns the omada_switch_port_enable tool: turn one
// switch port off or on with the dedicated status endpoint (PUT
// .../ports/{port}/status, body switch.SwitchPortStatus).
func (b *Base) SwitchPortEnable() mcpkit.Tool {
	return mcpkit.Tool{
		Name: "omada_switch_port_enable",
		Description: "Turn one switch port off (enable false) or on (enable true). It calls the dedicated port status endpoint (Omada operation SetPortModeForGivenPort1, body switch.SwitchPortStatus, status 0=off, 1=on). " +
			"Turning a port off cuts the link of the device on it; to bounce a port, turn it off, wait, then turn it on. " +
			"Do not turn off the port that connects the controller or this server to the network.",
		Group:    "switch",
		Mutating: true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id":  prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":    prop("string", "The site ID the switch belongs to (required)."),
				"switch_mac": prop("string", "The switch MAC address, e.g. A8-29-48-EE-EB-1D (required)."),
				"port":       prop("string", "The port number on the switch, e.g. \"2\" (required)."),
				"enable":     prop("boolean", "true turns the port on, false turns it off (required)."),
			}, "site_id", "switch_mac", "port", "enable")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[struct {
				OmacIDArg
				SiteID    string `json:"site_id"`
				SwitchMac string `json:"switch_mac"`
				Port      string `json:"port"`
				Enable    *bool  `json:"enable"`
			}](args)
			if err != nil {
				return nil, err
			}
			switch {
			case in.SiteID == "":
				return nil, fmt.Errorf("site_id is required")
			case in.SwitchMac == "":
				return nil, fmt.Errorf("switch_mac is required")
			case in.Port == "":
				return nil, fmt.Errorf("port is required")
			case in.Enable == nil:
				return nil, fmt.Errorf("enable is required (true or false)")
			}
			body := switchpkg.SwitchPortStatus{Status: 0}
			if *in.Enable {
				body.Status = 1
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Switch().SetPortModeForGivenPort1(ctx, omadacID, in.SiteID, in.SwitchMac, in.Port, body)
			})
		},
	}
}

// ClientIpSettingModify returns the omada_client_ip_setting_modify tool:
// a fixed (reserved) IP address for one client. The body mirrors the
// Omada request type client.OpenApiClientIpSetting.
func (b *Base) ClientIpSettingModify() mcpkit.Tool {
	return mcpkit.Tool{
		Name: "omada_client_ip_setting_modify",
		Description: "Set or clear the fixed IP address (DHCP reservation) of one client. body mirrors the Omada request type client.OpenApiClientIpSetting: " +
			"useFixedAddr (bool, required), netId (LAN network ID from omada_lan_networks_list), ip (address inside that network). " +
			"Optional serverType, serverMac (the DHCP server, usually the gateway). Send useFixedAddr false to remove the reservation.",
		Group:    "client",
		Mutating: true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id":  prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":    prop("string", "The site ID the client belongs to (required)."),
				"client_mac": prop("string", "The client MAC address, e.g. 8C-83-E8-95-46-AF (required)."),
				"body":       prop("object", "IP setting; mirrors the Omada request type client.OpenApiClientIpSetting."),
			}, "site_id", "client_mac", "body")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[struct {
				OmacIDArg
				SiteID    string                        `json:"site_id"`
				ClientMac string                        `json:"client_mac"`
				Body      client.OpenApiClientIpSetting `json:"body"`
			}](args)
			if err != nil {
				return nil, err
			}
			switch {
			case in.SiteID == "":
				return nil, fmt.Errorf("site_id is required")
			case in.ClientMac == "":
				return nil, fmt.Errorf("client_mac is required")
			case in.Body.UseFixedAddr && (in.Body.Ip == nil || *in.Body.Ip == "" || in.Body.NetId == nil || *in.Body.NetId == ""):
				return nil, fmt.Errorf("body.ip and body.netId are required when useFixedAddr is true")
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Clients().ModifyClientIpSetting(ctx, omadacID, in.SiteID, in.ClientMac, in.Body)
			})
		},
	}
}
