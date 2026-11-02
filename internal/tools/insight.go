package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/RonaldPhilipsen/omada/cloud/insight"

	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
	"github.com/RonaldPhilipsen/omada-mcp/internal/omadaclient"
)

// defaultTimeWindow returns the start and end time (in milliseconds) for a
// query time range: the given values when positive, otherwise the last 7
// days ending now.
func defaultTimeWindow(start, end int64) (int64, int64) {
	now := time.Now()
	if end <= 0 {
		end = now.UnixMilli()
	}
	if start <= 0 {
		start = now.Add(-7 * 24 * time.Hour).UnixMilli()
	}
	return start, end
}

// timeRangeProps returns the JSON schema properties for the optional
// start_time and end_time query arguments (epoch milliseconds).
func timeRangeProps() map[string]any {
	return map[string]any{
		"start_time": prop("integer", "Query start time in epoch milliseconds (default: 7 days ago)."),
		"end_time":   prop("integer", "Query end time in epoch milliseconds (default: now)."),
	}
}

// insightOverviewIn is the omada_insight_overview argument object.
type insightOverviewIn struct {
	OmacIDArg
	SiteID    string `json:"site_id"`
	StartTime int64  `json:"start_time,omitempty"`
	EndTime   int64  `json:"end_time,omitempty"`
}

// InsightOverview returns the omada_insight_overview tool: the anomaly
// overview statistics for a site over a time range. The time range
// defaults to the last 7 days.
func (b *Base) InsightOverview() mcpkit.Tool {
	props := map[string]any{
		"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
		"site_id":   prop("string", "The site ID to query (required)."),
	}
	for k, v := range timeRangeProps() {
		props[k] = v
	}
	return mcpkit.Tool{
		Name:        "omada_insight_overview",
		Description: "Get the anomaly overview statistics for an Omada site over a time range. The time range defaults to the last 7 days.",
		Group:       "insight",
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(props, "site_id")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[insightOverviewIn](args)
			if err != nil {
				return nil, err
			}
			if in.SiteID == "" {
				return nil, fmt.Errorf("site_id is required")
			}
			start, end := defaultTimeWindow(in.StartTime, in.EndTime)
			omadacID, err := b.ResolveOmac(in.OmacIDArg)
			if err != nil {
				return nil, err
			}
			resp, err := b.Omc.CloudAPI().Insight().GetOverviewStatistics(ctx, omadacID, in.SiteID, &insight.GetOverviewStatisticsParams{
				FiltersStartTime: start,
				FiltersEndTime:   end,
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

// InsightUnresolvedCount returns the omada_insight_unresolved_count tool:
// the count of unresolved incidents in a site.
func (b *Base) InsightUnresolvedCount() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "omada_insight_unresolved_count",
		Description: "Get the count of unresolved incidents in an Omada site.",
		Group:       "insight",
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
			resp, err := b.Omc.CloudAPI().Insight().GetUnresolvedIncidentCount(ctx, omadacID, in.SiteID)
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

// insightIncidentsIn is the omada_insight_incidents argument object. page
// defaults to 1, page_size to 20, and the time range to the last 7 days.
type insightIncidentsIn struct {
	OmacIDArg
	SiteID    string `json:"site_id"`
	Page      int    `json:"page,omitempty"`
	PageSize  int    `json:"page_size,omitempty"`
	StartTime int64  `json:"start_time,omitempty"`
	EndTime   int64  `json:"end_time,omitempty"`
}

// InsightIncidents returns the omada_insight_incidents tool: the paged list
// of incidents in a site over a time range. page defaults to 1, page_size
// to 20, and the time range to the last 7 days.
func (b *Base) InsightIncidents() mcpkit.Tool {
	props := map[string]any{
		"omadac_id": prop("string", "Omada controller ID; defaults to the configured OMADA_OMADAC_ID."),
		"site_id":   prop("string", "The site ID to query (required)."),
		"page":      prop("integer", "Page number, starting at 1 (default 1)."),
		"page_size": prop("integer", "Entries per page (default 20)."),
	}
	for k, v := range timeRangeProps() {
		props[k] = v
	}
	return mcpkit.Tool{
		Name:        "omada_insight_incidents",
		Description: "List incidents in an Omada site with paging over a time range. page starts at 1 (default 1); page_size is the number of entries per page (default 20); the time range defaults to the last 7 days.",
		Group:       "insight",
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(props, "site_id")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[insightIncidentsIn](args)
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
			resp, err := b.Omc.CloudAPI().Insight().GetGridIncidentList(ctx, omadacID, in.SiteID, &insight.GetGridIncidentListParams{
				Page:             int32(in.Page),
				PageSize:         int32(in.PageSize),
				FiltersStartTime: start,
				FiltersEndTime:   end,
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
