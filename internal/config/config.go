// Package config loads the omada-mcp runtime configuration from the
// environment. Every setting is an OMADA_* env var; there are no
// config files.
package config

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	EnvBaseURL        = "OMADA_BASE_URL"
	EnvClientID       = "OMADA_CLIENT_ID"
	EnvClientSecret   = "OMADA_CLIENT_SECRET"
	EnvOmadacID       = "OMADA_OMADAC_ID"
	EnvInsecureTLS    = "OMADA_INSECURE_TLS"
	EnvReadOnly       = "OMADA_READ_ONLY"
	EnvToolGroups     = "OMADA_TOOL_GROUPS"
	EnvEnableDispatch = "OMADA_ENABLE_DISPATCH"
	EnvTransport      = "OMADA_TRANSPORT"
	EnvPort           = "OMADA_PORT"
)

const (
	TransportStreamableHTTP = "streamable-http"
	TransportStdio          = "stdio"
)

type Config struct {
	BaseURL, OmadacID, ClientID, ClientSecret string
	InsecureTLS                               bool
	ReadOnly                                  bool
	ToolGroups                                []string // nil == all
	EnableDispatch                            bool
	Transport                                 string // "streamable-http" | "stdio"
	Port                                      int
}

// FromEnv parses the OMADA_* environment variables. The getter is
// parameterized so tests can drive it with fixtures instead of the
// process environment.
//
// Semantics: the four required vars (base URL, omadac ID, client ID,
// client secret) must be non-empty; a single combined startup error
// names every missing var. OMADA_READ_ONLY defaults true and only the
// literal string "false" unlocks write tools. OMADA_TOOL_GROUPS is a
// comma-separated allowlist whose entries are trimmed; empty entries
// are dropped and an empty result means all groups (nil).
// OMADA_ENABLE_DISPATCH and OMADA_INSECURE_TLS are opt-ins that only
// the literal string "true" enables. OMADA_TRANSPORT accepts only
// "streamable-http" (default) or "stdio", and stdio rejects an
// explicit OMADA_PORT. OMADA_PORT defaults to 8080 and must be within
// 1-65535.
func FromEnv(get func(envVar string) string) (Config, error) {
	cfg := Config{
		ReadOnly:  true,
		Transport: TransportStreamableHTTP,
		Port:      8080,
	}

	cfg.BaseURL = get(EnvBaseURL)
	cfg.OmadacID = get(EnvOmadacID)
	cfg.ClientID = get(EnvClientID)
	cfg.ClientSecret = get(EnvClientSecret)
	var missing []string
	if cfg.BaseURL == "" {
		missing = append(missing, EnvBaseURL)
	}
	if cfg.OmadacID == "" {
		missing = append(missing, EnvOmadacID)
	}
	if cfg.ClientID == "" {
		missing = append(missing, EnvClientID)
	}
	if cfg.ClientSecret == "" {
		missing = append(missing, EnvClientSecret)
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	cfg.InsecureTLS = get(EnvInsecureTLS) == "true"
	cfg.ReadOnly = get(EnvReadOnly) != "false"
	cfg.EnableDispatch = get(EnvEnableDispatch) == "true"
	cfg.ToolGroups = parseToolGroups(get(EnvToolGroups))

	transport := get(EnvTransport)
	if transport == "" {
		transport = TransportStreamableHTTP
	}
	switch transport {
	case TransportStreamableHTTP, TransportStdio:
		cfg.Transport = transport
	default:
		return Config{}, fmt.Errorf("unsupported %s %q (want %q or %q)",
			EnvTransport, transport, TransportStreamableHTTP, TransportStdio)
	}

	portRaw := strings.TrimSpace(get(EnvPort))
	if portRaw != "" {
		if cfg.Transport == TransportStdio {
			return Config{}, fmt.Errorf("%s is not supported with %s=%s", EnvPort, EnvTransport, TransportStdio)
		}
		port, err := strconv.Atoi(portRaw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid %s %q: not a number", EnvPort, portRaw)
		}
		if port < 1 || port > 65535 {
			return Config{}, fmt.Errorf("%s %d out of range (1-65535)", EnvPort, port)
		}
		cfg.Port = port
	}

	return cfg, nil
}

// parseToolGroups splits a comma-separated group list, trims each
// entry, and drops empty entries. It returns nil when no groups
// remain, which means all groups.
func parseToolGroups(raw string) []string {
	var groups []string
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			groups = append(groups, p)
		}
	}
	return groups
}
