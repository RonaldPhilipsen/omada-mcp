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

	"github.com/RonaldPhilipsen/omada/cloud/security"
	"github.com/RonaldPhilipsen/omada/cloud/wireless"

	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
	"github.com/RonaldPhilipsen/omada-mcp/internal/omadaclient"
)

// securityCall resolves the omadac ID, runs the operation and decodes the
// envelope result.
func securityCall(b *Base, in OmacIDArg, call func(omadacID string) (*http.Response, error)) (*mcp.CallToolResult, error) {
	omadacID, err := b.ResolveOmac(in)
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

type aclListIn struct {
	OmacIDArg
	SiteID   string `json:"site_id"`
	Page     int    `json:"page,omitempty"`
	PageSize int    `json:"page_size,omitempty"`
}

func aclListSchema() map[string]any {
	return objectSchema(map[string]any{
		"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
		"site_id":   prop("string", "The site ID to list ACL rules for (required)."),
		"page":      prop("integer", "Page number, starting at 1 (default 1)."),
		"page_size": prop("integer", "Entries per page (default 20)."),
	}, "site_id")
}

func validateAclList(in *aclListIn) (page, pageSize int, err error) {
	if in.SiteID == "" {
		return 0, 0, fmt.Errorf("site_id is required")
	}
	page, pageSize = in.Page, in.PageSize
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	return page, pageSize, nil
}

type aclIDIn struct {
	OmacIDArg
	SiteID string `json:"site_id"`
	AclID  string `json:"acl_id"`
}

func validateAclID(in *aclIDIn) error {
	if in.SiteID == "" {
		return fmt.Errorf("site_id is required")
	}
	if in.AclID == "" {
		return fmt.Errorf("acl_id is required")
	}
	return nil
}

type aclModifyIn[T any] struct {
	aclIDIn
	Body T `json:"body"`
}

func aclModifySchema(bodyDesc string) map[string]any {
	return objectSchema(map[string]any{
		"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
		"site_id":   prop("string", "The site ID the ACL rule belongs to (required)."),
		"acl_id":    prop("string", "The ACL rule ID to act on, from the matching ACL list tool (required)."),
		"body":      prop("object", bodyDesc),
	}, "site_id", "acl_id", "body")
}

// AclGatewayList returns the omada_acl_gateway_list tool: the site's
// gateway ACL rules (Omada: Gateway → Firewall → ACL).
func (b *Base) AclGatewayList() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_acl_gateway_list",
		Description: "List the site's gateway ACL rules (Omada: Gateway → Firewall → ACL) with paging. Rules are ordered by index; policy is allow or deny, direction in or out.",
		Group:       "security",
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
				return b.Omc.CloudAPI().Security().GetOsgAclList(ctx, omadacID, in.SiteID, &security.GetOsgAclListParams{
					Page:     int32(page),
					PageSize: int32(pageSize),
				})
			})
		},
	}
}

// AclGatewayCreate returns the omada_acl_gateway_create tool. The body
// mirrors the Omada request type security.GatewayACLConfig.
func (b *Base) AclGatewayCreate() mcpkit.Tool {
	return mcpkit.Tool{
		Name: "omada_acl_gateway_create",
		Description: "Create a gateway ACL rule (Omada: Gateway → Firewall → ACL). body mirrors the Omada request type security.GatewayACLConfig. " +
			"policy: 0=drop, 1=allow. sourceType/destinationType (int): 0=network, 1=IP group, 2=IP-port group, 4=SSID, 6=IPv6 group, 7=IPv6-port group, 10=domain group; " +
			"sourceIds/destinationIds carry the IDs for the chosen type. direction: object with lanToLan, lanToWan, wanInIds, vpnInIds. " +
			"stateMode: 0=auto, 1=manual; states: object with established, related, invalid, stateNew. protocols: list of int32 (protocol table in the Omada OpenAPI guide). " +
			"status: bool. description, syslog, timeRangeId optional.",
		Group:    "security",
		Mutating: true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID to create the rule in (required)."),
				"body":      prop("object", "Gateway ACL rule; mirrors the Omada request type security.GatewayACLConfig."),
			}, "site_id", "body")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[struct {
				OmacIDArg
				SiteID string                    `json:"site_id"`
				Body   security.GatewayACLConfig `json:"body"`
			}](args)
			if err != nil {
				return nil, err
			}
			if in.SiteID == "" {
				return nil, fmt.Errorf("site_id is required")
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Security().CreateOsgAcl(ctx, omadacID, in.SiteID, in.Body)
			})
		},
	}
}

