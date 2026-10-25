package main

import (
	"context"
	"github.com/Dhiraj1729/GpuNodeSelector/internal/mockinference"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg, err := mockinference.ParseConfig(os.Getenv("PORT"), os.Getenv("FAKE_GPU_DEVICES"))
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	log.Printf("mock inference listening on port %d", cfg.Port)
	if err := mockinference.Run(ctx, cfg); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
