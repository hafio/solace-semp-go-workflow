package semp

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	wferrors "solace-semp-workflow/internal/errors"
)

// SEMP error codes returned in the response meta.error.code field.
const (
	// NotFound is the SEMP code for a resource that does not exist.
	NotFound = 6
	// AlreadyExists is the SEMP code for a resource that already exists.
	AlreadyExists = 10
)

// Client is a low-level HTTP client for the Solace SEMP v2 Config API.
type Client struct {
	host     string
	msgVPN   string
	timeout  time.Duration
	username string
	password string
	http     *http.Client
}

// NewClient builds a SEMP client. verifySSL=false (the default) disables TLS
// verification, the default posture for lab brokers.
func NewClient(host, username, password, msgVPN string, verifySSL bool, timeoutSeconds int) *Client {
	host = trimTrailingSlash(host)

	base := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: !verifySSL}, //nolint:gosec // verifySSL is an explicit operator choice; the default disables TLS verification for lab brokers
	}
	transport := &retryTransport{
		base:          base,
		maxRetries:    3,
		backoffFactor: 500 * time.Millisecond,
		retryStatuses: map[int]bool{502: true, 503: true, 504: true},
	}

	return &Client{
		host:     host,
		msgVPN:   msgVPN,
		timeout:  time.Duration(timeoutSeconds) * time.Second,
		username: username,
		password: password,
		http:     &http.Client{Transport: transport, Timeout: time.Duration(timeoutSeconds) * time.Second},
	}
}

func trimTrailingSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

// configRoot is the base URL of the SEMP v2 config API. Every path passed to
// Exists/Create/Update/Delete is relative to it, so VPN-scoped objects carry
// their own "msgVpns/{vpn}/" prefix and broker-global objects (dmrClusters,
// oauthProfiles, certAuthorities, proxies, virtualHostnames, msgVpns itself)
// address the root directly.
func (c *Client) configRoot() string {
	return fmt.Sprintf("%s/SEMP/v2/config", c.host)
}

// DefaultMsgVPN returns the message VPN from the connection config. VPN-scoped
// modules target it unless the action names one explicitly.
func (c *Client) DefaultMsgVPN() string {
	return c.msgVPN
}

// request executes an HTTP request against the SEMP API, returning the response
// JSON "data" field on success and a *errors.SEMPError on failure. Success is
// decided by meta.responseCode == 200, not the HTTP status. An empty path
// addresses the Broker singleton, which is the config root itself.
func (c *Client) request(method, path string, payload map[string]any) (map[string]any, error) {
	reqURL := c.configRoot()
	if path != "" {
		reqURL += "/" + path
	}

	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, wferrors.NewSEMPError(fmt.Sprintf("Request failed: %v", err), 0, 0)
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, reqURL, body)
	if err != nil {
		return nil, wferrors.NewSEMPError(fmt.Sprintf("Request failed: %v", err), 0, 0)
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		if ue, ok := err.(*url.Error); ok && ue.Timeout() {
			return nil, wferrors.NewSEMPError(
				fmt.Sprintf("Request timed out after %ds", int(c.timeout.Seconds())), 0, 0)
		}
		return nil, wferrors.NewSEMPError(
			fmt.Sprintf("Connection failed: %s - is the broker reachable?", c.host), 0, 0)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, wferrors.NewSEMPError(
			fmt.Sprintf("Connection failed: %s - is the broker reachable?", c.host), 0, 0)
	}

	parsed := map[string]any{}
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, wferrors.NewSEMPError(
				fmt.Sprintf("Invalid JSON response from broker: %s", truncate(string(raw), 200)),
				resp.StatusCode, 0)
		}
	}

	meta, _ := parsed["meta"].(map[string]any)
	responseCode := resp.StatusCode
	if rc, ok := toInt(meta["responseCode"]); ok {
		responseCode = rc
	}

	if responseCode == 200 {
		data, _ := parsed["data"].(map[string]any)
		return data, nil
	}

	errObj, _ := meta["error"].(map[string]any)
	description := ""
	if d, ok := errObj["description"].(string); ok {
		description = d
	}
	if description == "" {
		if len(raw) > 0 {
			description = string(raw)
		} else {
			description = "Unknown SEMP error"
		}
	}
	sempCode, _ := toInt(errObj["code"])
	return nil, wferrors.NewSEMPError(description, responseCode, sempCode)
}

// Exists reports whether a resource exists, returning its data on success.
// A SEMP NotFound code or HTTP 404 maps to (false, nil, nil); other errors
// propagate.
func (c *Client) Exists(path string) (bool, map[string]any, error) {
	data, err := c.request("GET", path, nil)
	if err != nil {
		var se *wferrors.SEMPError
		if asSEMPError(err, &se) && (se.SempCode == NotFound || se.StatusCode == 404) {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, data, nil
}

// Create POSTs to create a resource.
func (c *Client) Create(path string, payload map[string]any) (map[string]any, error) {
	return c.request("POST", path, payload)
}

// Update PATCHes to update a resource.
func (c *Client) Update(path string, payload map[string]any) (map[string]any, error) {
	return c.request("PATCH", path, payload)
}

// Delete DELETEs a resource.
func (c *Client) Delete(path string) error {
	_, err := c.request("DELETE", path, nil)
	return err
}

// TestConnection verifies broker connectivity by fetching the configured VPN.
// It returns false on any transport error.
func (c *Client) TestConnection() bool {
	req, err := http.NewRequest("GET", c.configRoot()+"/msgVpns/"+Enc(c.msgVPN), nil)
	if err != nil {
		return false
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == 200
}

// retryTransport retries transient server errors (502/503/504) but never retries
// connection failures (up to 3 attempts, no connection-level retry).
type retryTransport struct {
	base          http.RoundTripper
	maxRetries    int
	backoffFactor time.Duration
	retryStatuses map[int]bool
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, err
		}
	}

	for attempt := 0; ; attempt++ {
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
		resp, err := t.base.RoundTrip(req)
		if err != nil {
			// Connection-level failure: never retried (connect=0).
			return nil, err
		}
		if attempt >= t.maxRetries || !t.retryStatuses[resp.StatusCode] {
			return resp, nil
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if d := backoffDelay(attempt, t.backoffFactor); d > 0 {
			time.Sleep(d)
		}
	}
}

// backoffDelay computes exponential backoff: no sleep before the first retry,
// then backoffFactor * 2^(attempt-1).
func backoffDelay(attempt int, factor time.Duration) time.Duration {
	if attempt < 1 {
		return 0
	}
	return factor * time.Duration(1<<(attempt-1))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// toInt converts a JSON-decoded number (float64) or int to int.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	}
	return 0, false
}

// asSEMPError is a thin wrapper around errors.As for *SEMPError.
func asSEMPError(err error, target **wferrors.SEMPError) bool {
	if se, ok := err.(*wferrors.SEMPError); ok {
		*target = se
		return true
	}
	return false
}
