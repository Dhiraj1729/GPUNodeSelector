package fakegpu

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	api "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

func connection(path string) (*grpc.ClientConn, error) {
	return grpc.NewClient("passthrough:///"+path, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", path)
	}))
}

func Check(ctx context.Context, dir string) error {
	conn, err := connection(filepath.Join(dir, SocketName))
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = api.NewDevicePluginClient(conn).GetDevicePluginOptions(ctx, &api.Empty{})
	return err
}

// Run holds a process lock and recovers its socket after kubelet recreation.
// Registration calls have deadlines, and retries back off from 1 to 8 seconds.
func (p *Plugin) Run(ctx context.Context, dir string) error {
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("socket directory must be absolute")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("socket directory is not a directory")
	}
	fd, err := unix.Open(filepath.Join(dir, "gpu-node-selector-fake.lock"), unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return fmt.Errorf("another plugin owns this socket: %w", err)
	}
	defer unix.Flock(fd, unix.LOCK_UN)
	socket := filepath.Join(dir, SocketName)
	for ctx.Err() == nil {
		err = p.serve(ctx, dir, socket)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Plugin) serve(ctx context.Context, dir, socket string) error {
	if info, err := os.Lstat(socket); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return fmt.Errorf("refusing to remove non-socket %s", socket)
		}
		// The lock excludes another instance of this plugin; do not replace a live listener.
		conn, err := net.DialTimeout("unix", socket, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return fmt.Errorf("socket already has a live listener")
		}
		if err := os.Remove(socket); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		return err
	}
	listener.SetUnlinkOnClose(false)
	original, err := os.Lstat(socket)
	if err != nil {
		listener.Close()
		return err
	}
	server := grpc.NewServer()
	api.RegisterDevicePluginServer(server, p)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	defer func() {
		server.Stop()
		listener.Close()
		if current, err := os.Lstat(socket); err == nil && os.SameFile(original, current) {
			_ = os.Remove(socket)
		}
	}()
	readyCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	err = Check(readyCtx, dir)
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("server readiness: %w", err)
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var registered os.FileInfo
	retryAt := time.Time{}
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return nil
		}
		current, err := os.Lstat(socket)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		} // kubelet removed our socket; rebuild it
		if err != nil {
			return err
		}
		if !os.SameFile(original, current) {
			return fmt.Errorf("plugin socket replaced by another process")
		}
		kubelet, statErr := os.Stat(filepath.Join(dir, "kubelet.sock"))
		changed := statErr == nil && (registered == nil || !os.SameFile(registered, kubelet))
		if changed && !time.Now().Before(retryAt) {
			err = p.register(ctx, dir)
			if err == nil {
				registered = kubelet
				backoff = time.Second
				log.Printf("registered resource=%s devices=%d", ResourceName, len(p.ids))
			} else {
				log.Printf("registration retry: %v", err)
				retryAt = time.Now().Add(backoff)
				if backoff < 8*time.Second {
					backoff *= 2
				}
			}
		}
		if statErr != nil {
			registered = nil
		}
		select {
		case <-ctx.Done():
			return nil
		case err := <-done:
			if err != nil {
				return fmt.Errorf("gRPC server: %w", err)
			}
			return fmt.Errorf("gRPC server stopped unexpectedly")
		case <-ticker.C:
		}
	}
}

func (p *Plugin) register(ctx context.Context, dir string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	conn, err := connection(filepath.Join(dir, "kubelet.sock"))
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = api.NewRegistrationClient(conn).Register(ctx, &api.RegisterRequest{Version: api.Version, Endpoint: SocketName, ResourceName: ResourceName, Options: &api.DevicePluginOptions{}})
	return err
}