// AclGatewayModify returns the omada_acl_gateway_modify tool. The body
// mirrors the Omada request type security.GatewayACLConfig.
func (b *Base) AclGatewayModify() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_acl_gateway_modify",
		Description: "Modify a gateway ACL rule. body mirrors the Omada request type security.GatewayACLConfig (policy 0=drop/1=allow; sourceType/destinationType are int enums; direction is an object); include the full rule entity, not just the changed fields.",
		Group:       "security",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = aclModifySchema("Gateway ACL rule; mirrors the Omada request type security.GatewayACLConfig.")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[aclModifyIn[security.GatewayACLConfig]](args)
			if err != nil {
				return nil, err
			}
			if err := validateAclID(&in.aclIDIn); err != nil {
				return nil, err
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Security().ModifyOsgAcl(ctx, omadacID, in.SiteID, in.AclID, in.Body)
			})
		},
	}
}

// AclApList returns the omada_acl_ap_list tool: the site's AP ACL rules.
func (b *Base) AclApList() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_acl_ap_list",
		Description: "List the site's AP ACL rules (per-traffic rules evaluated on the access points) with paging. policy is allow or deny; source/destination types with ID lists; protocols list.",
		Group:       "security",
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
				return b.Omc.CloudAPI().Security().GetEapAclList(ctx, omadacID, in.SiteID, &security.GetEapAclListParams{
					Page:     int32(page),
					PageSize: int32(pageSize),
				})
			})
		},
	}
}

// AclApCreate returns the omada_acl_ap_create tool. The body mirrors the
// Omada request type security.EapACLConfig.
func (b *Base) AclApCreate() mcpkit.Tool {
	return mcpkit.Tool{
		Name: "omada_acl_ap_create",
		Description: "Create an AP ACL rule. body mirrors the Omada request type security.EapACLConfig. " +
			"policy: 0=drop, 1=allow. sourceType/destinationType (int): 0=network, 1=IP group, 2=IP-port group, 4=SSID, 6=IPv6 group, 7=IPv6-port group; " +
			"sourceIds/destinationIds carry the IDs for the chosen type. protocols: list of int32. status: bool. description optional.",
		Group:    "security",
		Mutating: true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID to create the rule in (required)."),
				"body":      prop("object", "AP ACL rule; mirrors the Omada request type security.EapACLConfig."),
			}, "site_id", "body")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[struct {
				OmacIDArg
				SiteID string                `json:"site_id"`
				Body   security.EapACLConfig `json:"body"`
			}](args)
			if err != nil {
				return nil, err
			}
			if in.SiteID == "" {
				return nil, fmt.Errorf("site_id is required")
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Security().CreateEapAcl(ctx, omadacID, in.SiteID, in.Body)
			})
		},
	}
}

// AclApModify returns the omada_acl_ap_modify tool. The body mirrors the
// Omada request type security.EapACLConfig.
func (b *Base) AclApModify() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_acl_ap_modify",
		Description: "Modify an AP ACL rule. body mirrors the Omada request type security.EapACLConfig (policy 0=drop/1=allow; sourceType/destinationType are int enums); include the full rule entity, not just the changed fields.",
		Group:       "security",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = aclModifySchema("AP ACL rule; mirrors the Omada request type security.EapACLConfig.")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[aclModifyIn[security.EapACLConfig]](args)
			if err != nil {
				return nil, err
			}
			if err := validateAclID(&in.aclIDIn); err != nil {
				return nil, err
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Security().ModifyEapAcl(ctx, omadacID, in.SiteID, in.AclID, in.Body)
			})
		},
	}
}

// AclSwitchList returns the omada_acl_switch_list tool: the site's switch
// ACL rules.
func (b *Base) AclSwitchList() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_acl_switch_list",
		Description: "List the site's switch ACL rules (port/802.1X-based rules evaluated on the managed switches) with paging. policy is allow or deny; rules bind to ports, devices, VLANs or networks.",
		Group:       "security",
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
				return b.Omc.CloudAPI().Security().GetOswAclList(ctx, omadacID, in.SiteID, &security.GetOswAclListParams{
					Page:     int32(page),
					PageSize: int32(pageSize),
				})
			})
		},
	}
}

