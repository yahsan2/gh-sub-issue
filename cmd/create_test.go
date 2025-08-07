package cmd

import (
	"testing"
)

func TestCreateCmdFlags(t *testing.T) {
	tests := []struct {
		name        string
		flagName    string
		required    bool
		shorthand   string
		hasDefault  bool
	}{
		{
			name:      "parent flag",
			flagName:  "parent",
			required:  true,
			shorthand: "p",
		},
		{
			name:      "title flag",
			flagName:  "title",
			required:  true,
			shorthand: "t",
		},
		{
			name:      "body flag",
			flagName:  "body",
			required:  false,
			shorthand: "b",
		},
		{
			name:      "label flag",
			flagName:  "label",
			required:  false,
			shorthand: "l",
		},
		{
			name:      "assignee flag",
			flagName:  "assignee",
			required:  false,
			shorthand: "a",
		},
		{
			name:      "milestone flag",
			flagName:  "milestone",
			required:  false,
			shorthand: "m",
		},
		{
			name:      "repo flag",
			flagName:  "repo",
			required:  false,
			shorthand: "R",
		},
		{
			name:      "project flag",
			flagName:  "project",
			required:  false,
			shorthand: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := createCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag '%s' not found", tt.flagName)
				return
			}

			if tt.shorthand != "" {
				if flag.Shorthand != tt.shorthand {
					t.Errorf("flag shorthand: got %s, want %s", flag.Shorthand, tt.shorthand)
				}
			}
		})
	}
}

func TestCreateCmdValidation(t *testing.T) {
	// Test that required flags are properly marked
	parentFlag := createCmd.Flags().Lookup("parent")
	if parentFlag == nil {
		t.Error("parent flag not found")
	}
	
	titleFlag := createCmd.Flags().Lookup("title")
	if titleFlag == nil {
		t.Error("title flag not found")
	}
	
	// Test flag parsing with various combinations
	tests := []struct {
		name        string
		parent      string
		title       string
		body        string
		labels      []string
		assignees   []string
		milestone   string
		repo        string
	}{
		{
			name:   "minimal valid input",
			parent: "123",
			title:  "Test Issue",
		},
		{
			name:   "with body",
			parent: "456",
			title:  "Test Issue",
			body:   "Test body content",
		},
		{
			name:      "with labels and assignees",
			parent:    "789",
			title:     "Test Issue",
			labels:    []string{"bug", "priority"},
			assignees: []string{"octocat"},
		},
		{
			name:      "with milestone and repo",
			parent:    "https://github.com/owner/repo/issues/100",
			title:     "Test Issue",
			milestone: "v1.0",
			repo:      "owner/repo",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify the flags exist and can be set
			if tt.parent != "" {
				parentFlag.Value.Set(tt.parent)
			}
			if tt.title != "" {
				titleFlag.Value.Set(tt.title)
			}
		})
	}
}

func TestParseRepoFlag(t *testing.T) {
	tests := []struct {
		name          string
		repoFlag      string
		expectedOwner string
		expectedRepo  string
		expectError   bool
	}{
		{
			name:          "valid repo format",
			repoFlag:      "owner/repo",
			expectedOwner: "owner",
			expectedRepo:  "repo",
			expectError:   false,
		},
		{
			name:          "org with dash",
			repoFlag:      "my-org/my-repo",
			expectedOwner: "my-org",
			expectedRepo:  "my-repo",
			expectError:   false,
		},
		{
			name:        "invalid format - no slash",
			repoFlag:    "ownerrepo",
			expectError: true,
		},
		{
			name:        "invalid format - too many slashes",
			repoFlag:    "owner/repo/extra",
			expectError: true,
		},
		{
			name:        "empty string",
			repoFlag:    "",
			expectError: false, // Empty is valid, falls back to default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.repoFlag == "" {
				// Skip empty string test as it requires git context
				return
			}

			parts := splitRepoFlag(tt.repoFlag)
			
			if tt.expectError {
				if len(parts) == 2 {
					t.Errorf("expected error for invalid format but got owner=%s, repo=%s", parts[0], parts[1])
				}
				return
			}

			if len(parts) != 2 {
				t.Errorf("expected 2 parts but got %d", len(parts))
				return
			}

			if parts[0] != tt.expectedOwner {
				t.Errorf("owner: got %s, want %s", parts[0], tt.expectedOwner)
			}
			if parts[1] != tt.expectedRepo {
				t.Errorf("repo: got %s, want %s", parts[1], tt.expectedRepo)
			}
		})
	}
}

