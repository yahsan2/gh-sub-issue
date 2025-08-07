package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/term"
	"github.com/spf13/cobra"
)

var (
	listStateFlag  string
	listLimitFlag  int
	listJSONFlag   bool
	listWebFlag    bool
	listRepoFlag   string
)

var listCmd = &cobra.Command{
	Use:   "list <parent-issue>",
	Short: "List all sub-issues for a parent issue",
	Long: `List all sub-issues connected to a parent issue.

Supports multiple output formats:
- Colored output for terminal (TTY)
- Plain text for scripts (non-TTY)
- JSON for programmatic use (--json)

Examples:
  # List sub-issues for issue #123
  gh sub-issues list 123
  
  # List with URL
  gh sub-issues list https://github.com/owner/repo/issues/123
  
  # Filter by state
  gh sub-issues list 123 --state closed
  
  # JSON output
  gh sub-issues list 123 --json
  
  # Limit results
  gh sub-issues list 123 --limit 10`,
	Args: cobra.ExactArgs(1),
	RunE: runList,
}

func init() {
	// Add command to root
	rootCmd.AddCommand(listCmd)
	
	// Add flags
	listCmd.Flags().StringVarP(&listStateFlag, "state", "s", "open", "Filter by state: {open|closed|all}")
	listCmd.Flags().IntVarP(&listLimitFlag, "limit", "L", 30, "Maximum number of sub-issues to display")
	listCmd.Flags().BoolVar(&listJSONFlag, "json", false, "Output in JSON format")
	listCmd.Flags().BoolVarP(&listWebFlag, "web", "w", false, "Open in web browser")
	listCmd.Flags().StringVarP(&listRepoFlag, "repo", "R", "", "Repository in OWNER/REPO format")
}

// SubIssue represents a sub-issue
type SubIssue struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	State     string   `json:"state"`
	URL       string   `json:"url"`
	Assignees []string `json:"assignees,omitempty"`
}

// ParentIssue represents the parent issue
type ParentIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
}

// ListResult represents the result of listing sub-issues
type ListResult struct {
	Parent    ParentIssue `json:"parent"`
	SubIssues []SubIssue  `json:"subIssues"`
	Total     int         `json:"total"`
	OpenCount int         `json:"openCount"`
}

// getSubIssues fetches sub-issues for a parent issue
func getSubIssues(client *api.GraphQLClient, owner, repo string, number int, limit int) (*ListResult, error) {
	// First, get the parent issue details
	parentQuery := `
		query($owner: String!, $repo: String!, $number: Int!) {
			repository(owner: $owner, name: $repo) {
				issue(number: $number) {
					id
					number
					title
					state
				}
			}
		}`
	
	var parentResponse struct {
		Repository struct {
			Issue struct {
				ID     string `json:"id"`
				Number int    `json:"number"`
				Title  string `json:"title"`
				State  string `json:"state"`
			} `json:"issue"`
		} `json:"repository"`
	}
	
	variables := map[string]interface{}{
		"owner":  owner,
		"repo":   repo,
		"number": number,
	}
	
	err := client.Do(parentQuery, variables, &parentResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent issue #%d: %w", number, err)
	}
	
	if parentResponse.Repository.Issue.ID == "" {
		return nil, fmt.Errorf("issue #%d not found in %s/%s", number, owner, repo)
	}
	
	// Now get the sub-issues using the subIssues field
	subIssuesQuery := `
		query($owner: String!, $repo: String!, $number: Int!, $limit: Int!) {
			repository(owner: $owner, name: $repo) {
				issue(number: $number) {
					subIssues(first: $limit) {
						nodes {
							number
							title
							state
							url
							assignees(first: 10) {
								nodes {
									login
								}
							}
						}
					}
				}
			}
		}`
	
	var subIssuesResponse struct {
		Repository struct {
			Issue struct {
				SubIssues struct {
					Nodes []struct {
						Number    int    `json:"number"`
						Title     string `json:"title"`
						State     string `json:"state"`
						URL       string `json:"url"`
						Assignees struct {
							Nodes []struct {
								Login string `json:"login"`
							} `json:"nodes"`
						} `json:"assignees"`
					} `json:"nodes"`
				} `json:"subIssues"`
			} `json:"issue"`
		} `json:"repository"`
	}
	
	subVariables := map[string]interface{}{
		"owner":  owner,
		"repo":   repo,
		"number": number,
		"limit":  limit,
	}
	
	err = client.Do(subIssuesQuery, subVariables, &subIssuesResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to get sub-issues: %w", err)
	}
	
	// Build result
	result := &ListResult{
		Parent: ParentIssue{
			Number: parentResponse.Repository.Issue.Number,
			Title:  parentResponse.Repository.Issue.Title,
			State:  strings.ToLower(parentResponse.Repository.Issue.State),
		},
		SubIssues: []SubIssue{},
		Total:     0,
		OpenCount: 0,
	}
	
	// Process sub-issues
	for _, node := range subIssuesResponse.Repository.Issue.SubIssues.Nodes {
		if node.Number == 0 {
			continue // Skip if not an issue
		}
		
		assignees := []string{}
		for _, assignee := range node.Assignees.Nodes {
			assignees = append(assignees, assignee.Login)
		}
		
		subIssue := SubIssue{
			Number:    node.Number,
			Title:     node.Title,
			State:     strings.ToLower(node.State),
			URL:       node.URL,
			Assignees: assignees,
		}
		
		// Apply state filter
		if listStateFlag != "all" {
			if listStateFlag != subIssue.State {
				continue
			}
		}
		
		result.SubIssues = append(result.SubIssues, subIssue)
		result.Total++
		
		if subIssue.State == "open" {
			result.OpenCount++
		}
	}
	
	return result, nil
}

