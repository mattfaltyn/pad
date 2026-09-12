package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Installation records where a skill was installed for a specific tool in a specific project.
type Installation struct {
	// ProjectPath is the absolute path to the project directory.
	ProjectPath string `json:"project_path"`
	// Workspace is the workspace slug (from .pad.toml), if known.
	Workspace string `json:"workspace,omitempty"`
	// Tool is the canonical tool name (e.g., "claude", "agents", "copilot").
	Tool string `json:"tool"`
	// SkillPath is the full path to the installed skill file.
	SkillPath string `json:"skill_path"`
	// InstalledAt is when the skill was last installed or updated.
	InstalledAt time.Time `json:"installed_at"`
	// Version is the pad binary version that wrote this installation.
	Version string `json:"version,omitempty"`
}

// Registry tracks all skill installations across projects for a user.
type Registry struct {
	Installations []Installation `json:"installations"`
}

// registryPath returns ~/.pad/installations.json.
func registryPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".pad", "installations.json"), nil
}

// LoadRegistry reads the installation registry from disk.
// Returns an empty registry if the file doesn't exist.
func LoadRegistry() (*Registry, error) {
	path, err := registryPath()
	if err != nil {
		return nil, err
	}
	return loadRegistryPath(path)
}

// Save writes the registry to disk.
func (r *Registry) Save() error {
	path, err := registryPath()
	if err != nil {
		return err
	}

	// Ensure ~/.pad/ exists
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create registry directory: %w", err)
	}
	unlock, err := lockRegistry(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("lock registry: %w", err)
	}
	defer unlock()
	return r.saveUnlocked(path)
}

func (r *Registry) saveUnlocked(path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}

	if err := atomicWriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write registry: %w", err)
	}
	return nil
}

// MutateRegistry serializes a read-modify-write operation across Pad processes.
func MutateRegistry(mutate func(*Registry) error) error {
	path, err := registryPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create registry directory: %w", err)
	}
	unlock, err := lockRegistry(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("lock registry: %w", err)
	}
	defer unlock()

	reg, err := loadRegistryPath(path)
	if err != nil {
		return err
	}
	if err := mutate(reg); err != nil {
		return err
	}
	return reg.saveUnlocked(path)
}

func loadRegistryPath(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{}, nil
		}
		return nil, fmt.Errorf("read registry: %w", err)
	}
	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse registry %s: %w; repair or move the file, which was left unchanged", path, err)
	}
	return &reg, nil
}

// Record adds or updates an installation entry.
func (r *Registry) Record(projectPath, workspace, tool, skillPath, version string) {
	now := time.Now().UTC()
	projectPath = cleanAbs(projectPath)
	skillPath = cleanAbs(skillPath)

	// Update existing entry if same project + tool
	for i := range r.Installations {
		inst := &r.Installations[i]
		if inst.ProjectPath == projectPath && inst.Tool == tool {
			inst.SkillPath = skillPath
			inst.Workspace = workspace
			inst.InstalledAt = now
			inst.Version = version
			return
		}
	}

	// Add new entry
	r.Installations = append(r.Installations, Installation{
		ProjectPath: projectPath,
		Workspace:   workspace,
		Tool:        tool,
		SkillPath:   skillPath,
		InstalledAt: now,
		Version:     version,
	})
}

func cleanAbs(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(abs)
}

// RemoveProjects removes registry entries for the exact project paths given.
// It intentionally leaves installed skill files untouched.
func (r *Registry) RemoveProjects(projectPaths []string) int {
	wanted := make(map[string]struct{}, len(projectPaths))
	for _, path := range projectPaths {
		wanted[cleanAbs(path)] = struct{}{}
	}
	kept := r.Installations[:0]
	removed := 0
	for _, inst := range r.Installations {
		if _, ok := wanted[cleanAbs(inst.ProjectPath)]; ok {
			removed++
			continue
		}
		kept = append(kept, inst)
	}
	r.Installations = kept
	return removed
}

// Prune removes entries whose skill files no longer exist on disk.
func (r *Registry) Prune() int {
	pruned := 0
	kept := r.Installations[:0]
	for _, inst := range r.Installations {
		if _, err := os.Stat(inst.SkillPath); err == nil {
			kept = append(kept, inst)
		} else {
			pruned++
		}
	}
	r.Installations = kept
	return pruned
}

// InstallationStatus describes the state of a tracked installation.
type InstallationStatus struct {
	Installation
	Exists   bool `json:"exists"`
	Outdated bool `json:"outdated"`
}

// Status checks each tracked installation and returns its current state.
// embeddedContent is the raw embedded skill bytes, used for freshness comparison.
func (r *Registry) Status(embeddedContent []byte) []InstallationStatus {
	var results []InstallationStatus
	for _, inst := range r.Installations {
		s := InstallationStatus{Installation: inst}

		data, err := os.ReadFile(inst.SkillPath)
		if err != nil {
			s.Exists = false
			s.Outdated = true
			results = append(results, s)
			continue
		}

		s.Exists = true

		// Resolve what the content *should* be for this tool
		tool := ResolveTool(inst.Tool)
		if tool == nil {
			// Unknown tool — compare raw
			s.Outdated = !bytes.Equal(data, embeddedContent)
		} else {
			expected := FormatForTool(*tool, embeddedContent)
			s.Outdated = !bytes.Equal(data, expected)
		}

		results = append(results, s)
	}
	return results
}

// UpdateAll updates all tracked installations that are outdated.
// Returns the number of installations updated and any errors encountered.
func (r *Registry) UpdateAll(embeddedContent []byte, version string) (updated int, errors []error) {
	for i := range r.Installations {
		inst := &r.Installations[i]

		tool := ResolveTool(inst.Tool)
		if tool == nil {
			errors = append(errors, fmt.Errorf("%s: unknown tool %q", inst.ProjectPath, inst.Tool))
			continue
		}

		// Check if file exists
		currentData, err := os.ReadFile(inst.SkillPath)
		if err != nil {
			errors = append(errors, fmt.Errorf("%s (%s): file missing, skipping", inst.ProjectPath, tool.Label))
			continue
		}

		expected := FormatForTool(*tool, embeddedContent)
		if bytes.Equal(currentData, expected) {
			continue // already up to date
		}

		// Ensure directory exists (in case it was partially deleted)
		if err := os.MkdirAll(filepath.Dir(inst.SkillPath), 0755); err != nil {
			errors = append(errors, fmt.Errorf("%s (%s): %w", inst.ProjectPath, tool.Label, err))
			continue
		}

		if err := atomicWriteFile(inst.SkillPath, expected, 0644); err != nil {
			errors = append(errors, fmt.Errorf("%s (%s): %w", inst.ProjectPath, tool.Label, err))
			continue
		}

		inst.InstalledAt = time.Now().UTC()
		inst.Version = version
		updated++
	}

	return updated, errors
}
