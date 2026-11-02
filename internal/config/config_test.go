// Copyright 2026 Ronald Philipsen. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package config_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/RonaldPhilipsen/omada-mcp/internal/config"
)

// TestFromEnv covers every baked-in semantic: the four required vars,
// literal-string opt-ins (READ_ONLY / ENABLE_DISPATCH / INSECURE_TLS),
// tool-group parsing, transport validation, and port bounds.
func TestFromEnv(t *testing.T) {
	t.Parallel()

	base := map[string]string{
		"OMADA_BASE_URL":      "https://omada.example.com:8043",
		"OMADA_CLIENT_ID":     "client-1",
		"OMADA_CLIENT_SECRET": "secret-1",
		"OMADA_OMADAC_ID":     "omadac-1",
	}

	with := func(m map[string]string, k, v string) map[string]string {
		out := make(map[string]string, len(m)+1)
		for key, val := range m {
			out[key] = val
		}
		out[k] = v
		return out
	}

	without := func(m map[string]string, k string) map[string]string {
		out := make(map[string]string, len(m)-1)
		for key, val := range m {
			if key != k {
				out[key] = val
			}
		}
		return out
	}

	defaults := config.Config{
		BaseURL:      "https://omada.example.com:8043",
		OmadacID:     "omadac-1",
		ClientID:     "client-1",
		ClientSecret: "secret-1",
		ReadOnly:     true,
		Transport:    "streamable-http",
		Port:         8080,
	}

	cases := []struct {
		name    string
		env     map[string]string
		want    config.Config
		wantErr []string
	}{
		{
			name: "defaults with all required present",
			env:  base,
			want: defaults,
		},
		{
			name:    "missing OMADA_BASE_URL",
			env:     without(base, "OMADA_BASE_URL"),
			wantErr: []string{"OMADA_BASE_URL"},
		},
		{
			name:    "missing OMADA_CLIENT_ID",
			env:     without(base, "OMADA_CLIENT_ID"),
			wantErr: []string{"OMADA_CLIENT_ID"},
		},
		{
			name:    "missing OMADA_CLIENT_SECRET",
			env:     without(base, "OMADA_CLIENT_SECRET"),
			wantErr: []string{"OMADA_CLIENT_SECRET"},
		},
		{
			name:    "missing OMADA_OMADAC_ID",
			env:     without(base, "OMADA_OMADAC_ID"),
			wantErr: []string{"OMADA_OMADAC_ID"},
		},
		{
			name:    "multiple missing vars all named",
			env:     without(without(base, "OMADA_BASE_URL"), "OMADA_OMADAC_ID"),
			wantErr: []string{"OMADA_BASE_URL", "OMADA_OMADAC_ID"},
		},
		{
			name: "READ_ONLY=false unlocks writes",
			env:  with(base, "OMADA_READ_ONLY", "false"),
			want: mutate(defaults, func(c *config.Config) { c.ReadOnly = false }),
		},
		{
			name: "READ_ONLY=0 stays read-only",
			env:  with(base, "OMADA_READ_ONLY", "0"),
			want: defaults,
		},
		{
			name: "READ_ONLY=true stays read-only",
			env:  with(base, "OMADA_READ_ONLY", "true"),
			want: defaults,
		},
		{
			name: "READ_ONLY=FALSE stays read-only (case-sensitive)",
			env:  with(base, "OMADA_READ_ONLY", "FALSE"),
			want: defaults,
		},
		{
			name: "READ_ONLY with surrounding space stays read-only",
			env:  with(base, "OMADA_READ_ONLY", " false"),
			want: defaults,
		},
		{
			name: "TOOL_GROUPS trims entries and drops empties",
			env:  with(base, "OMADA_TOOL_GROUPS", "device, site "),
			want: mutate(defaults, func(c *config.Config) {
				c.ToolGroups = []string{"device", "site"}
			}),
		},
		{
			name: "TOOL_GROUPS empty string means all (nil)",
			env:  with(base, "OMADA_TOOL_GROUPS", ""),
			want: defaults,
		},
		{
			name: "TOOL_GROUPS only commas means all (nil)",
			env:  with(base, "OMADA_TOOL_GROUPS", ",,,"),
			want: defaults,
		},
		{
			name: "TOOL_GROUPS missing means all (nil)",
			env:  base,
			want: defaults,
		},
		{
			name: "ENABLE_DISPATCH=true enables",
			env:  with(base, "OMADA_ENABLE_DISPATCH", "true"),
			want: mutate(defaults, func(c *config.Config) { c.EnableDispatch = true }),
		},
		{
			name: "ENABLE_DISPATCH=false stays off",
			env:  with(base, "OMADA_ENABLE_DISPATCH", "false"),
			want: defaults,
		},
		{
			name: "ENABLE_DISPATCH=1 stays off (literal only)",
			env:  with(base, "OMADA_ENABLE_DISPATCH", "1"),
			want: defaults,
		},
		{
			name: "INSECURE_TLS=true enables",
			env:  with(base, "OMADA_INSECURE_TLS", "true"),
			want: mutate(defaults, func(c *config.Config) { c.InsecureTLS = true }),
		},
		{
			name: "INSECURE_TLS=false stays off",
			env:  with(base, "OMADA_INSECURE_TLS", "false"),
			want: defaults,
		},
		{
			name: "INSECURE_TLS=1 stays off (literal only)",
			env:  with(base, "OMADA_INSECURE_TLS", "1"),
			want: defaults,
		},
		{
			name: "transport explicit streamable-http",
			env:  with(base, "OMADA_TRANSPORT", "streamable-http"),
			want: defaults,
		},
		{
			name: "transport stdio",
			env:  with(base, "OMADA_TRANSPORT", "stdio"),
			want: mutate(defaults, func(c *config.Config) { c.Transport = "stdio" }),
		},
		{
			name:    "transport stdio with OMADA_PORT set rejected",
			env:     with(with(base, "OMADA_TRANSPORT", "stdio"), "OMADA_PORT", "9090"),
			wantErr: []string{"OMADA_PORT", "stdio"},
		},
		{
			name: "transport stdio with empty OMADA_PORT ok",
			env:  with(with(base, "OMADA_TRANSPORT", "stdio"), "OMADA_PORT", ""),
			want: mutate(defaults, func(c *config.Config) { c.Transport = "stdio" }),
		},
		{
			name:    "transport bogus rejected",
			env:     with(base, "OMADA_TRANSPORT", "sse"),
			wantErr: []string{"OMADA_TRANSPORT", "sse"},
		},
		{
			name:    "transport STDIO rejected (case-sensitive)",
			env:     with(base, "OMADA_TRANSPORT", "STDIO"),
			wantErr: []string{"OMADA_TRANSPORT"},
		},
		{
			name: "port 1 accepted",
			env:  with(base, "OMADA_PORT", "1"),
			want: mutate(defaults, func(c *config.Config) { c.Port = 1 }),
		},
		{
			name: "port 65535 accepted",
			env:  with(base, "OMADA_PORT", "65535"),
			want: mutate(defaults, func(c *config.Config) { c.Port = 65535 }),
		},
		{
			name:    "port 0 rejected",
			env:     with(base, "OMADA_PORT", "0"),
			wantErr: []string{"OMADA_PORT"},
		},
		{
			name:    "port 65536 rejected",
			env:     with(base, "OMADA_PORT", "65536"),
			wantErr: []string{"OMADA_PORT"},
		},
		{
			name:    "port negative rejected",
			env:     with(base, "OMADA_PORT", "-1"),
			wantErr: []string{"OMADA_PORT"},
		},
		{
			name:    "port non-numeric rejected",
			env:     with(base, "OMADA_PORT", "abc"),
			wantErr: []string{"OMADA_PORT"},
		},
		{
			name: "port with surrounding space parsed",
			env:  with(base, "OMADA_PORT", " 8080 "),
			want: defaults,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			get := func(key string) string { return tc.env[key] }
			got, err := config.FromEnv(get)
			if len(tc.wantErr) > 0 {
				if err == nil {
					t.Fatalf("FromEnv() error = nil, want error containing %q", tc.wantErr)
				}
				for _, sub := range tc.wantErr {
					if !strings.Contains(err.Error(), sub) {
						t.Errorf("FromEnv() error = %q, want it to contain %q", err.Error(), sub)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("FromEnv() unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("FromEnv() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// mutate returns a copy of c with fn applied to it, so each case keeps
// its own expected value.
func mutate(c config.Config, fn func(*config.Config)) config.Config {
	out := c
	fn(&out)
	return out
}
