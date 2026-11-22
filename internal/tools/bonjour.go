package tools

import (
	"context"
	"fmt"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/RonaldPhilipsen/omada/cloud/lan"
	"github.com/RonaldPhilipsen/omada/cloud/wireless"

	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
)

// bonjourServiceBodyDesc documents the BonjourServiceOpenApiVO body shared
// by the service create and modify tools.
const bonjourServiceBodyDesc = "Bonjour service profile; mirrors the Omada request type wireless.BonjourServiceOpenApiVO. " +
	"name (1-64 characters, required), serviceIds (required, at most 3): service IDs in \"_A._B.local\" format, e.g. \"_http._tcp.local\"."

// bonjourRuleBodyDesc documents the CreateMdnsRuleOpenApiVO body shared by
// the rule create and modify tools.
const bonjourRuleBodyDesc = "Bonjour (mDNS forwarding) rule; mirrors the Omada request type lan.CreateMdnsRuleOpenApiVO. " +
	"name (1-64 characters, required), status (bool, required), profileIds (required): Bonjour service profile IDs from omada_bonjour_service_list. " +
	"type (int): 0=by VLAN ID (AP forwarding, set ap: {serviceVlan, clientVlan}), 1=by network (gateway forwarding, set osg: {serviceNetworks, clientNetworks} with LAN network IDs from omada_lan_networks_list). " +
	"The service network is where the devices announce themselves; the client network is where the clients look for them."

type bonjourIDIn struct {
	OmacIDArg
	SiteID string `json:"site_id"`
	ID     string `json:"id"`
}

func validateBonjourID(in *bonjourIDIn) error {
	if in.SiteID == "" {
		return fmt.Errorf("site_id is required")
	}
	if in.ID == "" {
		return fmt.Errorf("id is required")
	}
	return nil
}

func bonjourIDSchema(idDesc string, extra map[string]any, required ...string) map[string]any {
	props := map[string]any{
		"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
		"site_id":   prop("string", "The site ID (required)."),
		"id":        prop("string", idDesc),
	}
	for k, v := range extra {
		props[k] = v
	}
	return objectSchema(props, append([]string{"site_id", "id"}, required...)...)
}

// BonjourServiceList returns the omada_bonjour_service_list tool: the
// site's Bonjour service profiles (Omada: Settings → Profiles → Bonjour
// Service).
func (b *Base) BonjourServiceList() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_bonjour_service_list",
		Description: "List the site's Bonjour service profiles (Omada: Settings → Profiles → Bonjour Service): named sets of mDNS service IDs that Bonjour rules forward between networks.",
		Group:       "bonjour",
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID to list Bonjour service profiles for (required)."),
			}, "site_id")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[struct {
				OmacIDArg
				SiteID string `json:"site_id"`
			}](args)
			if err != nil {
				return nil, err
			}
			if in.SiteID == "" {
				return nil, fmt.Errorf("site_id is required")
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Wireless().ListMdnsProfile(ctx, omadacID, in.SiteID)
			})
		},
	}
}

// BonjourServiceCreate returns the omada_bonjour_service_create tool. The
// body mirrors the Omada request type wireless.BonjourServiceOpenApiVO.
func (b *Base) BonjourServiceCreate() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_bonjour_service_create",
		Description: "Create a Bonjour service profile for use in Bonjour rules. " + bonjourServiceBodyDesc,
		Group:       "bonjour",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID to create the Bonjour service profile in (required)."),
				"body":      prop("object", "Bonjour service profile; mirrors the Omada request type wireless.BonjourServiceOpenApiVO."),
			}, "site_id", "body")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[struct {
				OmacIDArg
				SiteID string                           `json:"site_id"`
				Body   wireless.BonjourServiceOpenApiVO `json:"body"`
			}](args)
			if err != nil {
				return nil, err
			}
			if in.SiteID == "" {
				return nil, fmt.Errorf("site_id is required")
			}
			if err := validateBonjourService(&in.Body); err != nil {
				return nil, err
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Wireless().CreateMdnsProfile(ctx, omadacID, in.SiteID, in.Body)
			})
		},
	}
}

// BonjourServiceModify returns the omada_bonjour_service_modify tool. The
// body mirrors the Omada request type wireless.BonjourServiceOpenApiVO.
func (b *Base) BonjourServiceModify() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_bonjour_service_modify",
		Description: "Modify a Bonjour service profile; send the full entity, not just the changed fields. " + bonjourServiceBodyDesc,
		Group:       "bonjour",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = bonjourIDSchema("The Bonjour service profile ID, from omada_bonjour_service_list (required).", map[string]any{
				"body": prop("object", "Bonjour service profile; mirrors the Omada request type wireless.BonjourServiceOpenApiVO."),
			}, "body")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[struct {
				bonjourIDIn
				Body wireless.BonjourServiceOpenApiVO `json:"body"`
			}](args)
			if err != nil {
				return nil, err
			}
			if err := validateBonjourID(&in.bonjourIDIn); err != nil {
				return nil, err
			}
			if err := validateBonjourService(&in.Body); err != nil {
				return nil, err
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Wireless().ModifyMdnsProfile(ctx, omadacID, in.SiteID, in.ID, in.Body)
			})
		},
	}
}