// AclSwitchCreate returns the omada_acl_switch_create tool. The body
// mirrors the Omada request type security.SwitchACLConfig.
func (b *Base) AclSwitchCreate() mcpkit.Tool {
	return mcpkit.Tool{
		Name: "omada_acl_switch_create",
		Description: "Create a switch ACL rule. body mirrors the Omada request type security.SwitchACLConfig. " +
			"policy: 0=drop, 1=allow. bindingType (int): 0=all ports, 1=custom ports, 2=all switch vlans, 3=custom switch vlans; " +
			"binding-specific fields: customAclPorts (binding 1), networkId (binding 2/3), customAclDevices/customAclOsws/customAclStacks (binding 3). " +
			"sourceType/destinationType (int): 0=network, 1=IP group, 2=IP-port group, 4=SSID, 6=IPv6 group, 7=IPv6-port group; " +
			"protocols: list of int32. etherType: object {enable, value (4-hex, MAC-group rules only)}. biDirectional, status: bool.",
		Group:    "security",
		Mutating: true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID to create the rule in (required)."),
				"body":      prop("object", "Switch ACL rule; mirrors the Omada request type security.SwitchACLConfig."),
			}, "site_id", "body")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[struct {
				OmacIDArg
				SiteID string                   `json:"site_id"`
				Body   security.SwitchACLConfig `json:"body"`
			}](args)
			if err != nil {
				return nil, err
			}
			if in.SiteID == "" {
				return nil, fmt.Errorf("site_id is required")
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Security().CreateOswAcl(ctx, omadacID, in.SiteID, in.Body)
			})
		},
	}
}

// AclSwitchModify returns the omada_acl_switch_modify tool. The body
// mirrors the Omada request type security.SwitchACLConfig.
func (b *Base) AclSwitchModify() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_acl_switch_modify",
		Description: "Modify a switch ACL rule. body mirrors the Omada request type security.SwitchACLConfig (policy 0=drop/1=allow; bindingType and sourceType/destinationType are int enums); include the full rule entity, not just the changed fields.",
		Group:       "security",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = aclModifySchema("Switch ACL rule; mirrors the Omada request type security.SwitchACLConfig.")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[aclModifyIn[security.SwitchACLConfig]](args)
			if err != nil {
				return nil, err
			}
			if err := validateAclID(&in.aclIDIn); err != nil {
				return nil, err
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Security().ModifyOswAcl(ctx, omadacID, in.SiteID, in.AclID, in.Body)
			})
		},
	}
}

// AclDelete returns the omada_acl_delete tool: the shared ACL delete for
// gateway, AP and switch rules.
func (b *Base) AclDelete() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_acl_delete",
		Description: "Delete an ACL rule by ID. Works for gateway, AP and switch ACL rules (the controller shares one ACL ID space). Get the ID from the matching ACL list tool.",
		Group:       "security",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID the ACL rule belongs to (required)."),
				"acl_id":    prop("string", "The ACL rule ID to delete (required)."),
			}, "site_id", "acl_id")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[aclIDIn](args)
			if err != nil {
				return nil, err
			}
			if err := validateAclID(in); err != nil {
				return nil, err
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Security().DeleteAcl(ctx, omadacID, in.SiteID, in.AclID)
			})
		},
	}
}

// MacFilterAllowList returns the omada_mac_filter_allow_list tool: the
// site's MAC filter allow entries.
func (b *Base) MacFilterAllowList() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_mac_filter_allow_list",
		Description: "List the site's MAC filter allow entries (Omada: Wireless → MAC Filter → Allow) with paging. Each entry holds a name, MAC addresses and/or MAC group IDs.",
		Group:       "security",
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
				return b.Omc.CloudAPI().Security().GetGridAllowMacFiltering(ctx, omadacID, in.SiteID, &security.GetGridAllowMacFilteringParams{
					Page:     int32(page),
					PageSize: int32(pageSize),
				})
			})
		},
	}
}

