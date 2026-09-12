package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/spf13/cobra"

	pad "github.com/PerpetualSoftware/pad"
	"github.com/PerpetualSoftware/pad/internal/cli"
)

func agentGuideCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "guide [topic]",
		Short: "Print Pad agent guidance on demand",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := string(cli.StripFrontmatter(pad.PadSkill))
			if len(args) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Available Pad agent guide topics:")
				for _, topic := range agentGuideTopics(body) {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", topic)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "\nUse `pad agent guide <topic>` for one section or `pad agent guide all` for the full guide.")
				return nil
			}

			if args[0] == "all" {
				fmt.Fprint(cmd.OutOrStdout(), body)
				return nil
			}
			section, ok := agentGuideSection(body, args[0])
			if !ok {
				return fmt.Errorf("unknown guide topic %q; run `pad agent guide` to list topics", args[0])
			}
			fmt.Fprint(cmd.OutOrStdout(), section)
			return nil
		},
	}
}

func agentGuideTopics(markdown string) []string {
	var topics []string
	for _, line := range strings.Split(markdown, "\n") {
		_, topic, ok := agentGuideHeading(line)
		if ok {
			topics = append(topics, topic)
		}
	}
	return topics
}

func agentGuideSection(markdown, topic string) (string, bool) {
	want := agentGuideSlug(topic)
	lines := strings.Split(markdown, "\n")
	start, level := -1, 0
	for i, line := range lines {
		lineLevel, lineTopic, ok := agentGuideHeading(line)
		if start < 0 {
			if ok && lineTopic == want {
				start, level = i, lineLevel
			}
			continue
		}
		if ok && lineLevel <= level {
			return strings.Join(lines[start:i], "\n") + "\n", true
		}
	}
	if start >= 0 {
		return strings.Join(lines[start:], "\n"), true
	}
	return "", false
}

func agentGuideHeading(line string) (level int, topic string, ok bool) {
	line = strings.TrimSpace(line)
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level < 2 || level >= len(line) || line[level] != ' ' {
		return 0, "", false
	}
	topic = agentGuideSlug(line[level+1:])
	return level, topic, topic != ""
}

func agentGuideSlug(s string) string {
	var out strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if dash && out.Len() > 0 {
				out.WriteByte('-')
			}
			out.WriteRune(r)
			dash = false
		} else {
			dash = true
		}
	}
	return out.String()
}

func installCmd() *cobra.Command {
	var projectPaths []string
	cmd := &cobra.Command{
		Use:   "install [tool]",
		Short: "Install the /pad skill for your AI coding tools",
		Long: `Install the Pad skill file for AI coding tools.

With no arguments, auto-detects tools in use and offers to install for each.
Specify a tool name to install for that tool directly.

Supported tools:
  claude       Claude Code (.claude/skills/)
  cursor       Cursor (.agents/skills/) — also covers Codex, Windsurf & OpenCode
  codex        OpenAI Codex (.agents/skills/)
  windsurf     Windsurf (.agents/skills/)
  opencode     OpenCode (.agents/skills/)
  copilot      GitHub Copilot (.github/instructions/)
  amazon-q     Amazon Q Developer (.amazonq/rules/)
  junie        JetBrains Junie (.junie/guidelines/)

Examples:
  pad agent install              # Auto-detect and install
  pad agent install claude       # Install for Claude Code
  pad agent install cursor       # Install for Cursor/Codex/Windsurf/OpenCode
  pad agent install cursor --project ../api --project ../web
  pad agent install opencode     # Install for OpenCode
  pad agent install --all        # Install for all detected tools
  pad agent status               # Show supported tools and status`,
		ValidArgs: cli.AllToolNames(),
		Args:      cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			listFlag, _ := cmd.Flags().GetBool("list")
			allFlag, _ := cmd.Flags().GetBool("all")
			updateFlag, _ := cmd.Flags().GetBool("update")
			if len(projectPaths) > 0 {
				if listFlag || allFlag || updateFlag {
					return errors.New("--project requires one explicit tool and cannot be combined with --list, --all, or --update")
				}
				if len(args) != 1 {
					return errors.New("--project requires exactly one tool")
				}
				return installForProjects(cmd, args[0], projectPaths)
			}

			if listFlag {
				return installList()
			}

			if updateFlag {
				return installUpdate()
			}

			if len(args) > 0 {
				return installForTool(args[0])
			}

			if allFlag {
				return installAll()
			}

			return installInteractive()
		},
	}
	cmd.Flags().Bool("list", false, "list supported tools and installation status")
	cmd.Flags().Bool("all", false, "install for all detected tools")
	cmd.Flags().Bool("update", false, "update all installed tool integrations")
	cmd.Flags().StringArrayVar(&projectPaths, "project", nil, "project directory to install into (repeatable; requires a tool)")
	return cmd
}

