// dispatch.go is the generic dispatch escape hatch: ops_lookup searches
// the full generated northbound operation index and ops_call executes one
// operation by its index operationId, going straight through the client's
// auth transport. Unlike the curated groups, these two tools are not
// gated by group or read-only at registration (both are Group "" and
// non-mutating); the wire registers them only when
// config.EnableDispatch is set, and ops_call enforces read-only per op at
// call time.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
	"github.com/RonaldPhilipsen/omada-mcp/internal/omadaclient"
	"github.com/RonaldPhilipsen/omada-mcp/internal/opsindex"
)

// OpsLookupIn is the ops_lookup argument object.
type OpsLookupIn struct {
	Keyword string `json:"keyword,omitempty"`
	Tag     string `json:"tag,omitempty"`
	Limit   int    `json:"limit,omitempty"` // default 10, cap 50
}

// OpsCallIn is the ops_call argument object.
type OpsCallIn struct {
	OperationID string            `json:"operation_id"` // required
	PathParams  map[string]string `json:"path_params,omitempty"`
	Query       map[string]string `json:"query,omitempty"`
	Body        json.RawMessage   `json:"body,omitempty"`
}

// pathParamRe matches a single {placeholder} in an operation path.
var pathParamRe = regexp.MustCompile(`\{([A-Za-z0-9_]+)\}`)

// OpsLookup returns the ops_lookup tool: a read-only search over the
// generated operation index. keyword is a case-insensitive substring match
// across id, summary, and tag; tag narrows exclusively to that exact tag
// (case-insensitive); results are ordered by id and limited (default 10,
// cap 50). Each hit carries {id, method, path, tag, summary, mutating}.
func (b *Base) OpsLookup() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "ops_lookup",
		Description: "Search the Omada northbound operation index. keyword is a case-insensitive substring across operation id, summary, and tag; tag narrows to an exact tag; results are ordered by id and limited (default 10, max 50). Use the returned operation_id with ops_call.",
		Group:       "",
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"keyword": prop("string", "Case-insensitive substring to match across operation id, summary, and tag."),
				"tag":     prop("string", "Narrow to operations with this exact tag (case-insensitive)."),
				"limit":   prop("integer", "Maximum number of results (default 10, max 50)."),
			})
		},
		Handler: func(_ context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[OpsLookupIn](args)
			if err != nil {
				return nil, err
			}
			limit := in.Limit
			if limit <= 0 {
				limit = 10
			}
			if limit > 50 {
				limit = 50
			}

			hits := make([]map[string]any, 0, limit)
			for _, op := range opsindex.Ops {
				if in.Tag != "" && !strings.EqualFold(op.Tag, in.Tag) {
					continue
				}
				if in.Keyword != "" && !opMatchesKeyword(op, in.Keyword) {
					continue
				}
				hits = append(hits, opHit(op))
				if len(hits) >= limit {
					break
				}
			}
			return mcpkit.JSONResult(hits)
		},
	}
}

// opMatchesKeyword reports whether kw (already non-empty) is a
// case-insensitive substring of the op's id, summary, or tag.
func opMatchesKeyword(op opsindex.Op, kw string) bool {
	kw = strings.ToLower(kw)
	return strings.Contains(strings.ToLower(op.ID), kw) ||
		strings.Contains(strings.ToLower(op.Summary), kw) ||
		strings.Contains(strings.ToLower(op.Tag), kw)
}

// opHit is one ops_lookup result row.
func opHit(op opsindex.Op) map[string]any {
	return map[string]any{
		"id":       op.ID,
		"method":   op.Method,
		"path":     op.Path,
		"tag":      op.Tag,
		"summary":  op.Summary,
		"mutating": op.Mutating,
	}
}

