package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/Dhiraj1729/GpuNodeSelector/internal/fakegpu"
)

func main() {
	count := flag.Int("devices", 2, "Number of simulated devices (1..256)")
	dir := flag.String("socket-dir", "/var/lib/kubelet/device-plugins", "Kubelet device-plugin socket directory")
	health := flag.Bool("health", false, "Check plugin gRPC serving health and exit")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if *health {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := fakegpu.Check(ctx, *dir); err != nil {
			log.Fatal(err)
		}
		return
	}
	p, err := fakegpu.New(*count)
	if err != nil {
		log.Fatal(err)
	}
	if err = p.Run(ctx, *dir); err != nil {
		log.Fatal(err)
	}
}
