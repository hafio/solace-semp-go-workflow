package semp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wferrors "solace-semp-workflow/internal/errors"
)

// writeJSON writes a SEMP-style response body with the given HTTP status.
func writeJSON(w http.ResponseWriter, status int, body map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, "user", "pass", "test-vpn", false, 5)
	return c, srv
}

func TestExistsFound(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		writeJSON(w, 200, map[string]any{
			"meta": map[string]any{"responseCode": 200},
			"data": map[string]any{"queueName": "q1"},
		})
	})
	exists, data, err := c.Exists("queues/q1")
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "q1", data["queueName"])
}

func TestExistsNotFoundBySempCode(t *testing.T) {
	// responseCode != 200 with SEMP error code NOT_FOUND maps to (false, nil, nil).
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 400, map[string]any{
			"meta": map[string]any{
				"responseCode": 400,
				"error":        map[string]any{"code": NotFound, "description": "not found"},
			},
		})
	})
	exists, _, err := c.Exists("queues/missing")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestExistsPropagatesOtherErrors(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 403, map[string]any{
			"meta": map[string]any{
				"responseCode": 403,
				"error":        map[string]any{"code": 89, "description": "forbidden"},
			},
		})
	})
	_, _, err := c.Exists("queues/denied")
	require.Error(t, err)
	var se *wferrors.SEMPError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, 403, se.StatusCode)
	assert.Equal(t, 89, se.SempCode)
}

func TestCreateSuccess(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		writeJSON(w, 200, map[string]any{
			"meta": map[string]any{"responseCode": 200},
			"data": map[string]any{"queueName": "q1"},
		})
	})
	data, err := c.Create("queues", map[string]any{"queueName": "q1"})
	require.NoError(t, err)
	assert.Equal(t, "q1", data["queueName"])
}

func TestCreateAlreadyExists(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 400, map[string]any{
			"meta": map[string]any{
				"responseCode": 400,
				"error":        map[string]any{"code": AlreadyExists, "description": "already exists"},
			},
		})
	})
	_, err := c.Create("queues/subscriptions", map[string]any{"subscriptionTopic": "t"})
	require.Error(t, err)
	var se *wferrors.SEMPError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, AlreadyExists, se.SempCode)
}

func TestUpdateAndDelete(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PATCH", "DELETE":
			writeJSON(w, 200, map[string]any{"meta": map[string]any{"responseCode": 200}})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})
	_, err := c.Update("queues/q1", map[string]any{"egressEnabled": true})
	require.NoError(t, err)
	require.NoError(t, c.Delete("queues/q1"))
}

func TestConnectionFailed(t *testing.T) {
	// Start then immediately close a server so the port refuses connections.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	c := NewClient(url, "u", "p", "vpn", false, 2)
	_, _, err := c.Exists("queues/q1")
	require.Error(t, err)
	var se *wferrors.SEMPError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, 0, se.StatusCode) // transport failure → status 0
	assert.Contains(t, err.Error(), "Connection failed")
}

func TestRetryOnServerError(t *testing.T) {
	var attempts int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n == 1 {
			w.WriteHeader(503) // first attempt: transient failure
			return
		}
		writeJSON(w, 200, map[string]any{
			"meta": map[string]any{"responseCode": 200},
			"data": map[string]any{"queueName": "q1"},
		})
	})
	exists, _, err := c.Exists("queues/q1")
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, int32(2), atomic.LoadInt32(&attempts)) // one retry
}

func TestNoRetryOnClientError(t *testing.T) {
	var attempts int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		writeJSON(w, 400, map[string]any{
			"meta": map[string]any{
				"responseCode": 400,
				"error":        map[string]any{"code": 3, "description": "bad request"},
			},
		})
	})
	_, err := c.Create("queues", map[string]any{"queueName": "q1"})
	require.Error(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&attempts)) // 400 is not retried
}

func TestTimeout(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1500 * time.Millisecond)
		writeJSON(w, 200, map[string]any{"meta": map[string]any{"responseCode": 200}})
	})
	// Rebuild with a 1s timeout (helper uses 5s).
	c = NewClient(c.host, "u", "p", "test-vpn", false, 1)
	_, _, err := c.Exists("queues/slow")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestTestConnection(t *testing.T) {
	ok, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"meta": map[string]any{"responseCode": 200}})
	})
	assert.True(t, ok.TestConnection())

	bad, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	})
	assert.False(t, bad.TestConnection())
}
