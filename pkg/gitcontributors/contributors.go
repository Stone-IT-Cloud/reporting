package gitcontributors // <-- The package name is now 'gitcontributors'

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Contributor holds aggregated information about a single repository contributor.
type Contributor struct {
	Name            string
	Email           string
	Commits         int       // Number of commits within the specified date range (if any)
	FirstCommitDate time.Time // First commit date within the specified date range (if any)
	LastCommitDate  time.Time // Last commit date within the specified date range (if any)
}

// Options allows configuring the behavior of GetContributors.
type Options struct {
	IncludeMergeCommits bool
	StartDate           *time.Time // Optional: Only count commits on or after this date/time (inclusive).
	EndDate             *time.Time // Optional: Only count commits on or before this date/time (inclusive).
}

// Internal struct to hold aggregated data during processing.
type aggregatedContributorData struct {
	Name            string
	Email           string
	Commits         int
	FirstCommitDate time.Time
	LastCommitDate  time.Time
}

// GetContributors retrieves a list of contributors for a given Git repository path.
// It parses the Git log to aggregate contributor data such as name, email, number of commits,
// and the first and last commit dates.
//
// Parameters:
//   - repoPath: The file system path to the Git repository.
//   - opts: Optional parameters to filter and customize the retrieval of contributors.
//     If nil, default options will be used.
//
// Returns:
//   - A slice of Contributor structs containing aggregated contributor data.
//   - An error if the operation fails.
//
// Behavior:
//   - Filters commits based on the provided options (e.g., date range, inclusion of merge commits).
//   - Aggregates contributor data by name and email, ignoring case.
//   - Skips malformed or unparseable Git log entries.
//   - Returns an empty slice if the repository has no commits.
//
// Errors:
//   - Returns an error if the repository path is invalid or inaccessible.
//   - Returns an error if the Git log command fails for reasons other than an empty repository.
//
// Example:
//
//	contributors, err := GetContributors("/path/to/repo", &Options{
//		StartDate:          timePtr(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)),
//		EndDate:            timePtr(time.Date(2022, 12, 31, 23, 59, 59, 0, time.UTC)),
//		IncludeMergeCommits: false,
//	})
//	if err != nil {
//		log.Fatalf("Failed to get contributors: %v", err)
//	}
//	for _, contributor := range contributors {
//		fmt.Printf("Name: %s, Email: %s, Commits: %d\n", contributor.Name, contributor.Email, contributor.Commits)
//	}
func GetContributors(repoPath string, opts *Options) ([]Contributor, error) {
	// --- Input Validation & Path Setup ---
	absRepoPath, err := validateRepoPath(repoPath)
	if err != nil {
		return nil, err
	}

	// --- Prepare Options ---
	if opts == nil {
		opts = &Options{}
	}

	// --- Execute Git Log Command ---
	const logFormat = "--pretty=format:%aN|%aE|%aI"
	const separator = "|"
	args := []string{"log", logFormat}

	if opts.StartDate != nil {
		args = append(args, "--after="+opts.StartDate.Format(time.RFC3339))
	}
	if opts.EndDate != nil {
		args = append(args, "--before="+opts.EndDate.Format(time.RFC3339))
	}
	if !opts.IncludeMergeCommits {
		args = append(args, "--no-merges")
	}
	args = append(args, "--")

	cmd := exec.Command("git", args...)
	cmd.Dir = absRepoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		if strings.Contains(stderrStr, "does not have any commits") ||
			strings.Contains(stderrStr, "bad default revision 'HEAD'") ||
			stdout.Len() == 0 {
			return []Contributor{}, nil
		}
		return nil, fmt.Errorf("git log command failed (path: %q, args: %v): %w\nstderr: %s",
			absRepoPath, args, err, stderrStr)
	}

	// --- Aggregate Data ---
	contributorsMap := make(map[string]*aggregatedContributorData)
	scanner := bufio.NewScanner(&stdout)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, separator, 3)
		if len(parts) != 3 {
			fmt.Fprintf(os.Stderr, "Warning: malformed git log output line: %q\n", line)
			continue
		}

		name := strings.TrimSpace(parts[0])
		email := strings.TrimSpace(parts[1])
		dateStr := strings.TrimSpace(parts[2])

		if name == "" && email == "" {
			continue
		}

		commitDate, err := time.Parse(time.RFC3339, dateStr)
		if err != nil {
			continue
		} // Skip commits with unparseable dates?

		mapKey := strings.ToLower(fmt.Sprintf("%s<%s>", name, email))
		aggData, exists := contributorsMap[mapKey]
		if !exists {
			aggData = &aggregatedContributorData{
				Name:            name,
				Email:           email,
				Commits:         1,
				FirstCommitDate: commitDate,
				LastCommitDate:  commitDate,
			}
			contributorsMap[mapKey] = aggData
		} else {
			aggData.Commits++
			if aggData.FirstCommitDate.IsZero() || commitDate.Before(aggData.FirstCommitDate) {
				aggData.FirstCommitDate = commitDate
			}
			if aggData.LastCommitDate.IsZero() || commitDate.After(aggData.LastCommitDate) {
				aggData.LastCommitDate = commitDate
			}
			if aggData.Name == "" && name != "" {
				aggData.Name = name
			}
			if aggData.Email == "" && email != "" {
				aggData.Email = email
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading git log output: %w", err)
	}

	// --- Convert Map to Slice ---
	contributors := make([]Contributor, 0, len(contributorsMap))
	for _, data := range contributorsMap {
		if data.FirstCommitDate.IsZero() || data.LastCommitDate.IsZero() {
			continue
		}
		contributors = append(contributors, Contributor{
			Name:            data.Name,
			Email:           data.Email,
			Commits:         data.Commits,
			FirstCommitDate: data.FirstCommitDate.UTC(),
			LastCommitDate:  data.LastCommitDate.UTC(),
		})
	}

	// --- Sorting ---
	sortContributors(contributors)
	return contributors, nil
}

// validateRepoPath validates the provided repository path to ensure it is a valid
// Git repository. It performs the following checks:
//   - The repository path is not empty.
//   - The repository path can be resolved to an absolute path.
//   - The resolved path exists and is a directory.
//   - The directory contains a ".git" subdirectory, indicating it is a Git repository.
//
// Parameters:
//   - repoPath: The path to the repository to validate.
//
// Returns:
//   - A string representing the absolute path of the repository if validation succeeds.
//   - An error if the validation fails, describing the reason for failure.
func validateRepoPath(repoPath string) (string, error) {
	// ... (implementation identical to previous version) ...
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

// sortContributors sorts a slice of Contributor structs in a stable manner.
// The sorting is performed first by the Name field (case-insensitive) and,
// in case of ties, by the Email field (also case-insensitive).
//
// Parameters:
//   - contributors: A slice of Contributor structs to be sorted.
//
// The function modifies the input slice in place, arranging the contributors
// in ascending order based on their names and emails.
func sortContributors(contributors []Contributor) {
	// ... (implementation identical to previous version) ...
	sort.SliceStable(contributors, func(i, j int) bool {
		nameI := strings.ToLower(contributors[i].Name)
		nameJ := strings.ToLower(contributors[j].Name)
		if nameI != nameJ {
			return nameI < nameJ
		}
		emailI := strings.ToLower(contributors[i].Email)
		emailJ := strings.ToLower(contributors[j].Email)
		return emailI < emailJ
	})
}