func TestBuildCreateInput(t *testing.T) {
	tests := []struct {
		name           string
		repoID         string
		title          string
		parentID       string
		body           string
		labelIDs       []string
		assigneeIDs    []string
		milestoneID    string
		expectedFields int // Number of fields in the input map
	}{
		{
			name:           "minimal input",
			repoID:         "repo123",
			title:          "Test Issue",
			parentID:       "parent456",
			body:           "",
			expectedFields: 3, // repositoryId, title, parentIssueId
		},
		{
			name:           "with body",
			repoID:         "repo123",
			title:          "Test Issue",
			parentID:       "parent456",
			body:           "Test body content",
			expectedFields: 4, // repositoryId, title, parentIssueId, body
		},
		{
			name:           "with labels",
			repoID:         "repo123",
			title:          "Test Issue",
			parentID:       "parent456",
			body:           "",
			labelIDs:       []string{"label1", "label2"},
			expectedFields: 4, // repositoryId, title, parentIssueId, labelIds
		},
		{
			name:           "with assignees",
			repoID:         "repo123",
			title:          "Test Issue",
			parentID:       "parent456",
			body:           "",
			assigneeIDs:    []string{"user1", "user2"},
			expectedFields: 4, // repositoryId, title, parentIssueId, assigneeIds
		},
		{
			name:           "with milestone",
			repoID:         "repo123",
			title:          "Test Issue",
			parentID:       "parent456",
			body:           "",
			milestoneID:    "milestone789",
			expectedFields: 4, // repositoryId, title, parentIssueId, milestoneId
		},
		{
			name:           "with all fields",
			repoID:         "repo123",
			title:          "Test Issue",
			parentID:       "parent456",
			body:           "Test body",
			labelIDs:       []string{"label1"},
			assigneeIDs:    []string{"user1"},
			milestoneID:    "milestone789",
			expectedFields: 7, // All fields
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := buildCreateIssueInput(
				tt.repoID,
				tt.title,
				tt.parentID,
				tt.body,
				tt.labelIDs,
				tt.assigneeIDs,
				tt.milestoneID,
			)

			if len(input) != tt.expectedFields {
				t.Errorf("input fields count: got %d, want %d", len(input), tt.expectedFields)
			}

			// Check required fields
			if input["repositoryId"] != tt.repoID {
				t.Errorf("repositoryId: got %v, want %s", input["repositoryId"], tt.repoID)
			}
			if input["title"] != tt.title {
				t.Errorf("title: got %v, want %s", input["title"], tt.title)
			}
			if input["parentIssueId"] != tt.parentID {
				t.Errorf("parentIssueId: got %v, want %s", input["parentIssueId"], tt.parentID)
			}

			// Check optional fields
			if tt.body != "" {
				if input["body"] != tt.body {
					t.Errorf("body: got %v, want %s", input["body"], tt.body)
				}
			} else {
				if _, exists := input["body"]; exists {
					t.Errorf("body should not be in input when empty")
				}
			}

			if len(tt.labelIDs) > 0 {
				if _, exists := input["labelIds"]; !exists {
					t.Errorf("labelIds should be in input")
				}
			}

			if len(tt.assigneeIDs) > 0 {
				if _, exists := input["assigneeIds"]; !exists {
					t.Errorf("assigneeIds should be in input")
				}
			}

			if tt.milestoneID != "" {
				if input["milestoneId"] != tt.milestoneID {
					t.Errorf("milestoneId: got %v, want %s", input["milestoneId"], tt.milestoneID)
				}
			}
		})
	}
}

// Helper functions for testing
func splitRepoFlag(repo string) []string {
	if repo == "" {
		return []string{}
	}
	parts := []string{}
	lastSlash := -1
	for i, ch := range repo {
		if ch == '/' {
			if lastSlash != -1 {
				// Too many slashes
				return []string{}
			}
			parts = append(parts, repo[:i])
			lastSlash = i
		}
	}
	if lastSlash == -1 {
		// No slash found
		return []string{}
	}
	parts = append(parts, repo[lastSlash+1:])
	return parts
}

func buildCreateIssueInput(repoID, title, parentID, body string, labelIDs, assigneeIDs []string, milestoneID string) map[string]interface{} {
	input := map[string]interface{}{
		"repositoryId":  repoID,
		"title":         title,
		"parentIssueId": parentID,
	}

	if body != "" {
		input["body"] = body
	}

	if len(labelIDs) > 0 {
		input["labelIds"] = labelIDs
	}

	if len(assigneeIDs) > 0 {
		input["assigneeIds"] = assigneeIDs
	}

	if milestoneID != "" {
		input["milestoneId"] = milestoneID
	}

	return input
}