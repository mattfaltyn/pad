package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallForToolAtDoesNotDependOnWorkingDirectory(t *testing.T) {
	project := t.TempDir()
	before, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tool := *ResolveTool("cursor")
	path, err := InstallForToolAt(project, tool, []byte("skill body\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(project, ".agents", "skills", "pad", "SKILL.md")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "skill body\n" {
		t.Fatalf("content = %q", got)
	}
	after, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("working directory changed from %q to %q", before, after)
	}
}

func TestLoadRegistryRejectsCorruptionWithoutReplacingIt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".pad", "installations.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	corrupt := []byte("{not-json\n")
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRegistry()
	if err == nil || !strings.Contains(err.Error(), "left unchanged") {
		t.Fatalf("LoadRegistry error = %v", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(corrupt) {
		t.Fatalf("corrupt registry changed: %q", got)
	}
}

func TestRemoveProjectsIsExactAndLeavesFiles(t *testing.T) {
	project := t.TempDir()
	other := project + "-other"
	skill := filepath.Join(project, "SKILL.md")
	if err := os.WriteFile(skill, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	reg := &Registry{Installations: []Installation{
		{ProjectPath: project, Tool: "agents", SkillPath: skill},
		{ProjectPath: other, Tool: "agents", SkillPath: filepath.Join(other, "SKILL.md")},
	}}
	if removed := reg.RemoveProjects([]string{project}); removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if len(reg.Installations) != 1 || reg.Installations[0].ProjectPath != other {
		t.Fatalf("remaining = %#v", reg.Installations)
	}
	if _, err := os.Stat(skill); err != nil {
		t.Fatalf("skill file was removed: %v", err)
	}
}
