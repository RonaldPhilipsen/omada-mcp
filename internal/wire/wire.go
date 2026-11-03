// Package wire assembles the omada-mcp server from configuration and the
// Omada client: every curated group's tools registered through a shared
// tools.Base, the dispatch escape hatch only when enabled, into an
// mcpkit.Server gated by the config's read-only flag and group allowlist.
package wire

import (
	"github.com/RonaldPhilipsen/omada-mcp/internal/config"
	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
	"github.com/RonaldPhilipsen/omada-mcp/internal/omadaclient"
	"github.com/RonaldPhilipsen/omada-mcp/internal/tools"
)

const ServerName = "omada"

func Build(cfg config.Config, omc *omadaclient.Client) (*mcpkit.Server, error) {
	b := &tools.Base{Omc: omc, Cfg: cfg}
	s := mcpkit.NewServer(ServerName, mcpkit.Options{
		ReadOnly:   cfg.ReadOnly,
		ToolGroups: cfg.ToolGroups,
	})

	all := []mcpkit.Tool{
		b.SiteList(), b.SiteGet(), b.SiteCreate(), b.SiteDelete(),
		b.ClientList(), b.ClientGet(), b.ClientBlock(), b.ClientUnblock(), b.ClientReconnect(),
		b.DeviceList(), b.DevicesBySite(), b.DeviceReboot(), b.TopologyGet(),
		b.LanNetworksList(), b.LanNetworkGet(), b.LanNetworkCreate(), b.LanNetworkDelete(), b.DhcpList(),
		b.VpnList(), b.VpnS2sList(), b.VpnC2sServersList(), b.VpnS2sModify(),
		b.InsightOverview(), b.InsightUnresolvedCount(), b.InsightIncidents(),
		b.LogAlerts(), b.LogEvents(),
	}
	for _, tl := range all {
		s.Register(tl)
	}

	if cfg.EnableDispatch {
		s.Register(b.OpsLookup())
		s.Register(b.OpsCall())
	}

	return s, nil
}
