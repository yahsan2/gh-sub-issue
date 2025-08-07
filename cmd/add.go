package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/spf13/cobra"
)

var repoFlag string

var addCmd = &cobra.Command{
	Use:   "add <parent-issue> <sub-issue>",
	Short: "Add an existing issue as a sub-issue to a parent issue",
	Long: `Link an existing issue to a parent issue using GitHub's issue hierarchy feature.

Examples:
  # Link issues by numbers
  gh sub-issues add 123 456
  
  # Link using parent URL
  gh sub-issues add https://github.com/owner/repo/issues/123 456
  
  # Cross-repository linking
  gh sub-issues add 123 456 --repo owner/repo`,
	Args: cobra.ExactArgs(2),
	RunE: runAdd,
}

func init() {
	// Add command to root
	rootCmd.AddCommand(addCmd)
	
	// Add flags
	addCmd.Flags().StringVarP(&repoFlag, "repo", "R", "", "Repository in OWNER/REPO format")
}

// IssueReference represents a parsed issue reference
type IssueReference struct {
	Owner  string
	Repo   string
	Number int
}

// parseIssueReference parses issue number or URL
func parseIssueReference(ref string, defaultOwner, defaultRepo string) (*IssueReference, error) {
	// Check if it's a URL
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return parseIssueURL(ref)
	}
	
	// Otherwise, treat as issue number
	number, err := strconv.Atoi(ref)
	if err != nil {
		return nil, fmt.Errorf("invalid issue reference: %s", ref)
	}
	
	if number <= 0 {
		return nil, fmt.Errorf("invalid issue number: %d", number)
	}
	
	return &IssueReference{
		Owner:  defaultOwner,
		Repo:   defaultRepo,
		Number: number,
	}, nil
}

// parseIssueURL extracts owner, repo, and issue number from GitHub URL
func parseIssueURL(url string) (*IssueReference, error) {
	// Expected format: https://github.com/owner/repo/issues/123
	// Remove trailing slash if present
	url = strings.TrimSuffix(url, "/")
	
	parts := strings.Split(url, "/")
	if len(parts) < 7 {
		return nil, fmt.Errorf("invalid GitHub issue URL format: %s", url)
	}
	
	// Verify it's a GitHub URL
	if !strings.Contains(parts[2], "github.com") {
		return nil, fmt.Errorf("not a GitHub URL: %s", url)
	}
	
	// Verify it's an issues URL
	if parts[5] != "issues" {
		return nil, fmt.Errorf("not an issue URL (expected /issues/): %s", url)
	}
	
	number, err := strconv.Atoi(parts[6])
	if err != nil {
		return nil, fmt.Errorf("invalid issue number in URL: %s", parts[6])
	}
	
	if number <= 0 {
		return nil, fmt.Errorf("invalid issue number: %d", number)
	}
	
	return &IssueReference{
		Owner:  parts[3],
		Repo:   parts[4],
		Number: number,
	}, nil
}

// getIssueNodeID gets the GraphQL node ID for an issue
func getIssueNodeID(client *api.GraphQLClient, owner, repo string, number int) (string, error) {
	query := `
		query($owner: String!, $repo: String!, $number: Int!) {
			repository(owner: $owner, name: $repo) {
				issue(number: $number) {
					id
				}
			}
		}`
	
	variables := map[string]interface{}{
		"owner":  owner,
		"repo":   repo,
		"number": number,
	}
	
	var response struct {
		Repository struct {
			Issue struct {
				ID string `json:"id"`
			} `json:"issue"`
		} `json:"repository"`
	}
	
	err := client.Do(query, variables, &response)
	if err != nil {
		return "", fmt.Errorf("failed to get issue #%d: %w", number, err)
	}
	
	if response.Repository.Issue.ID == "" {
		return "", fmt.Errorf("issue #%d not found in %s/%s", number, owner, repo)
	}
	
	return response.Repository.Issue.ID, nil
}