// BonjourServiceDelete returns the omada_bonjour_service_delete tool.
func (b *Base) BonjourServiceDelete() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_bonjour_service_delete",
		Description: "Delete a Bonjour service profile by ID. The controller refuses to delete a profile that a Bonjour rule still uses.",
		Group:       "bonjour",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = bonjourIDSchema("The Bonjour service profile ID, from omada_bonjour_service_list (required).", nil)
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[bonjourIDIn](args)
			if err != nil {
				return nil, err
			}
			if err := validateBonjourID(in); err != nil {
				return nil, err
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Wireless().DeleteMdnsProfile(ctx, omadacID, in.SiteID, in.ID)
			})
		},
	}
}

// BonjourRuleList returns the omada_bonjour_rule_list tool: the site's
// Bonjour forwarding rules (Omada: Settings → Services → Bonjour Service).
func (b *Base) BonjourRuleList() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_bonjour_rule_list",
		Description: "List the site's Bonjour (mDNS forwarding) rules with paging (Omada: Settings → Services → Bonjour Service). A rule forwards the selected Bonjour services from service networks to client networks.",
		Group:       "bonjour",
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID to list Bonjour rules for (required)."),
				"page":      prop("integer", "Page number, starting at 1 (default 1)."),
				"page_size": prop("integer", "Entries per page, 1-1000 (default 20)."),
			}, "site_id")
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
				return b.Omc.CloudAPI().Lan().GetMdnsGrid(ctx, omadacID, in.SiteID, &lan.GetMdnsGridParams{
					Page:     int32(page),
					PageSize: int32(pageSize),
				})
			})
		},
	}
}

// BonjourRuleCreate returns the omada_bonjour_rule_create tool. The body
// mirrors the Omada request type lan.CreateMdnsRuleOpenApiVO.
func (b *Base) BonjourRuleCreate() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_bonjour_rule_create",
		Description: "Create a Bonjour (mDNS forwarding) rule. " + bonjourRuleBodyDesc,
		Group:       "bonjour",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID to create the Bonjour rule in (required)."),
				"body":      prop("object", "Bonjour rule; mirrors the Omada request type lan.CreateMdnsRuleOpenApiVO."),
			}, "site_id", "body")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[struct {
				OmacIDArg
				SiteID string                      `json:"site_id"`
				Body   lan.CreateMdnsRuleOpenApiVO `json:"body"`
			}](args)
			if err != nil {
				return nil, err
			}
			if in.SiteID == "" {
				return nil, fmt.Errorf("site_id is required")
			}
			if err := validateBonjourRule(&in.Body); err != nil {
				return nil, err
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Lan().CreateMdns(ctx, omadacID, in.SiteID, in.Body)
			})
		},
	}
}

// BonjourRuleModify returns the omada_bonjour_rule_modify tool. The body
// mirrors the Omada request type lan.CreateMdnsRuleOpenApiVO.
func (b *Base) BonjourRuleModify() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_bonjour_rule_modify",
		Description: "Modify a Bonjour (mDNS forwarding) rule; send the full entity, not just the changed fields. " + bonjourRuleBodyDesc,
		Group:       "bonjour",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = bonjourIDSchema("The Bonjour rule ID, from omada_bonjour_rule_list (required).", map[string]any{
				"body": prop("object", "Bonjour rule; mirrors the Omada request type lan.CreateMdnsRuleOpenApiVO."),
			}, "body")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[struct {
				bonjourIDIn
				Body lan.CreateMdnsRuleOpenApiVO `json:"body"`
			}](args)
			if err != nil {
				return nil, err
			}
			if err := validateBonjourID(&in.bonjourIDIn); err != nil {
				return nil, err
			}
			if err := validateBonjourRule(&in.Body); err != nil {
				return nil, err
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Lan().ModifyMdns(ctx, omadacID, in.SiteID, in.ID, in.Body)
			})
		},
	}
}

// BonjourRuleDelete returns the omada_bonjour_rule_delete tool.
func (b *Base) BonjourRuleDelete() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_bonjour_rule_delete",
		Description: "Delete a Bonjour (mDNS forwarding) rule by ID.",
		Group:       "bonjour",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = bonjourIDSchema("The Bonjour rule ID, from omada_bonjour_rule_list (required).", nil)
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[bonjourIDIn](args)
			if err != nil {
				return nil, err
			}
			if err := validateBonjourID(in); err != nil {
				return nil, err
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Lan().DeleteMdns(ctx, omadacID, in.SiteID, in.ID)
			})
		},
	}
}

func validateBonjourService(body *wireless.BonjourServiceOpenApiVO) error {
	switch {
	case body.Name == "":
		return fmt.Errorf("body.name is required")
	case len(body.ServiceIds) == 0:
		return fmt.Errorf("body.serviceIds needs at least one service ID")
	case len(body.ServiceIds) > 3:
		return fmt.Errorf("body.serviceIds allows at most 3 service IDs")
	}
	return nil
}

func validateBonjourRule(body *lan.CreateMdnsRuleOpenApiVO) error {
	switch {
	case body.Name == "":
		return fmt.Errorf("body.name is required")
	case len(body.ProfileIds) == 0:
		return fmt.Errorf("body.profileIds needs at least one Bonjour service profile ID")
	}
	if body.Type != nil && *body.Type == 1 {
		if body.Osg == nil || len(body.Osg.ServiceNetworks) == 0 || len(body.Osg.ClientNetworks) == 0 {
			return fmt.Errorf("body.osg.serviceNetworks and body.osg.clientNetworks are required when type is 1")
		}
	}
	if body.Type == nil || *body.Type == 0 {
		if body.Ap == nil || body.Ap.ServiceVlan == "" || body.Ap.ClientVlan == "" {
			return fmt.Errorf("body.ap.serviceVlan and body.ap.clientVlan are required when type is 0")
		}
	}
	return nil
}
