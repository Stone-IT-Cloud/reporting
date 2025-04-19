package gitlogs_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	// --- ★★★ IMPORTANTE: Usar la ruta correcta al paquete probado ★★★ ---
	"github.com/Stone-IT-Cloud/reporting/pkg/gitlogs" // Ajusta si tu ruta de módulo es diferente
)

// --- Test Suite Setup ---

// Constants for test authors (pueden ser los mismos que en contributors_test)
const (
	author1Name  = "Alice Alpha"
	author1Email = "alice@example.com"
	author2Name  = "Bob Bravo"
	author2Email = "bob@example.com"
	mergerName   = "Core Maintainer" // Alguien que haga merges
	mergerEmail  = "core@example.com"
)

// expectedLogEntry defines the structure we expect after unmarshalling the JSON result.
// Used for comparison in tests. Field names match JSON tags in gitlogs.logEntry.
type expectedLogEntry struct {
	CommitDateTime string   `json:"commit_date_time"` // Compare as RFC3339 string
	AuthorName     string   `json:"author_name"`
	AuthorEmail    string   `json:"author_email"`
	Message        string   `json:"commit_message"`
	ModifiedFiles  []string `json:"modified_files"` // Expect strings directly
}

// --- Test Helpers (Idénticos a los de contributors_test) ---

func setupGitRepo(t *testing.T) string {
	t.Helper()
	repoPath := t.TempDir()
	runGitCommand(t, repoPath, "init", "-b", "main")
	runGitCommand(t, repoPath, "config", "user.name", "Test User")
	runGitCommand(t, repoPath, "config", "user.email", "test@example.com")
	// Initial commit needed for some operations like checking out branches
	runGitCommand(t, repoPath, "commit", "--allow-empty", "-m", "Initial commit")
	return repoPath
}

func runGitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git command failed (args: %v): %v\nOutput:\n%s", args, err, string(output))
	}
}

func gitCommit(t *testing.T, repoPath, message, authorName, authorEmail string, commitDate time.Time, files map[string]string) {
	t.Helper()

	// Write/modify specified files
	for file, content := range files {
		filePath := filepath.Join(repoPath, file)
		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			t.Fatalf("Failed to create directory for file %s: %v", file, err)
		}
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("Failed to write file %s for commit: %v", file, err)
		}
		runGitCommand(t, repoPath, "add", file) // Add specific file
	}

	// If no files specified, create a dummy file to ensure commit isn't empty
	if len(files) == 0 {
		dummyFile := filepath.Join(repoPath, fmt.Sprintf("dummy-%d.txt", time.Now().UnixNano()))
		content := fmt.Sprintf("%s\n%s\n%s", message, authorName, commitDate.String())
		if err := os.WriteFile(dummyFile, []byte(content), 0o644); err != nil {
			t.Fatalf("Failed to write dummy file for commit: %v", err)
		}
		runGitCommand(t, repoPath, "add", dummyFile)
	}

	// Use environment variables to set author and date precisely
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoPath
	isoDate := commitDate.Format(time.RFC3339) // Git log %aI format matches RFC3339

	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME="+authorName,
		"GIT_AUTHOR_EMAIL="+authorEmail,
		"GIT_AUTHOR_DATE="+isoDate,
		"GIT_COMMITTER_NAME="+authorName, // Keep committer same as author for test simplicity
		"GIT_COMMITTER_EMAIL="+authorEmail,
		"GIT_COMMITTER_DATE="+isoDate,
	)

	// Run commit
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Ignore "nothing to commit" which might happen in race conditions or complex setups
		if strings.Contains(string(output), "nothing to commit") {
			t.Logf("Ignoring 'nothing to commit' for commit: %s", message)
			return
		}
		t.Fatalf("git commit failed for %q: %v\nOutput: %s", message, err, string(output))
	}
}

// Helper to make times easier to define in tests (UTC)
func testTime(year int, month time.Month, day, hour, min, sec int) time.Time {
	return time.Date(year, month, day, hour, min, sec, 0, time.UTC)
}

// Helper to easily create a pointer to a time.Time object
func PtrTime(t time.Time) *time.Time {
	return &t
}

// Helper to sort file lists within expectedLogEntry for consistent comparison
func sortFiles(data []expectedLogEntry) {
	for i := range data {
		if data[i].ModifiedFiles != nil {
			sort.Strings(data[i].ModifiedFiles)
		}
	}
}

// --- Test Cases ---

