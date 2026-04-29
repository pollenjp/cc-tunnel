package dockerhost

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAPIVersion = "1.47"

// newTestClient creates a Client backed by the given httptest.Server.
// The server URL (http://127.0.0.1:PORT) is converted to tcp://127.0.0.1:PORT.
func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	host := "tcp://" + strings.TrimPrefix(srv.URL, "http://")
	cli, err := NewClient(host)
	require.NoError(t, err)
	return cli
}

// mockDockerMux returns an http.Handler that routes Docker daemon API requests.
// It handles /_ping for version negotiation and delegates other paths to handler.
func mockDockerMux(t *testing.T, handler http.HandlerFunc) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Version negotiation: HEAD /_ping or GET /_ping
		if r.URL.Path == "/_ping" {
			w.Header().Set("Api-Version", testAPIVersion)
			w.Header().Set("Ostype", "linux")
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		handler(w, r)
	})
}

func TestNewClient_InvalidHost(t *testing.T) {
	_, err := NewClient("invalid-host-without-scheme")
	assert.Error(t, err)
}

func TestRunAgentContainer_Success(t *testing.T) {
	var createCalled, startCalled bool
	containerID := "abc123containerid"

	srv := httptest.NewServer(mockDockerMux(t, func(w http.ResponseWriter, r *http.Request) {
		versionedPrefix := "/v" + testAPIVersion
		path := strings.TrimPrefix(r.URL.Path, versionedPrefix)

		switch {
		case r.Method == http.MethodPost && path == "/containers/create":
			createCalled = true
			// Verify name query param
			assert.Equal(t, "test-container", r.URL.Query().Get("name"))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Id":       containerID,
				"Warnings": []string{},
			})
		case r.Method == http.MethodPost && path == "/containers/"+containerID+"/start":
			startCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	cli := newTestClient(t, srv)
	err := cli.RunAgentContainer(context.Background(), "test-image:latest", "test-container", 9091, 9091)
	require.NoError(t, err)
	assert.True(t, createCalled, "ContainerCreate should have been called")
	assert.True(t, startCalled, "ContainerStart should have been called")
}

func TestStopContainer_Success(t *testing.T) {
	var stopCalled bool
	containerName := "session-abc123"

	srv := httptest.NewServer(mockDockerMux(t, func(w http.ResponseWriter, r *http.Request) {
		versionedPrefix := "/v" + testAPIVersion
		path := strings.TrimPrefix(r.URL.Path, versionedPrefix)

		if r.Method == http.MethodPost && path == "/containers/"+containerName+"/stop" {
			stopCalled = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cli := newTestClient(t, srv)
	err := cli.StopContainer(context.Background(), containerName)
	require.NoError(t, err)
	assert.True(t, stopCalled, "ContainerStop should have been called")
}

func TestRemoveContainer_Success(t *testing.T) {
	var removeCalled bool
	containerName := "session-abc123"

	srv := httptest.NewServer(mockDockerMux(t, func(w http.ResponseWriter, r *http.Request) {
		versionedPrefix := "/v" + testAPIVersion
		path := strings.TrimPrefix(r.URL.Path, versionedPrefix)

		if r.Method == http.MethodDelete && path == "/containers/"+containerName {
			removeCalled = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cli := newTestClient(t, srv)
	err := cli.RemoveContainer(context.Background(), containerName)
	require.NoError(t, err)
	assert.True(t, removeCalled, "ContainerRemove should have been called")
}

func TestIsReady_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_ping" {
			w.Header().Set("Api-Version", testAPIVersion)
			w.WriteHeader(http.StatusOK)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cli := newTestClient(t, srv)
	assert.True(t, cli.IsReady(context.Background()))
}

func TestIsReady_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return error for all ping requests
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cli := newTestClient(t, srv)
	assert.False(t, cli.IsReady(context.Background()))
}
