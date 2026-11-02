package omadaclient

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/RonaldPhilipsen/omada/cloud"

	"github.com/RonaldPhilipsen/omada-mcp/internal/config"
	"github.com/RonaldPhilipsen/omada-mcp/internal/fakectrl"
)

func TestNewBuildsClientInsecureTLS(t *testing.T) {
	fake := fakectrl.New(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"data": []map[string]any{{"site": "site-1", "name": "Site One"}},
		})
	}))

	cfg := config.Config{
		BaseURL:      fake.URL(),
		OmadacID:     "omac-123",
		ClientID:     "client-id-value",
		ClientSecret: "client-secret-value",
		InsecureTLS:  true,
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := c.BaseURL(); got != fake.URL() {
		t.Errorf("BaseURL() = %q, want %q", got, fake.URL())
	}
	if got := c.OmadacID(); got != "omac-123" {
		t.Errorf("OmadacID() = %q, want %q", got, "omac-123")
	}

	// InsecureTLS passthrough: the transport handed to cloud.New must
	// skip TLS verification.
	tr, ok := c.rt.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("wrapped transport is %T, want *http.Transport", c.rt.Transport)
	}
	if tr.TLSClientConfig == nil || !tr.TLSClientConfig.InsecureSkipVerify {
		t.Error("InsecureTLS did not propagate to the wrapped transport's TLS config")
	}

	// The built client is usable against fakectrl: a full round trip
	// through the auth transport (token fetch) and the envelope.
	resp, err := c.CloudAPI().Site().GetSiteList(context.Background(), "omac-123", nil)
	if err != nil {
		t.Fatalf("GetSiteList: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		Data []struct {
			Site string `json:"site"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := Envelope(resp, &out); err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	if len(out.Data) != 1 || out.Data[0].Name != "Site One" {
		t.Errorf("decoded result = %+v, want one row named %q", out.Data, "Site One")
	}

	// Preflight at construction plus the client's own token provider:
	// exactly two token requests.
	if n := fake.TokenRequests(); n != 2 {
		t.Errorf("TokenRequests() = %d, want 2 (preflight + request auth)", n)
	}

	reqs := fake.Requests()
	var sitesReq *fakectrl.Entry
	for i := range reqs {
		if reqs[i].Path == "/openapi/v1/omac-123/sites" {
			sitesReq = &reqs[i]
		}
	}
	if sitesReq == nil {
		t.Fatalf("no request captured for /openapi/v1/omac-123/sites: %+v", reqs)
	}
	if sitesReq.AuthHeader != "AccessToken=test-token" {
		t.Errorf("sites request Authorization = %q, want %q", sitesReq.AuthHeader, "AccessToken=test-token")
	}
}

func TestPreflightTokenErrorPropagates(t *testing.T) {
	const (
		clientID     = "client-id-value"
		clientSecret = "client-secret-value"
	)
	fake := fakectrl.New(t)
	fake.SetTokenHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errorCode":-30109,"msg":"bad creds","result":null}`))
	}))

	cfg := config.Config{
		BaseURL:      fake.URL(),
		OmadacID:     "omac-123",
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}
	_, err := New(cfg)
	if err == nil {
		t.Fatal("New succeeded, want preflight token error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "bad creds") {
		t.Errorf("error %q does not contain the controller msg %q", msg, "bad creds")
	}
	if !strings.Contains(msg, fake.URL()) {
		t.Errorf("error %q does not contain the literal controller URL %q", msg, fake.URL())
	}
	if strings.Contains(msg, clientID) {
		t.Errorf("error %q leaks the client ID", msg)
	}
	if strings.Contains(msg, clientSecret) {
		t.Errorf("error %q leaks the client secret", msg)
	}
}

func TestEnvelopeBusinessError(t *testing.T) {
	resp := envelopeResponse(`{"errorCode":-1171,"msg":"no privilege","result":null}`)
	var out struct {
		Data []int `json:"data"`
	}
	err := Envelope(resp, &out)
	if err == nil {
		t.Fatal("Envelope succeeded, want *cloud.APIError")
	}
	apiErr, ok := err.(*cloud.APIError)
	if !ok {
		t.Fatalf("Envelope returned %T (%v), want *cloud.APIError", err, err)
	}
	if apiErr.Code != -1171 {
		t.Errorf("APIError.Code = %d, want -1171", apiErr.Code)
	}
	if apiErr.Msg != "no privilege" {
		t.Errorf("APIError.Msg = %q, want %q", apiErr.Msg, "no privilege")
	}
	if out.Data != nil {
		t.Errorf("business error decoded a result into out: %+v", out.Data)
	}
}

func TestEnvelopeSuccessDecodesResult(t *testing.T) {
	resp := envelopeResponse(`{"errorCode":0,"msg":"","result":{"data":[1,2]}}`)
	var out struct {
		Data []int64 `json:"data"`
	}
	if err := Envelope(resp, &out); err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	if len(out.Data) != 2 || out.Data[0] != 1 || out.Data[1] != 2 {
		t.Errorf("decoded result = %v, want [1 2]", out.Data)
	}
}

// envelopeResponse builds a 200 *http.Response with the given body.
func envelopeResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
