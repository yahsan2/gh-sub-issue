package cmd

import (
	"fmt"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/spf13/cobra"
)

var (
	removeRepoFlag  string
	removeForceFlag bool
)

var removeCmd = &cobra.Command{
	Use:   "remove <parent-issue> <sub-issue> [sub-issue...]",
	Short: "Remove sub-issues from a parent issue",
	Long: `Remove the relationship between sub-issues and a parent issue.
This command unlinks sub-issues from their parent without deleting them.

Examples:
  # Remove sub-issue #456 from parent #123
  gh sub-issue remove 123 456

  # Remove multiple sub-issues
  gh sub-issue remove 123 456 457 458

  # Remove using URLs
  gh sub-issue remove https://github.com/owner/repo/issues/123 456

  # Cross-repository removal
  gh sub-issue remove 123 456 --repo owner/repo

  # Skip confirmation prompt
  gh sub-issue remove 123 456 --force`,
	Args: cobra.MinimumNArgs(2),
	RunE: runRemove,
}

func init() {
	rootCmd.AddCommand(removeCmd)
	removeCmd.Flags().StringVarP(&removeRepoFlag, "repo", "R", "", "Repository in OWNER/REPO format")
	removeCmd.Flags().BoolVarP(&removeForceFlag, "force", "f", false, "Skip confirmation prompt")
}

func runRemove(cmd *cobra.Command, args []string) error {
	// Get default repository if not specified
	var defaultOwner, defaultRepo string
	if removeRepoFlag != "" {
		parts := strings.Split(removeRepoFlag, "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid repository format: %s (expected OWNER/REPO)", removeRepoFlag)
		}
		defaultOwner = parts[0]
		defaultRepo = parts[1]
	} else {
		var err error
		defaultOwner, defaultRepo, err = getDefaultRepo()
		if err != nil {
			return fmt.Errorf("no repository specified and could not determine from current directory: %w", err)
		}
	}

	// Parse parent issue reference
	parentArg := args[0]
	parentRef, err := parseIssueReference(parentArg, defaultOwner, defaultRepo)
	if err != nil {
		return fmt.Errorf("invalid parent issue: %w", err)
	}

	// Parse sub-issue references
	var subRefs []*IssueReference
	for _, arg := range args[1:] {
		subRef, err := parseIssueReference(arg, defaultOwner, defaultRepo)
		if err != nil {
			return fmt.Errorf("invalid sub-issue %s: %w", arg, err)
		}
		subRefs = append(subRefs, subRef)
	}

	// Create GitHub API client
	opts := api.ClientOptions{}
	client, err := api.NewGraphQLClient(opts)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Get confirmation if not forced
	if !removeForceFlag {
		var subNumbers []string
		for _, ref := range subRefs {
			subNumbers = append(subNumbers, fmt.Sprintf("#%d", ref.Number))
		}
		
		var prompt string
		if len(subRefs) == 1 {
			prompt = fmt.Sprintf("Are you sure you want to remove %s from parent #%d? (y/N): ", 
				subNumbers[0], parentRef.Number)
		} else {
			prompt = fmt.Sprintf("Are you sure you want to remove %d sub-issues (%s) from parent #%d? (y/N): ", 
				len(subRefs), strings.Join(subNumbers, ", "), parentRef.Number)
		}
		
		fmt.Fprint(cmd.OutOrStderr(), prompt)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Fprintln(cmd.OutOrStderr(), "Removal cancelled")
			return nil
		}
	}

	// Get parent issue node ID
	fmt.Fprintf(cmd.OutOrStderr(), "Getting parent issue #%d from %s/%s...\n", 
		parentRef.Number, parentRef.Owner, parentRef.Repo)
	parentID, err := getIssueNodeID(client, parentRef.Owner, parentRef.Repo, parentRef.Number)
	if err != nil {
		if strings.Contains(err.Error(), "Could not resolve") {
			return fmt.Errorf("parent issue #%d not found in %s/%s", 
				parentRef.Number, parentRef.Owner, parentRef.Repo)
		}
		return fmt.Errorf("failed to get parent issue: %w", err)
	}

	// Remove each sub-issue
	var removedIssues []string
	var errors []error
	
	for _, subRef := range subRefs {
		// Get sub-issue node ID
		fmt.Fprintf(cmd.OutOrStderr(), "Removing sub-issue #%d...\n", subRef.Number)
		subID, err := getIssueNodeID(client, subRef.Owner, subRef.Repo, subRef.Number)
		if err != nil {
			if strings.Contains(err.Error(), "Could not resolve") {
				err = fmt.Errorf("sub-issue #%d not found in %s/%s", 
					subRef.Number, subRef.Owner, subRef.Repo)
			}
			errors = append(errors, err)
			continue
		}

		// Execute GraphQL mutation to remove sub-issue
		err = removeSubIssue(client, parentID, subID)
		if err != nil {
			if strings.Contains(err.Error(), "not a sub-issue") {
				err = fmt.Errorf("warning: #%d is not a sub-issue of #%d", 
					subRef.Number, parentRef.Number)
			}
			errors = append(errors, err)
			continue
		}
		
		removedIssues = append(removedIssues, fmt.Sprintf("#%d", subRef.Number))
	}

	// Display results
	if len(removedIssues) > 0 {
		if len(removedIssues) == 1 {
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Removed sub-issue %s from parent #%d\n", 
				removedIssues[0], parentRef.Number)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Removed %d sub-issues from parent #%d:\n", 
				len(removedIssues), parentRef.Number)
			for _, issue := range removedIssues {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", issue)
			}
		}
	}

	// Display errors if any
	if len(errors) > 0 {
		fmt.Fprintln(cmd.OutOrStderr(), "\nErrors encountered:")
		for _, err := range errors {
			fmt.Fprintf(cmd.OutOrStderr(), "  - %v\n", err)
		}
		if len(removedIssues) == 0 {
			return fmt.Errorf("failed to remove any sub-issues")
		}
	}

	return nil
}

func removeSubIssue(client *api.GraphQLClient, parentID, subIssueID string) error {
	// GraphQL mutation to remove sub-issue relationship
	mutation := `
		mutation RemoveSubIssue($parentId: ID!, $subIssueId: ID!) {
			removeSubIssue(input: {
				issueId: $parentId,
				subIssueId: $subIssueId
			}) {
				issue {
					number
					title
				}
				subIssue {
					number
					title
				}
			}
		}`

	variables := map[string]interface{}{
		"parentId":   parentID,
		"subIssueId": subIssueID,
	}

	var result struct {
		RemoveSubIssue struct {
			Issue struct {
				Number int
				Title  string
			}
			SubIssue struct {
				Number int
				Title  string
			}
		}
	}

	err := client.Do(mutation, variables, &result)
	if err != nil {
		// Handle authentication errors
		if strings.Contains(err.Error(), "authentication") || strings.Contains(err.Error(), "401") {
			return fmt.Errorf("authentication required. Run 'gh auth login' first")
		}
		// Handle permission errors
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "403") {
			return fmt.Errorf("insufficient permissions to modify issues")
		}
		return err
	}

	return nil
}