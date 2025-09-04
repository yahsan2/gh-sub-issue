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
	listStateFlag    string
	listLimitFlag    int
	listJSONFlag     string
	listWebFlag      bool
	listRepoFlag     string
	listRelationFlag string
)

var listCmd = &cobra.Command{
	Use:   "list <issue>",
	Short: "List issues related to the specified issue",
	Long: `List issues related to the specified issue based on relationship type.

Supports multiple output formats:
- Colored output for terminal (TTY)
- Plain text for scripts (non-TTY)
- JSON for programmatic use (--json)

Examples:
  # List child sub-issues for issue #123 (default)
  gh sub-issues list 123
  
  # List parent issue for sub-issue #456
  gh sub-issues list 456 --relation parent
  
  # List sibling issues for sub-issue #789
  gh sub-issues list 789 --relation siblings
  
  # List with URL
  gh sub-issues list https://github.com/owner/repo/issues/123
  
  # Filter by state
  gh sub-issues list 123 --state closed
  
  # JSON output with selected fields
  gh sub-issues list 123 --json number,title,state
  
  # JSON output with parent and meta info
  gh sub-issues list 123 --json parent.number,parent.title,total,openCount
  
  # Limit results
  gh sub-issues list 123 --limit 10`,
	Args: cobra.ExactArgs(1),
	RunE: runList,
}

