package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/spf13/cobra"
)

var (
	parentFlag     string
	titleFlag      string
	bodyFlag       string
	labelsFlag     []string
	assigneesFlag  []string
	milestoneFlag  string
	projectFlag    string
	createRepoFlag string
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new issue as a sub-issue of a parent issue",
	Long: `Create a new issue directly linked to a parent issue using GitHub's issue hierarchy feature.

Examples:
  # Create with minimal options
  gh sub-issue create --parent 123 --title "Implement feature X"
  
  # Create with body text
  gh sub-issue create --parent 123 --title "Bug fix" --body "Description of the issue"
  
  # Create with labels and assignees
  gh sub-issue create --parent 123 --title "Task" --label bug --label priority --assignee username
  
  # Cross-repository parent issue
  gh sub-issue create --parent https://github.com/owner/repo/issues/123 --title "Sub-task"
  
  # Specify repository for new issue
  gh sub-issue create --parent 123 --title "Task" --repo owner/repo`,
	RunE: runCreate,
}

func init() {
	rootCmd.AddCommand(createCmd)
	
	createCmd.Flags().StringVarP(&parentFlag, "parent", "p", "", "Parent issue number or URL (required)")
	createCmd.Flags().StringVarP(&titleFlag, "title", "t", "", "Title for the new sub-issue (required)")
	createCmd.Flags().StringVarP(&bodyFlag, "body", "b", "", "Body text for the new sub-issue")
	createCmd.Flags().StringSliceVarP(&labelsFlag, "label", "l", []string{}, "Add labels to the issue")
	createCmd.Flags().StringSliceVarP(&assigneesFlag, "assignee", "a", []string{}, "Assign users to the issue")
	createCmd.Flags().StringVarP(&milestoneFlag, "milestone", "m", "", "Set milestone for the issue")
	createCmd.Flags().StringVar(&projectFlag, "project", "", "Add issue to project")
	createCmd.Flags().StringVarP(&createRepoFlag, "repo", "R", "", "Repository for the new issue in OWNER/REPO format")
	
	createCmd.MarkFlagRequired("parent")
	createCmd.MarkFlagRequired("title")
}

// getRepositoryID gets the GraphQL node ID for a repository
func getRepositoryID(client *api.GraphQLClient, owner, repo string) (string, error) {
	query := `
		query($owner: String!, $repo: String!) {
			repository(owner: $owner, name: $repo) {
				id
			}
		}`
	
	variables := map[string]interface{}{
		"owner": owner,
		"repo":  repo,
	}
	
	var response struct {
		Repository struct {
			ID string `json:"id"`
		} `json:"repository"`
	}
	
	err := client.Do(query, variables, &response)
	if err != nil {
		return "", fmt.Errorf("failed to get repository %s/%s: %w", owner, repo, err)
	}
	
	if response.Repository.ID == "" {
		return "", fmt.Errorf("repository %s/%s not found", owner, repo)
	}
	
	return response.Repository.ID, nil
}

