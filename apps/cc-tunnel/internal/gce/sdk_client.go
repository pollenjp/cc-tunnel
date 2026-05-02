package gce

import (
	"context"
	"fmt"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/proto"
)

const (
	cosImage = "projects/cos-cloud/global/images/family/cos-stable"
)

// SDKGCEClient は cloud.google.com/go/compute/apiv1 を使った GCEClient 実装
type SDKGCEClient struct {
	instancesClient *compute.InstancesClient
}

var _ GCEClient = (*SDKGCEClient)(nil) // コンパイル時インターフェース確認

// NewSDKGCEClient は Application Default Credentials を使って SDKGCEClient を作成する
func NewSDKGCEClient(ctx context.Context) (*SDKGCEClient, error) {
	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("compute instances client: %w", err)
	}
	return &SDKGCEClient{instancesClient: c}, nil
}

// Close はクライアントを閉じる
func (c *SDKGCEClient) Close() error {
	return c.instancesClient.Close()
}

// CreateInstance は GCE VM インスタンスを作成する
// COS (Container-Optimized OS) を使用し、startup-script で cc-remote-agent を起動する
func (c *SDKGCEClient) CreateInstance(ctx context.Context, req *CreateInstanceRequest) (*Instance, error) {
	machineTypeURL := fmt.Sprintf("zones/%s/machineTypes/%s", req.Zone, req.MachineType)

	metadata := []*computepb.Items{
		{
			Key:   proto.String("startup-script"),
			Value: proto.String(req.StartupScript),
		},
	}

	var labels map[string]string
	if len(req.Labels) > 0 {
		labels = req.Labels
	}

	var tags *computepb.Tags
	if len(req.Tags) > 0 {
		tags = &computepb.Tags{Items: req.Tags}
	}

	insertReq := &computepb.InsertInstanceRequest{
		Project: req.Project,
		Zone:    req.Zone,
		InstanceResource: &computepb.Instance{
			Name:        proto.String(req.Name),
			MachineType: proto.String(machineTypeURL),
			Labels:      labels,
			Tags:        tags,
			Disks: []*computepb.AttachedDisk{
				{
					Boot:       proto.Bool(true),
					AutoDelete: proto.Bool(true),
					InitializeParams: &computepb.AttachedDiskInitializeParams{
						SourceImage: proto.String(cosImage),
					},
				},
			},
			NetworkInterfaces: []*computepb.NetworkInterface{
				{
					Name: proto.String("global/networks/default"),
				},
			},
			Metadata: &computepb.Metadata{
				Items: metadata,
			},
		},
	}

	op, err := c.instancesClient.Insert(ctx, insertReq)
	if err != nil {
		return nil, fmt.Errorf("insert instance: %w", err)
	}
	if err := op.Wait(ctx); err != nil {
		return nil, fmt.Errorf("wait insert instance: %w", err)
	}

	return c.GetInstance(ctx, req.Project, req.Zone, req.Name)
}

// GetInstance は VM インスタンスの情報を取得する
func (c *SDKGCEClient) GetInstance(ctx context.Context, project, zone, name string) (*Instance, error) {
	getReq := &computepb.GetInstanceRequest{
		Project:  project,
		Zone:     zone,
		Instance: name,
	}
	inst, err := c.instancesClient.Get(ctx, getReq)
	if err != nil {
		return nil, fmt.Errorf("get instance %q: %w", name, err)
	}
	return toInstance(inst), nil
}

// DeleteInstance は VM インスタンスを削除する
func (c *SDKGCEClient) DeleteInstance(ctx context.Context, project, zone, name string) error {
	delReq := &computepb.DeleteInstanceRequest{
		Project:  project,
		Zone:     zone,
		Instance: name,
	}
	op, err := c.instancesClient.Delete(ctx, delReq)
	if err != nil {
		return fmt.Errorf("delete instance %q: %w", name, err)
	}
	if err := op.Wait(ctx); err != nil {
		return fmt.Errorf("wait delete instance %q: %w", name, err)
	}
	return nil
}

// ListInstances は指定プロジェクト・ゾーンのインスタンス一覧を返す
func (c *SDKGCEClient) ListInstances(ctx context.Context, project, zone string) ([]*Instance, error) {
	listReq := &computepb.ListInstancesRequest{
		Project: project,
		Zone:    zone,
	}
	it := c.instancesClient.List(ctx, listReq)

	var instances []*Instance
	for {
		inst, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("list instances: %w", err)
		}
		instances = append(instances, toInstance(inst))
	}
	return instances, nil
}

// toInstance は computepb.Instance を内部 Instance 型に変換する
func toInstance(inst *computepb.Instance) *Instance {
	result := &Instance{
		Labels: inst.GetLabels(),
	}
	if inst.Name != nil {
		result.Name = *inst.Name
	}
	if inst.Status != nil {
		result.Status = *inst.Status
	}
	// 最初のネットワークインターフェースの内部 IP を取得
	if len(inst.NetworkInterfaces) > 0 {
		ni := inst.NetworkInterfaces[0]
		if ni.NetworkIP != nil {
			result.NetworkIP = *ni.NetworkIP
		}
	}
	return result
}