func init() {
	// Add command to root
	rootCmd.AddCommand(listCmd)
	
	// Add flags
	listCmd.Flags().StringVar(&listRelationFlag, "relation", "children", "Relation type: {children|parent|siblings}")
	listCmd.Flags().StringVarP(&listStateFlag, "state", "s", "open", "Filter by state: {open|closed|all}")
	listCmd.Flags().IntVarP(&listLimitFlag, "limit", "L", 30, "Maximum number of issues to display")
	listCmd.Flags().StringVar(&listJSONFlag, "json", "", "Output JSON with the specified fields")
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

// getParentIssue fetches the parent issue of a sub-issue
func getParentIssue(client *api.GraphQLClient, owner, repo string, number int) (*ListResult, error) {
	// Get the issue and its parent
	parentQuery := `
		query($owner: String!, $repo: String!, $number: Int!) {
			repository(owner: $owner, name: $repo) {
				issue(number: $number) {
					id
					number
					title
					state
					parent {
						number
						title
						state
					}
				}
			}
		}`
	
	var response struct {
		Repository struct {
			Issue struct {
				ID     string `json:"id"`
				Number int    `json:"number"`
				Title  string `json:"title"`
				State  string `json:"state"`
				Parent *struct {
					Number int    `json:"number"`
					Title  string `json:"title"`
					State  string `json:"state"`
				} `json:"parent"`
			} `json:"issue"`
		} `json:"repository"`
	}
	
	variables := map[string]interface{}{
		"owner":  owner,
		"repo":   repo,
		"number": number,
	}
	
	err := client.Do(parentQuery, variables, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue #%d: %w", number, err)
	}
	
	if response.Repository.Issue.ID == "" {
		return nil, fmt.Errorf("issue #%d not found in %s/%s", number, owner, repo)
	}
	
	// Build result
	result := &ListResult{
		Parent: ParentIssue{
			Number: response.Repository.Issue.Number,
			Title:  response.Repository.Issue.Title,
			State:  strings.ToLower(response.Repository.Issue.State),
		},
		SubIssues: []SubIssue{},
		Total:     0,
		OpenCount: 0,
	}
	
	// If there's a parent, add it as a sub-issue for consistency
	if response.Repository.Issue.Parent != nil {
		parent := response.Repository.Issue.Parent
		parentIssue := SubIssue{
			Number:    parent.Number,
			Title:     parent.Title,
			State:     strings.ToLower(parent.State),
			URL:       fmt.Sprintf("https://github.com/%s/%s/issues/%d", owner, repo, parent.Number),
			Assignees: []string{},
		}
		
		// Apply state filter
		if listStateFlag == "all" || listStateFlag == parentIssue.State {
			result.SubIssues = append(result.SubIssues, parentIssue)
			result.Total++
			if parentIssue.State == "open" {
				result.OpenCount++
			}
		}
	}
	
	return result, nil
}

// getSiblingIssues fetches sibling issues of a sub-issue
func getSiblingIssues(client *api.GraphQLClient, owner, repo string, number int, limit int) (*ListResult, error) {
	// First get the issue and its parent
	parentQuery := `
		query($owner: String!, $repo: String!, $number: Int!) {
			repository(owner: $owner, name: $repo) {
				issue(number: $number) {
					id
					number
					title
					state
					parent {
						number
						title
						state
					}
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
				Parent *struct {
					Number int    `json:"number"`
					Title  string `json:"title"`
					State  string `json:"state"`
				} `json:"parent"`
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
		return nil, fmt.Errorf("failed to get issue #%d: %w", number, err)
	}
	
	if parentResponse.Repository.Issue.ID == "" {
		return nil, fmt.Errorf("issue #%d not found in %s/%s", number, owner, repo)
	}
	
	// If no parent, return empty result
	if parentResponse.Repository.Issue.Parent == nil {
		return &ListResult{
			Parent: ParentIssue{
				Number: parentResponse.Repository.Issue.Number,
				Title:  parentResponse.Repository.Issue.Title,
				State:  strings.ToLower(parentResponse.Repository.Issue.State),
			},
			SubIssues: []SubIssue{},
			Total:     0,
			OpenCount: 0,
		}, nil
	}
	
	// Get all sub-issues of the parent
	parentNumber := parentResponse.Repository.Issue.Parent.Number
	parentResult, err := getSubIssues(client, owner, repo, parentNumber, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get siblings: %w", err)
	}
	
	// Filter out the current issue from siblings
	var siblings []SubIssue
	for _, issue := range parentResult.SubIssues {
		if issue.Number != number {
			siblings = append(siblings, issue)
		}
	}
	
	// Build result with current issue as parent and siblings as sub-issues
	result := &ListResult{
		Parent: ParentIssue{
			Number: parentResponse.Repository.Issue.Number,
			Title:  parentResponse.Repository.Issue.Title,
			State:  strings.ToLower(parentResponse.Repository.Issue.State),
		},
		SubIssues: siblings,
		Total:     len(siblings),
		OpenCount: 0,
	}
	
	// Count open siblings
	for _, sibling := range siblings {
		if sibling.State == "open" {
			result.OpenCount++
		}
	}
	
	return result, nil
}

// formatTTY formats output for terminal with colors
func formatTTY(result *ListResult) string {
	var output strings.Builder
	
	// Header varies based on relation type
	var headerLabel string
	var emptyMessage string
	switch listRelationFlag {
	case "children":
		headerLabel = "Parent"
		emptyMessage = "No sub-issues found."
	case "parent":
		headerLabel = "Issue"
		emptyMessage = "No parent issue found."
	case "siblings":
		headerLabel = "Issue"
		emptyMessage = "No sibling issues found."
	}
	
	output.WriteString(fmt.Sprintf("\n%s: #%d - %s\n\n", headerLabel, result.Parent.Number, result.Parent.Title))
	
	if result.Total == 0 {
		output.WriteString(emptyMessage + "\n")
		return output.String()
	}
	
	// Summary varies based on relation type
	closedCount := result.Total - result.OpenCount
	var summaryLabel string
	switch listRelationFlag {
	case "children":
		summaryLabel = "SUB-ISSUES"
	case "parent":
		summaryLabel = "PARENT ISSUE"
	case "siblings":
		summaryLabel = "SIBLING ISSUES"
	}
	
	output.WriteString(fmt.Sprintf("%s (%d total, %d open, %d closed)\n", 
		summaryLabel, result.Total, result.OpenCount, closedCount))
	output.WriteString("─────────────────────────────\n")
	
	// Issues
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

// formatJSONWithFields formats output as JSON with selected fields
func formatJSONWithFields(result *ListResult, fields []string) (string, error) {
	// Validate fields
	validFields := map[string]bool{
		"number":        true,
		"title":         true,
		"state":         true,
		"url":           true,
		"assignees":     true,
		"parent.number": true,
		"parent.title":  true,
		"parent.state":  true,
		"total":         true,
		"openCount":     true,
		"relation":      true,
	}
	
	for _, field := range fields {
		if !validFields[field] {
			return "", fmt.Errorf("invalid field: %s. Valid fields are: number, title, state, url, assignees, parent.number, parent.title, parent.state, total, openCount, relation", field)
		}
	}
	
	// Create a map to store selected data
	output := make(map[string]interface{})
	
	// Check which fields are requested and build output
	fieldSet := make(map[string]bool)
	for _, field := range fields {
		fieldSet[field] = true
	}
	
	// Add parent fields if requested
	if fieldSet["parent.number"] || fieldSet["parent.title"] || fieldSet["parent.state"] {
		parent := make(map[string]interface{})
		if fieldSet["parent.number"] {
			parent["number"] = result.Parent.Number
		}
		if fieldSet["parent.title"] {
			parent["title"] = result.Parent.Title
		}
		if fieldSet["parent.state"] {
			parent["state"] = result.Parent.State
		}
		output["parent"] = parent
	}
	
	// Add meta fields if requested
	if fieldSet["total"] {
		output["total"] = result.Total
	}
	if fieldSet["openCount"] {
		output["openCount"] = result.OpenCount
	}
	if fieldSet["relation"] {
		output["relation"] = listRelationFlag
	}
	
	// Add sub-issues with selected fields
	if fieldSet["number"] || fieldSet["title"] || fieldSet["state"] || fieldSet["url"] || fieldSet["assignees"] {
		var subIssues []map[string]interface{}
		for _, issue := range result.SubIssues {
			subIssue := make(map[string]interface{})
			if fieldSet["number"] {
				subIssue["number"] = issue.Number
			}
			if fieldSet["title"] {
				subIssue["title"] = issue.Title
			}
			if fieldSet["state"] {
				subIssue["state"] = issue.State
			}
			if fieldSet["url"] {
				subIssue["url"] = issue.URL
			}
			if fieldSet["assignees"] {
				subIssue["assignees"] = issue.Assignees
			}
			subIssues = append(subIssues, subIssue)
		}
		output["subIssues"] = subIssues
	}
	
	jsonBytes, err := json.MarshalIndent(output, "", "  ")
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
	// Validate relation flag
	if listRelationFlag != "children" && listRelationFlag != "parent" && listRelationFlag != "siblings" {
		return fmt.Errorf("invalid relation type: %s (must be children, parent, or siblings)", listRelationFlag)
	}
	
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
	
	// Parse issue reference
	issueRef, err := parseIssueReference(args[0], defaultOwner, defaultRepo)
	if err != nil {
		return fmt.Errorf("invalid issue reference: %w", err)
	}
	
	// Handle --web flag
	if listWebFlag {
		url := fmt.Sprintf("https://github.com/%s/%s/issues/%d", 
			issueRef.Owner, issueRef.Repo, issueRef.Number)
		fmt.Fprintf(cmd.OutOrStderr(), "Opening %s in browser...\n", url)
		return openInBrowser(url)
	}
	
	// Create GraphQL client
	client, err := api.NewGraphQLClient(api.ClientOptions{})
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}
	
	// Get issues based on relation type
	var result *ListResult
	switch listRelationFlag {
	case "children":
		result, err = getSubIssues(client, issueRef.Owner, issueRef.Repo, issueRef.Number, listLimitFlag)
	case "parent":
		result, err = getParentIssue(client, issueRef.Owner, issueRef.Repo, issueRef.Number)
	case "siblings":
		result, err = getSiblingIssues(client, issueRef.Owner, issueRef.Repo, issueRef.Number, listLimitFlag)
	}
	if err != nil {
		return err
	}
	
	// Format output
	var output string
	
	if cmd.Flags().Changed("json") {
		// JSON output requires field specification
		if listJSONFlag == "" {
			// Print available fields when no fields specified
			fmt.Fprintln(cmd.OutOrStderr(), "Specify one or more comma-separated fields for `--json`:\n  assignees\n  number\n  openCount\n  parent.number\n  parent.state\n  parent.title\n  relation\n  state\n  title\n  total\n  url")
			return fmt.Errorf("")
		}
		
		// Field selection: --json field1,field2,...
		fields := strings.Split(listJSONFlag, ",")
		for i, field := range fields {
			fields[i] = strings.TrimSpace(field)
		}
		output, err = formatJSONWithFields(result, fields)
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