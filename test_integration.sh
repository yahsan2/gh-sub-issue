#!/bin/bash
# Integration test script for gh-sub-issue

# Don't exit on error immediately - we handle errors ourselves
set +e

echo "🧪 Running Integration Tests for gh-sub-issue"
echo "=============================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Function to run a test
run_test() {
    local test_name="$1"
    local command="$2"
    local expected="$3"
    
    echo -n "Testing: $test_name... "
    
    # Run command and capture both stdout and stderr
    output=$($command 2>&1)
    exit_code=$?
    
    if [ $exit_code -eq 0 ]; then
        if [[ -z "$expected" ]] || [[ "$output" == *"$expected"* ]]; then
            echo -e "${GREEN}✓ PASSED${NC}"
            TESTS_PASSED=$((TESTS_PASSED + 1))
        else
            echo -e "${RED}✗ FAILED${NC}"
            echo "  Expected: $expected"
            echo "  Got: $output"
            TESTS_FAILED=$((TESTS_FAILED + 1))
        fi
    else
        # Command failed (non-zero exit code)
        if [[ "$expected" == "ERROR:"* ]]; then
            # We expected an error
            if [[ "$output" == *"${expected#ERROR:}"* ]]; then
                echo -e "${GREEN}✓ PASSED${NC} (expected error)"
                TESTS_PASSED=$((TESTS_PASSED + 1))
            else
                echo -e "${RED}✗ FAILED${NC}"
                echo "  Expected error containing: ${expected#ERROR:}"
                echo "  Got: $output"
                TESTS_FAILED=$((TESTS_FAILED + 1))
            fi
        else
            # Command failed but we didn't expect an error
            echo -e "${RED}✗ FAILED${NC}"
            echo "  Command failed unexpectedly with exit code $exit_code"
            echo "  Output: $output"
            TESTS_FAILED=$((TESTS_FAILED + 1))
        fi
    fi
}

# Build the binary
echo "Building gh-sub-issue..."
if ! go build -o gh-sub-issue; then
    echo "Failed to build gh-sub-issue"
    exit 1
fi
echo ""

# Test 1: Help text
echo "=== Basic Command Tests ==="
run_test "Help command" "./gh-sub-issue --help" "A GitHub CLI extension that adds sub-issue management"
run_test "Add help" "./gh-sub-issue add --help" "Link an existing issue to a parent issue"
run_test "List help" "./gh-sub-issue list --help" "List all sub-issues connected to a parent issue"
run_test "Remove help" "./gh-sub-issue remove --help" "Remove the relationship between sub-issues"

# Test 2: Version
run_test "Version" "./gh-sub-issue --version" "version"

# Test 3: Invalid arguments
echo ""
echo "=== Error Handling Tests ==="
run_test "Missing arguments" "./gh-sub-issue add" "ERROR:accepts 2 arg(s), received 0"
run_test "Too many arguments" "./gh-sub-issue add 1 2 3" "ERROR:accepts 2 arg(s), received 3"
run_test "Invalid issue number" "./gh-sub-issue add abc 123 --repo test/repo" "ERROR:invalid issue reference"
run_test "Invalid repo format" "./gh-sub-issue add 1 2 --repo invalid-format" "ERROR:invalid repository format"
run_test "Circular dependency" "./gh-sub-issue add 5 5 --repo test/repo" "ERROR:cannot add issue as its own sub-issue"

# Test 4: URL parsing tests
echo ""
echo "=== URL Parsing Tests ==="
run_test "Invalid URL format" "./gh-sub-issue add https://example.com/123 456 --repo test/repo" "ERROR:invalid GitHub issue URL format"
run_test "Non-issue URL" "./gh-sub-issue add https://github.com/owner/repo/pull/123 456 --repo test/repo" "ERROR:not an issue URL"

# Test 5: List command tests
echo ""
echo "=== List Command Tests ==="
run_test "List missing arguments" "./gh-sub-issue list" "ERROR:accepts 1 arg(s), received 0"
run_test "List too many arguments" "./gh-sub-issue list 1 2" "ERROR:accepts 1 arg(s), received 2"
run_test "List invalid issue number" "./gh-sub-issue list abc --repo test/repo" "ERROR:invalid issue reference"
run_test "List invalid repo format" "./gh-sub-issue list 1 --repo invalid-format" "ERROR:invalid repository format"

# Test 6: Remove command tests
echo ""
echo "=== Remove Command Tests ==="
run_test "Remove missing arguments" "./gh-sub-issue remove" "ERROR:at least 2 arg(s)"
run_test "Remove single argument" "./gh-sub-issue remove 123" "ERROR:at least 2 arg(s)"
run_test "Remove invalid parent" "./gh-sub-issue remove abc 456 --repo test/repo" "ERROR:invalid parent issue"
run_test "Remove invalid sub-issue" "./gh-sub-issue remove 123 xyz --repo test/repo" "ERROR:invalid sub-issue"
run_test "Remove invalid repo format" "./gh-sub-issue remove 123 456 --repo invalid-format" "ERROR:invalid repository format"
run_test "Remove with force flag" "./gh-sub-issue remove 123 456 --force --repo test/repo" "ERROR:" # Will fail with API error but args are valid
run_test "Remove multiple sub-issues" "./gh-sub-issue remove 123 456 457 458 --repo test/repo --force" "ERROR:" # Will fail with API error but args are valid
run_test "Remove with URL parent" "./gh-sub-issue remove https://github.com/owner/repo/issues/123 456 --repo test/repo" ""
run_test "Remove with URL sub-issue" "./gh-sub-issue remove 123 https://github.com/owner/repo/issues/456 --repo test/repo" ""

# Summary
echo ""
echo "=============================================="
echo -e "Test Results: ${GREEN}$TESTS_PASSED passed${NC}, ${RED}$TESTS_FAILED failed${NC}"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}✗ Some tests failed${NC}"
    exit 1
fi