// Copyright 2026 Ronald Philipsen. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package opsindex_test

import (
	"strings"
	"testing"

	"github.com/RonaldPhilipsen/omada-mcp/internal/opsindex"
)

// curatedOps are representative operations from the curated tool groups
// built in tasks 4-9 (internal/tools). Each row pins the spec operationId
// behind one tool, its HTTP method, and its mutability classification.
var curatedOps = []struct {
	tool     string
	id       string
	method   string
	mutating bool
}{
	// site
	{"omada_site_list", "getSiteList", "GET", false},
	{"omada_site_get", "getSiteEntity", "GET", false},
	{"omada_site_create", "createNewSite", "POST", true},
	{"omada_site_delete", "deleteSite", "DELETE", true},
	// client
	{"omada_client_list", "getGridAllClients", "POST", false},
	{"omada_client_get", "getClientDetail", "GET", false},
	{"omada_client_block", "blockClient", "POST", true},
	// device
	{"omada_device_list", "getDeviceList", "GET", false},
	{"omada_device_reboot", "rebootDevice", "POST", true},
	{"omada_topology_get", "getTopology", "GET", false},
	// lan
	{"omada_lan_network_create", "createLanNetwork", "POST", true},
	{"omada_lan_network_delete", "deleteLanNetwork", "DELETE", true},
	{"omada_lan_network_get", "getLanNetwork", "GET", false},
	// vpn
	{"omada_vpn_s2s_list", "getSiteToSiteVpnList", "GET", false},
	{"omada_vpn_s2s_modify", "modifySiteToSiteVpn", "PATCH", true},
	// insight
	{"omada_insight_overview", "getOverviewStatistics", "GET", false},
	// log
	{"omada_log_alerts", "getAlertLogsForSite", "GET", false},
	{"omada_log_events", "getEventLogsForSite", "GET", false},
}

// TestOpsIndexCoversGroups asserts that every curated tool operation from
// tasks 4-9 is present in the generated index with the correct method and
// mutability classification.
func TestOpsIndexCoversGroups(t *testing.T) {
	for _, row := range curatedOps {
		op, ok := opsindex.Lookup(row.id)
		if !ok {
			t.Errorf("%s: Lookup(%q) not found in Ops", row.tool, row.id)
			continue
		}
		if op.Method != row.method {
			t.Errorf("%s: %s method = %s, want %s", row.tool, row.id, op.Method, row.method)
		}
		if op.Mutating != row.mutating {
			t.Errorf("%s: %s mutating = %v, want %v", row.tool, row.id, op.Mutating, row.mutating)
		}
	}
}

// TestMutatingRule pins the generator's mutability rule across the whole
// index: GET ops are never mutating; DELETE and PUT ops are always
// mutating; non-GET ops whose operationId begins with "Get" (case
// insensitive) are non-mutating read-style endpoints.
func TestMutatingRule(t *testing.T) {
	if len(opsindex.Ops) == 0 {
		t.Fatal("Ops is empty; the generated index is missing")
	}
	getPrefixNonGET := 0
	for _, op := range opsindex.Ops {
		switch {
		case op.Method == "GET":
			if op.Mutating {
				t.Errorf("GET op %q is mutating; GET ops are never mutating", op.ID)
			}
		case op.Method == "DELETE" || op.Method == "PUT":
			if !op.Mutating {
				t.Errorf("%s op %q is not mutating; DELETE/PUT ops are always mutating", op.Method, op.ID)
			}
		}
		if op.Method != "GET" && strings.HasPrefix(strings.ToLower(op.ID), "get") {
			getPrefixNonGET++
			if op.Mutating {
				t.Errorf("op %q begins with Get but is mutating; read-style Get ops are non-mutating", op.ID)
			}
		}
	}
	if getPrefixNonGET == 0 {
		t.Error("expected at least one non-GET op whose ID begins with Get (read-style search endpoint)")
	}
}

// TestOpsSortedAndUnique pins the generator's determinism contract: the
// emitted Ops slice is sorted by ID with no duplicates, which is what
// makes Lookup well-defined and the output reproducible.
func TestOpsSortedAndUnique(t *testing.T) {
	for i := 1; i < len(opsindex.Ops); i++ {
		if opsindex.Ops[i-1].ID >= opsindex.Ops[i].ID {
			t.Fatalf("Ops not sorted by ID (or duplicate ID): %q >= %q at index %d",
				opsindex.Ops[i-1].ID, opsindex.Ops[i].ID, i)
		}
	}
}
