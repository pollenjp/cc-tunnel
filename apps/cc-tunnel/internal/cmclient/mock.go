package cmclient

import "context"

// MockContainerManager is a test double for ContainerManager.
type MockContainerManager struct {
	RunAgentContainerFunc func(ctx context.Context, image, name string, hostPort, containerPort int) error
	StopContainerFunc     func(ctx context.Context, name string) error
	RemoveContainerFunc   func(ctx context.Context, name string) error
	IsReadyFunc           func(ctx context.Context) bool
}

var _ ContainerManager = (*MockContainerManager)(nil)

func (m *MockContainerManager) RunAgentContainer(ctx context.Context, image, name string, hostPort, containerPort int) error {
	if m.RunAgentContainerFunc != nil {
		return m.RunAgentContainerFunc(ctx, image, name, hostPort, containerPort)
	}
	return nil
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
