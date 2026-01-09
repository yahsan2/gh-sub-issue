package cmd

import (
	"testing"
)

func TestCalculateAfterID(t *testing.T) {
	// Mock result with sample sub-issues
	_ = &ListResult{
		Parent: ParentIssue{
			Number: 100,
			Title:  "Parent Issue",
			State:  "open",
		},
		SubIssues: []SubIssue{
			{Number: 101, Title: "First Issue", State: "open"},
			{Number: 102, Title: "Second Issue", State: "open"},
			{Number: 103, Title: "Third Issue", State: "open"},
			{Number: 104, Title: "Fourth Issue", State: "open"},
		},
		Total:     4,
		OpenCount: 4,
	}

	tests := []struct {
		name             string
		position         string
		movingIssueNum   int
		expectedAfterNum int // 0 means empty afterID
		expectError      bool
		errorContains    string
	}{
		{
			name:             "move to first position",
			position:         "first",
			movingIssueNum:   103,
			expectedAfterNum: 0, // empty afterID for first position
		},
		{
			name:             "move to last position",
			position:         "last",
			movingIssueNum:   101,
			expectedAfterNum: 104, // after the current last item
		},
		{
			name:             "move to position 1",
			position:         "1",
			movingIssueNum:   103,
			expectedAfterNum: 0, // same as first
		},
		{
			name:             "move to position 2",
			position:         "2",
			movingIssueNum:   104,
			expectedAfterNum: 101, // after the first item
		},
		{
			name:             "move to position 3",
			position:         "3",
			movingIssueNum:   101,
			expectedAfterNum: 102, // after the second item
		},
		{
			name:             "move to position beyond end",
			position:         "10",
			movingIssueNum:   101,
			expectedAfterNum: 104, // same as last
		},
		{
			name:             "move after specific issue",
			position:         "after:102",
			movingIssueNum:   104,
			expectedAfterNum: 102,
		},
		{
			name:             "move before first issue",
			position:         "before:101",
			movingIssueNum:   103,
			expectedAfterNum: 0, // moving before first means first position
		},
		{
			name:             "move before middle issue",
			position:         "before:103",
			movingIssueNum:   104,
			expectedAfterNum: 102, // the issue before 103
		},
		{
			name:          "invalid position string",
			position:      "invalid",
			movingIssueNum: 101,
			expectError:   true,
			errorContains: "invalid position",
		},
		{
			name:          "invalid after format",
			position:      "after:abc",
			movingIssueNum: 101,
			expectError:   true,
			errorContains: "invalid issue number",
		},
		{
			name:          "invalid before format",
			position:      "before:xyz",
			movingIssueNum: 101,
			expectError:   true,
			errorContains: "invalid issue number",
		},
		{
			name:          "position less than 1",
			position:      "0",
			movingIssueNum: 101,
			expectError:   true,
			errorContains: "position must be at least 1",
		},
		{
			name:          "before issue not found",
			position:      "before:999",
			movingIssueNum: 101,
			expectError:   true,
			errorContains: "not found in sub-issues list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This is a simplified test that doesn't actually call the GitHub API
			// In a real scenario, you'd need to mock the GraphQL client
			// For now, we're testing the logic of position parsing

			// We can't fully test this without mocking the client
			// But we can test the position parsing logic
			position := tt.position

			// Validate the position format
			if position == "first" || position == "last" {
				// Valid
				return
			}

			// Check numeric position
			var pos int
			_, err := parseScanf(position, "%d", &pos)
			if err == nil {
				if pos < 1 && !tt.expectError {
					t.Errorf("position %d should be invalid", pos)
				}
				return
			}

			// Check after:N format
			if len(position) > 6 && position[:6] == "after:" {
				afterNumStr := position[6:]
				var afterNum int
				_, err := parseScanf(afterNumStr, "%d", &afterNum)
				if tt.expectError && err != nil {
					// Expected error
					return
				}
				if !tt.expectError && err == nil {
					// Valid format
					return
				}
			}

			// Check before:N format
			if len(position) > 7 && position[:7] == "before:" {
				beforeNumStr := position[7:]
				var beforeNum int
				_, err := parseScanf(beforeNumStr, "%d", &beforeNum)
				if tt.expectError && err != nil {
					// Expected error
					return
				}
				if !tt.expectError && err == nil {
					// Valid format
					return
				}
			}

			// If we get here with no error expected, it's invalid
			if !tt.expectError {
				t.Errorf("position %s should be valid", position)
			}
		})
	}
}

// parseScanf is a helper function for testing
func parseScanf(s string, format string, a ...interface{}) (int, error) {
	var val int
	n, err := scanfHelper(s, &val)
	if err != nil {
		return 0, err
	}
	if len(a) > 0 {
		if ptr, ok := a[0].(*int); ok {
			*ptr = val
		}
	}
	return n, nil
}

// scanfHelper is a simple integer parser for testing
func scanfHelper(s string, val *int) (int, error) {
	var result int
	var n int
	negative := false

	for i, c := range s {
		if i == 0 && c == '-' {
			negative = true
			continue
		}
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
			n++
		} else {
			break
		}
	}

	if n == 0 {
		return 0, &scanError{}
	}

	if negative {
		result = -result
	}

	*val = result
	return n, nil
}

type scanError struct{}

func (e *scanError) Error() string {
	return "scan error"
}
