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