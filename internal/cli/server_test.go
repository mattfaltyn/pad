package cli

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/PerpetualSoftware/pad/internal/config"
)

func TestEnsureServerDoesNotSpawnWhenAutoStartDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeLocal
	cfg.LoadedFromFile = true
	cfg.AutoStartLocalServer = false
	cause := errors.New("connection refused")
	err := ensureServer(cfg, func(string, int) error { return cause })
	if err == nil || !strings.Contains(err.Error(), "automatic startup is disabled") {
		t.Fatalf("EnsureServer error = %v", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("EnsureServer error = %v, want wrapped health failure", err)
	}
}

func TestEnsureServerReportsBlockedHealthCheckWithoutSpawning(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeLocal
	cfg.LoadedFromFile = true
	cfg.AutoStartLocalServer = true
	cause := fmt.Errorf("dial tcp: %w", os.ErrPermission)

	err := ensureServer(cfg, func(string, int) error { return cause })
	if err == nil || !strings.Contains(err.Error(), "blocked by execution permissions") {
		t.Fatalf("EnsureServer error = %v", err)
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("EnsureServer error = %v, want wrapped permission failure", err)
	}
}

func TestEnsureServerDoesNotSpawnWhenServerRespondsUnhealthy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeLocal
	cfg.LoadedFromFile = true
	cfg.AutoStartLocalServer = true
	cause := fmt.Errorf("%w: 503 Service Unavailable", errServerUnhealthy)

	err := ensureServer(cfg, func(string, int) error { return cause })
	if err == nil || !strings.Contains(err.Error(), "refusing to start a duplicate process") {
		t.Fatalf("EnsureServer error = %v", err)
	}
	if !errors.Is(err, errServerUnhealthy) {
		t.Fatalf("EnsureServer error = %v, want wrapped unhealthy-server failure", err)
	}
}

func TestCheckServerHealthReportsUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	host, portText, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}

	err = checkServerHealth(host, port)
	if err == nil || !strings.Contains(err.Error(), "503 Service Unavailable") {
		t.Fatalf("checkServerHealth error = %v", err)
	}
	if !errors.Is(err, errServerUnhealthy) {
		t.Fatalf("checkServerHealth error = %v, want unhealthy-server classification", err)
	}
}

func TestServerHealthEndpointFormatsHost(t *testing.T) {
	for _, test := range []struct {
		host string
		want string
	}{
		{host: "127.0.0.1", want: "http://127.0.0.1:7777/api/v1/health"},
		{host: "::1", want: "http://[::1]:7777/api/v1/health"},
		{host: "[::1]", want: "http://[::1]:7777/api/v1/health"},
	} {
		t.Run(test.host, func(t *testing.T) {
			if got := serverHealthEndpoint(test.host, 7777); got != test.want {
				t.Fatalf("serverHealthEndpoint = %q, want %q", got, test.want)
			}
		})
	}
}

func TestEnsureServerSkipsWhenClientDoesNotManageLocalServer(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := config.DefaultConfig()
	if err := EnsureServer(cfg); err != nil {
		t.Fatalf("EnsureServer with default config: %v", err)
	}
	if _, err := os.Stat(cfg.PIDFile()); !os.IsNotExist(err) {
		t.Fatalf("expected no PID file for unconfigured client, got err=%v", err)
	}

	cfg.Mode = config.ModeRemote
	cfg.LoadedFromFile = true
	if err := EnsureServer(cfg); err != nil {
		t.Fatalf("EnsureServer with remote mode: %v", err)
	}
	if _, err := os.Stat(cfg.PIDFile()); !os.IsNotExist(err) {
		t.Fatalf("expected no PID file for remote client, got err=%v", err)
	}
}
