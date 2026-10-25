package mockinference

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEndpoints(t *testing.T) {
	for _, devices := range []string{"", "fake-gpu-0", "fake-gpu-0,fake-gpu-1"} {
		s := New(devices)
		for _, tc := range []struct {
			method, path, body string
			status             int
		}{
			{"GET", "/healthz", "", 200}, {"GET", "/readyz", "", 200}, {"GET", "/device", "", 200},
			{"POST", "/inference", `{"prompt":"hello"}`, 200},
			{"GET", "/inference", "", 405}, {"POST", "/device", "", 405}, {"GET", "/missing", "", 404},
			{"POST", "/inference", `{`, 400}, {"POST", "/inference", `{}`, 400}, {"POST", "/inference", `null`, 400},
			{"POST", "/inference", `{"prompt":" "}`, 400}, {"POST", "/inference", `{"prompt":2}`, 400},
			{"POST", "/inference", `{"prompt":"hi","extra":true}`, 400}, {"POST", "/inference", `{"prompt":"hi"}{}`, 400},
			{"POST", "/inference", strings.Repeat("x", int(MaxBodyBytes)+1), 413},
		} {
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("%s %s body=%q: got %d", tc.method, tc.path, tc.body[:min(len(tc.body), 40)], w.Code)
			}
			if w.Header().Get("Content-Type") != "application/json" {
				t.Fatal("not JSON")
			}
			var body map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if tc.status == 200 && (tc.path == "/device" || tc.path == "/inference") && body["devices"] != devices {
				t.Fatal("device identity lost")
			}
			if tc.path == "/inference" && tc.status == 200 && body["response"] != "mock: hello" {
				t.Fatal("response not deterministic")
			}
			if tc.status == 405 && w.Header().Get("Allow") == "" {
				t.Fatal("missing Allow")
			}
		}
	}
}

func TestConfig(t *testing.T) {
	for _, port := range []string{"", "8080", "1", "65535"} {
		if _, err := ParseConfig(port, ""); err != nil {
			t.Fatal(err)
		}
	}
	for _, port := range []string{"0", "-1", "65536", "abc", "1.5"} {
		if _, err := ParseConfig(port, ""); err == nil {
			t.Fatalf("accepted %s", port)
		}
	}
}

func TestLifecycle(t *testing.T) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s := New("")
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, listener) }()
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}}
	address := "http://" + net.JoinHostPort("localhost", fmt.Sprint(listener.Addr().(*net.TCPAddr).Port))
	response, err := client.Get(address + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("server not ready")
	}
	cfg := Config{Port: listener.Addr().(*net.TCPAddr).Port}
	if err := Run(ctx, cfg); err == nil {
		t.Fatal("occupied port accepted")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("shutdown timed out")
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 503 {
		t.Fatal("still ready after shutdown")
	}
	if _, err := client.Get(address + "/healthz"); err == nil {
		t.Fatal("listener remains open")
	}
}
