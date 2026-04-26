package gce

import (
	"context"
	"fmt"
)

// MockGCEClient はテスト用の GCEClient モック実装
type MockGCEClient struct {
	instances map[string]*Instance // name -> instance

	// テスト用フック（nil の場合はデフォルト実装を使用）
	CreateInstanceFn func(ctx context.Context, req *CreateInstanceRequest) (*Instance, error)
	GetInstanceFn    func(ctx context.Context, project, zone, name string) (*Instance, error)
	DeleteInstanceFn func(ctx context.Context, project, zone, name string) error
	ListInstancesFn  func(ctx context.Context, project, zone string) ([]*Instance, error)
}

var _ GCEClient = (*MockGCEClient)(nil) // コンパイル時インターフェース確認

// NewMockGCEClient は新しい MockGCEClient を返す
func NewMockGCEClient() *MockGCEClient {
	return &MockGCEClient{
		instances: make(map[string]*Instance),
	}
}

// CreateInstance はインスタンスを作成する（デフォルト: in-memory 保存）
func (m *MockGCEClient) CreateInstance(ctx context.Context, req *CreateInstanceRequest) (*Instance, error) {
	if m.CreateInstanceFn != nil {
		return m.CreateInstanceFn(ctx, req)
	}
	inst := &Instance{
		Name:      req.Name,
		Status:    "RUNNING",
		NetworkIP: "10.0.0.1",
		Labels:    req.Labels,
	}
	m.instances[req.Name] = inst
	return inst, nil
}

// GetInstance は指定インスタンスの情報を返す（デフォルト: in-memory 参照）
func (m *MockGCEClient) GetInstance(ctx context.Context, project, zone, name string) (*Instance, error) {
	if m.GetInstanceFn != nil {
		return m.GetInstanceFn(ctx, project, zone, name)
	}
	inst, ok := m.instances[name]
	if !ok {
		return nil, fmt.Errorf("instance %q not found", name)
	}
	return inst, nil
}

// DeleteInstance はインスタンスを削除する（デフォルト: in-memory から除去）
func (m *MockGCEClient) DeleteInstance(ctx context.Context, project, zone, name string) error {
	if m.DeleteInstanceFn != nil {
		return m.DeleteInstanceFn(ctx, project, zone, name)
	}
	if _, ok := m.instances[name]; !ok {
		return fmt.Errorf("instance %q not found", name)
	}
	delete(m.instances, name)
	return nil
}

// ListInstances は保存済みインスタンスの一覧を返す（デフォルト: in-memory 列挙）
func (m *MockGCEClient) ListInstances(ctx context.Context, project, zone string) ([]*Instance, error) {
	if m.ListInstancesFn != nil {
		return m.ListInstancesFn(ctx, project, zone)
	}
	result := make([]*Instance, 0, len(m.instances))
	for _, inst := range m.instances {
		result = append(result, inst)
	}
	return result, nil
}
