package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "gh-sub-issue",
	Short: "GitHub CLI extension for managing sub-issues",
	Long: `A GitHub CLI extension that adds sub-issue management capabilities to GitHub issues.
	
This extension allows you to:
- Link existing issues as sub-issues to parent issues
- Create new sub-issues directly linked to parent issues
- List all sub-issues for a given parent issue`,
	Version: Version,
}

func Execute() int {
	// Add subcommands here (will be added in next tasks)
	
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}