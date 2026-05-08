package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerops "github.com/pollenjp/cc-tunnel/apps/container-manager/internal/docker"
)

type fakeManager struct {
	pingErr   error
	runID     string
	runErr    error
	stopErr   error
	removeErr error
	lastRun   dockerops.RunAgentRequest
	lastStop  string
	lastRm    string
}

func (f *fakeManager) Ping(_ context.Context) error { return f.pingErr }
func (f *fakeManager) RunAgent(_ context.Context, req dockerops.RunAgentRequest) (string, error) {
	f.lastRun = req
	return f.runID, f.runErr
}
func (f *fakeManager) StopAgent(_ context.Context, name string) error {
	f.lastStop = name
	return f.stopErr
}
func (f *fakeManager) RemoveAgent(_ context.Context, name string) error {
	f.lastRm = name
	return f.removeErr
}

func newServer(mgr AgentManager) *httptest.Server {
	return httptest.NewServer(NewServer(mgr).Routes())
}

func TestHealthz_OK(t *testing.T) {
	srv := newServer(&fakeManager{})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHealthz_Unhealthy(t *testing.T) {
	srv := newServer(&fakeManager{pingErr: errors.New("boom")})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestCreateAgent_Success(t *testing.T) {
	mgr := &fakeManager{runID: "abc123"}
	srv := newServer(mgr)
	defer srv.Close()

	body := `{"image":"img:tag","name":"sess-1","host_port":9091,"container_port":9090,"memory_mib":256,"network":"my-net","env":["PORT=9090"]}`
	resp, err := http.Post(srv.URL+"/v1/agents", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var got CreateAgentResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.Equal(t, "abc123", got.Id)
	assert.Equal(t, "img:tag", mgr.lastRun.Image)
	assert.Equal(t, "sess-1", mgr.lastRun.Name)
	assert.Equal(t, 9091, mgr.lastRun.HostPort)
	assert.Equal(t, 9090, mgr.lastRun.ContainerPort)
	assert.Equal(t, int64(256*1024*1024), mgr.lastRun.MemoryBytes)
	assert.Equal(t, "my-net", mgr.lastRun.Network)
	assert.Equal(t, []string{"PORT=9090"}, mgr.lastRun.Env)
}

func TestCreateAgent_NoHostPort(t *testing.T) {
	mgr := &fakeManager{runID: "abc"}
	srv := newServer(mgr)
	defer srv.Close()

	body := `{"image":"img:tag","name":"sess-1","container_port":9090}`
	resp, err := http.Post(srv.URL+"/v1/agents", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, 0, mgr.lastRun.HostPort)
}

func TestCreateAgent_BadRequest(t *testing.T) {
	srv := newServer(&fakeManager{})
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/agents", "application/json", strings.NewReader(`{"image":"img"}`))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateAgent_RunFailure(t *testing.T) {
	mgr := &fakeManager{runErr: errors.New("create container: No such image")}
	srv := newServer(mgr)
	defer srv.Close()

	body := `{"image":"img:tag","name":"sess-1","host_port":9091,"container_port":9090}`
	resp, err := http.Post(srv.URL+"/v1/agents", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestStopAgent(t *testing.T) {
	mgr := &fakeManager{}
	srv := newServer(mgr)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/agents/sess-1/stop", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, "sess-1", mgr.lastStop)
}

func TestRemoveAgent(t *testing.T) {
	mgr := &fakeManager{}
	srv := newServer(mgr)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/v1/agents/sess-1", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, "sess-1", mgr.lastRm)
}

func TestRemoveAgent_NotFound(t *testing.T) {
	mgr := &fakeManager{removeErr: fmt.Errorf("remove container: No such container: sess-x")}
	srv := newServer(mgr)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/v1/agents/sess-x", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
