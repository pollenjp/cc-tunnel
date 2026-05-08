package cmclient

import "context"

// MockContainerManager is a test double for ContainerManager.
type MockContainerManager struct {
	RunAgentFunc          func(ctx context.Context, req RunAgentRequest) error
	RunAgentContainerFunc func(ctx context.Context, image, name string, hostPort, containerPort int) error
	StopContainerFunc     func(ctx context.Context, name string) error
	RemoveContainerFunc   func(ctx context.Context, name string) error
	IsReadyFunc           func(ctx context.Context) bool
}

var _ ContainerManager = (*MockContainerManager)(nil)

func (m *MockContainerManager) RunAgent(ctx context.Context, req RunAgentRequest) error {
	if m.RunAgentFunc != nil {
		return m.RunAgentFunc(ctx, req)
	}
	if m.RunAgentContainerFunc != nil {
		return m.RunAgentContainerFunc(ctx, req.Image, req.Name, req.HostPort, req.ContainerPort)
	}
	return nil
}

func (m *MockContainerManager) RunAgentContainer(ctx context.Context, image, name string, hostPort, containerPort int) error {
	return m.RunAgent(ctx, RunAgentRequest{
		Image: image, Name: name, HostPort: hostPort, ContainerPort: containerPort,
	})
}

func (m *MockContainerManager) StopContainer(ctx context.Context, name string) error {
	if m.StopContainerFunc != nil {
		return m.StopContainerFunc(ctx, name)
	}
	return nil
}

func (m *MockContainerManager) RemoveContainer(ctx context.Context, name string) error {
	if m.RemoveContainerFunc != nil {
		return m.RemoveContainerFunc(ctx, name)
	}
	return nil
}

func (m *MockContainerManager) IsReady(ctx context.Context) bool {
	if m.IsReadyFunc != nil {
		return m.IsReadyFunc(ctx)
	}
	return true
}
