package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	gc "github.com/Stone-IT-Cloud/reporting/pkg/gitcontributors"
)

// Define a layout constant for parsing dates
const dateLayout = "2006-01-02" // YYYY-MM-DD

func main() {
	// Define flags
	includeMerges := flag.Bool("m", false, "Include merge commits in the analysis")
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

	// --- Prepare Options for GetContributors ---
	// --- ★★★ Use the types from the imported package (gc) ★★★ ---
	opts := &gc.Options{
		IncludeMergeCommits: *includeMerges,
		StartDate:           startDate,
		EndDate:             endDate,
	}

	// --- Get Contributors ---
	var filterDesc []string
	if opts.IncludeMergeCommits {
		filterDesc = append(filterDesc, "Including Merges")
	} else {
		filterDesc = append(filterDesc, "Excluding Merges")
	}
	if opts.StartDate != nil {
		filterDesc = append(filterDesc, fmt.Sprintf("From %s", opts.StartDate.Format(dateLayout)))
	}
	if opts.EndDate != nil {
		filterDesc = append(filterDesc, fmt.Sprintf("Until %s", *endDateStr))
	}
	filterDesc = append(filterDesc, "Sorted by Name/Email")

	fmt.Printf("Contributors for %s (%s):\n", repoPath, strings.Join(filterDesc, ", "))

	// --- ★★★ Call the function from the imported package (gc) ★★★ ---
	contributors, err := gc.GetContributors(repoPath, opts)
	if err != nil {
		log.Fatalf("Error getting contributors: %v", err)
	}

	// --- Print Results ---
	printContributors(contributors) // Call the local helper function
}

// printContributors helper function (can remain here or move to internal if it grows)
// --- ★★★ Use the type from the imported package (gc) ★★★ ---
func printContributors(contributors []gc.Contributor) {
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
	fmt.Printf("  %-"+fmt.Sprintf("%d", maxWidth)+"s | %-10s | %-10s | %s\n",
		strings.Repeat("-", maxWidth), strings.Repeat("-", 10), strings.Repeat("-", 10), strings.Repeat("-", 20))

	for _, c := range contributors {
		fmt.Printf("  "+countFormat+" | %s | %s | %s <%s>\n",
			c.Commits,
			c.FirstCommitDate.Format(dateLayout),
			c.LastCommitDate.Format(dateLayout),
			c.Name,
			c.Email,
		)
	}
}
