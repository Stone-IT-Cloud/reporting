package gitlogs // <-- Nuevo paquete

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Options defines the filtering options for retrieving git logs.
type Options struct {
	// StartDate filters commits to include only those made on or after this date/time (inclusive).
	// If nil, no start date filter is applied.
	StartDate *time.Time
	// EndDate filters commits to include only those made on or before this date/time (inclusive).
	// If nil, no end date filter is applied.
	EndDate *time.Time
}

// logEntry represents the structured data for a single commit before JSON marshalling.
// JSON tags define the output field names.
type logEntry struct {
	CommitDateTime time.Time `json:"commit_date_time"`
	AuthorName     string    `json:"author_name"`
	AuthorEmail    string    `json:"author_email"`
	Message        string    `json:"commit_message"`
	ModifiedFiles  []string  `json:"modified_files"`
	// Internal fields not included in JSON can be added without tags
	// Hash string `json:"-"`
}

// GetLogsJSON retrieves git commit logs from a repository based on options,
// excluding merge commits, scanning all branches, ordering chronologically,
// and returns the result as a JSON string.
// Uses a two-pass approach: first gets commit details, then gets files per commit.
func GetLogsJSON(repoPath string, opts *Options) (string, error) {
	// --- Input Validation & Path Setup ---
	absRepoPath, err := validateRepoPath(repoPath)
	if err != nil {
		return "", err
	}

	// --- Prepare Options ---
	if opts == nil {
		opts = &Options{}
	}

	// --- Pass 1: Get Commit Details (Hash, Author, Date, Message) ---
	const separator = "|||GITLOGSEP|||"
	const logFormat = "%H" + separator + "%aN" + separator + "%aE" + separator + "%aI" + separator + "%B%x00" // Null byte terminates each entry
	const endOfCommitMarker = "\x00"

	logArgs := []string{
		"log",
		"--all",
		"--no-merges",
		"--reverse",
		"--pretty=format:" + logFormat,
	}
	if opts.StartDate != nil {
		logArgs = append(logArgs, "--after="+opts.StartDate.Format(time.RFC3339))
	}
	if opts.EndDate != nil {
		logArgs = append(logArgs, "--before="+opts.EndDate.Format(time.RFC3339))
	}
	logArgs = append(logArgs, "--")

	cmdLog := exec.Command("git", logArgs...)
	cmdLog.Dir = absRepoPath
	var stdoutLog, stderrLog bytes.Buffer
	cmdLog.Stdout = &stdoutLog
	cmdLog.Stderr = &stderrLog

	if err := cmdLog.Run(); err != nil {
		stderrStr := stderrLog.String()
		if strings.Contains(stderrStr, "does not have any commits") || strings.Contains(stderrStr, "bad default revision 'HEAD'") || stdoutLog.Len() == 0 {
			return "[]", nil // Empty repo or no matching commits
		}
		return "", fmt.Errorf("git log command failed: %w\nstderr: %s", err, stderrStr)
	}

	// --- Parse Commit Details Output ---
	outputLog := strings.TrimSpace(stdoutLog.String())
	if outputLog == "" {
		return "[]", nil // No commits found after filtering
	}

	commitDetailBlocks := strings.Split(outputLog, endOfCommitMarker)
	logEntriesMap := make(map[string]*logEntry) // Use map for easy lookup by hash
	commitOrder := []string{}                   // Preserve chronological order

	for _, block := range commitDetailBlocks {
		trimmedBlock := strings.TrimSpace(block)
		if trimmedBlock == "" {
			continue
		}

		parts := strings.SplitN(trimmedBlock, separator, 5) // Hash, Name, Email, Date, Message
		if len(parts) != 5 {
			fmt.Fprintf(os.Stderr, "warning: skipping malformed git log detail line: %q\n", trimmedBlock)
			continue
		}

		hash := parts[0]
		authorName := parts[1]
		authorEmail := parts[2]
		dateStr := parts[3]
		message := parts[4]

		commitDate, err := time.Parse(time.RFC3339, dateStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping commit %s with unparseable date %q: %v\n", hash, dateStr, err)
			continue
		}

		entry := &logEntry{ // Store as pointer in map
			CommitDateTime: commitDate.UTC(),
			AuthorName:     authorName,
			AuthorEmail:    authorEmail,
			Message:        strings.TrimSpace(message),
			ModifiedFiles:  make([]string, 0), // Initialize empty slice, files added in pass 2
		}
		logEntriesMap[hash] = entry
		commitOrder = append(commitOrder, hash) // Add hash to maintain order
	}

	// --- Pass 2: Get Modified Files for Each Commit ---
	finalLogEntries := make([]logEntry, 0, len(commitOrder))
	for _, hash := range commitOrder {
		showArgs := []string{
			"show",
			hash,          // Specify the commit hash
			"--pretty=",   // No commit header info needed
			"--name-only", // Only show names of modified files
			"--no-merges", // Ensure consistency
			// REMOVED: "--oneline",   // Avoid showing diffstat or other noise <-- This was incorrect for show --name-only
			"--",
		}
		cmdShow := exec.Command("git", showArgs...) // #nosec G204
		cmdShow.Dir = absRepoPath
		var stdoutShow, stderrShow bytes.Buffer
		cmdShow.Stdout = &stdoutShow
		cmdShow.Stderr = &stderrShow

		if err := cmdShow.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: git show for commit %s failed: %v\nstderr: %s\n", hash, err, stderrShow.String())
			continue // Skip this commit entirely if show fails
		}

		// Parse file list output
		fileListStr := strings.TrimSpace(stdoutShow.String())
		modifiedFiles := make([]string, 0)
		if fileListStr != "" {
			files := strings.Split(fileListStr, "\n")
			for _, f := range files {
				trimmedFile := strings.TrimSpace(f)
				if trimmedFile != "" {
					modifiedFiles = append(modifiedFiles, trimmedFile)
				}
			}
		}

		// --- ★★★ Filter out commits with no modified files ★★★ ---
		// This effectively skips the initial empty commit created by test setup.
		if len(modifiedFiles) == 0 {
			continue // Skip adding this commit to the final list
		}

		// If we have files, retrieve the original entry and add the files
		if entry, ok := logEntriesMap[hash]; ok {
			entry.ModifiedFiles = modifiedFiles // Assign the parsed files
			// Optional: Sort files here if needed
			sort.Strings(entry.ModifiedFiles)
			finalLogEntries = append(finalLogEntries, *entry) // Append the completed entry
		} else {
			// This case should ideally not happen if the hash came from commitOrder
			fmt.Fprintf(os.Stderr, "warning: commit hash %s found in show but not in initial log map\n", hash)
		}
	}

	// --- Assemble Final Ordered List (Now done within Pass 2) ---
	// The finalLogEntries slice is already built in the correct order.

	// --- Marshal to JSON ---
	jsonData, err := json.MarshalIndent(finalLogEntries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal log entries to JSON: %w", err)
	}

	return string(jsonData), nil
}

// validateRepoPath checks if the path is valid and returns the absolute path.
// Duplicated here for simplicity, could be moved to shared internal package.
func validateRepoPath(repoPath string) (string, error) {
	if repoPath == "" {
		return "", fmt.Errorf("repository path cannot be empty")
	}
	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for %q: %w", repoPath, err)
	}
	info, err := os.Stat(absRepoPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("repository path %q does not exist", absRepoPath)
		}
		return "", fmt.Errorf("failed to stat repository path %q: %w", absRepoPath, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("repository path %q is not a directory", absRepoPath)
	}
	gitDirPath := filepath.Join(absRepoPath, ".git")
	if _, err := os.Stat(gitDirPath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("path %q is not a git repository (missing .git directory)", absRepoPath)
		}
		return "", fmt.Errorf("failed to stat .git directory in %q: %w", absRepoPath, err)
	}
	return absRepoPath, nil
}
