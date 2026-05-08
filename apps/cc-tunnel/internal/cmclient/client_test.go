package cmclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunAgentContainer_Success(t *testing.T) {
	var (
		called bool
		got    createAgentRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/agents", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		called = true
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "abc"})
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL)
	require.NoError(t, err)

	err = c.RunAgentContainer(context.Background(), "img:tag", "sess-1", 9091, 9090)
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "img:tag", got.Image)
	assert.Equal(t, "sess-1", got.Name)
	assert.Equal(t, 9091, got.HostPort)
	assert.Equal(t, 9090, got.ContainerPort)
}

func TestRunAgentContainer_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL)
	err := c.RunAgentContainer(context.Background(), "img", "n", 1, 2)
	require.Error(t, err)
}

func TestStopContainer_Success(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/agents/sess-1/stop", r.URL.Path)
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL)
	require.NoError(t, c.StopContainer(context.Background(), "sess-1"))
	assert.True(t, called)
}

func TestRemoveContainer_Success(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v1/agents/sess-1", r.URL.Path)
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL)
	require.NoError(t, c.RemoveContainer(context.Background(), "sess-1"))
	assert.True(t, called)
}

func TestIsReady_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/healthz", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL)
	assert.True(t, c.IsReady(context.Background()))
}

func TestIsReady_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL)
	assert.False(t, c.IsReady(context.Background()))
}
