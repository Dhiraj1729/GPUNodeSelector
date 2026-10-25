package fakegpu

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	api "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

func TestAllocate(t *testing.T) {
	for _, n := range []int{-1, 0, 257} {
		if _, err := New(n); err == nil {
			t.Fatalf("accepted %d", n)
		}
	}
	p, _ := New(2)
	cases := []struct {
		name  string
		ids   [][]string
		valid bool
		want  string
	}{
		{"one", [][]string{{"fake-gpu-0"}}, true, "fake-gpu-0"},
		{"two", [][]string{{"fake-gpu-1", "fake-gpu-0"}}, true, "fake-gpu-0,fake-gpu-1"},
		{"containers", [][]string{{"fake-gpu-0"}, {"fake-gpu-1"}}, true, "fake-gpu-0"},
		{"unknown", [][]string{{"unknown"}}, false, ""},
		{"duplicate", [][]string{{"fake-gpu-0", "fake-gpu-0"}}, false, ""},
		{"cross-container", [][]string{{"fake-gpu-0"}, {"fake-gpu-0"}}, false, ""},
		{"empty", [][]string{{}}, false, ""},
		{"no-containers", nil, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &api.AllocateRequest{}
			for _, ids := range tc.ids {
				req.ContainerRequests = append(req.ContainerRequests, &api.ContainerAllocateRequest{DevicesIds: ids})
			}
			for i := 0; i < 2; i++ {
				response, err := p.Allocate(context.Background(), req)
				if !tc.valid {
					if status.Code(err) != codes.InvalidArgument {
						t.Fatalf("expected invalid: %v", err)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				if len(response.ContainerResponses) != len(tc.ids) || response.ContainerResponses[0].Envs["FAKE_GPU_DEVICES"] != tc.want {
					t.Fatal(response)
				}
			}
		})
	}
	for _, req := range []*api.AllocateRequest{nil, {ContainerRequests: []*api.ContainerAllocateRequest{nil}}} {
		if _, err := p.Allocate(context.Background(), req); status.Code(err) != codes.InvalidArgument {
			t.Fatal(err)
		}
	}
	opts, _ := p.GetDevicePluginOptions(context.Background(), &api.Empty{})
	if opts.PreStartRequired || opts.GetPreferredAllocationAvailable {
		t.Fatal("unsupported options advertised")
	}
}

type registrar struct {
	api.UnimplementedRegistrationServer
	dir        string
	requests   chan *api.RegisterRequest
	rejectOnce atomic.Bool
}

func (r *registrar) Register(ctx context.Context, req *api.RegisterRequest) (*api.Empty, error) {
	if err := Check(ctx, r.dir); err != nil {
		return nil, err
	}
	if r.rejectOnce.Swap(false) {
		return nil, status.Error(codes.Unavailable, "simulated kubelet startup")
	}
	select {
	case r.requests <- req:
	default:
	}
	return &api.Empty{}, nil
}

func TestSocketLifecycle(t *testing.T) {
	// Keep below macOS Unix socket path limits.
	dir, err := os.MkdirTemp("/tmp", "fakegpu-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	requests := make(chan *api.RegisterRequest, 8)
	startKubelet := func() func() {
		t.Helper()
		listener, err := net.Listen("unix", filepath.Join(dir, "kubelet.sock"))
		if err != nil {
			t.Fatal(err)
		}
		s := grpc.NewServer()
		registration := &registrar{dir: dir, requests: requests}
		registration.rejectOnce.Store(true)
		api.RegisterRegistrationServer(s, registration)
		go s.Serve(listener)
		return func() { s.Stop(); listener.Close() }
	}
	stopKubelet := startKubelet()
	defer func() { stopKubelet() }()
	p, _ := New(2)
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx, dir) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(4 * time.Second):
			t.Error("shutdown timeout")
		}
		if _, err := os.Lstat(filepath.Join(dir, SocketName)); !os.IsNotExist(err) {
			t.Errorf("plugin socket not removed: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "kubelet.sock")); err != nil {
			t.Error("kubelet socket removed during shutdown")
		}
	}()
	registered := func() {
		t.Helper()
		select {
		case req := <-requests:
			if req.ResourceName != ResourceName || req.Endpoint != SocketName || req.Version != api.Version {
				t.Fatal(req)
			}
		case err := <-done:
			t.Fatalf("plugin exited: %v", err)
		case <-ctx.Done():
			t.Fatal("registration timeout")
		}
	}
	registered()
	conn, err := connection(filepath.Join(dir, SocketName))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := api.NewDevicePluginClient(conn)
	streamCtx, stopStream := context.WithCancel(ctx)
	stream, err := client.ListAndWatch(streamCtx, &api.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Devices) != 2 || inventory.Devices[0].ID != "fake-gpu-0" || inventory.Devices[1].ID != "fake-gpu-1" || inventory.Devices[0].Health != api.Healthy {
		t.Fatal(inventory)
	}
	stopStream()
	if _, err := stream.Recv(); status.Code(err) != codes.Canceled {
		t.Fatalf("stream cancellation: %v", err)
	}
	req := &api.AllocateRequest{ContainerRequests: []*api.ContainerAllocateRequest{{DevicesIds: []string{"fake-gpu-1"}}}}
	for i := 0; i < 2; i++ {
		res, err := client.Allocate(ctx, req)
		if err != nil || res.ContainerResponses[0].Envs["FAKE_GPU_DEVICES"] != "fake-gpu-1" {
			t.Fatalf("RPC: %v %v", res, err)
		}
	}
	second, _ := New(2)
	if err := second.Run(ctx, dir); err == nil {
		t.Fatal("concurrent listener accepted")
	}
	// Socket-only deletion, then kubelet restart (both paths) are separate cases.
	if err := os.Remove(filepath.Join(dir, SocketName)); err != nil {
		t.Fatal(err)
	}
	registered()
	stopKubelet()
	if err := os.Remove(filepath.Join(dir, SocketName)); err != nil {
		t.Fatal(err)
	}
	stopKubelet = startKubelet()
	registered()
	cancel()
	// Deferred shutdown validates the goroutine exits. Kubelet's socket must survive.
	if _, err := os.Stat(filepath.Join(dir, "kubelet.sock")); err != nil {
		t.Fatal("plugin removed kubelet socket")
	}
}

func TestRefusesNonSocket(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, SocketName)
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	p, _ := New(2)
	if err := p.Run(context.Background(), dir); err == nil {
		t.Fatal("accepted regular file")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "keep" {
		t.Fatal("unrelated file changed")
	}
	if err := p.Run(context.Background(), "relative"); err == nil {
		t.Fatal("accepted relative path")
	}
}
