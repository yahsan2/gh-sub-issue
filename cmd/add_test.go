package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func TestParseIssueReference(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		defaultOwner  string
		defaultRepo   string
		expectedOwner string
		expectedRepo  string
		expectedNum   int
		expectError   bool
		errorContains string
	}{
		{
			name:          "valid issue number",
			input:         "123",
			defaultOwner:  "owner",
			defaultRepo:   "repo",
			expectedOwner: "owner",
			expectedRepo:  "repo",
			expectedNum:   123,
		},
		{
			name:          "github url with https",
			input:         "https://github.com/octocat/hello-world/issues/456",
			defaultOwner:  "default",
			defaultRepo:   "default",
			expectedOwner: "octocat",
			expectedRepo:  "hello-world",
			expectedNum:   456,
		},
		{
			name:          "github url with http",
			input:         "http://github.com/owner/repo/issues/789",
			defaultOwner:  "default",
			defaultRepo:   "default",
			expectedOwner: "owner",
			expectedRepo:  "repo",
			expectedNum:   789,
		},
		{
			name:          "invalid issue number",
			input:         "abc",
			defaultOwner:  "owner",
			defaultRepo:   "repo",
			expectError:   true,
			errorContains: "invalid issue reference",
		},
		{
			name:          "negative issue number",
			input:         "-5",
			defaultOwner:  "owner",
			defaultRepo:   "repo",
			expectError:   true,
			errorContains: "invalid issue number",
		},
		{
			name:          "zero issue number",
			input:         "0",
			defaultOwner:  "owner",
			defaultRepo:   "repo",
			expectError:   true,
			errorContains: "invalid issue number",
		},
		{
			name:          "invalid url - not github",
			input:         "https://gitlab.com/owner/repo/issues/123",
			expectError:   true,
			errorContains: "not a GitHub URL",
		},
		{
			name:          "invalid url - wrong path",
			input:         "https://github.com/owner/repo/pulls/123",
			expectError:   true,
			errorContains: "not an issue URL",
		},
		{
			name:          "invalid url - too short",
			input:         "https://github.com/owner",
			expectError:   true,
			errorContains: "invalid GitHub issue URL format",
		},
		{
			name:          "url with trailing slash",
			input:         "https://github.com/owner/repo/issues/123/",
			defaultOwner:  "default",
			defaultRepo:   "default",
			expectedOwner: "owner",
			expectedRepo:  "repo",
			expectedNum:   123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := parseIssueReference(tt.input, tt.defaultOwner, tt.defaultRepo)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
					t.Errorf("error message should contain '%s', got: %s", tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if ref.Owner != tt.expectedOwner {
				t.Errorf("owner: got %s, want %s", ref.Owner, tt.expectedOwner)
			}
			if ref.Repo != tt.expectedRepo {
				t.Errorf("repo: got %s, want %s", ref.Repo, tt.expectedRepo)
			}
			if ref.Number != tt.expectedNum {
				t.Errorf("number: got %d, want %d", ref.Number, tt.expectedNum)
			}
		})
	}
}

func TestParseIssueURL(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedOwner string
		expectedRepo  string
		expectedNum   int
		expectError   bool
		errorContains string
	}{
		{
			name:          "valid github issue url",
			input:         "https://github.com/owner/repo/issues/123",
			expectedOwner: "owner",
			expectedRepo:  "repo",
			expectedNum:   123,
		},
		{
			name:          "url with trailing slash",
			input:         "https://github.com/owner/repo/issues/456/",
			expectedOwner: "owner",
			expectedRepo:  "repo",
			expectedNum:   456,
		},
		{
			name:          "url with complex repo name",
			input:         "https://github.com/my-org/my-awesome-repo/issues/789",
			expectedOwner: "my-org",
			expectedRepo:  "my-awesome-repo",
			expectedNum:   789,
		},
		{
			name:          "invalid - not github",
			input:         "https://gitlab.com/owner/repo/issues/123",
			expectError:   true,
			errorContains: "not a GitHub URL",
		},
		{
			name:          "invalid - pull request url",
			input:         "https://github.com/owner/repo/pull/123",
			expectError:   true,
			errorContains: "not an issue URL",
		},
		{
			name:          "invalid - malformed url",
			input:         "https://github.com/owner",
			expectError:   true,
			errorContains: "invalid GitHub issue URL format",
		},
		{
			name:          "invalid - non-numeric issue",
			input:         "https://github.com/owner/repo/issues/abc",
			expectError:   true,
			errorContains: "invalid issue number in URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := parseIssueURL(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
					t.Errorf("error message should contain '%s', got: %s", tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if ref.Owner != tt.expectedOwner {
				t.Errorf("owner: got %s, want %s", ref.Owner, tt.expectedOwner)
			}
			if ref.Repo != tt.expectedRepo {
				t.Errorf("repo: got %s, want %s", ref.Repo, tt.expectedRepo)
			}
			if ref.Number != tt.expectedNum {
				t.Errorf("number: got %d, want %d", ref.Number, tt.expectedNum)
			}
		})
	}
}

func TestAddCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantErr     bool
		errContains string
	}{
		{
			name:        "no arguments",
			args:        []string{},
			wantErr:     true,
			errContains: "requires at least 2 arg(s)",
		},
		{
			name:        "only parent issue",
			args:        []string{"123"},
			wantErr:     true,
			errContains: "requires at least 2 arg(s)",
		},
		{
			name:    "valid parent and single sub-issue",
			args:    []string{"123", "456"},
			wantErr: false,
		},
		{
			name:    "valid parent and multiple sub-issues",
			args:    []string{"123", "456", "457", "458"},
			wantErr: false,
		},
		{
			name:    "parent as URL",
			args:    []string{"https://github.com/owner/repo/issues/123", "456"},
			wantErr: false,
		},
		{
			name:    "sub-issue as URL",
			args:    []string{"123", "https://github.com/owner/repo/issues/456"},
			wantErr: false,
		},
		{
			name:    "multiple sub-issues with URLs",
			args:    []string{"123", "456", "https://github.com/owner/repo/issues/457", "458"},
			wantErr: false,
		},
		{
			name:    "with repo flag",
			args:    []string{"123", "456", "--repo", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "with short repo flag",
			args:    []string{"123", "456", "-R", "owner/repo"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new root command for each test
			cmd := &cobra.Command{Use: "test"}
			cmd.AddCommand(addCmd)
			cmd.SetArgs(append([]string{"add"}, tt.args...))

			// Capture output
			var outBuf, errBuf bytes.Buffer
			cmd.SetOut(&outBuf)
			cmd.SetErr(&errBuf)

			// Execute command (will fail due to no API client, but we're testing argument parsing)
			err := cmd.Execute()

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("error message should contain '%s', got: %s", tt.errContains, err.Error())
				}
			} else {
				// These will fail with API errors, but argument parsing should succeed
				// We check that the error is not about arguments
				if err != nil {
					if containsString(err.Error(), "arg(s)") || containsString(err.Error(), "unknown flag") {
						t.Errorf("unexpected argument parsing error: %v", err)
					}
				}
			}
		})
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsString(s[1:], substr) || len(substr) > 0 && s[:len(substr)] == substr)
}