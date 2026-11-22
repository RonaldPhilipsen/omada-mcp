package tools

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/RonaldPhilipsen/omada/cloud/wireless"

	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
)

// groupProfileTypes are the group profile type values the Omada API
// accepts in the {groupType} path segment and the body's type field.
const groupProfileTypes = "0=IP group, 1=IP-port group, 2=MAC group, 3=IPv6 group, 4=IPv6-port group, 5=country group, 7=domain group"

// groupProfileBodyDesc documents the CreateGroupOpenApiVO body shared by
// the create and modify tools.
const groupProfileBodyDesc = "Group profile; mirrors the Omada request type wireless.CreateGroupOpenApiVO. " +
	"name (required), type (int, required): " + groupProfileTypes + ". " +
	"ipList: list of {ip, mask, description?} for type 0 or 1. portList: list of port strings (e.g. \"53\", \"80-100\") for type 1 or 4 with portType 0. " +
	"portType (int): 0=port range, 1=port mask (portMaskList), required for type 1 or 4. macAddressList: list of {macAddress, name?} for type 2. " +
	"ipv6List: list of {ip, prefix} for type 3 or 4. countryList for type 5. domainNamePort: list of {address, port?} for type 7."

// validGroupType reports whether s is one of the Omada group profile
// type values.
func validGroupType(s string) bool {
	n, err := strconv.Atoi(s)
	if err != nil {
		return false
	}
	switch n {
	case 0, 1, 2, 3, 4, 5, 7:
		return true
	}
	return false
}

type groupProfileIDIn struct {
	OmacIDArg
	SiteID    string `json:"site_id"`
	GroupType string `json:"group_type"`
	GroupID   string `json:"group_id"`
}

func validateGroupProfileID(in *groupProfileIDIn) error {
	if in.SiteID == "" {
		return fmt.Errorf("site_id is required")
	}
	if !validGroupType(in.GroupType) {
		return fmt.Errorf("group_type must be one of: %s", groupProfileTypes)
	}
	if in.GroupID == "" {
		return fmt.Errorf("group_id is required")
	}
	return nil
}

func groupProfileIDSchema(extra map[string]any, required ...string) map[string]any {
	props := map[string]any{
		"omadac_id":  prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
		"site_id":    prop("string", "The site ID the group profile belongs to (required)."),
		"group_type": prop("string", "The group profile type (required): "+groupProfileTypes+"."),
		"group_id":   prop("string", "The group profile ID to act on, from omada_group_profile_list (required)."),
	}
	for k, v := range extra {
		props[k] = v
	}
	return objectSchema(props, append([]string{"site_id", "group_type", "group_id"}, required...)...)
}

// GroupProfileList returns the omada_group_profile_list tool: the site's
// group profiles (Omada: Settings → Profiles → Groups), optionally
// filtered by type.
func (b *Base) GroupProfileList() mcpkit.Tool {
	return mcpkit.Tool{
		Name: "omada_group_profile_list",
		Description: "List the site's group profiles (Omada: Settings → Profiles → Groups): the IP, IP-port, MAC, IPv6, country and domain groups that ACL rules reference by ID. " +
			"Pass group_type to list one type only (" + groupProfileTypes + ").",
		Group: "security",
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id":  prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":    prop("string", "The site ID to list group profiles for (required)."),
				"group_type": prop("string", "Optional group profile type filter: "+groupProfileTypes+"."),
			}, "site_id")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[struct {
				OmacIDArg
				SiteID    string `json:"site_id"`
				GroupType string `json:"group_type"`
			}](args)
			if err != nil {
				return nil, err
			}
			if in.SiteID == "" {
				return nil, fmt.Errorf("site_id is required")
			}
			if in.GroupType != "" && !validGroupType(in.GroupType) {
				return nil, fmt.Errorf("group_type must be one of: %s", groupProfileTypes)
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				if in.GroupType == "" {
					return b.Omc.CloudAPI().Wireless().GetGroupProfiles(ctx, omadacID, in.SiteID)
				}
				return b.Omc.CloudAPI().Wireless().GetGroupProfilesByType(ctx, omadacID, in.SiteID, in.GroupType)
			})
		},
	}
}

// GroupProfileCreate returns the omada_group_profile_create tool. The body
// mirrors the Omada request type wireless.CreateGroupOpenApiVO.
func (b *Base) GroupProfileCreate() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_group_profile_create",
		Description: "Create a group profile for use in ACL rules. " + groupProfileBodyDesc,
		Group:       "security",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID to create the group profile in (required)."),
				"body":      prop("object", "Group profile; mirrors the Omada request type wireless.CreateGroupOpenApiVO."),
			}, "site_id", "body")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[struct {
				OmacIDArg
				SiteID string                        `json:"site_id"`
				Body   wireless.CreateGroupOpenApiVO `json:"body"`
			}](args)
			if err != nil {
				return nil, err
			}
			if in.SiteID == "" {
				return nil, fmt.Errorf("site_id is required")
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Wireless().CreateGroupProfile(ctx, omadacID, in.SiteID, in.Body)
			})
		},
	}
}

// GroupProfileModify returns the omada_group_profile_modify tool. The body
// mirrors the Omada request type wireless.CreateGroupOpenApiVO.
func (b *Base) GroupProfileModify() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_group_profile_modify",
		Description: "Modify a group profile; include the full entity, not just the changed fields. " + groupProfileBodyDesc,
		Group:       "security",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = groupProfileIDSchema(map[string]any{
				"body": prop("object", "Group profile; mirrors the Omada request type wireless.CreateGroupOpenApiVO."),
			}, "body")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[struct {
				groupProfileIDIn
				Body wireless.CreateGroupOpenApiVO `json:"body"`
			}](args)
			if err != nil {
				return nil, err
			}
			if err := validateGroupProfileID(&in.groupProfileIDIn); err != nil {
				return nil, err
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Wireless().ModifyGroupProfile(ctx, omadacID, in.SiteID, in.GroupType, in.GroupID, in.Body)
			})
		},
	}
}

// GroupProfileDelete returns the omada_group_profile_delete tool.
func (b *Base) GroupProfileDelete() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_group_profile_delete",
		Description: "Delete a group profile by type and ID. Get both from omada_group_profile_list.",
		Group:       "security",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = groupProfileIDSchema(nil)
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[groupProfileIDIn](args)
			if err != nil {
				return nil, err
			}
			if err := validateGroupProfileID(in); err != nil {
				return nil, err
			}
			return securityCall(b, in.OmacIDArg, func(omadacID string) (*http.Response, error) {
				return b.Omc.CloudAPI().Wireless().DeleteGroupProfile(ctx, omadacID, in.SiteID, in.GroupType, in.GroupID)
			})
		},
	}
}