func installList() error {
	// Show local tool status (current directory)
	detected := map[string]bool{}
	for _, t := range cli.DetectTools() {
		detected[t.Name] = true
	}

	fmt.Println("Supported tools:")
	fmt.Println()
	for _, tool := range cli.SupportedTools {
		status := "  not installed"
		if cli.ToolInstalled(tool) {
			status = "  installed ✓"
		}
		det := ""
		if detected[tool.Name] {
			det = " (detected)"
		}
		aliases := ""
		if len(tool.Aliases) > 0 {
			aliases = fmt.Sprintf(" [aliases: %s]", strings.Join(tool.Aliases, ", "))
		}
		fmt.Printf("  %-12s %s%s%s%s\n", tool.Name, tool.Label, aliases, det, status)
	}

	// Show global installation registry
	var statuses []cli.InstallationStatus
	if err := cli.MutateRegistry(func(reg *cli.Registry) error {
		reg.Prune()
		statuses = reg.Status(pad.PadSkill)
		return nil
	}); err != nil {
		return err
	}
	if len(statuses) == 0 {
		return nil
	}

	fmt.Println()
	fmt.Println("Tracked installations:")
	fmt.Println()

	outdatedCount := 0
	for _, s := range statuses {
		tool := cli.ResolveTool(s.Tool)
		toolLabel := s.Tool
		if tool != nil {
			toolLabel = tool.Label
		}

		state := "✓ up to date"
		if !s.Exists {
			state = "✗ missing"
		} else if s.Outdated {
			state = "⟳ update available"
			outdatedCount++
		}

		fmt.Printf("  %-40s  %-28s  %s\n", s.ProjectPath, toolLabel, state)
	}

	if outdatedCount > 0 {
		fmt.Printf("\n  %d installation(s) can be updated. Run 'pad agent update' to update all.\n", outdatedCount)
	}

	return nil
}

func installUpdate() error {
	// Phase 1: Update tools installed in the current directory
	localUpdated := 0
	for _, tool := range cli.SupportedTools {
		if !cli.ToolInstalled(tool) {
			continue
		}
		content := cli.FormatForTool(tool, pad.PadSkill)
		path, err := cli.InstallForTool(tool, content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ %s: %v\n", tool.Label, err)
			continue
		}
		fmt.Printf("  ✓ Updated %s → %s\n", tool.Label, path)
		if err := recordInstallation(tool.Name, path); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: record %s installation: %v\n", tool.Label, err)
		}
		localUpdated++
	}

	// Phase 2: Update all tracked installations across other projects
	globalUpdated := 0
	installationCount := 0
	var updateErrors []error
	err := cli.MutateRegistry(func(reg *cli.Registry) error {
		reg.Prune()
		globalUpdated, updateErrors = reg.UpdateAll(pad.PadSkill, version)
		installationCount = len(reg.Installations)
		return nil
	})
	if err != nil {
		return err
	}

	for _, e := range updateErrors {
		fmt.Fprintf(os.Stderr, "  warning: %v\n", e)
	}

	remoteUpdated := globalUpdated
	total := localUpdated + remoteUpdated
	if total == 0 {
		if localUpdated == 0 && installationCount == 0 {
			fmt.Println("No tools installed. Run 'pad agent install' first.")
		} else {
			fmt.Println("All installations are up to date.")
		}
	} else {
		if remoteUpdated > 0 {
			fmt.Printf("\nUpdated %d installation(s) across all projects.\n", total)
		} else {
			fmt.Printf("\nUpdated %d tool(s) in current project.\n", localUpdated)
		}
	}
	return nil
}

// recordInstallation stores a skill install in the global registry (~/.pad/installations.json).
func recordInstallation(toolName, skillPath string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	return recordInstallationAt(cwd, toolName, skillPath)
}

func recordInstallationAt(projectPath, toolName, skillPath string) error {
	ws, _ := cli.DetectWorkspaceFrom(projectPath, "")
	return cli.MutateRegistry(func(reg *cli.Registry) error {
		reg.Record(projectPath, ws, toolName, skillPath, version)
		return nil
	})
}

func installForTool(name string) error {
	tool := cli.ResolveTool(name)
	if tool == nil {
		return fmt.Errorf("unknown tool %q. Run 'pad agent status' to see supported tools", name)
	}

	content := cli.FormatForTool(*tool, pad.PadSkill)
	path, err := cli.InstallForTool(*tool, content)
	if err != nil {
		return err
	}
	fmt.Printf("Installed /pad skill for %s → %s\n", tool.Label, path)
	return recordInstallation(tool.Name, path)
}

