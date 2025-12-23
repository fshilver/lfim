package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/lunit-heesungyang/issue-manager/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "im",
	Short: "Local-first markdown-based issue management CLI/TUI",
	Long: `Issue Manager is a local-first, markdown-based issue management tool.

It stores issues as markdown files with YAML frontmatter in your project's
issues/ directory, allowing you to track issues alongside your code.

Features:
  - Create and manage issues with a TUI interface
  - AI-powered issue analysis and planning via Claude
  - Git integration for automatic staging
  - Filter issues by status (Active/All/Closed)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("path")

		model := tui.New(path)
		p := tea.NewProgram(model, tea.WithAltScreen())

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("running TUI: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.Flags().StringP("path", "p", "", "Project root path (default: current directory)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
