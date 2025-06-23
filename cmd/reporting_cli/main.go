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

	issues := flag.Bool("issues", false, "Fetch repository issues")
	projects := flag.Bool("projects", false, "List organization projects")
	orgName := flag.String("org", "", "Organization name (required when using -projects flag)")

	flag.Parse()

	// --- Validate Arguments ---
	// Validate -org flag usage
	if (*projects || *generateReportFlag) && *orgName == "" {
		log.Fatal("Error: -org flag is required when using -projects or -generate-report with -projects flag.")
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

	// Determine primary action - only -log and -generate-report are mutually exclusive
	// -projects and -issues can be used with -generate-report but not standalone without -org
	primaryActionCount := 0
	if *getLogsFlag {
		primaryActionCount++
	}
	if *generateReportFlag {
		primaryActionCount++
	}

	// If none of the primary flags are set and not using -projects standalone, default to contributors
	isContributorReport := primaryActionCount == 0 && !*projects

	// Only -log and -generate-report are mutually exclusive
	if *getLogsFlag && *generateReportFlag {
		log.Fatal("Error: -log and -generate-report flags are mutually exclusive.")
	}

	// -projects without -generate-report requires -org
	if *projects && !*generateReportFlag && primaryActionCount == 0 {
		if *orgName == "" {
			log.Fatal("Error: -org flag is required when using -projects flag standalone.")
		}
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

		orgProjects := []*gp.OrganizationProjectV2{}
		if *projects {
			log.Println("Fetching organization projects for AI report...")
			// Fetch organization projects
			orgProjects, err = getOrganizationProjects(ctx, *orgName)
			if err != nil {
				log.Fatalf("Error fetching organization projects: %v", err)
			}
		}

		report, err := ar.GenerateReport(ctx, gitLogsJSON, repoIssues, orgProjects, *configPath, *reportPath)
		if err != nil {
			log.Fatalf("Error generating AI activity report: %v", err)
		}
		fmt.Println("--- Generated Report ---")
		fmt.Println(report)
		fmt.Println("--- End Report ---")
		log.Println("Step 2: AI Activity Report Generation Finished.")

	case *projects:
		if !*generateReportFlag {
			// --- List Organization Projects ---
			log.Printf("Fetching projects for organization: %s", *orgName)

			projects, err := getOrganizationProjects(ctx, *orgName)
			if err != nil {
				log.Fatalf("Error fetching organization projects: %v", err)
			}

			// Print projects

			printOrganizationProjects(projects, *orgName)
		}

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

func getOrganizationProjects(ctx context.Context, orgName string) ([]*gp.OrganizationProjectV2, error) {
	// Create GitHub client
	githubClient, err := gp.NewGitHubClient(ctx)
	if err != nil {
		log.Fatalf("Error creating GitHub client: %v", err)
	}

	// List organization projects
	projects, err := githubClient.ListOrganizationProjectsWithDetails(orgName, nil, true)
	if err != nil {
		return nil, fmt.Errorf("error fetching organization projects: %w", err)
	}
	return projects, nil
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
