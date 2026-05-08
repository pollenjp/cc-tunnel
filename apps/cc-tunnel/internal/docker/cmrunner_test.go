package docker

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/cmclient"
)

func TestCMRunner_ContainerCreate_RunsAgent(t *testing.T) {
	var got cmclient.RunAgentRequest
	mock := &cmclient.MockContainerManager{
		RunAgentFunc: func(_ context.Context, req cmclient.RunAgentRequest) error {
			got = req
			return nil
		},
	}
	r := NewCMRunner(mock, 9090)

	id, err := r.ContainerCreate(context.Background(), ContainerCreateOpts{
		Name:    "cctunnel-session-abc",
		Image:   "cc-remote-agent:latest",
		Env:     []string{"PORT=9090"},
		Network: "apps_default",
	})
	require.NoError(t, err)
	assert.Equal(t, "cctunnel-session-abc", id, "the container name is used as ID")
	assert.Equal(t, "cc-remote-agent:latest", got.Image)
	assert.Equal(t, "cctunnel-session-abc", got.Name)
	assert.Equal(t, 0, got.HostPort, "host port mapping is skipped")
	assert.Equal(t, 9090, got.ContainerPort)
	assert.Equal(t, "apps_default", got.Network)
	assert.Equal(t, []string{"PORT=9090"}, got.Env)
}

func TestCMRunner_ContainerCreate_PropagatesError(t *testing.T) {
	mock := &cmclient.MockContainerManager{
		RunAgentFunc: func(_ context.Context, _ cmclient.RunAgentRequest) error {
			return errors.New("boom")
		},
	}
	r := NewCMRunner(mock, 9090)
	_, err := r.ContainerCreate(context.Background(), ContainerCreateOpts{Name: "n", Image: "i"})
	require.Error(t, err)
}

func TestCMRunner_StopRemove(t *testing.T) {
	var stopped, removed string
	mock := &cmclient.MockContainerManager{
		StopContainerFunc:   func(_ context.Context, name string) error { stopped = name; return nil },
		RemoveContainerFunc: func(_ context.Context, name string) error { removed = name; return nil },
	}
	r := NewCMRunner(mock, 9090)

	require.NoError(t, r.ContainerStop(context.Background(), "x"))
	require.NoError(t, r.ContainerRemove(context.Background(), "x"))
	assert.Equal(t, "x", stopped)
	assert.Equal(t, "x", removed)
}

func TestCMRunner_StartIsNoop(t *testing.T) {
	r := NewCMRunner(&cmclient.MockContainerManager{}, 9090)
	require.NoError(t, r.ContainerStart(context.Background(), "anything"))
}

func TestCMRunner_InspectStubsRunning(t *testing.T) {
	r := NewCMRunner(&cmclient.MockContainerManager{}, 9090)
	info, err := r.ContainerInspect(context.Background(), "x")
	require.NoError(t, err)
	assert.Equal(t, "running", info.State)
}

func TestCMRunner_ListIsEmpty(t *testing.T) {
	r := NewCMRunner(&cmclient.MockContainerManager{}, 9090)
	list, err := r.ContainerList(context.Background(), "p", true)
	require.NoError(t, err)
	assert.Empty(t, list)
}