// MacFilterDenyList returns the omada_mac_filter_deny_list tool: the
// site's MAC filter deny entries.
func (b *Base) MacFilterDenyList() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_mac_filter_deny_list",
		Description: "List the site's MAC filter deny entries (Omada: Wireless → MAC Filter → Deny) with paging. Each entry holds a name, MAC addresses and/or MAC group IDs.",
		Group:       "security",
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
				return b.Omc.CloudAPI().Security().GetGridDenyMacFiltering(ctx, omadacID, in.SiteID, &security.GetGridDenyMacFilteringParams{
					Page:     int32(page),
					PageSize: int32(pageSize),
				})
			})
		},
	}
}

// MacFilterAdd returns the omada_mac_filter_add tool. The body mirrors
// the Omada request type security.MacFiltering.
func (b *Base) MacFilterAdd() mcpkit.Tool {
	return mcpkit.Tool{
		Name: "omada_mac_filter_add",
		Description: "Add a MAC filter entry. body mirrors the Omada request type security.MacFiltering. " +
			"name (required). type (int): 0=macAddresses, 1=macGroupIds. filterMode (int): 0=allow, 1=deny. " +
			"macAddresses: list of {macAddress, name?} objects. macGroupIds: list of MAC group profile IDs.",
		Group:    "security",
		Mutating: true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID to add the entry to (required)."),
				"body":      prop("object", "MAC filter entry; mirrors the Omada request type security.MacFiltering."),
			}, "site_id", "body")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[struct {
				OmacIDArg
				SiteID string                `json:"site_id"`
				Body   security.MacFiltering `json:"body"`
			}](args)
			if err != nil {
				return nil, err
			}
			if in.SiteID == "" {
				return nil, fmt.Errorf("site_id is required")
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Security().CreateMacFiltering(ctx, omadacID, in.SiteID, in.Body)
			})
		},
	}
}

type filterIDIn struct {
	OmacIDArg
	SiteID   string `json:"site_id"`
	FilterID string `json:"filter_id"`
}

// MacFilterRemove returns the omada_mac_filter_remove tool.
func (b *Base) MacFilterRemove() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_mac_filter_remove",
		Description: "Remove a MAC filter entry by ID (allow or deny). Get the ID from omada_mac_filter_allow_list or omada_mac_filter_deny_list.",
		Group:       "security",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID the entry belongs to (required)."),
				"filter_id": prop("string", "The MAC filter entry ID to remove (required)."),
			}, "site_id", "filter_id")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[filterIDIn](args)
			if err != nil {
				return nil, err
			}
			if in.SiteID == "" {
				return nil, fmt.Errorf("site_id is required")
			}
			if in.FilterID == "" {
				return nil, fmt.Errorf("filter_id is required")
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Security().DeleteMacFiltering(ctx, omadacID, in.SiteID, in.FilterID)
			})
		},
	}
}

// SsidMacFilterModify returns the omada_ssid_mac_filter_modify tool. The
// body mirrors the Omada request type wireless.UpdateSsidMacFilterOpenApiVO.
func (b *Base) SsidMacFilterModify() mcpkit.Tool {
	return mcpkit.Tool{
		Name: "omada_ssid_mac_filter_modify",
		Description: "Modify the MAC filter config of an Omada SSID. body mirrors the Omada request type wireless.UpdateSsidMacFilterOpenApiVO: " +
			"macFilterEnable (on/off), macFilterId (MAC group profile from the allow/deny lists) and/or ouiProfileIdList (OUI profiles).",
		Group:    "security",
		Mutating: true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID the SSID belongs to (required)."),
				"ssid_id":   prop("string", "The SSID ID to act on, from omada_wireless_ssids_list (required)."),
				"body":      prop("object", "SSID MAC filter config; mirrors the Omada request type wireless.UpdateSsidMacFilterOpenApiVO."),
			}, "site_id", "ssid_id", "body")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[struct {
				OmacIDArg
				SiteID string                                `json:"site_id"`
				SsidID string                                `json:"ssid_id"`
				Body   wireless.UpdateSsidMacFilterOpenApiVO `json:"body"`
			}](args)
			if err != nil {
				return nil, err
			}
			if in.SiteID == "" {
				return nil, fmt.Errorf("site_id is required")
			}
			if in.SsidID == "" {
				return nil, fmt.Errorf("ssid_id is required")
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Wireless().UpdateSsidMacFilterConfigBySite(ctx, omadacID, in.SiteID, in.SsidID, in.Body)
			})
		},
	}
}
