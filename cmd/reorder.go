package cmd

import (
	"fmt"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/spf13/cobra"
)

var (
	reorderRepoFlag string
)

var reorderCmd = &cobra.Command{
	Use:   "reorder <parent-issue> <sub-issue> <position>",
	Short: "Reorder a sub-issue within its parent",
	Long: `Reorder a sub-issue to a different position within its parent issue.

Position can be specified in multiple ways:
  - A number (1, 2, 3, etc.) to move to that position (1 = first)
  - "first" to move to the beginning
  - "last" to move to the end
  - "after:N" to move after issue #N
  - "before:N" to move before issue #N

Examples:
  # Move sub-issue #456 to first position in parent #123
  gh sub-issue reorder 123 456 first

  # Move sub-issue #456 to position 3 in parent #123
  gh sub-issue reorder 123 456 3

  # Move sub-issue #456 after sub-issue #789
  gh sub-issue reorder 123 456 after:789

  # Move sub-issue #456 before sub-issue #789
  gh sub-issue reorder 123 456 before:789

  # Move sub-issue #456 to last position
  gh sub-issue reorder 123 456 last

  # Using URLs
  gh sub-issue reorder https://github.com/owner/repo/issues/123 456 first

  # Cross-repository
  gh sub-issue reorder 123 456 first --repo owner/repo`,
	Args: cobra.ExactArgs(3),
	RunE: runReorder,
}

func init() {
	rootCmd.AddCommand(reorderCmd)
	reorderCmd.Flags().StringVarP(&reorderRepoFlag, "repo", "R", "", "Repository in OWNER/REPO format")
}

