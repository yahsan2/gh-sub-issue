package cmd

import (
	"encoding/json"
	"testing"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string",
			input:    "Hello",
			maxLen:   10,
			expected: "Hello",
		},
		{
			name:     "exact length",
			input:    "Hello World",
			maxLen:   11,
			expected: "Hello World",
		},
		{
			name:     "long string",
			input:    "This is a very long string that needs truncation",
			maxLen:   20,
			expected: "This is a very lo...",
		},
		{
			name:     "unicode string",
			input:    "Hello 世界 World",
			maxLen:   10,
			expected: "Hello 世...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestFormatPlain(t *testing.T) {
	result := &ListResult{
		Parent: ParentIssue{
			Number: 1,
			Title:  "Parent Issue",
			State:  "open",
		},
		SubIssues: []SubIssue{
			{
				Number:    2,
				Title:     "First sub-issue",
				State:     "open",
				URL:       "https://github.com/owner/repo/issues/2",
				Assignees: []string{"user1", "user2"},
			},
			{
				Number:    3,
				Title:     "Second sub-issue",
				State:     "closed",
				URL:       "https://github.com/owner/repo/issues/3",
				Assignees: []string{},
			},
		},
		Total:     2,
		OpenCount: 1,
	}

	expected := "2\topen\tFirst sub-issue\tuser1,user2\n3\tclosed\tSecond sub-issue\t\n"
	output := formatPlain(result)
	
	if output != expected {
		t.Errorf("formatPlain() output mismatch\nGot:\n%s\nExpected:\n%s", output, expected)
	}
}

func TestFormatJSON(t *testing.T) {
	result := &ListResult{
		Parent: ParentIssue{
			Number: 1,
			Title:  "Parent Issue",
			State:  "open",
		},
		SubIssues: []SubIssue{
			{
				Number:    2,
				Title:     "Sub-issue",
				State:     "open",
				URL:       "https://github.com/owner/repo/issues/2",
				Assignees: []string{"user1"},
			},
		},
		Total:     1,
		OpenCount: 1,
	}

	output, err := formatJSON(result)
	if err != nil {
		t.Fatalf("formatJSON() error = %v", err)
	}

	// Parse the output to verify it's valid JSON
	var parsed ListResult
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("formatJSON() produced invalid JSON: %v", err)
	}

	// Verify the parsed data matches the original
	if parsed.Parent.Number != result.Parent.Number {
		t.Errorf("JSON parent number = %d, want %d", parsed.Parent.Number, result.Parent.Number)
	}
	if len(parsed.SubIssues) != len(result.SubIssues) {
		t.Errorf("JSON sub-issues count = %d, want %d", len(parsed.SubIssues), len(result.SubIssues))
	}
}

func TestFormatTTY(t *testing.T) {
	tests := []struct {
		name     string
		result   *ListResult
		contains []string
	}{
		{
			name: "with sub-issues",
			result: &ListResult{
				Parent: ParentIssue{
					Number: 1,
					Title:  "Parent Issue",
					State:  "open",
				},
				SubIssues: []SubIssue{
					{
						Number: 2,
						Title:  "Open sub-issue",
						State:  "open",
					},
					{
						Number: 3,
						Title:  "Closed sub-issue",
						State:  "closed",
					},
				},
				Total:     2,
				OpenCount: 1,
			},
			contains: []string{
				"Parent: #1 - Parent Issue",
				"SUB-ISSUES (2 total, 1 open, 1 closed)",
				"🔵 #2",
				"✅ #3",
			},
		},
		{
			name: "no sub-issues",
			result: &ListResult{
				Parent: ParentIssue{
					Number: 10,
					Title:  "Lonely Issue",
					State:  "open",
				},
				SubIssues: []SubIssue{},
				Total:     0,
				OpenCount: 0,
			},
			contains: []string{
				"Parent: #10 - Lonely Issue",
				"No sub-issues found",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := formatTTY(tt.result)
			for _, expected := range tt.contains {
				if !containsString(output, expected) {
					t.Errorf("formatTTY() output missing expected string: %q\nFull output:\n%s", expected, output)
				}
			}
		})
	}
}

func TestListCommandFlags(t *testing.T) {
	// Test that the parent flag exists and is properly defined
	parentFlag := listCmd.Flags().Lookup("parent")
	if parentFlag == nil {
		t.Errorf("--parent flag not found")
	}
	if parentFlag.Usage != "Show parent issue instead of sub-issues" {
		t.Errorf("--parent flag usage incorrect, got: %s", parentFlag.Usage)
	}
	
	// Test that the flag is a boolean flag
	if parentFlag.Value.Type() != "bool" {
		t.Errorf("--parent flag should be boolean, got: %s", parentFlag.Value.Type())
	}
}

func TestFormatTTYParent(t *testing.T) {
	result := &ListResult{
		Parent: ParentIssue{
			Number: 123,
			Title:  "Main Feature Implementation", 
			State:  "open",
		},
		SubIssues: []SubIssue{}, // Empty for parent display
		Total:     0,
		OpenCount: 0,
	}

	output := formatTTYParent(result)
	
	expectedContents := []string{
		"Parent Issue: #123",
		"Main Feature Implementation",
		"[open]",
	}
	
	for _, expected := range expectedContents {
		if !containsString(output, expected) {
			t.Errorf("formatTTYParent() output missing expected string: %q\nFull output:\n%s", expected, output)
		}
	}
}

