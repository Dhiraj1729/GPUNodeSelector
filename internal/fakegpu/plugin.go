// Package fakegpu implements a simulated device inventory; kubelet owns allocation.
package fakegpu

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	api "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const ResourceName = "ai.dhiraj.io/gpu"
const SocketName = "gpu-node-selector-fake.sock"

type Plugin struct {
	api.UnimplementedDevicePluginServer
	ids []string
}

func New(count int) (*Plugin, error) {
	if count < 1 || count > 256 {
		return nil, fmt.Errorf("device count must be between 1 and 256")
	}
	p := &Plugin{}
	for i := 0; i < count; i++ {
		p.ids = append(p.ids, fmt.Sprintf("fake-gpu-%d", i))
	}
	return p, nil
}

func (p *Plugin) GetDevicePluginOptions(context.Context, *api.Empty) (*api.DevicePluginOptions, error) {
	return &api.DevicePluginOptions{}, nil
}

func (p *Plugin) ListAndWatch(_ *api.Empty, stream grpc.ServerStreamingServer[api.ListAndWatchResponse]) error {
	devices := make([]*api.Device, 0, len(p.ids))
	for _, id := range p.ids {
		devices = append(devices, &api.Device{ID: id, Health: api.Healthy})
	}
	if err := stream.Send(&api.ListAndWatchResponse{Devices: devices}); err != nil {
		return err
	}
	<-stream.Context().Done()
	return stream.Context().Err()
}

func (p *Plugin) Allocate(_ context.Context, req *api.AllocateRequest) (*api.AllocateResponse, error) {
	if req == nil || len(req.ContainerRequests) == 0 {
		return nil, status.Error(codes.InvalidArgument, "container requests are required")
	}
	known := map[string]bool{}
	for _, id := range p.ids {
		known[id] = true
	}
	seen := map[string]bool{}
	result := &api.AllocateResponse{}
	for _, container := range req.ContainerRequests {
		if container == nil || len(container.DevicesIds) == 0 {
			return nil, status.Error(codes.InvalidArgument, "device IDs are required")
		}
		ids := append([]string(nil), container.DevicesIds...)
		for _, id := range ids {
			if !known[id] || seen[id] {
				return nil, status.Errorf(codes.InvalidArgument, "unknown or duplicate device ID %q", id)
			}
			seen[id] = true
		}
		sort.Strings(ids)
		result.ContainerResponses = append(result.ContainerResponses, &api.ContainerAllocateResponse{Envs: map[string]string{"FAKE_GPU_DEVICES": strings.Join(ids, ",")}})
	}
	return result, nil
}