// reprioritizeSubIssue moves a sub-issue to a new position
func reprioritizeSubIssue(client *api.GraphQLClient, parentID, subIssueID, afterID string) error {
	mutation := `
		mutation ReprioritizeSubIssue($parentId: ID!, $subIssueId: ID!, $afterId: ID) {
			reprioritizeSubIssue(input: {
				issueId: $parentId,
				subIssueId: $subIssueId,
				afterId: $afterId
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

	// Only include afterId if it's not empty
	if afterID != "" {
		variables["afterId"] = afterID
	}

	var response struct {
		ReprioritizeSubIssue struct {
			Issue struct {
				Number int    `json:"number"`
				Title  string `json:"title"`
			} `json:"issue"`
			SubIssue struct {
				Number int    `json:"number"`
				Title  string `json:"title"`
			} `json:"subIssue"`
		} `json:"reprioritizeSubIssue"`
	}

	err := client.Do(mutation, variables, &response)
	if err != nil {
		// Handle authentication errors
		if strings.Contains(err.Error(), "authentication") || strings.Contains(err.Error(), "401") {
			return fmt.Errorf("authentication required. Run 'gh auth login' first")
		}
		// Handle permission errors
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "403") {
			return fmt.Errorf("insufficient permissions to modify issues")
		}
		// Handle not a sub-issue error
		if strings.Contains(err.Error(), "not a sub-issue") {
			return fmt.Errorf("the specified issue is not a sub-issue of the parent")
		}
		return err
	}

	return nil
}

func runReorder(cmd *cobra.Command, args []string) error {
	// Get default repository
	var defaultOwner, defaultRepo string
	if reorderRepoFlag != "" {
		parts := strings.Split(reorderRepoFlag, "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid repository format: %s (expected OWNER/REPO)", reorderRepoFlag)
		}
		defaultOwner = parts[0]
		defaultRepo = parts[1]
	} else {
		var err error
		defaultOwner, defaultRepo, err = getDefaultRepo()
		if err != nil {
			return fmt.Errorf("could not determine repository (use --repo flag): %w", err)
		}
	}

	// Parse parent issue reference
	parentRef, err := parseIssueReference(args[0], defaultOwner, defaultRepo)
	if err != nil {
		return fmt.Errorf("invalid parent issue: %w", err)
	}

	// Parse sub-issue reference
	subRef, err := parseIssueReference(args[1], defaultOwner, defaultRepo)
	if err != nil {
		return fmt.Errorf("invalid sub-issue: %w", err)
	}

	// Parse position argument
	position := args[2]

	// Create GraphQL client
	client, err := api.NewGraphQLClient(api.ClientOptions{
		Headers: map[string]string{
			"GraphQL-Features": "sub_issues",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}

	// Get parent issue ID
	fmt.Fprintf(cmd.OutOrStderr(), "Getting parent issue #%d from %s/%s...\n",
		parentRef.Number, parentRef.Owner, parentRef.Repo)

	parentID, err := getIssueNodeID(client, parentRef.Owner, parentRef.Repo, parentRef.Number)
	if err != nil {
		if strings.Contains(err.Error(), "authentication") || strings.Contains(err.Error(), "401") {
			return fmt.Errorf("authentication required. Run 'gh auth login' first")
		}
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "403") {
			return fmt.Errorf("insufficient permissions to access %s/%s",
				parentRef.Owner, parentRef.Repo)
		}
		return err
	}

	// Get sub-issue ID
	fmt.Fprintf(cmd.OutOrStderr(), "Getting sub-issue #%d...\n", subRef.Number)

	subID, err := getIssueNodeID(client, subRef.Owner, subRef.Repo, subRef.Number)
	if err != nil {
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "403") {
			return fmt.Errorf("insufficient permissions to access %s/%s",
				subRef.Owner, subRef.Repo)
		}
		return err
	}

	// Get all sub-issues to determine the correct afterId based on position
	result, err := getSubIssues(client, parentRef.Owner, parentRef.Repo, parentRef.Number, 100)
	if err != nil {
		return fmt.Errorf("failed to get sub-issues: %w", err)
	}

	// Calculate the afterId based on the position argument
	afterID, err := calculateAfterID(client, position, subRef.Number, result, defaultOwner, defaultRepo)
	if err != nil {
		return err
	}

	// Reorder the sub-issue
	fmt.Fprintf(cmd.OutOrStderr(), "Reordering sub-issue #%d...\n", subRef.Number)
	err = reprioritizeSubIssue(client, parentID, subID, afterID)
	if err != nil {
		return err
	}

	// Success message
	fmt.Fprintf(cmd.OutOrStdout(), "✓ Reordered sub-issue #%d in parent #%d\n", subRef.Number, parentRef.Number)

	return nil
}

// calculateAfterID determines the afterId based on the position string
func calculateAfterID(client *api.GraphQLClient, position string, movingIssueNum int, result *ListResult, defaultOwner, defaultRepo string) (string, error) {
	position = strings.ToLower(strings.TrimSpace(position))

	// Filter out the moving issue from the list
	var currentSubIssues []SubIssue
	for _, issue := range result.SubIssues {
		if issue.Number != movingIssueNum {
			currentSubIssues = append(currentSubIssues, issue)
		}
	}

	// Handle special positions
	if position == "first" {
		// To move to first, don't specify afterId (or it could be null)
		return "", nil
	}

	if position == "last" {
		// To move to last, use the ID of the current last issue
		if len(currentSubIssues) == 0 {
			return "", nil
		}
		lastIssue := currentSubIssues[len(currentSubIssues)-1]
		return getIssueNodeIDByNumber(client, defaultOwner, defaultRepo, lastIssue.Number)
	}

	// Handle "after:N" format
	if strings.HasPrefix(position, "after:") {
		afterNumStr := strings.TrimPrefix(position, "after:")
		var afterNum int
		_, err := fmt.Sscanf(afterNumStr, "%d", &afterNum)
		if err != nil {
			return "", fmt.Errorf("invalid issue number in 'after:' position: %s", afterNumStr)
		}
		return getIssueNodeIDByNumber(client, defaultOwner, defaultRepo, afterNum)
	}

	// Handle "before:N" format
	if strings.HasPrefix(position, "before:") {
		beforeNumStr := strings.TrimPrefix(position, "before:")
		var beforeNum int
		_, err := fmt.Sscanf(beforeNumStr, "%d", &beforeNum)
		if err != nil {
			return "", fmt.Errorf("invalid issue number in 'before:' position: %s", beforeNumStr)
		}

		// Find the issue before the target
		for i, issue := range currentSubIssues {
			if issue.Number == beforeNum {
				if i == 0 {
					// Moving before the first item means moving to first
					return "", nil
				}
				// Get the issue that comes before the target
				return getIssueNodeIDByNumber(client, defaultOwner, defaultRepo, currentSubIssues[i-1].Number)
			}
		}
		return "", fmt.Errorf("issue #%d not found in sub-issues list", beforeNum)
	}

	// Handle numeric position (1-based)
	var pos int
	_, err := fmt.Sscanf(position, "%d", &pos)
	if err != nil {
		return "", fmt.Errorf("invalid position: %s (must be a number, 'first', 'last', 'after:N', or 'before:N')", position)
	}

	if pos < 1 {
		return "", fmt.Errorf("position must be at least 1")
	}

	if pos == 1 {
		// Moving to position 1 means first
		return "", nil
	}

	// Position is 1-based, so position 2 means after item at index 0
	targetIndex := pos - 2
	if targetIndex >= len(currentSubIssues) {
		// Position is beyond the end, move to last
		if len(currentSubIssues) == 0 {
			return "", nil
		}
		lastIssue := currentSubIssues[len(currentSubIssues)-1]
		return getIssueNodeIDByNumber(client, defaultOwner, defaultRepo, lastIssue.Number)
	}

	targetIssue := currentSubIssues[targetIndex]
	return getIssueNodeIDByNumber(client, defaultOwner, defaultRepo, targetIssue.Number)
}

// getIssueNodeIDByNumber is a helper to get node ID for an issue number
func getIssueNodeIDByNumber(client *api.GraphQLClient, owner, repo string, number int) (string, error) {
	return getIssueNodeID(client, owner, repo, number)
}