// addSubIssue links a sub-issue to a parent issue
func addSubIssue(client *api.GraphQLClient, parentID, subIssueID string) (int, int, error) {
	mutation := `
		mutation($parentId: ID!, $subIssueId: ID!) {
			addSubIssue(input: {
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
	
	var response struct {
		AddSubIssue struct {
			Issue struct {
				Number int    `json:"number"`
				Title  string `json:"title"`
			} `json:"issue"`
			SubIssue struct {
				Number int    `json:"number"`
				Title  string `json:"title"`
			} `json:"subIssue"`
		} `json:"addSubIssue"`
	}
	
	err := client.Do(mutation, variables, &response)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to add sub-issue: %w", err)
	}
	
	return response.AddSubIssue.Issue.Number, response.AddSubIssue.SubIssue.Number, nil
}

// getDefaultRepo gets the repository from current directory
func getDefaultRepo() (string, string, error) {
	// Try to get from git remote using gh CLI
	cmd := exec.Command("gh", "repo", "view", "--json", "owner,name")
	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("could not determine repository from current directory")
	}
	
	var repo struct {
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
		Name string `json:"name"`
	}
	
	if err := json.Unmarshal(output, &repo); err != nil {
		return "", "", fmt.Errorf("failed to parse repository info: %w", err)
	}
	
	if repo.Owner.Login == "" || repo.Name == "" {
		return "", "", fmt.Errorf("could not determine repository owner or name")
	}
	
	return repo.Owner.Login, repo.Name, nil
}

// runAdd is the main command logic
func runAdd(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	
	// Get default repository from current directory or --repo flag
	var defaultOwner, defaultRepo string
	var err error
	
	if repoFlag != "" {
		// Parse --repo flag
		parts := strings.Split(repoFlag, "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid repository format: %s (expected OWNER/REPO)", repoFlag)
		}
		defaultOwner = parts[0]
		defaultRepo = parts[1]
	} else {
		// Try to get from current directory
		defaultOwner, defaultRepo, err = getDefaultRepo()
		if err != nil {
			return fmt.Errorf("could not determine repository (use --repo flag): %w", err)
		}
	}
	
	// Parse parent and sub-issue references
	parentRef, err := parseIssueReference(args[0], defaultOwner, defaultRepo)
	if err != nil {
		return fmt.Errorf("invalid parent issue: %w", err)
	}
	
	subRef, err := parseIssueReference(args[1], defaultOwner, defaultRepo)
	if err != nil {
		return fmt.Errorf("invalid sub-issue: %w", err)
	}
	
	// Check for circular dependency
	if parentRef.Owner == subRef.Owner && 
	   parentRef.Repo == subRef.Repo && 
	   parentRef.Number == subRef.Number {
		return fmt.Errorf("cannot add issue as its own sub-issue")
	}
	
	// Create GraphQL client using default options
	client, err := api.NewGraphQLClient(api.ClientOptions{})
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}
	
	// Get node IDs for both issues
	fmt.Fprintf(cmd.OutOrStderr(), "Getting parent issue #%d from %s/%s...\n", 
		parentRef.Number, parentRef.Owner, parentRef.Repo)
	
	parentID, err := getIssueNodeID(client, parentRef.Owner, parentRef.Repo, parentRef.Number)
	if err != nil {
		// Check if it's an authentication error
		if strings.Contains(err.Error(), "authentication") || strings.Contains(err.Error(), "401") {
			return fmt.Errorf("authentication required. Run 'gh auth login' first")
		}
		// Check if it's a permission error
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "403") {
			return fmt.Errorf("insufficient permissions to access %s/%s", 
				parentRef.Owner, parentRef.Repo)
		}
		return err
	}
	
	fmt.Fprintf(cmd.OutOrStderr(), "Getting sub-issue #%d from %s/%s...\n", 
		subRef.Number, subRef.Owner, subRef.Repo)
	
	subID, err := getIssueNodeID(client, subRef.Owner, subRef.Repo, subRef.Number)
	if err != nil {
		// Check if it's a permission error
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "403") {
			return fmt.Errorf("insufficient permissions to access %s/%s", 
				subRef.Owner, subRef.Repo)
		}
		return err
	}
	
	// Link the issues
	fmt.Fprintf(cmd.OutOrStderr(), "Linking issues...\n")
	parentNum, subNum, err := addSubIssue(client, parentID, subID)
	if err != nil {
		// Check for specific error cases
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "403") {
			return fmt.Errorf("insufficient permissions to modify issues in this repository")
		}
		if strings.Contains(err.Error(), "already") {
			return fmt.Errorf("issue #%d is already a sub-issue of #%d", 
				subRef.Number, parentRef.Number)
		}
		return err
	}
	
	// Success message
	fmt.Fprintf(cmd.OutOrStdout(), "✓ Added issue #%d as a sub-issue of #%d\n", subNum, parentNum)
	
	_ = ctx // Use context if needed in future
	return nil
}