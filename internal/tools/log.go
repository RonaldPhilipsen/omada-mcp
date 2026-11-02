package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	omadalog "github.com/RonaldPhilipsen/omada/cloud/log"

	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
	"github.com/RonaldPhilipsen/omada-mcp/internal/omadaclient"
)

// logQueryIn is the argument object shared by the log tools (alerts,
// events). page defaults to 1, page_size to 20, and the time range to the
// last 7 days.
type logQueryIn struct {
	OmacIDArg
	SiteID    string `json:"site_id"`
	Page      int    `json:"page,omitempty"`
	PageSize  int    `json:"page_size,omitempty"`
	StartTime int64  `json:"start_time,omitempty"`
	EndTime   int64  `json:"end_time,omitempty"`
}

// logQueryProps returns the JSON schema properties for the log tools.
func logQueryProps() map[string]any {
	props := map[string]any{
		"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
		"site_id":   prop("string", "The site ID to query (required)."),
		"page":      prop("integer", "Page number, starting at 1 (default 1)."),
		"page_size": prop("integer", "Entries per page (default 20)."),
	}
	for k, v := range timeRangeProps() {
		props[k] = v
	}
	return props
}

// LogAlerts returns the omada_log_alerts tool: the paged list of alert logs
// in a site over a time range. page defaults to 1, page_size to 20, and the
// time range to the last 7 days.
func (b *Base) LogAlerts() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_log_alerts",
		Description: "List alert logs in an Omada site with paging over a time range. page starts at 1 (default 1); page_size is the number of entries per page (default 20); the time range defaults to the last 7 days.",
		Group:       "log",
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(logQueryProps(), "site_id")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[logQueryIn](args)
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
			start, end := defaultTimeWindow(in.StartTime, in.EndTime)
			omadacID, err := b.ResolveOmac(in.OmacIDArg)
			if err != nil {
				return nil, err
			}
			resp, err := b.Omc.CloudAPI().OmadaLog().GetAlertLogsForSite(ctx, omadacID, in.SiteID, &omadalog.GetAlertLogsForSiteParams{
				Page:             int32(in.Page),
				PageSize:         int32(in.PageSize),
				FiltersTimeStart: start,
				FiltersTimeEnd:   end,
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

// LogEvents returns the omada_log_events tool: the paged list of event logs
// in a site over a time range. page defaults to 1, page_size to 20, and the
// time range to the last 7 days.
func (b *Base) LogEvents() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_log_events",
		Description: "List event logs in an Omada site with paging over a time range. page starts at 1 (default 1); page_size is the number of entries per page (default 20); the time range defaults to the last 7 days.",
		Group:       "log",
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(logQueryProps(), "site_id")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[logQueryIn](args)
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
			start, end := defaultTimeWindow(in.StartTime, in.EndTime)
			omadacID, err := b.ResolveOmac(in.OmacIDArg)
			if err != nil {
				return nil, err
			}
			resp, err := b.Omc.CloudAPI().OmadaLog().GetEventLogsForSite(ctx, omadacID, in.SiteID, &omadalog.GetEventLogsForSiteParams{
				Page:             int32(in.Page),
				PageSize:         int32(in.PageSize),
				FiltersTimeStart: start,
				FiltersTimeEnd:   end,
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
