package main

import (
	"testing"

	"github.com/PerpetualSoftware/pad/internal/config"
)

func TestSetupHostPort(t *testing.T) {
	for _, test := range []struct {
		host string
		want string
	}{
		{host: "127.0.0.1", want: "127.0.0.1:7777"},
		{host: "::1", want: "[::1]:7777"},
		{host: "::", want: "<your-host>:7777"},
	} {
		cfg := &config.Config{Host: test.host, Port: 7777}
		if got := setupHostPort(cfg); got != test.want {
			t.Fatalf("setupHostPort for %q = %q, want %q", test.host, got, test.want)
		}
	}
}
