package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/PerpetualSoftware/pad/internal/config"
)

func TestEnsureServerDoesNotSpawnWhenAutoStartDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeLocal
	cfg.LoadedFromFile = true
	cfg.AutoStartLocalServer = false
	cfg.Port = 1
	err := EnsureServer(cfg)
	if err == nil || !strings.Contains(err.Error(), "automatic startup is disabled") {
		t.Fatalf("EnsureServer error = %v", err)
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
