package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Stone-IT-Cloud/reporting/pkg/gitproviders"
)

func main() {
	// Check if GITHUB_TOKEN is set
	if os.Getenv("GITHUB_TOKEN") == "" {
		log.Fatal("GITHUB_TOKEN environment variable is required for GraphQL API access")
	}

	// Check if organization name is provided
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run list_org_projects.go <organization-name>")
	}

	orgName := os.Args[1]

	// Create GitHub client
	ctx := context.Background()
	client, err := gitproviders.NewGitHubClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create GitHub client: %v", err)
	}

	// List organization projects using GraphQL API
	fmt.Printf("Listing projects (beta) for organization: %s\n", orgName)
	fmt.Println("================================================")

	// List projects using the new GraphQL API
	projects, err := client.ListOrganizationProjects(orgName, nil)
	if err != nil {
		log.Fatalf("Failed to list organization projects: %v", err)
	}

	if len(projects) == 0 {
		fmt.Println("No projects found for this organization.")
		fmt.Println("\nNote: This uses the GraphQL API to fetch 'Projects (beta)'.")
		fmt.Println("If you're looking for 'Projects (classic)', they have been deprecated by GitHub.")
		return
	}

	fmt.Printf("Found %d project(s):\n\n", len(projects))

	for i, project := range projects {
		fmt.Printf("%d. Project: %s\n", i+1, project.Title)
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
		fmt.Println()
	}

	fmt.Printf("\nTotal: %d project(s)\n", len(projects))
	fmt.Println("\nNote: These are 'Projects (beta)' retrieved via GraphQL API.")
	fmt.Println("The old 'Projects (classic)' REST API has been deprecated by GitHub.")
}
