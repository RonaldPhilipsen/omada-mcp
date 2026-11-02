// Package fakectrl is a fake Omada northbound controller for tests.
//
// It serves the on-prem client-credentials token endpoint out of the
// box and records every request it receives so tests can assert on
// method, path, query, auth header, and body. Additional routes are
// added on the handler returned by Handler after New.
package fakectrl

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// Entry is one captured request.
type Entry struct {
	Method     string
	Path       string
	Query      string
	AuthHeader string
	Body       []byte
}

// Fake is a fake Omada northbound controller.
type Fake struct {
	server *httptest.Server
	mux    *http.ServeMux

	mu           sync.Mutex
	tokenHandler http.Handler
	tokenReqs    int
	entries      []Entry
}

// New starts a fake controller. It serves
// POST /openapi/authorize/token?grant_type=client_credentials with the
// envelope {errorCode:0, result:{accessToken:"test-token",
// expiresIn:3600}} and counts token requests. Routes are added on the
// handler returned by Handler after New; the server is closed when the
// test ends.
func New(t *testing.T) *Fake {
	t.Helper()
	f := &Fake{}
	f.mux = http.NewServeMux()
	f.tokenHandler = http.HandlerFunc(f.handleToken)
	f.mux.HandleFunc("POST /openapi/authorize/token", f.serveToken)
	f.server = httptest.NewServer(f.capture(f.mux))
	t.Cleanup(f.server.Close)
	return f
}

// SetTokenHandler replaces the handler for
// POST /openapi/authorize/token, for example to make the token
// endpoint fail. Call it after New and before driving requests.
func (f *Fake) SetTokenHandler(h http.Handler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokenHandler = h
}

// URL returns the fake controller's base URL.
func (f *Fake) URL() string {
	return f.server.URL
}

// Handler returns the mux to add routes on after New. The token
// endpoint is owned by the fake; replace it with SetTokenHandler.
func (f *Fake) Handler() http.Handler {
	return f.mux
}

// TokenRequests returns how many token endpoint requests were received.
func (f *Fake) TokenRequests() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tokenReqs
}

// Requests returns every captured request, in arrival order.
func (f *Fake) Requests() []Entry {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Entry, len(f.entries))
	copy(out, f.entries)
	return out
}

// EnvelopeJSON writes an Omada northbound envelope
// {errorCode, msg, result} with the given code and result payload.
func EnvelopeJSON(w http.ResponseWriter, code int32, result any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"errorCode": code,
		"msg":       "",
		"result":    result,
	})
}

func (f *Fake) serveToken(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	h := f.tokenHandler
	f.tokenReqs++
	f.mu.Unlock()
	h.ServeHTTP(w, r)
}

func (f *Fake) handleToken(w http.ResponseWriter, _ *http.Request) {
	EnvelopeJSON(w, 0, map[string]any{
		"accessToken": "test-token",
		"expiresIn":   3600,
	})
}

// capture records each request (replacing the body reader so handlers
// can still read it) and then forwards it to next.
func (f *Fake) capture(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "fake controller: read body: "+err.Error(), http.StatusInternalServerError)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		f.mu.Lock()
		f.entries = append(f.entries, Entry{
			Method:     r.Method,
			Path:       r.URL.Path,
			Query:      r.URL.RawQuery,
			AuthHeader: r.Header.Get("Authorization"),
			Body:       body,
		})
		f.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}