// getLabelIDs gets the GraphQL node IDs for labels
func getLabelIDs(client *api.GraphQLClient, owner, repo string, labels []string) ([]string, error) {
	if len(labels) == 0 {
		return nil, nil
	}
	
	query := `
		query($owner: String!, $repo: String!) {
			repository(owner: $owner, name: $repo) {
				labels(first: 100) {
					nodes {
						id
						name
					}
				}
			}
		}`
	
	variables := map[string]interface{}{
		"owner": owner,
		"repo":  repo,
	}
	
	var response struct {
		Repository struct {
			Labels struct {
				Nodes []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"nodes"`
			} `json:"labels"`
		} `json:"repository"`
	}
	
	err := client.Do(query, variables, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to get labels: %w", err)
	}
	
	labelMap := make(map[string]string)
	for _, label := range response.Repository.Labels.Nodes {
		labelMap[strings.ToLower(label.Name)] = label.ID
	}
	
	var labelIDs []string
	for _, labelName := range labels {
		if id, ok := labelMap[strings.ToLower(labelName)]; ok {
			labelIDs = append(labelIDs, id)
		} else {
			fmt.Printf("Warning: label '%s' not found in repository\n", labelName)
		}
	}
	
	return labelIDs, nil
}

// getUserIDs gets the GraphQL node IDs for users
func getUserIDs(client *api.GraphQLClient, usernames []string) ([]string, error) {
	if len(usernames) == 0 {
		return nil, nil
	}
	
	var userIDs []string
	for _, username := range usernames {
		query := `
			query($login: String!) {
				user(login: $login) {
					id
				}
			}`
		
		variables := map[string]interface{}{
			"login": username,
		}
		
		var response struct {
			User struct {
				ID string `json:"id"`
			} `json:"user"`
		}
		
		err := client.Do(query, variables, &response)
		if err != nil {
			fmt.Printf("Warning: user '%s' not found\n", username)
			continue
		}
		
		if response.User.ID != "" {
			userIDs = append(userIDs, response.User.ID)
		}
	}
	
	return userIDs, nil
}

// getMilestoneID gets the GraphQL node ID for a milestone
func getMilestoneID(client *api.GraphQLClient, owner, repo, milestone string) (string, error) {
	if milestone == "" {
		return "", nil
	}
	
	query := `
		query($owner: String!, $repo: String!) {
			repository(owner: $owner, name: $repo) {
				milestones(first: 100, states: OPEN) {
					nodes {
						id
						title
					}
				}
			}
		}`
	
	variables := map[string]interface{}{
		"owner": owner,
		"repo":  repo,
	}
	
	var response struct {
		Repository struct {
			Milestones struct {
				Nodes []struct {
					ID    string `json:"id"`
					Title string `json:"title"`
				} `json:"nodes"`
			} `json:"milestones"`
		} `json:"repository"`
	}
	
	err := client.Do(query, variables, &response)
	if err != nil {
		return "", fmt.Errorf("failed to get milestones: %w", err)
	}
	
	for _, m := range response.Repository.Milestones.Nodes {
		if strings.EqualFold(m.Title, milestone) {
			return m.ID, nil
		}
	}
	
	fmt.Printf("Warning: milestone '%s' not found\n", milestone)
	return "", nil
}

// getProjectID gets the GraphQL node ID for a project
func getProjectID(client *api.GraphQLClient, owner, repo, project string) (string, error) {
	if project == "" {
		return "", nil
	}
	
	query := `
		query($owner: String!, $repo: String!) {
			repository(owner: $owner, name: $repo) {
				projects(first: 100, states: OPEN) {
					nodes {
						id
						name
					}
				}
			}
		}`
	
	variables := map[string]interface{}{
		"owner": owner,
		"repo":  repo,
	}
	
	var response struct {
		Repository struct {
			Projects struct {
				Nodes []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"nodes"`
			} `json:"projects"`
		} `json:"repository"`
	}
	
	err := client.Do(query, variables, &response)
	if err != nil {
		return "", fmt.Errorf("failed to get projects: %w", err)
	}
	
	for _, p := range response.Repository.Projects.Nodes {
		if strings.EqualFold(p.Name, project) {
			return p.ID, nil
		}
	}
	
	fmt.Printf("Warning: project '%s' not found in repository\n", project)
	return "", nil
}

// createSubIssue creates a new issue with a parent issue
func createSubIssue(client *api.GraphQLClient, input map[string]interface{}) (int, string, error) {
	mutation := `
		mutation CreateSubIssue($input: CreateIssueInput!) {
			createIssue(input: $input) {
				issue {
					number
					url
					title
				}
			}
		}`
	
	variables := map[string]interface{}{
		"input": input,
	}
	
	var response struct {
		CreateIssue struct {
			Issue struct {
				Number int    `json:"number"`
				URL    string `json:"url"`
				Title  string `json:"title"`
			} `json:"issue"`
		} `json:"createIssue"`
	}
	
	err := client.Do(mutation, variables, &response)
	if err != nil {
		return 0, "", fmt.Errorf("failed to create sub-issue: %w", err)
	}
	
	return response.CreateIssue.Issue.Number, response.CreateIssue.Issue.URL, nil
}

