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
	projectsFlag   []string  // Changed to support multiple projects
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
  
  # Add to a single project
  gh sub-issue create --parent 123 --title "Task" --project "Roadmap"
  
  # Add to multiple projects (GitHub CLI compatible)
  gh sub-issue create --parent 123 --title "Task" --project "Dev Sprint" --project "Q1 Goals"
  
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
	createCmd.Flags().StringSliceVar(&projectsFlag, "project", []string{}, "Add issue to projects (can specify multiple times)")
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

// getProjectV2ID gets the GraphQL node ID for a ProjectV2
func getProjectV2ID(client *api.GraphQLClient, owner, repo, project string) (string, error) {
	if project == "" {
		return "", nil
	}
	
	// First, try to find project in repository
	repoQuery := `
		query($owner: String!, $repo: String!) {
			repository(owner: $owner, name: $repo) {
				projectsV2(first: 100) {
					nodes {
						id
						title
						number
					}
				}
			}
		}`
	
	variables := map[string]interface{}{
		"owner": owner,
		"repo":  repo,
	}
	
	var repoResponse struct {
		Repository struct {
			ProjectsV2 struct {
				Nodes []struct {
					ID     string `json:"id"`
					Title  string `json:"title"`
					Number int    `json:"number"`
				} `json:"nodes"`
			} `json:"projectsV2"`
		} `json:"repository"`
	}
	
	err := client.Do(repoQuery, variables, &repoResponse)
	if err == nil {
		// Check by title or number
		for _, p := range repoResponse.Repository.ProjectsV2.Nodes {
			if strings.EqualFold(p.Title, project) || fmt.Sprint(p.Number) == project {
				return p.ID, nil
			}
		}
	}
	
	// Try user-level projects
	userQuery := `
		query($login: String!) {
			user(login: $login) {
				projectsV2(first: 100) {
					nodes {
						id
						title
						number
					}
				}
			}
		}`
	
	userVars := map[string]interface{}{
		"login": owner,
	}
	
	var userResponse struct {
		User struct {
			ProjectsV2 struct {
				Nodes []struct {
					ID     string `json:"id"`
					Title  string `json:"title"`
					Number int    `json:"number"`
				} `json:"nodes"`
			} `json:"projectsV2"`
		} `json:"user"`
	}
	
	err = client.Do(userQuery, userVars, &userResponse)
	if err == nil {
		for _, p := range userResponse.User.ProjectsV2.Nodes {
			if strings.EqualFold(p.Title, project) || fmt.Sprint(p.Number) == project {
				return p.ID, nil
			}
		}
	}
	
	// Try organization-level projects
	orgQuery := `
		query($login: String!) {
			organization(login: $login) {
				projectsV2(first: 100) {
					nodes {
						id
						title
						number
					}
				}
			}
		}`
	
	var orgResponse struct {
		Organization struct {
			ProjectsV2 struct {
				Nodes []struct {
					ID     string `json:"id"`
					Title  string `json:"title"`
					Number int    `json:"number"`
				} `json:"nodes"`
			} `json:"projectsV2"`
		} `json:"organization"`
	}
	
	err = client.Do(orgQuery, userVars, &orgResponse)
	if err == nil {
		for _, p := range orgResponse.Organization.ProjectsV2.Nodes {
			if strings.EqualFold(p.Title, project) || fmt.Sprint(p.Number) == project {
				return p.ID, nil
			}
		}
	}
	
	fmt.Printf("Warning: project '%s' not found\n", project)
	return "", nil
}

// assignToProjectV2 assigns an issue to a ProjectV2 using the addProjectV2ItemById mutation
func assignToProjectV2(client *api.GraphQLClient, projectID, issueID string) error {
	if projectID == "" || issueID == "" {
		return nil
	}
	
	mutation := `
		mutation AddProjectV2Item($projectId: ID!, $contentId: ID!) {
			addProjectV2ItemById(input: {projectId: $projectId, contentId: $contentId}) {
				item {
					id
				}
			}
		}`
	
	variables := map[string]interface{}{
		"projectId": projectID,
		"contentId": issueID,
	}
	
	var response struct {
		AddProjectV2ItemById struct {
			Item struct {
				ID string `json:"id"`
			} `json:"item"`
		} `json:"addProjectV2ItemById"`
	}
	
	err := client.Do(mutation, variables, &response)
	if err != nil {
		return fmt.Errorf("failed to add issue to project: %w", err)
	}
	
	return nil
}

// createSubIssue creates a new issue with a parent issue
func createSubIssue(client *api.GraphQLClient, input map[string]interface{}) (int, string, string, error) {
	mutation := `
		mutation CreateSubIssue($input: CreateIssueInput!) {
			createIssue(input: $input) {
				issue {
					id
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
				ID     string `json:"id"`
				Number int    `json:"number"`
				URL    string `json:"url"`
				Title  string `json:"title"`
			} `json:"issue"`
		} `json:"createIssue"`
	}
	
	err := client.Do(mutation, variables, &response)
	if err != nil {
		return 0, "", "", fmt.Errorf("failed to create sub-issue: %w", err)
	}
	
	return response.CreateIssue.Issue.Number, response.CreateIssue.Issue.URL, response.CreateIssue.Issue.ID, nil
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
	
	// Get project IDs if specified (will be assigned after issue creation)
	var projectIDs []string
	if len(projectsFlag) > 0 {
		fmt.Fprintf(cmd.OutOrStderr(), "Getting project IDs...\n")
		for _, project := range projectsFlag {
			projectID, err := getProjectV2ID(client, defaultOwner, defaultRepo, project)
			if err != nil {
				return err
			}
			if projectID != "" {
				projectIDs = append(projectIDs, projectID)
			}
		}
	}
	
	// Create the sub-issue
	fmt.Fprintf(cmd.OutOrStderr(), "Creating sub-issue...\n")
	number, url, issueID, err := createSubIssue(client, input)
	if err != nil {
		if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "403") {
			return fmt.Errorf("insufficient permissions to create issues in %s/%s",
				defaultOwner, defaultRepo)
		}
		return err
	}
	
	// Assign to projects if specified
	if len(projectIDs) > 0 {
		fmt.Fprintf(cmd.OutOrStderr(), "Assigning issue to projects...\n")
		for i, projectID := range projectIDs {
			err := assignToProjectV2(client, projectID, issueID)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStderr(), "Warning: Failed to add to project %s: %v\n", projectsFlag[i], err)
			}
		}
	}
	
	// Success message
	fmt.Fprintf(cmd.OutOrStdout(), "✓ Created sub-issue #%d: %s\n", number, url)
	
	return nil
}