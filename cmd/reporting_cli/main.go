package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	// Import both packages from pkg/
	gc "github.com/Stone-IT-Cloud/reporting/pkg/gitcontributors"
	gl "github.com/Stone-IT-Cloud/reporting/pkg/gitlogs" // <-- Import gitlogs
)

// Define a layout constant for parsing dates
const dateLayout = "2006-01-02" // YYYY-MM-DD

func main() {
	// --- Flags ---
	// Contributor flags
	includeMerges := flag.Bool("m", false, "Contributor report: Include merge commits")

	// Log flags
	getLogs := flag.Bool("log", false, "Generate git log report instead of contributors") // <-- New flag

	// Common flags
	startDateStr := flag.String("start", "", fmt.Sprintf("Start date filter (inclusive), format %s", dateLayout))
	endDateStr := flag.String("end", "", fmt.Sprintf("End date filter (inclusive), format %s", dateLayout))

	flag.Parse()

	// --- Validate Arguments ---
	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <path-to-git-repo>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		os.Exit(1)
	}
	repoPath := flag.Arg(0)

	// --- Parse Dates ---
	var startDate, endDate *time.Time
	if *startDateStr != "" {
		parsedDate, err := time.ParseInLocation(dateLayout, *startDateStr, time.Local)
		if err != nil {
			log.Fatalf("Error parsing start date %q: %v", *startDateStr, err)
		}
		startDate = &parsedDate
	}
	if *endDateStr != "" {
		parsedDate, err := time.ParseInLocation(dateLayout, *endDateStr, time.Local)
		if err != nil {
			log.Fatalf("Error parsing end date %q: %v", *endDateStr, err)
		}
		endOfDay := parsedDate.Add(24*time.Hour - time.Nanosecond)
		endDate = &endOfDay
	}

	// --- Execute requested action ---
	if *getLogs {
		// --- Generate Log Report ---
		logOpts := &gl.Options{ // <-- Use gitlogs options
			StartDate: startDate,
			EndDate:   endDate,
		}

		fmt.Printf("Generating Git Log JSON for %s", repoPath)
		if logOpts.StartDate != nil {
			fmt.Printf(" from %s", logOpts.StartDate.Format(dateLayout))
		}
		if logOpts.EndDate != nil {
			fmt.Printf(" until %s", *endDateStr)
		} // Show user input date
		fmt.Println(" (excluding merges, all branches, chronological):")

		logJSON, err := gl.GetLogsJSON(repoPath, logOpts) // <-- Call gitlogs function
		if err != nil {
			log.Fatalf("Error getting git logs: %v", err)
		}
		fmt.Println(logJSON) // Print the JSON output

	} else {
		// --- Generate Contributor Report (Existing Logic) ---
		contributorOpts := &gc.Options{
			IncludeMergeCommits: *includeMerges,
			StartDate:           startDate,
			EndDate:             endDate,
		}
		// ... (rest of the contributor reporting logic from previous main.go) ...
		var filterDesc []string
		if contributorOpts.IncludeMergeCommits {
			filterDesc = append(filterDesc, "Including Merges")
		} else {
			filterDesc = append(filterDesc, "Excluding Merges")
		}
		if contributorOpts.StartDate != nil {
			filterDesc = append(filterDesc, fmt.Sprintf("From %s", contributorOpts.StartDate.Format(dateLayout)))
		}
		if contributorOpts.EndDate != nil {
			filterDesc = append(filterDesc, fmt.Sprintf("Until %s", *endDateStr))
		}
		filterDesc = append(filterDesc, "Sorted by Name/Email")
		fmt.Printf("Contributors for %s (%s):\n", repoPath, strings.Join(filterDesc, ", "))
		contributors, err := gc.GetContributors(repoPath, contributorOpts)
		if err != nil {
			log.Fatalf("Error getting contributors: %v", err)
		}
		printContributors(contributors) // Assumes printContributors is still defined below
	}
}

// printContributors helper function (using gc.Contributor type)
func printContributors(contributors []gc.Contributor) {
	// ... (implementation identical to previous version) ...
	if len(contributors) == 0 {
		fmt.Println("  No contributors found (or repository is empty/filtered out).")
		return
	}
	maxWidth := 0
	for _, c := range contributors {
		lenStr := fmt.Sprintf("%d", c.Commits)
		if len(lenStr) > maxWidth {
			maxWidth = len(lenStr)
		}
	}
	headerWidth := len("Commits")
	if maxWidth < headerWidth {
		maxWidth = headerWidth
	}
	countFormat := fmt.Sprintf("%%%dd", maxWidth)
	fmt.Println("  Commits | First Commit | Last Commit  | Name & Email")
	fmt.Printf("  %-"+fmt.Sprintf("%d", maxWidth)+"s | %-10s | %-10s | %s\n", strings.Repeat("-", maxWidth), strings.Repeat("-", 10), strings.Repeat("-", 10), strings.Repeat("-", 20))
	for _, c := range contributors {
		fmt.Printf("  "+countFormat+" | %s | %s | %s <%s>\n", c.Commits, c.FirstCommitDate.Format(dateLayout), c.LastCommitDate.Format(dateLayout), c.Name, c.Email)
	}
}