func runCreate(cmd *cobra.Command, args []string) error {
	_ = context.Background() // Reserved for future use
	
	// Get default repository
	var defaultOwner, defaultRepo string
	var err error
	
	if createRepoFlag != "" {
		parts := strings.Split(createRepoFlag, "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid repository format: %s (expected OWNER/REPO)", createRepoFlag)
		}
		defaultOwner = parts[0]
		defaultRepo = parts[1]
	} else {
		defaultOwner, defaultRepo, err = getDefaultRepo()
		if err != nil {
			return fmt.Errorf("could not determine repository (use --repo flag): %w", err)
		}
	}
	
	// Parse parent issue reference
	parentRef, err := parseIssueReference(parentFlag, defaultOwner, defaultRepo)
	if err != nil {
		return fmt.Errorf("invalid parent issue: %w", err)
	}
	
	// Create GraphQL client
	client, err := api.NewGraphQLClient(api.ClientOptions{})
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
	
	// Get repository ID for the new issue
	fmt.Fprintf(cmd.OutOrStderr(), "Getting repository information...\n")
	repoID, err := getRepositoryID(client, defaultOwner, defaultRepo)
	if err != nil {
		return err
	}
	
	// Build the mutation input
	input := map[string]interface{}{
		"repositoryId":  repoID,
		"title":         titleFlag,
		"parentIssueId": parentID,
	}
	
	if bodyFlag != "" {
		input["body"] = bodyFlag
	}
	
	// Get label IDs if specified
	if len(labelsFlag) > 0 {
		fmt.Fprintf(cmd.OutOrStderr(), "Getting label IDs...\n")
		labelIDs, err := getLabelIDs(client, defaultOwner, defaultRepo, labelsFlag)
		if err != nil {
			return err
		}
		if len(labelIDs) > 0 {
			input["labelIds"] = labelIDs
		}
	}
	
	// Get assignee IDs if specified
	if len(assigneesFlag) > 0 {
		fmt.Fprintf(cmd.OutOrStderr(), "Getting assignee IDs...\n")
		assigneeIDs, err := getUserIDs(client, assigneesFlag)
		if err != nil {
			return err
		}
		if len(assigneeIDs) > 0 {
			input["assigneeIds"] = assigneeIDs
		}
	}
	
	// Get milestone ID if specified
	if milestoneFlag != "" {
		fmt.Fprintf(cmd.OutOrStderr(), "Getting milestone ID...\n")
		milestoneID, err := getMilestoneID(client, defaultOwner, defaultRepo, milestoneFlag)
		if err != nil {
			return err
		}
		if milestoneID != "" {
			input["milestoneId"] = milestoneID
		}
	}
	
	// Get project ID if specified
	if projectFlag != "" {
		fmt.Fprintf(cmd.OutOrStderr(), "Getting project ID...\n")
		projectID, err := getProjectID(client, defaultOwner, defaultRepo, projectFlag)
		if err != nil {
			return err
		}
		if projectID != "" {
			input["projectIds"] = []string{projectID}
		}
	}
	
	// Create the sub-issue
	fmt.Fprintf(cmd.OutOrStderr(), "Creating sub-issue...\n")
	number, url, err := createSubIssue(client, input)
	if err != nil {
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "403") {
			return fmt.Errorf("insufficient permissions to create issues in %s/%s",
				defaultOwner, defaultRepo)
		}
		return err
	}
	
	// Success message
	fmt.Fprintf(cmd.OutOrStdout(), "✓ Created sub-issue #%d: %s\n", number, url)
	
	return nil
}