func installForProjects(cmd *cobra.Command, name string, projectPaths []string) error {
	tool := cli.ResolveTool(name)
	if tool == nil {
		return fmt.Errorf("unknown tool %q. Run 'pad agent status' to see supported tools", name)
	}

	type installed struct {
		projectPath string
		skillPath   string
		workspace   string
	}
	var successes []installed
	var errs []error
	content := cli.FormatForTool(*tool, pad.PadSkill)
	for _, rawPath := range projectPaths {
		projectPath, err := filepath.Abs(rawPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: resolve project path: %w", rawPath, err))
			continue
		}
		projectPath = filepath.Clean(projectPath)
		path, err := cli.InstallForToolAt(projectPath, *tool, content)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ %s: %v\n", projectPath, err)
			errs = append(errs, err)
			continue
		}
		workspace, _ := cli.DetectWorkspaceFrom(projectPath, "")
		successes = append(successes, installed{projectPath: projectPath, skillPath: path, workspace: workspace})
		fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s → %s\n", projectPath, path)
	}

	if len(successes) > 0 {
		if err := cli.MutateRegistry(func(reg *cli.Registry) error {
			for _, success := range successes {
				reg.Record(success.projectPath, success.workspace, tool.Name, success.skillPath, version)
			}
			return nil
		}); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func agentForgetCmd() *cobra.Command {
	var projectPaths []string
	cmd := &cobra.Command{
		Use:   "forget",
		Short: "Remove projects from the installation registry without deleting skill files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(projectPaths) == 0 {
				return errors.New("at least one --project is required")
			}
			removed := 0
			if err := cli.MutateRegistry(func(reg *cli.Registry) error {
				removed = reg.RemoveProjects(projectPaths)
				return nil
			}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Forgot %d installation record(s); skill files were left unchanged.\n", removed)
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&projectPaths, "project", nil, "project directory to forget (repeatable)")
	return cmd
}

func installAll() error {
	detected := cli.DetectTools()
	if len(detected) == 0 {
		fmt.Println("No AI coding tools detected. Installing for Claude Code by default.")
		detected = []cli.AgentTool{cli.SupportedTools[0]} // Claude
	}

	for _, tool := range detected {
		content := cli.FormatForTool(tool, pad.PadSkill)
		path, err := cli.InstallForTool(tool, content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ %s: %v\n", tool.Label, err)
			continue
		}
		fmt.Printf("  ✓ %s → %s\n", tool.Label, path)
		if err := recordInstallation(tool.Name, path); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: record %s installation: %v\n", tool.Label, err)
		}
	}
	return nil
}

func installInteractive() error {
	detected := cli.DetectTools()

	// Always include Claude if not already detected
	hasClaude := false
	for _, t := range detected {
		if t.Name == "claude" {
			hasClaude = true
			break
		}
	}
	if !hasClaude {
		detected = append([]cli.AgentTool{cli.SupportedTools[0]}, detected...)
	}

	// BUG-2593: gate on canPromptForConfig() (stdin AND stdout are
	// terminals) rather than cli.IsTerminal() (stdin only) — the same
	// swap offerSkillInstall got for BUG-2577 (PR #1111, which see for
	// the boundary): a pty-backed harness can make stdin look like a
	// char device with nobody able to answer, which left the "(Y/n): "
	// prompt printed even though the choice auto-defaults. A caller with
	// BOTH stdin and stdout attached to a pty but nothing driving it
	// still reads as promptable — that case can't be distinguished from
	// a real interactive terminal by any check available here.
	if !canPromptForConfig() {
		// Non-interactive: install for all detected tools
		for _, tool := range detected {
			content := cli.FormatForTool(tool, pad.PadSkill)
			path, err := cli.InstallForTool(tool, content)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ %s: %v\n", tool.Label, err)
				continue
			}
			fmt.Printf("  ✓ %s → %s\n", tool.Label, path)
			if err := recordInstallation(tool.Name, path); err != nil {
				fmt.Fprintf(os.Stderr, "  warning: record %s installation: %v\n", tool.Label, err)
			}
		}
		return nil
	}

	fmt.Println("Detected AI coding tools:")
	fmt.Println()
	for i, tool := range detected {
		installed := ""
		if cli.ToolInstalled(tool) {
			installed = " (already installed)"
		}
		fmt.Printf("  %d. %s%s\n", i+1, tool.Label, installed)
	}
	fmt.Println()
	fmt.Printf("Install /pad skill for all %d? (Y/n): ", len(detected))

	choice := readChoice()
	if choice == "n" || choice == "N" {
		fmt.Println()
		fmt.Println("Install individually with: pad agent install <tool>")
		fmt.Println("Supported tools:", strings.Join(cli.AllToolNames(), ", "))
		return nil
	}

	fmt.Println()
	for _, tool := range detected {
		content := cli.FormatForTool(tool, pad.PadSkill)
		path, err := cli.InstallForTool(tool, content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ %s: %v\n", tool.Label, err)
			continue
		}
		fmt.Printf("  ✓ %s → %s\n", tool.Label, path)
		if err := recordInstallation(tool.Name, path); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: record %s installation: %v\n", tool.Label, err)
		}
	}
	return nil
}

// --- workspaces ---
