package main

import (
	"context" // Import context
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	gc "github.com/Stone-IT-Cloud/reporting/pkg/gitcontributors"
	gl "github.com/Stone-IT-Cloud/reporting/pkg/gitlogs"
	gp "github.com/Stone-IT-Cloud/reporting/pkg/gitproviders"

	// --- ★★★ Import activityreport from internal ★★★ ---
	ar "github.com/Stone-IT-Cloud/reporting/internal/activityreport"
)

const dateLayout = "2006-01-02"

func main() {
	// --- Flags ---
	// Existing flags
	includeMerges := flag.Bool("m", false, "Contributor report: Include merge commits")
	getLogsFlag := flag.Bool("log", false, "Generate git log JSON report") // Renamed for clarity
	startDateStr := flag.String("start", "", fmt.Sprintf("Start date filter (inclusive), format %s", dateLayout))
	endDateStr := flag.String("end", "", fmt.Sprintf("End date filter (inclusive), format %s", dateLayout))

	// --- ★★★ New flag for Activity Report ★★★ ---
	generateReportFlag := flag.Bool("generate-report", false, "Generate AI activity report from git logs")
	configPath := flag.String("config", "configs/activity_report_config.yaml", "Path to activity report config file")
	reportPath := flag.String("report-path", "", "Path to save the generated AI activity report")

	issues := flag.Bool("issues", false, "Fetch repository issues (not implemented)")
	projects := flag.Bool("projects", false, "List organization projects")
	orgName := flag.String("org", "", "Organization name (required when using -projects flag)")

	flag.Parse()

	// --- Validate Arguments ---
	// For projects flag, we don't need a repository path
	if *projects && *orgName == "" {
		log.Fatal("Error: -org flag is required when using -projects flag. Please specify the organization name.")
	}

	// For all other operations, require repository path
	if !*projects && flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <path-to-git-repo>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "   or: %s -projects -org <organization-name>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	var repoPath string
	if flag.NArg() > 0 {
		repoPath = flag.Arg(0)
	}

	// Determine mutually exclusive actions
	actionCount := 0
	if *getLogsFlag {
		actionCount++
	}
	if *generateReportFlag {
		actionCount++
	}
	if *projects {
		actionCount++
	}
	// If none of the specific flags are set, default to contributors
	isContributorReport := actionCount == 0
	if actionCount > 1 {
		log.Fatal("Error: -log, -generate-report, and -projects flags are mutually exclusive.")
	}

	// --- Parse Dates ---
	var startDate, endDate *time.Time
	// ... (Date parsing identical to before) ...
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
	ctx := context.Background() // Create a background context

	switch {
	case *getLogsFlag:
		// --- Generate Log Report (JSON) ---
		logOpts := &gl.Options{StartDate: startDate, EndDate: endDate}
		fmt.Printf("Generating Git Log JSON for %s", repoPath)
		if logOpts.StartDate != nil {
			fmt.Printf(" from %s", logOpts.StartDate.Format(dateLayout))
		}
		if logOpts.EndDate != nil {
			fmt.Printf(" until %s", *endDateStr)
		}
		fmt.Println(" (excluding merges, all branches, chronological):")
		logJSON, err := gl.GetLogsJSON(repoPath, logOpts) // Renamed logJson to logJSON
		if err != nil {
			log.Fatalf("Error getting git logs: %v", err)
		}
		fmt.Println(logJSON) // Use logJSON

	/* case *issues:
	// --- Fetch Repository Issues (not implemented) ---
	log.Println("Fetching repository issues is not implemented yet.")
	var err error
	projectIssues, err = getRepositoryIssues(ctx, repoPath)

	if err != nil {
		log.Fatalf("Error fetching repository issues: %v", err)
	}
	log.Println("Fetching repository issues is not implemented yet.")
	fmt.Println("Repository Issues:")
	for _, issue := range projectIssues {
		fmt.Printf("Issue #%d: %s\n", issue.ID, issue.Title)
		fmt.Printf("  Created at: %s\n", issue.CreatedAt.Format(dateLayout))
		fmt.Printf("  State: %s\n", issue.State)
		fmt.Printf("  URL: %s\n", issue.URL)
		for _, commnent := range issue.Comments {
			fmt.Printf("  Comment by: %s\n", commnent.Author)
			fmt.Printf("  Comment at: %s\n", commnent.CreatedAt.Format(dateLayout))
			fmt.Printf("  Comment: %s\n", commnent.Body)
		}
	} */

	case *generateReportFlag:
		// --- ★★★ Generate AI Activity Report ★★★ ---
		log.Println("Step 1: Fetching Git Logs for AI Report...")
		logOpts := &gl.Options{StartDate: startDate, EndDate: endDate}
		gitLogsJSON, err := gl.GetLogsJSON(repoPath, logOpts)
		if err != nil {
			log.Fatalf("Error getting git logs for AI report generation: %v", err)
		}
		log.Println("Step 1: Git Logs Fetched.")

		log.Println("Step 2: Generating AI Activity Report...")

		repoIssues := []gp.Issue{}
		if *issues {
			repoIssues, err = getRepoIssues(ctx, repoPath)
			if err != nil {
				log.Fatalf("Error fetching repository issues: %v", err)
			}
		}
		report, err := ar.GenerateReport(ctx, gitLogsJSON, repoIssues, *configPath, *reportPath)
		if err != nil {
			log.Fatalf("Error generating AI activity report: %v", err)
		}
		fmt.Println("--- Generated Report ---")
		fmt.Println(report)
		fmt.Println("--- End Report ---")
		log.Println("Step 2: AI Activity Report Generation Finished.")

	case *projects:
		// --- List Organization Projects ---
		log.Printf("Fetching projects for organization: %s", *orgName)

		// Create GitHub client
		githubClient, err := gp.NewGitHubClient(ctx)
		if err != nil {
			log.Fatalf("Error creating GitHub client: %v", err)
		}

		// List organization projects
		projects, err := githubClient.ListOrganizationProjectsWithDetails(*orgName, nil, true)
		if err != nil {
			log.Fatalf("Error fetching organization projects: %v", err)
		}

		// Print projects
		printOrganizationProjects(projects, *orgName)

	case isContributorReport: // Default case when no other flag is set
		// --- Generate Contributor Report (Default Action) ---
		contributorOpts := &gc.Options{IncludeMergeCommits: *includeMerges, StartDate: startDate, EndDate: endDate}
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
		printContributors(contributors)
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

func getRepoIssues(ctx context.Context, repoPath string) ([]gp.Issue, error) {
	repoMetadata, err := gp.ExtractRepoMetadata(ctx, repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract repository metadata: %w", err)
	}
	repo, err := gp.NewGitHubClient(ctx) // <-- Commented out or remove undefined function call
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}
	issues, err := repo.GetIssues(repoMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to get issues from GitHub: %w", err)
	}

	return issues, nil
}

// printOrganizationProjects prints organization projects in a formatted way
func printOrganizationProjects(projects []*gp.OrganizationProjectV2, orgName string) {
	if len(projects) == 0 {
		fmt.Printf("No projects found for organization '%s'.\n", orgName)
		fmt.Println("\nNote: This uses the GraphQL API to fetch 'Projects (beta)'.")
		fmt.Println("If you're looking for 'Projects (classic)', they have been deprecated by GitHub.")
		return
	}

	fmt.Printf("Organization Projects (beta) for '%s' (%d found):\n", orgName, len(projects))
	fmt.Println(strings.Repeat("=", 60))

	for i, project := range projects {
		fmt.Printf("\n%d. %s\n", i+1, project.Title)
		fmt.Printf("   ID: %s\n", project.ID)
		fmt.Printf("   Number: %d\n", project.Number)

		status := "Open"
		if project.Closed {
			status = "Closed"
		}
		fmt.Printf("   Status: %s\n", status)

		visibility := "Private"
		if project.Public {
			visibility = "Public"
		}
		fmt.Printf("   Visibility: %s\n", visibility)

		if project.ShortDescription != nil && *project.ShortDescription != "" {
			fmt.Printf("   Description: %s\n", *project.ShortDescription)
		}

		fmt.Printf("   URL: %s\n", project.URL)
		fmt.Printf("   Created: %s\n", project.CreatedAt)
		fmt.Printf("   Updated: %s\n", project.UpdatedAt)

		if project.Creator != nil {
			fmt.Printf("   Creator: %s\n", project.Creator.Login)
		}

		// Show columns and items if available
		if len(project.Columns) > 0 {
			fmt.Printf("   Columns (%d):\n", len(project.Columns))
			for _, column := range project.Columns {
				fmt.Printf("     - %s (%d items)\n", column.Name, len(column.Items))
				for _, item := range column.Items {
					if item.Content != nil {
						itemType := "Item"
						switch item.Type {
						case "ISSUE":
							itemType = "Issue"
						case "PULL_REQUEST":
							itemType = "PR"
						case "DRAFT_ISSUE":
							itemType = "Draft"
						}

						title := item.Content.Title
						if len(title) > 50 {
							title = title[:47] + "..."
						}

						repoInfo := ""
						if item.Content.Repository != nil {
							repoInfo = fmt.Sprintf(" [%s/%s]", item.Content.Repository.Owner.Login, item.Content.Repository.Name)
						}

						if item.Content.Number > 0 {
							fmt.Printf("       • %s #%d: %s%s\n", itemType, item.Content.Number, title, repoInfo)
						} else {
							fmt.Printf("       • %s: %s%s\n", itemType, title, repoInfo)
						}
					}
				}
			}
		}
	}

	fmt.Printf("\nTotal: %d project(s)\n", len(projects))
	fmt.Println("\nNote: These are 'Projects (beta)' - the modern GitHub Projects experience.")
}