func TestGetLogsJSON(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name                string
		setupRepo           func(t *testing.T, repoPath string) // Function to set up repo state
		repoPathOverride    string                              // Use this path instead of setting up a repo (for error cases)
		opts                *gitlogs.Options                    // Options for GetLogsJSON
		expectedError       bool
		expectedErrorSubstr string             // Optional: check for specific error text
		expectedData        []expectedLogEntry // Expected Go struct after unmarshalling JSON
	}{
		// --- Error Cases ---
		{
			name:             "Error: Empty path",
			repoPathOverride: "",
			expectedError:    true,
			expectedData:     nil,
		},
		{
			name:             "Error: Non-existent path",
			repoPathOverride: filepath.Join(os.TempDir(), "nonexistent-log-path-"+fmt.Sprintf("%d", time.Now().UnixNano())),
			expectedError:    true,
			expectedData:     nil,
		},
		{
			name:             "Error: Not a git repository",
			repoPathOverride: t.TempDir(), // Provide a plain temp directory
			expectedError:    true,
			expectedData:     nil,
		},

		// --- Success Cases ---
		{
			name: "Success: Empty repository (after initial commit)",
			setupRepo: func(t *testing.T, repoPath string) {
				// setupGitRepo makes one initial commit. GetLogs excludes it by default if it's a merge/empty? Let's test.
				// The initial empty commit usually doesn't list files with --name-only.
				// Let's add one actual commit to be sure.
				gitCommit(t, repoPath, "First Real Commit", author1Name, author1Email, testTime(2023, 1, 1, 10, 0, 0), map[string]string{"file1.txt": "a"})
				runGitCommand(t, repoPath, "rm", "file1.txt")                                                         // Remove file so repo seems empty for next test
				gitCommit(t, repoPath, "Remove file", author1Name, author1Email, testTime(2023, 1, 1, 11, 0, 0), nil) // This commit might not show if only file1.txt existed

				// Test scenario: Call GetLogsJSON for a range *after* all commits.
			},
			opts:          &gitlogs.Options{StartDate: PtrTime(testTime(2024, 1, 1, 0, 0, 0))},
			expectedData:  []expectedLogEntry{}, // Expect empty JSON array "[]" -> empty slice
			expectedError: false,
		},
		{
			name: "Success: Empty repository (no commits match filter)",
			setupRepo: func(t *testing.T, repoPath string) {
				gitCommit(t, repoPath, "Commit 1", author1Name, author1Email, testTime(2023, 1, 1, 10, 0, 0), map[string]string{"file1.txt": "a"})
			},
			opts:          &gitlogs.Options{StartDate: PtrTime(testTime(2024, 1, 1, 0, 0, 0))}, // Filter excludes the commit
			expectedData:  []expectedLogEntry{},                                                // Expect empty JSON array "[]" -> empty slice
			expectedError: false,
		},
		{
			name: "Success: Single commit",
			setupRepo: func(t *testing.T, repoPath string) {
				gitCommit(t, repoPath, "feat: Initial feature\n\nAdds the first feature.", author1Name, author1Email, testTime(2023, 1, 15, 14, 30, 0), map[string]string{"feature.txt": "content", "main.go": "package main"})
			},
			opts: nil, // No filters
			expectedData: []expectedLogEntry{
				{
					CommitDateTime: testTime(2023, 1, 15, 14, 30, 0).Format(time.RFC3339),
					AuthorName:     author1Name,
					AuthorEmail:    author1Email,
					Message:        "feat: Initial feature\n\nAdds the first feature.",
					ModifiedFiles:  []string{"feature.txt", "main.go"}, // Expect sorted by helper
				},
			},
			expectedError: false,
		},
		{
			name: "Success: Multiple commits, chronological order",
			setupRepo: func(t *testing.T, repoPath string) {
				// Commit oldest first
				gitCommit(t, repoPath, "Commit 1", author1Name, author1Email, testTime(2023, 2, 10, 9, 0, 0), map[string]string{"file_a.txt": "a"})
				gitCommit(t, repoPath, "Commit 2", author2Name, author2Email, testTime(2023, 2, 12, 11, 0, 0), map[string]string{"file_b.txt": "b", "file_a.txt": "a updated"})
			},
			opts: nil,
			expectedData: []expectedLogEntry{
				{ // Oldest
					CommitDateTime: testTime(2023, 2, 10, 9, 0, 0).Format(time.RFC3339),
					AuthorName:     author1Name, AuthorEmail: author1Email, Message: "Commit 1", ModifiedFiles: []string{"file_a.txt"},
				},
				{ // Newest
					CommitDateTime: testTime(2023, 2, 12, 11, 0, 0).Format(time.RFC3339),
					AuthorName:     author2Name, AuthorEmail: author2Email, Message: "Commit 2", ModifiedFiles: []string{"file_a.txt", "file_b.txt"},
				},
			},
			expectedError: false,
		},
		{
			name: "Success: Date Filter Applied",
			setupRepo: func(t *testing.T, repoPath string) {
				gitCommit(t, repoPath, "Commit Before", author1Name, author1Email, testTime(2023, 3, 1, 10, 0, 0), map[string]string{"f1": "1"})
				gitCommit(t, repoPath, "Commit During", author2Name, author2Email, testTime(2023, 3, 15, 12, 0, 0), map[string]string{"f2": "2"}) // This one should be included
				gitCommit(t, repoPath, "Commit After", author1Name, author1Email, testTime(2023, 3, 30, 14, 0, 0), map[string]string{"f3": "3"})
			},
			opts: &gitlogs.Options{
				StartDate: PtrTime(testTime(2023, 3, 10, 0, 0, 0)),    // From Mar 10
				EndDate:   PtrTime(testTime(2023, 3, 20, 23, 59, 59)), // Until Mar 20 EOD
			},
			expectedData: []expectedLogEntry{
				{ // Only the middle commit
					CommitDateTime: testTime(2023, 3, 15, 12, 0, 0).Format(time.RFC3339),
					AuthorName:     author2Name, AuthorEmail: author2Email, Message: "Commit During", ModifiedFiles: []string{"f2"},
				},
			},
			expectedError: false,
		},
		{
			name: "Success: Merge commit excluded",
			setupRepo: func(t *testing.T, repoPath string) {
				// main: C1(A)
				// branch feat: C2(B)
				// main: Merge feat -> C3(Merger) - Should be excluded
				// main: C4(A) - Should be included
				gitCommit(t, repoPath, "C1 main", author1Name, author1Email, testTime(2023, 4, 1, 10, 0, 0), map[string]string{"main.txt": "m1"}) // C1
				runGitCommand(t, repoPath, "checkout", "-b", "feat")
				gitCommit(t, repoPath, "C2 feat", author2Name, author2Email, testTime(2023, 4, 2, 11, 0, 0), map[string]string{"feat.txt": "f1"}) // C2
				runGitCommand(t, repoPath, "checkout", "main")
				// Create merge commit explicitly setting committer/author
				mergeDate := testTime(2023, 4, 3, 12, 0, 0)
				cmd := exec.Command("git", "merge", "--no-ff", "-m", "Merge branch 'feat'", "feat")
				cmd.Dir = repoPath
				cmd.Env = append(os.Environ(),
					"GIT_AUTHOR_NAME="+mergerName, "GIT_AUTHOR_EMAIL="+mergerEmail, "GIT_AUTHOR_DATE="+mergeDate.Format(time.RFC3339),
					"GIT_COMMITTER_NAME="+mergerName, "GIT_COMMITTER_EMAIL="+mergerEmail, "GIT_COMMITTER_DATE="+mergeDate.Format(time.RFC3339),
				)
				output, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("git merge failed: %v\nOutput: %s", err, string(output))
				}
				// Commit after merge
				gitCommit(t, repoPath, "C4 main", author1Name, author1Email, testTime(2023, 4, 4, 13, 0, 0), map[string]string{"main.txt": "m2"}) // C4
			},
			opts: nil, // No filters, relies on --no-merges default in GetLogsJSON
			expectedData: []expectedLogEntry{
				{ // C1
					CommitDateTime: testTime(2023, 4, 1, 10, 0, 0).Format(time.RFC3339),
					AuthorName:     author1Name, AuthorEmail: author1Email, Message: "C1 main", ModifiedFiles: []string{"main.txt"},
				},
				{ // C2 (from feat branch, included due to --all)
					CommitDateTime: testTime(2023, 4, 2, 11, 0, 0).Format(time.RFC3339),
					AuthorName:     author2Name, AuthorEmail: author2Email, Message: "C2 feat", ModifiedFiles: []string{"feat.txt"},
				},
				// Merge commit C3 is SKIPPED
				{ // C4
					CommitDateTime: testTime(2023, 4, 4, 13, 0, 0).Format(time.RFC3339),
					AuthorName:     author1Name, AuthorEmail: author1Email, Message: "C4 main", ModifiedFiles: []string{"main.txt"},
				},
			},
			expectedError: false,
		},
		{
			name: "Success: All branches included",
			setupRepo: func(t *testing.T, repoPath string) {
				gitCommit(t, repoPath, "Commit main", author1Name, author1Email, testTime(2023, 5, 1, 10, 0, 0), map[string]string{"main.txt": "m1"})
				runGitCommand(t, repoPath, "checkout", "-b", "develop")
				gitCommit(t, repoPath, "Commit develop", author2Name, author2Email, testTime(2023, 5, 5, 11, 0, 0), map[string]string{"dev.txt": "d1"})
				runGitCommand(t, repoPath, "checkout", "main") // Go back to main, log should still find develop commit
			},
			opts: nil,
			expectedData: []expectedLogEntry{
				{ // Commit main (oldest)
					CommitDateTime: testTime(2023, 5, 1, 10, 0, 0).Format(time.RFC3339),
					AuthorName:     author1Name, AuthorEmail: author1Email, Message: "Commit main", ModifiedFiles: []string{"main.txt"},
				},
				{ // Commit develop
					CommitDateTime: testTime(2023, 5, 5, 11, 0, 0).Format(time.RFC3339),
					AuthorName:     author2Name, AuthorEmail: author2Email, Message: "Commit develop", ModifiedFiles: []string{"dev.txt"},
				},
			},
			expectedError: false,
		},
	}

	// --- Run Test Cases ---
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repoPath := tc.repoPathOverride
			if tc.setupRepo != nil && repoPath == "" {
				repoPath = setupGitRepo(t)
				tc.setupRepo(t, repoPath)
			} else if repoPath == "" && !tc.expectedError {
				t.Fatal("Test case needs either setupRepo or repoPathOverride")
			}

			// Call the function under test
			actualJSONString, err := gitlogs.GetLogsJSON(repoPath, tc.opts)

			// Check for errors
			if tc.expectedError {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				} else if tc.expectedErrorSubstr != "" && !strings.Contains(err.Error(), tc.expectedErrorSubstr) {
					t.Errorf("Expected error containing %q, got: %v", tc.expectedErrorSubstr, err)
				}
				// If error was expected, we might not need to check JSON content further
				return
			}

			// No error expected
			if err != nil {
				t.Fatalf("Expected no error, but got: %v", err)
			}

			// Unmarshal actual JSON to compare structures
			var actualData []expectedLogEntry
			// Use DisallowUnknownFields to catch unexpected fields in JSON? Maybe too strict for tests.
			decoder := json.NewDecoder(strings.NewReader(actualJSONString))
			// decoder.DisallowUnknownFields()
			if err := decoder.Decode(&actualData); err != nil {
				t.Fatalf("Failed to unmarshal actual JSON response: %v\nJSON was:\n%s", err, actualJSONString)
			}

			// Special handling for nil vs empty slice comparison
			isEmptyExpected := len(tc.expectedData) == 0
			isEmptyActual := len(actualData) == 0

			if isEmptyExpected && isEmptyActual {
				// Both are effectively empty, consider it a match for this test purpose
				return
			}
			// Handle case where expected is nil but actual is empty slice (or vice versa) after unmarshal
			if tc.expectedData == nil && isEmptyActual {
				// OK - JSON "[]" unmarshals to empty slice, treat nil expected as matching empty
				return
			}
			if isEmptyExpected && actualData == nil {
				// Unmarshal of "[]" usually yields empty slice, not nil, but handle defensively
				return
			}

			// Sort file lists in both actual and expected data for consistent comparison
			sortFiles(actualData)
			sortFiles(tc.expectedData)

			// Compare the unmarshalled Go structures
			if !reflect.DeepEqual(actualData, tc.expectedData) {
				// Use MarshalIndent for readable diff in test output
				expectedJSONBytes, _ := json.MarshalIndent(tc.expectedData, "", "  ")
				actualJSONBytesUnmarshalled, _ := json.MarshalIndent(actualData, "", "  ") // Re-marshal actual data for consistent view

				t.Errorf("JSON data mismatch (-Expected +Actual):\n--- Expected:\n%s\n--- Actual:\n%s\n--- Raw Actual JSON String:\n%s",
					string(expectedJSONBytes), string(actualJSONBytesUnmarshalled), actualJSONString)

				// Add more detailed field comparison if needed for debugging DeepEqual failures
				if len(actualData) != len(tc.expectedData) {
					t.Errorf("Length mismatch: Expected %d, got %d", len(tc.expectedData), len(actualData))
				} else {
					for i := range tc.expectedData {
						if !reflect.DeepEqual(actualData[i], tc.expectedData[i]) {
							t.Errorf("Mismatch at index %d:\nExpected: %+v\nActual:   %+v", i, tc.expectedData[i], actualData[i])
						}
					}
				}
			}
		})
	}
}
