package cmd

import (
	"testing"
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

func TestCheckCurrentParent(t *testing.T) {
	base := IssueReference{Owner: "owner", Repo: "repo"}
	sub := IssueReference{Owner: "owner", Repo: "repo", Number: 212}
	requested := IssueReference{Owner: "owner", Repo: "repo", Number: 211}
	const requestedID = "I_211"

	tests := []struct {
		name          string
		sub           IssueReference
		requested     IssueReference
		current       *issueParent
		replaceParent bool
		wantDone      bool
		wantErr       string // empty means no error
	}{
		{
			name:      "no current parent",
			sub:       sub,
			requested: requested,
		},
		{
			name:          "no current parent, with replace",
			sub:           sub,
			requested:     requested,
			replaceParent: true,
		},
		{
			name:      "already under the requested parent",
			sub:       sub,
			requested: requested,
			current: &issueParent{
				ID:  requestedID,
				Ref: IssueReference{Owner: "owner", Repo: "repo", Number: 211},
			},
			wantDone: true,
		},
		{
			name:      "already under the requested parent, its repository since renamed",
			sub:       sub,
			requested: requested,
			current: &issueParent{
				ID:  requestedID,
				Ref: IssueReference{Owner: "owner", Repo: "renamed", Number: 211},
			},
			wantDone: true,
		},
		{
			name:          "already under the requested parent, with replace",
			sub:           sub,
			requested:     requested,
			replaceParent: true,
			current: &issueParent{
				ID:  requestedID,
				Ref: IssueReference{Owner: "owner", Repo: "repo", Number: 211},
			},
			wantDone: true,
		},
		{
			name:      "under another parent in the same repository",
			sub:       sub,
			requested: requested,
			current: &issueParent{
				ID:  "I_300",
				Ref: IssueReference{Owner: "owner", Repo: "repo", Number: 300},
			},
			wantErr: "issue #212 is already a sub-issue of #300; use --replace-parent to move it under #211",
		},
		{
			name:      "under another parent with the same number, in another repository",
			sub:       sub,
			requested: requested,
			current: &issueParent{
				ID:  "I_other_211",
				Ref: IssueReference{Owner: "owner", Repo: "other", Number: 211},
			},
			wantErr: "issue #212 is already a sub-issue of owner/other#211; use --replace-parent to move it under #211",
		},
		{
			name:      "requested parent in another repository",
			sub:       sub,
			requested: IssueReference{Owner: "owner", Repo: "other", Number: 211},
			current: &issueParent{
				ID:  "I_300",
				Ref: IssueReference{Owner: "owner", Repo: "repo", Number: 300},
			},
			wantErr: "issue #212 is already a sub-issue of #300; use --replace-parent to move it under owner/other#211",
		},
		{
			name:      "sub-issue in another repository",
			sub:       IssueReference{Owner: "owner", Repo: "other", Number: 212},
			requested: requested,
			current: &issueParent{
				ID:  "I_other_300",
				Ref: IssueReference{Owner: "owner", Repo: "other", Number: 300},
			},
			wantErr: "issue owner/other#212 is already a sub-issue of owner/other#300; use --replace-parent to move it under #211",
		},
		{
			name:          "under another parent, with replace",
			sub:           sub,
			requested:     requested,
			replaceParent: true,
			current: &issueParent{
				ID:  "I_300",
				Ref: IssueReference{Owner: "owner", Repo: "repo", Number: 300},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done, err := checkCurrentParent(tt.sub, tt.requested, requestedID, tt.current, tt.replaceParent, base)
			if done != tt.wantDone {
				t.Errorf("done: got %v, want %v", done, tt.wantDone)
			}
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("got error %q, want none", err.Error())
				}
				return
			}
			if err == nil {
				t.Fatalf("got no error, want %q", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Errorf("got %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestAlreadyMessage(t *testing.T) {
	base := IssueReference{Owner: "owner", Repo: "repo"}
	parent := IssueReference{Owner: "owner", Repo: "repo", Number: 211}

	tests := []struct {
		name string
		sub  IssueReference
		want string
	}{
		{
			name: "sub-issue in the same repository",
			sub:  IssueReference{Owner: "owner", Repo: "repo", Number: 212},
			want: "✓ Issue #212 is already a sub-issue of #211",
		},
		{
			name: "sub-issue in another repository",
			sub:  IssueReference{Owner: "owner", Repo: "other", Number: 212},
			want: "✓ Issue owner/other#212 is already a sub-issue of #211",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := alreadyMessage(tt.sub, parent, base); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAddedMessage(t *testing.T) {
	base := IssueReference{Owner: "owner", Repo: "repo"}
	sub := IssueReference{Owner: "owner", Repo: "repo", Number: 212}

	tests := []struct {
		name     string
		sub      IssueReference
		parent   IssueReference
		previous *IssueReference
		want     string
	}{
		{
			name:   "added, parent in the same repository",
			sub:    sub,
			parent: IssueReference{Owner: "owner", Repo: "repo", Number: 211},
			want:   "✓ Added issue #212 as a sub-issue of #211",
		},
		{
			name:   "added, parent in another repository",
			sub:    sub,
			parent: IssueReference{Owner: "owner", Repo: "other", Number: 211},
			want:   "✓ Added issue #212 as a sub-issue of owner/other#211",
		},
		{
			name:   "added, sub-issue in another repository",
			sub:    IssueReference{Owner: "owner", Repo: "other", Number: 212},
			parent: IssueReference{Owner: "owner", Repo: "repo", Number: 211},
			want:   "✓ Added issue owner/other#212 as a sub-issue of #211",
		},
		{
			name:     "moved within the same repository",
			sub:      sub,
			parent:   IssueReference{Owner: "owner", Repo: "repo", Number: 211},
			previous: &IssueReference{Owner: "owner", Repo: "repo", Number: 300},
			want:     "✓ Moved issue #212 from #300 to #211",
		},
		{
			name:     "moved from a parent in another repository",
			sub:      sub,
			parent:   IssueReference{Owner: "owner", Repo: "repo", Number: 211},
			previous: &IssueReference{Owner: "owner", Repo: "other", Number: 300},
			want:     "✓ Moved issue #212 from owner/other#300 to #211",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := addedMessage(tt.sub, tt.parent, tt.previous, base); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReplaceParentFlag(t *testing.T) {
	flag := addCmd.Flags().Lookup("replace-parent")
	if flag == nil {
		t.Fatal("add has no --replace-parent flag")
	}
	if flag.Value.Type() != "bool" || flag.DefValue != "false" {
		t.Errorf("got a %s flag defaulting to %q, want a bool defaulting to \"false\"",
			flag.Value.Type(), flag.DefValue)
	}

	defer func() { _ = flag.Value.Set("false") }()
	if err := flag.Value.Set("true"); err != nil {
		t.Fatalf("setting --replace-parent: %v", err)
	}
	if !replaceParentFlag {
		t.Error("--replace-parent does not set replaceParentFlag")
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsString(s[1:], substr) || len(substr) > 0 && s[:len(substr)] == substr)
}