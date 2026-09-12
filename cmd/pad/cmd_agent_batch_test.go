package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/PerpetualSoftware/pad/internal/cli"
)

func TestAgentInstallProjectsAndForget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	projects := []string{t.TempDir(), t.TempDir()}
	for i, project := range projects {
		body := []byte("workspace = \"fleet-" + string(rune('a'+i)) + "\"\n")
		if err := os.WriteFile(filepath.Join(project, ".pad.toml"), body, 0600); err != nil {
			t.Fatal(err)
		}
	}

	cmd := installCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"cursor", "--project", projects[0], "--project", projects[1]})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	reg, err := cli.LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Installations) != 2 {
		t.Fatalf("installations = %d, want 2", len(reg.Installations))
	}
	for _, project := range projects {
		if _, err := os.Stat(filepath.Join(project, ".agents", "skills", "pad", "SKILL.md")); err != nil {
			t.Fatalf("skill missing from %s: %v", project, err)
		}
	}

	forget := agentForgetCmd()
	forget.SetOut(&out)
	forget.SetArgs([]string{"--project", projects[0]})
	if err := forget.Execute(); err != nil {
		t.Fatal(err)
	}
	reg, err = cli.LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Installations) != 1 || reg.Installations[0].ProjectPath != projects[1] {
		t.Fatalf("remaining installations = %#v", reg.Installations)
	}
	if _, err := os.Stat(filepath.Join(projects[0], ".agents", "skills", "pad", "SKILL.md")); err != nil {
		t.Fatalf("forget removed skill file: %v", err)
	}
}

func TestAgentInstallProjectRequiresOneTool(t *testing.T) {
	cmd := installCmd()
	cmd.SetArgs([]string{"--project", t.TempDir()})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected missing tool error")
	}
}
