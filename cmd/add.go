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
var replaceParentFlag bool

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
  gh sub-issues add 123 456 --repo owner/repo

  # Move an issue that already has a parent under a new parent
  gh sub-issue add 123 456 --replace-parent`,
	Args: cobra.ExactArgs(2),
	RunE: runAdd,
}

func init() {
	// Add command to root
	rootCmd.AddCommand(addCmd)
	
	// Add flags
	addCmd.Flags().StringVarP(&repoFlag, "repo", "R", "", "Repository in OWNER/REPO format")
	addCmd.Flags().BoolVar(&replaceParentFlag, "replace-parent", false, "Move the issue from its current parent, if it has one")
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

// issueParent is an issue's parent as GitHub reports it
type issueParent struct {
	ID  string
	Ref IssueReference
}

// getIssueNodeIDAndParent gets the issue's GraphQL node ID and its current
// parent in one query. The parent is nil when the issue has none.
func getIssueNodeIDAndParent(client *api.GraphQLClient, owner, repo string, number int) (string, *issueParent, error) {
	query := `
		query($owner: String!, $repo: String!, $number: Int!) {
			repository(owner: $owner, name: $repo) {
				issue(number: $number) {
					id
					parent {
						id
						number
						repository {
							name
							owner {
								login
							}
						}
					}
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
				ID     string `json:"id"`
				Parent *struct {
					ID         string `json:"id"`
					Number     int    `json:"number"`
					Repository struct {
						Name  string `json:"name"`
						Owner struct {
							Login string `json:"login"`
						} `json:"owner"`
					} `json:"repository"`
				} `json:"parent"`
			} `json:"issue"`
		} `json:"repository"`
	}

	if err := client.Do(query, variables, &response); err != nil {
		return "", nil, fmt.Errorf("failed to get issue #%d: %w", number, err)
	}

	issue := response.Repository.Issue
	if issue.ID == "" {
		return "", nil, fmt.Errorf("issue #%d not found in %s/%s", number, owner, repo)
	}
	if issue.Parent == nil {
		return issue.ID, nil, nil
	}

	return issue.ID, &issueParent{
		ID: issue.Parent.ID,
		Ref: IssueReference{
			Owner:  issue.Parent.Repository.Owner.Login,
			Repo:   issue.Parent.Repository.Name,
			Number: issue.Parent.Number,
		},
	}, nil
}

// sameRepo reports whether two references are in the same repository; GitHub
// owner and repository names are case-insensitive
func sameRepo(a, b IssueReference) bool {
	return strings.EqualFold(a.Owner, b.Owner) && strings.EqualFold(a.Repo, b.Repo)
}

// formatIssueRef writes ref as #N when it is in the same repository as
// base, and as owner/repo#N when it is not
func formatIssueRef(ref, base IssueReference) string {
	if sameRepo(ref, base) {
		return fmt.Sprintf("#%d", ref.Number)
	}
	return fmt.Sprintf("%s/%s#%d", ref.Owner, ref.Repo, ref.Number)
}

// checkCurrentParent decides, from the sub-issue's current parent, what the
// add does. done is true when the issue is already under the requested
// parent, whose node ID is requestedID, so there is nothing to change. An
// error means the issue is under another parent and replaceParent is not
// set, so nothing should be changed. References are written relative to
// base, the repository the command runs against.
func checkCurrentParent(sub, requested IssueReference, requestedID string, current *issueParent, replaceParent bool, base IssueReference) (done bool, err error) {
	if current == nil {
		return false, nil
	}
	// Node IDs, not names: renaming or transferring a repository changes its name
	if current.ID == requestedID {
		return true, nil
	}
	if replaceParent {
		return false, nil
	}
	return false, fmt.Errorf("issue %s is already a sub-issue of %s; use --replace-parent to move it under %s",
		formatIssueRef(sub, base), formatIssueRef(current.Ref, base), formatIssueRef(requested, base))
}

// alreadyMessage reports an issue that is already under the requested
// parent, with references written relative to base
func alreadyMessage(sub, parent, base IssueReference) string {
	return fmt.Sprintf("✓ Issue %s is already a sub-issue of %s",
		formatIssueRef(sub, base), formatIssueRef(parent, base))
}

// addedMessage describes a successful add, with references written relative
// to base. previous is the parent the issue was moved from, or nil when it
// had none.
func addedMessage(sub, parent IssueReference, previous *IssueReference, base IssueReference) string {
	if previous == nil {
		return fmt.Sprintf("✓ Added issue %s as a sub-issue of %s",
			formatIssueRef(sub, base), formatIssueRef(parent, base))
	}
	return fmt.Sprintf("✓ Moved issue %s from %s to %s",
		formatIssueRef(sub, base), formatIssueRef(*previous, base), formatIssueRef(parent, base))
}

// addSubIssue links a sub-issue to a parent issue. With replaceParent, an
// issue that already has a parent is moved from it.
func addSubIssue(client *api.GraphQLClient, parentID, subIssueID string, replaceParent bool) error {
	mutation := `
		mutation($parentId: ID!, $subIssueId: ID!, $replaceParent: Boolean) {
			addSubIssue(input: {
				issueId: $parentId,
				subIssueId: $subIssueId,
				replaceParent: $replaceParent
			}) {
				subIssue {
					id
				}
			}
		}`
	
	variables := map[string]interface{}{
		"parentId":      parentID,
		"subIssueId":    subIssueID,
		"replaceParent": replaceParent,
	}
	
	// Only success matters; nothing in the response is read
	var response struct{}
	
	err := client.Do(mutation, variables, &response)
	if err != nil {
		return fmt.Errorf("failed to add sub-issue: %w", err)
	}

	return nil
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
	
	subID, currentParent, err := getIssueNodeIDAndParent(client, subRef.Owner, subRef.Repo, subRef.Number)
	if err != nil {
		// Check if it's a permission error
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "403") {
			return fmt.Errorf("insufficient permissions to access %s/%s", 
				subRef.Owner, subRef.Repo)
		}
		return err
	}
	
	// The check above compares names as typed, so it misses references that
	// differ in case or use a renamed repository's old name; node IDs don't
	if parentID == subID {
		return fmt.Errorf("cannot add issue as its own sub-issue")
	}

	// Decide from the sub-issue's current parent before changing anything
	base := IssueReference{Owner: defaultOwner, Repo: defaultRepo}
	done, err := checkCurrentParent(*subRef, *parentRef, parentID, currentParent, replaceParentFlag, base)
	if err != nil {
		return err
	}
	if done {
		fmt.Fprintln(cmd.OutOrStdout(), alreadyMessage(*subRef, *parentRef, base))
		return nil
	}

	// Link the issues
	fmt.Fprintf(cmd.OutOrStderr(), "Linking issues...\n")
	if err := addSubIssue(client, parentID, subID, replaceParentFlag); err != nil {
		// Check for specific error cases
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "403") {
			return fmt.Errorf("insufficient permissions to modify issues in this repository")
		}
		return err
	}

	// Success message; after the check, currentParent is set only if it was replaced
	var previous *IssueReference
	if currentParent != nil {
		previous = &currentParent.Ref
	}
	fmt.Fprintln(cmd.OutOrStdout(), addedMessage(*subRef, *parentRef, previous, base))
	
	_ = ctx // Use context if needed in future
	return nil
}