// OpsCall returns the ops_call tool: execute one operation by its index
// operationId through the client's auth transport. {omadacId} is filled
// from path_params or the configured OMADA_OMADAC_ID; every other
// {placeholder} must be supplied in path_params. In read-only mode a
// mutating op is rejected before any HTTP request.
func (b *Base) OpsCall() mcpkit.Tool {
	return mcpkit.Tool{
		Name:        "ops_call",
		Description: "Execute one Omada northbound operation by its index operation_id. path_params fills {placeholder} path segments ({omadacId} defaults to the configured OMADA_OMADAC_ID); query adds query parameters; body is the raw JSON request body. In read-only mode, mutating operations are rejected.",
		Group:       "",
		Register: func(t *mcp.Tool) {
			t.InputSchema = objectSchema(map[string]any{
				"operation_id": prop("string", "The operation id from ops_lookup to execute (required)."),
				"path_params":  prop("object", "Map of {placeholder} names to values for the path. {omadacId} defaults to the configured OMADA_OMADAC_ID."),
				"query":        prop("object", "Map of query parameter names to values."),
				"body":         prop("object", "Raw JSON request body for methods that take one."),
			}, "operation_id")
		},
		Handler: func(ctx context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[OpsCallIn](args)
			if err != nil {
				return nil, err
			}
			if in.OperationID == "" {
				return nil, fmt.Errorf("ops_call: operation_id is required")
			}
			op, ok := opsindex.Lookup(in.OperationID)
			if !ok {
				return nil, fmt.Errorf("ops_call: unknown operation_id %q", in.OperationID)
			}

			// Read-only is enforced per op, before any HTTP request.
			if b.Cfg.ReadOnly && op.Mutating {
				return nil, fmt.Errorf("ops_call: operation %q is mutating and the server is in read-only mode", in.OperationID)
			}

			filledPath, err := fillPath(op.Path, in.PathParams, b.Omc.OmadacID())
			if err != nil {
				return nil, err
			}

			query := ""
			if len(in.Query) > 0 {
				q := make(url.Values, len(in.Query))
				for k, v := range in.Query {
					q.Set(k, v)
				}
				query = q.Encode()
			}

			req, err := b.newDispatchRequest(ctx, op.Method, filledPath, query, in.Body)
			if err != nil {
				return nil, err
			}
			resp, err := b.Omc.HTTPClient().Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			var result json.RawMessage
			if err := omadaclient.Envelope(resp, &result); err != nil {
				return nil, err
			}
			return mcpkit.JSONResult(map[string]any{
				"status": resp.StatusCode,
				"result": resultPayload(result),
			})
		},
	}
}

// fillPath substitutes every {placeholder} in path. {omadacId} falls back
// to defaultOmadac when not supplied; every other placeholder must be in
// params. It returns an error naming any unfilled placeholder. Values are
// URL-escaped (simple style, matching the generated clients).
func fillPath(path string, params map[string]string, defaultOmadac string) (string, error) {
	var missing []string
	seen := make(map[string]bool)
	filled := pathParamRe.ReplaceAllStringFunc(path, func(m string) string {
		name := m[1 : len(m)-1]
		var v string
		if name == "omadacId" {
			v = params["omadacId"]
			if v == "" {
				v = defaultOmadac
			}
		} else {
			v = params[name]
		}
		if v == "" {
			if !seen[name] {
				seen[name] = true
				missing = append(missing, name)
			}
			return m
		}
		return url.PathEscape(v)
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("ops_call: missing path param(s) %s for operation path %q", strings.Join(missing, ", "), path)
	}
	return filled, nil
}

// newDispatchRequest builds an http.Request for the generic dispatch call,
// composing the URL exactly like the generated clients: the op path is
// resolved against the base URL (with a "." prefix so a leading "/" lands
// at the host root), the query string is attached, and a JSON body sets the
// Content-Type. Auth is left to the client's transport.
func (b *Base) newDispatchRequest(ctx context.Context, method, path, query string, body []byte) (*http.Request, error) {
	baseURL, err := url.Parse(b.Omc.BaseURL())
	if err != nil {
		return nil, fmt.Errorf("ops_call: parse base URL %q: %w", b.Omc.BaseURL(), err)
	}
	op := path
	if op[0] == '/' {
		op = "." + op
	}
	reqURL, err := baseURL.Parse(op)
	if err != nil {
		return nil, fmt.Errorf("ops_call: resolve %q against %q: %w", path, b.Omc.BaseURL(), err)
	}
	if query != "" {
		reqURL.RawQuery = query
	}

	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("ops_call: build request: %w", err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}
