package selfreaper

import (
	"context"
	"fmt"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
)

// SDKDeleter is the production InstanceDeleter backed by
// cloud.google.com/go/compute/apiv1.
//
// The deletion is dispatched but the long-running operation is
// intentionally NOT awaited. The VM SA only needs
// compute.instances.delete on the calling instance, and once the API
// accepts the request GCE will tear the VM down without further help
// from this process — which is convenient, since this process is about
// to be killed by that very teardown.
type SDKDeleter struct {
	client *compute.InstancesClient
}

// NewSDKDeleter constructs an SDKDeleter using application default
// credentials (the VM's attached SA on GCE).
func NewSDKDeleter(ctx context.Context) (*SDKDeleter, error) {
	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("new InstancesRESTClient: %w", err)
	}
	return &SDKDeleter{client: c}, nil
}

// Close releases the underlying gRPC/REST client.
func (d *SDKDeleter) Close() error {
	if d.client == nil {
		return nil
	}
	return d.client.Close()
}

// DeleteInstance calls instances.Delete and returns once the API has
// accepted the request. The returned operation is not waited on; see
// the package doc for rationale.
func (d *SDKDeleter) DeleteInstance(ctx context.Context, project, zone, name string) error {
	_, err := d.client.Delete(ctx, &computepb.DeleteInstanceRequest{
		Project:  project,
		Zone:     zone,
		Instance: name,
	})
	if err != nil {
		return fmt.Errorf("instances.Delete(%s/%s/%s): %w", project, zone, name, err)
	}
	return nil
}