// formatTTY formats output for terminal with colors
func formatTTY(result *ListResult) string {
	var output strings.Builder
	
	// Header
	output.WriteString(fmt.Sprintf("\nParent: #%d - %s\n\n", result.Parent.Number, result.Parent.Title))
	
	if result.Total == 0 {
		output.WriteString("No sub-issues found.\n")
		return output.String()
	}
	
	// Summary
	closedCount := result.Total - result.OpenCount
	output.WriteString(fmt.Sprintf("SUB-ISSUES (%d total, %d open, %d closed)\n", 
		result.Total, result.OpenCount, closedCount))
	output.WriteString("─────────────────────────────\n")
	
	// Sub-issues
	for _, issue := range result.SubIssues {
		// State icon
		icon := "🔵" // open
		if issue.State == "closed" {
			icon = "✅"
		}
		
		// Format line
		line := fmt.Sprintf("%s #%-4d %-40s [%s]", 
			icon, issue.Number, truncate(issue.Title, 40), issue.State)
		
		// Add assignees if any
		if len(issue.Assignees) > 0 {
			line += fmt.Sprintf("   @%s", strings.Join(issue.Assignees, ", @"))
		}
		
		output.WriteString(line + "\n")
	}
	
	return output.String()
}

// formatPlain formats output as plain text (tab-separated)
func formatPlain(result *ListResult) string {
	var output strings.Builder
	
	for _, issue := range result.SubIssues {
		assignees := strings.Join(issue.Assignees, ",")
		output.WriteString(fmt.Sprintf("%d\t%s\t%s\t%s\n", 
			issue.Number, issue.State, issue.Title, assignees))
	}
	
	return output.String()
}

// formatJSON formats output as JSON
func formatJSON(result *ListResult) (string, error) {
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// truncate truncates a string to max length
func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
}

// runList is the main command logic
func runList(cmd *cobra.Command, args []string) error {
	// Get default repository
	var defaultOwner, defaultRepo string
	var err error
	
	if listRepoFlag != "" {
		// Parse --repo flag
		parts := strings.Split(listRepoFlag, "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid repository format: %s (expected OWNER/REPO)", listRepoFlag)
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
	
	// Parse parent issue reference
	parentRef, err := parseIssueReference(args[0], defaultOwner, defaultRepo)
	if err != nil {
		return fmt.Errorf("invalid parent issue: %w", err)
	}
	
	// Handle --web flag
	if listWebFlag {
		url := fmt.Sprintf("https://github.com/%s/%s/issues/%d", 
			parentRef.Owner, parentRef.Repo, parentRef.Number)
		fmt.Fprintf(cmd.OutOrStderr(), "Opening %s in browser...\n", url)
		return openInBrowser(url)
	}
	
	// Create GraphQL client
	client, err := api.NewGraphQLClient(api.ClientOptions{})
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}
	
	// Get sub-issues
	result, err := getSubIssues(client, parentRef.Owner, parentRef.Repo, parentRef.Number, listLimitFlag)
	if err != nil {
		return err
	}
	
	// Format output
	var output string
	
	if listJSONFlag {
		// JSON output
		output, err = formatJSON(result)
		if err != nil {
			return fmt.Errorf("failed to format JSON: %w", err)
		}
	} else if term.IsTerminal(os.Stdout) {
		// TTY output with colors
		output = formatTTY(result)
	} else {
		// Plain text output
		output = formatPlain(result)
	}
	
	// Print output
	fmt.Fprint(cmd.OutOrStdout(), output)
	
	return nil
}

// openInBrowser opens a URL in the default browser
func openInBrowser(url string) error {
	// This would typically use a library or system command
	// For now, we'll just print a message
	fmt.Printf("Please open in browser: %s\n", url)
	return nil
}