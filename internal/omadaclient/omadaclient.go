// Package omadaclient wraps the generated Omada northbound cloud
// client for on-prem controller use: construction via cloud.New, a
// startup preflight token check, and envelope decoding for
// /openapi/... responses.
package omadaclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/RonaldPhilipsen/omada/cloud"

	"github.com/RonaldPhilipsen/omada-mcp/internal/config"
)

// Client wraps the generated northbound cloud.Client for on-prem
// controller use.
type Client struct {
	*cloud.Client
	base     string
	omadacID string
}

// New builds an on-prem northbound client from cfg. Auth is never
// reimplemented: construction goes through cloud.New, and the
// InsecureTLS setting is passed through the module's native
// cloud.Options.InsecureTLS field. A preflight token request via the
// module's own org token provider verifies the credentials up front so
// a bad controller answer fails the construction, not a later tool
// call. The returned error names the controller URL and the
// controller's message; it never contains the client ID or secret.
func New(cfg config.Config) (*Client, error) {
	cl, err := cloud.New(cloud.Options{
		BaseURL:      cfg.BaseURL,
		OmadacID:     cfg.OmadacID,
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		InsecureTLS:  cfg.InsecureTLS,
	})
	if err != nil {
		return nil, fmt.Errorf("omadaclient: %w", err)
	}

	if err := cl.PreflightToken(context.Background()); err != nil {
		return nil, fmt.Errorf("omadaclient: preflight token check against %s failed: %w", cfg.BaseURL, err)
	}

	return &Client{Client: cl, base: cfg.BaseURL, omadacID: cfg.OmadacID}, nil
}

// CloudAPI returns the wrapped generated client.
func (c *Client) CloudAPI() *cloud.Client {
	return c.Client
}

// BaseURL returns the configured controller base URL.
func (c *Client) BaseURL() string {
	return c.base
}

// OmadacID returns the configured omadac ID.
func (c *Client) OmadacID() string {
	return c.omadacID
}

// Envelope decodes an /openapi/... response body in the northbound
// {errorCode, msg, result} envelope. A non-zero errorCode returns the
// corresponding *cloud.APIError. On success the result payload is
// decoded into out (a non-nil pointer); a null or absent result leaves
// out untouched. The response body is read but not closed; the caller
// closes it.
func Envelope(resp *http.Response, out any) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("omadaclient: read response body: %w", err)
	}
	var env struct {
		ErrorCode int32           `json:"errorCode"`
		Msg       string          `json:"msg"`
		Result    json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("omadaclient: decode envelope: %w", err)
	}
	if env.ErrorCode != 0 {
		return cloud.NewAPIError(int(env.ErrorCode), env.Msg)
	}
	if len(env.Result) == 0 || string(env.Result) == "null" {
		return nil
	}
	if err := json.Unmarshal(env.Result, out); err != nil {
		return fmt.Errorf("omadaclient: decode envelope result: %w", err)
	}
	return nil
}