func TestFormatJSONWithFields(t *testing.T) {
	result := &ListResult{
		Parent: ParentIssue{
			Number: 1,
			Title:  "Parent Issue",
			State:  "open",
		},
		SubIssues: []SubIssue{
			{
				Number:    2,
				Title:     "First sub-issue",
				State:     "open",
				URL:       "https://github.com/owner/repo/issues/2",
				Assignees: []string{"user1", "user2"},
			},
			{
				Number:    3,
				Title:     "Second sub-issue",
				State:     "closed",
				URL:       "https://github.com/owner/repo/issues/3",
				Assignees: []string{},
			},
		},
		Total:     2,
		OpenCount: 1,
	}

	tests := []struct {
		name      string
		fields    []string
		wantError bool
		checkFunc func(t *testing.T, output string)
	}{
		{
			name:   "number and title only",
			fields: []string{"number", "title"},
			checkFunc: func(t *testing.T, output string) {
				var parsed map[string]interface{}
				if err := json.Unmarshal([]byte(output), &parsed); err != nil {
					t.Fatalf("formatJSONWithFields() produced invalid JSON: %v", err)
				}
				
				subIssues, ok := parsed["subIssues"].([]interface{})
				if !ok || len(subIssues) != 2 {
					t.Errorf("Expected 2 sub-issues, got %v", subIssues)
					return
				}
				
				firstIssue := subIssues[0].(map[string]interface{})
				if firstIssue["number"].(float64) != 2 {
					t.Errorf("Expected number 2, got %v", firstIssue["number"])
				}
				if firstIssue["title"].(string) != "First sub-issue" {
					t.Errorf("Expected title 'First sub-issue', got %v", firstIssue["title"])
				}
				if _, hasState := firstIssue["state"]; hasState {
					t.Errorf("Expected no state field, but found one")
				}
				if _, hasURL := firstIssue["url"]; hasURL {
					t.Errorf("Expected no url field, but found one")
				}
			},
		},
		{
			name:   "parent fields only",
			fields: []string{"parent.number", "parent.title"},
			checkFunc: func(t *testing.T, output string) {
				var parsed map[string]interface{}
				if err := json.Unmarshal([]byte(output), &parsed); err != nil {
					t.Fatalf("formatJSONWithFields() produced invalid JSON: %v", err)
				}
				
				parent, ok := parsed["parent"].(map[string]interface{})
				if !ok {
					t.Errorf("Expected parent object, got %v", parsed["parent"])
					return
				}
				
				if parent["number"].(float64) != 1 {
					t.Errorf("Expected parent number 1, got %v", parent["number"])
				}
				if parent["title"].(string) != "Parent Issue" {
					t.Errorf("Expected parent title 'Parent Issue', got %v", parent["title"])
				}
				if _, hasState := parent["state"]; hasState {
					t.Errorf("Expected no parent state field, but found one")
				}
				if _, hasSubIssues := parsed["subIssues"]; hasSubIssues {
					t.Errorf("Expected no subIssues field, but found one")
				}
			},
		},
		{
			name:   "meta fields only",
			fields: []string{"total", "openCount"},
			checkFunc: func(t *testing.T, output string) {
				var parsed map[string]interface{}
				if err := json.Unmarshal([]byte(output), &parsed); err != nil {
					t.Fatalf("formatJSONWithFields() produced invalid JSON: %v", err)
				}
				
				if parsed["total"].(float64) != 2 {
					t.Errorf("Expected total 2, got %v", parsed["total"])
				}
				if parsed["openCount"].(float64) != 1 {
					t.Errorf("Expected openCount 1, got %v", parsed["openCount"])
				}
				if _, hasSubIssues := parsed["subIssues"]; hasSubIssues {
					t.Errorf("Expected no subIssues field, but found one")
				}
			},
		},
		{
			name:   "mixed fields",
			fields: []string{"number", "state", "parent.number", "total"},
			checkFunc: func(t *testing.T, output string) {
				var parsed map[string]interface{}
				if err := json.Unmarshal([]byte(output), &parsed); err != nil {
					t.Fatalf("formatJSONWithFields() produced invalid JSON: %v", err)
				}
				
				// Check sub-issues have only number and state
				subIssues := parsed["subIssues"].([]interface{})
				firstIssue := subIssues[0].(map[string]interface{})
				if firstIssue["number"].(float64) != 2 {
					t.Errorf("Expected number 2, got %v", firstIssue["number"])
				}
				if firstIssue["state"].(string) != "open" {
					t.Errorf("Expected state 'open', got %v", firstIssue["state"])
				}
				if _, hasTitle := firstIssue["title"]; hasTitle {
					t.Errorf("Expected no title field, but found one")
				}
				
				// Check parent has only number
				parent := parsed["parent"].(map[string]interface{})
				if parent["number"].(float64) != 1 {
					t.Errorf("Expected parent number 1, got %v", parent["number"])
				}
				
				// Check total
				if parsed["total"].(float64) != 2 {
					t.Errorf("Expected total 2, got %v", parsed["total"])
				}
			},
		},
		{
			name:      "invalid field",
			fields:    []string{"invalid"},
			wantError: true,
		},
		{
			name:      "mixed valid and invalid fields",
			fields:    []string{"number", "invalid"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := formatJSONWithFields(result, tt.fields)
			
			if tt.wantError {
				if err == nil {
					t.Errorf("formatJSONWithFields() expected error, but got none")
				}
				return
			}
			
			if err != nil {
				t.Fatalf("formatJSONWithFields() unexpected error: %v", err)
			}
			
			if tt.checkFunc != nil {
				tt.checkFunc(t, output)
			}
		})
	}
}