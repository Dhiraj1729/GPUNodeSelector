// Package mockinference serves deterministic CPU-only responses.
package mockinference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const MaxBodyBytes int64 = 64 << 10

type Config struct {
	Port    int
	Devices string
}

func ParseConfig(port, devices string) (Config, error) {
	if port == "" {
		port = "8080"
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return Config{}, fmt.Errorf("PORT must be an integer from 1 to 65535")
	}
	return Config{Port: n, Devices: devices}, nil
}

type Server struct {
	devices string
	ready   atomic.Bool
}

func New(devices string) *Server { s := &Server{devices: devices}; s.ready.Store(true); return s }

func reply(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	method := "GET"
	switch r.URL.Path {
	case "/healthz", "/readyz", "/device":
	case "/inference":
		method = "POST"
	default:
		reply(w, 404, map[string]string{"error": "not found"})
		return
	}
	if r.Method != method {
		w.Header().Set("Allow", method)
		reply(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	switch r.URL.Path {
	case "/healthz":
		reply(w, 200, map[string]string{"status": "ok"})
	case "/readyz":
		if !s.ready.Load() {
			reply(w, 503, map[string]string{"status": "not ready"})
			return
		}
		reply(w, 200, map[string]string{"status": "ready"})
	case "/device":
		reply(w, 200, map[string]string{"devices": s.devices})
	case "/inference":
		if !s.ready.Load() {
			reply(w, 503, map[string]string{"error": "shutting down"})
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				reply(w, 413, map[string]string{"error": "request too large"})
			} else {
				reply(w, 400, map[string]string{"error": "cannot read request"})
			}
			return
		}
		var request struct {
			Prompt string `json:"prompt"`
		}
		decoder := json.NewDecoder(strings.NewReader(string(body)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil || strings.TrimSpace(request.Prompt) == "" {
			reply(w, 400, map[string]string{"error": "a nonempty prompt is required"})
			return
		}
		if decoder.Decode(new(any)) != io.EOF {
			reply(w, 400, map[string]string{"error": "exactly one JSON object is required"})
			return
		}
		reply(w, 200, map[string]string{"response": "mock: " + request.Prompt, "devices": s.devices})
	}
}

// Serve owns listener lifecycle and drains requests on cancellation.
func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	server := &http.Server{Handler: s, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case err := <-done:
		s.ready.Store(false)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		s.ready.Store(false)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			<-done
			return fmt.Errorf("shutdown: %w", err)
		}
		err := <-done
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func Run(ctx context.Context, cfg Config) error {
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("invalid port %d", cfg.Port)
	}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	return New(cfg.Devices).Serve(ctx, listener)
}
