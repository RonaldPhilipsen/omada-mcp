// Package tools is the omada-mcp MCP tool catalog. Each curated group
// (site, client, device, ...) is one file that exports one mcpkit.Tool
// constructor per operation, all built on the shared Base. The site file
// is the pattern exemplar every later curated-group file copies:
//
//  1. a per-tool In struct with OmacIDArg embedded (json tags per the
//     group's inventory table);
//  2. an mcpkit.Tool with Group, Mutating, a Register that sets the
//     input schema, and a Handler;
//  3. the Handler: decode args with mcpkit.OnArgs, validate required
//     fields BEFORE any HTTP, resolve the omadac ID with ResolveOmac,
//     call the generated client's non-WithResponse method,
//     defer resp.Body.Close(), decode the northbound envelope with
//     omadaclient.Envelope, and return the result payload via
//     mcpkit.JSONResult.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/RonaldPhilipsen/omada/cloud/site"

	"github.com/RonaldPhilipsen/omada-mcp/internal/config"
	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
	"github.com/RonaldPhilipsen/omada-mcp/internal/omadaclient"
)

// Base carries the shared controller client and resolved config for
// every tool constructor.
type Base struct {
	Omc *omadaclient.Client
	Cfg config.Config
}

// OmacIDArg is the optional omadac_id argument shared by every tool.
type OmacIDArg struct {
	OmadacID string `json:"omadac_id,omitempty"`
}

// ResolveOmac returns in.OmadacID when it is set, otherwise the
// configured OMADA_OMADAC_ID (b.Omc.OmadacID()). It errors when the
// resolved ID would be blank.
func (b *Base) ResolveOmac(in OmacIDArg) (omadacID string, err error) {
	id := in.OmadacID
	if id == "" {
		id = b.Omc.OmadacID()
	}
	if id == "" {
		return "", fmt.Errorf("omadac_id is required: pass omadac_id or set OMADA_OMADAC_ID")
	}
	return id, nil
}

// objectSchema returns a JSON schema for a tool input: a type object with
// the given properties, the given required fields, and no additional
// properties.
func objectSchema(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// prop returns a JSON schema property object for a field: its type and
// description.
func prop(typ, description string) map[string]any {
	return map[string]any{"type": typ, "description": description}
}

// resultPayload returns the decoded envelope result for the tool result:
// the raw result JSON, or an empty object when the controller sent no
// result (null or absent).
func resultPayload(result json.RawMessage) any {
	if len(result) == 0 || string(result) == "null" {
		return map[string]any{}
	}
	return result
}

// siteListIn is the omada_site_list argument object.
type siteListIn struct {
	OmacIDArg
	Page     int `json:"page,omitempty"`
	PageSize int `json:"page_size,omitempty"`
}

// SiteList returns the omada_site_list tool: the paged list of sites in
// the controller. page defaults to 1 and page_size to 20.
func (b *Base) SiteList() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_site_list",
		Description: "List Omada sites in the controller with paging. page starts at 1 (default 1); page_size is the number of entries per page (default 20).",
		Group:       "site",
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"page":      prop("integer", "Page number, starting at 1 (default 1)."),
				"page_size": prop("integer", "Entries per page (default 20)."),
			})
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[siteListIn](args)
			if err != nil {
				return nil, err
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
			resp, err := b.Omc.CloudAPI().Site().GetSiteList(ctx, omadacID, &site.GetSiteListParams{
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

// siteGetIn is the omada_site_get argument object.
type siteGetIn struct {
	OmacIDArg
	SiteID string `json:"site_id"`
}

// SiteGet returns the omada_site_get tool: one site's full entity by ID.
func (b *Base) SiteGet() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_site_get",
		Description: "Get one Omada site's full entity by site ID.",
		Group:       "site",
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID to fetch (required)."),
			}, "site_id")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[siteGetIn](args)
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
			resp, err := b.Omc.CloudAPI().Site().GetSiteEntity(ctx, omadacID, in.SiteID)
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

// siteCreateIn is the omada_site_create argument object. Body mirrors
// the Omada request type site.CreateSiteEntity (the generated
// site.CreateNewSiteJSONRequestBody).
type siteCreateIn struct {
	OmacIDArg
	Body site.CreateSiteEntity `json:"body"`
}

// SiteCreate returns the omada_site_create tool: create a new site. The
// body mirrors the Omada request type; name, region, scenario, and
// timeZone are the recommended minimum set.
func (b *Base) SiteCreate() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_site_create",
		Description: "Create a new Omada site. body mirrors the Omada request type site.CreateSiteEntity; name, region, scenario, and timeZone are the recommended minimum set.",
		Group:       "site",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"body":      prop("object", "Site entity; mirrors the Omada request type site.CreateSiteEntity."),
			})
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[siteCreateIn](args)
			if err != nil {
				return nil, err
			}
			omadacID, err := b.ResolveOmac(in.OmacIDArg)
			if err != nil {
				return nil, err
			}
			resp, err := b.Omc.CloudAPI().Site().CreateNewSite(ctx, omadacID, in.Body)
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

// siteDeleteIn is the omada_site_delete argument object.
type siteDeleteIn struct {
	OmacIDArg
	SiteID string `json:"site_id"`
}

// SiteDelete returns the omada_site_delete tool: delete a site by ID.
func (b *Base) SiteDelete() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_site_delete",
		Description: "Delete an Omada site by site ID.",
		Group:       "site",
		Mutating:    true,
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
				"site_id":   prop("string", "The site ID to delete (required)."),
			}, "site_id")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[siteDeleteIn](args)
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
			resp, err := b.Omc.CloudAPI().Site().DeleteSite(ctx, omadacID, in.SiteID)
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
