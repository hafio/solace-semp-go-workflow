package semp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wferrors "solace-semp-workflow/internal/errors"
)

// errReader is an io.Reader that always fails, used to exercise the request-body
// read-error branch in retryTransport.RoundTrip.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

// roundTripFunc adapts a function to http.RoundTripper so tests can drive
// retryTransport with a fully controlled base transport.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestRequestBuildErrors covers the two pre-flight failure branches in
// request(): payload marshal failure and http.NewRequest failure. Both return a
// "Request failed" SEMPError with status 0 and never touch the network.
func TestRequestBuildErrors(t *testing.T) {
	c := NewClient("http://example.com", "u", "p", "vpn", false, 5)
	cases := []struct {
		name    string
		method  string
		payload map[string]any
	}{
		{"marshal failure", "POST", map[string]any{"bad": make(chan int)}},
		{"newrequest failure", "BAD METHOD", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := c.request(tc.method, "queues", tc.payload)
			require.Error(t, err)
			var se *wferrors.SEMPError
			require.ErrorAs(t, err, &se)
			assert.Equal(t, 0, se.StatusCode)
			assert.Contains(t, err.Error(), "Request failed")
		})
	}
}

// TestRequestBodyReadError covers the io.ReadAll(resp.Body) failure branch: the
// server promises a body via Content-Length but closes the connection first, so
// the client hits an unexpected EOF while reading and reports "Connection failed".
func TestRequestBodyReadError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			return
		}
		// Promise 50 bytes, send none, then close: the client's body read
		// terminates with an unexpected EOF.
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 50\r\n\r\n"))
		_ = conn.Close()
	})
	_, _, err := c.Exists("queues/x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Connection failed")
}

// TestRequestMalformedJSON covers the json.Unmarshal failure branch: a 200 with a
// non-JSON body surfaces as an "Invalid JSON response" SEMPError carrying the HTTP
// status.
func TestRequestMalformedJSON(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("this is not json{"))
	})
	_, _, err := c.Exists("queues/x")
	require.Error(t, err)
	var se *wferrors.SEMPError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, 200, se.StatusCode)
	assert.Contains(t, err.Error(), "Invalid JSON response")
}

// TestRequestMissingMeta covers the branch where the response has no meta block:
// responseCode falls back to the HTTP status (200) and the data field is returned.
func TestRequestMissingMeta(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{
			"data": map[string]any{"queueName": "q1"},
		})
	})
	exists, data, err := c.Exists("queues/q1")
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "q1", data["queueName"])
}

// TestRequestErrorWithoutDescription covers the branch where a non-200 response
// carries an error code but no description: the raw body is used as the message.
func TestRequestErrorWithoutDescription(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 400, map[string]any{
			"meta": map[string]any{
				"responseCode": 400,
				"error":        map[string]any{"code": 99},
			},
		})
	})
	_, _, err := c.Exists("queues/x")
	require.Error(t, err)
	var se *wferrors.SEMPError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, 400, se.StatusCode)
	assert.Equal(t, 99, se.SempCode)
	assert.Contains(t, err.Error(), "responseCode")
}

// TestRequestErrorEmptyBody covers the final fallback: a non-200 response with an
// empty body and no error object yields the "Unknown SEMP error" message.
func TestRequestErrorEmptyBody(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
	})
	_, _, err := c.Exists("queues/x")
	require.Error(t, err)
	var se *wferrors.SEMPError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, 400, se.StatusCode)
	assert.Equal(t, "Unknown SEMP error", err.Error())
}

// TestTestConnectionTransportError covers TestConnection's transport-failure
// branch: a closed server refuses the connection, so it returns false.
func TestTestConnectionTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	c := NewClient(url, "u", "p", "vpn", false, 2)
	assert.False(t, c.TestConnection())
}

// TestTestConnectionRequestBuildError covers TestConnection's http.NewRequest
// failure branch: a host containing a control character makes URL parsing fail, so
// it returns false without a network call.
func TestTestConnectionRequestBuildError(t *testing.T) {
	c := NewClient("http://\x7f", "u", "p", "vpn", false, 2)
	assert.False(t, c.TestConnection())
}

// TestTrimTrailingSlashMultiple covers the trimming loop body: multiple trailing
// slashes are all stripped when the client is built.
func TestTrimTrailingSlashMultiple(t *testing.T) {
	c := NewClient("http://example.com///", "u", "p", "vpn", false, 5)
	assert.Equal(t, "http://example.com", c.host)
}

// TestDefaultMsgVPN covers the accessor VPN-scoped modules use to resolve an
// implicit message VPN. Module tests exercise this through a fake, so without
// this the real implementation would never run.
func TestDefaultMsgVPN(t *testing.T) {
	c := NewClient("http://example.com", "u", "p", "my-vpn", false, 5)
	assert.Equal(t, "my-vpn", c.DefaultMsgVPN())
}

// TestRetryTransportBodyReadError covers the request-body read-error branch in
// RoundTrip: a request whose body fails to read returns the read error before the
// base transport is ever invoked.
func TestRetryTransportBodyReadError(t *testing.T) {
	base := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})
	tr := &retryTransport{
		base:          base,
		maxRetries:    3,
		backoffFactor: time.Millisecond,
		retryStatuses: map[int]bool{503: true},
	}
	req, err := http.NewRequest("POST", "http://example.com", errReader{})
	require.NoError(t, err)

	_, err = tr.RoundTrip(req)
	require.Error(t, err)
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

// TestRetryTransportBackoffSleep covers the retry loop's nonzero-backoff branch:
// two consecutive 503s force a second retry whose backoff delay is > 0, so the
// sleep path executes before the successful third attempt.
func TestRetryTransportBackoffSleep(t *testing.T) {
	var calls int32
	base := roundTripFunc(func(*http.Request) (*http.Response, error) {
		status := 200
		if atomic.AddInt32(&calls, 1) <= 2 {
			status = 503
		}
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})
	tr := &retryTransport{
		base:          base,
		maxRetries:    3,
		backoffFactor: time.Millisecond,
		retryStatuses: map[int]bool{503: true},
	}
	req, err := http.NewRequest("GET", "http://example.com", nil)
	require.NoError(t, err)

	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls))
}

// TestBackoffDelayValues covers backoffDelay's zero branch (attempt < 1) and the
// exponential branch.
func TestBackoffDelayValues(t *testing.T) {
	assert.Equal(t, time.Duration(0), backoffDelay(0, time.Second))
	assert.Equal(t, time.Second, backoffDelay(1, time.Second))
	assert.Equal(t, 2*time.Second, backoffDelay(2, time.Second))
	assert.Equal(t, 4*time.Second, backoffDelay(3, time.Second))
}

// TestCoerceBoolDefaultBranch covers the default arm of CoerceBool: any non-nil
// value of an unhandled type is truthy.
func TestCoerceBoolDefaultBranch(t *testing.T) {
	values := []any{
		[]int{1},
		map[string]int{"a": 1},
		struct{ X int }{X: 1},
	}
	for _, v := range values {
		assert.Truef(t, CoerceBool(v), "expected %#v truthy", v)
	}
}
