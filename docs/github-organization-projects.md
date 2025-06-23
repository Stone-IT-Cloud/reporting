# GitHub Provider - Organization Projects

This document explains how to use the `ListOrganizationProjects` function to retrieve all projects from a GitHub organization.

## Overview

The `ListOrganizationProjects` function allows you to list all "Projects (classic)" for a given GitHub organization. This function uses the GitHub REST API directly since the go-github library doesn't provide a built-in method for this functionality.

⚠️ **Important Note**: This API endpoint is for "Projects (classic)" which are deprecated by GitHub. For newer "Projects (beta)", you would need to use the GraphQL API instead.

## Prerequisites

- A GitHub personal access token with appropriate permissions
- The token should be set in the `GITHUB_TOKEN` environment variable
- Access to the organization whose projects you want to list

## Usage

### Basic Example

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/Stone-IT-Cloud/reporting/pkg/gitproviders"
)

func main() {
    // Create GitHub client
    ctx := context.Background()
    client, err := gitproviders.NewGitHubClient(ctx)
    if err != nil {
        log.Fatalf("Failed to create GitHub client: %v", err)
    }

    // List organization projects with default options
    projects, err := client.ListOrganizationProjects("your-org-name", nil)
    if err != nil {
        log.Fatalf("Failed to list projects: %v", err)
    }

    fmt.Printf("Found %d projects\n", len(projects))
    for _, project := range projects {
        if project.Name != nil {
            fmt.Printf("- %s\n", *project.Name)
        }
    }
}
```

### Advanced Example with Options

```go
// List all projects (including closed ones) with pagination
options := &gitproviders.ListOrganizationProjectsOptions{
    State:   "all",     // "open", "closed", or "all"
    PerPage: 50,        // Number of results per page (max 100)
    Page:    1,         // Page number
}

projects, err := client.ListOrganizationProjects("your-org-name", options)
if err != nil {
    log.Fatalf("Failed to list projects: %v", err)
}
```

## API Reference

### Function Signature

```go
func (gc *GitHubClient) ListOrganizationProjects(org string, opts *ListOrganizationProjectsOptions) ([]*OrganizationProject, error)
```

### Parameters

- `org` (string): The organization name (required)
- `opts` (*ListOrganizationProjectsOptions): Optional parameters for filtering and pagination

### Options Structure

```go
type ListOrganizationProjectsOptions struct {
    State   string // "open", "closed", or "all" (default: "open")
    PerPage int    // Number of results per page, max 100 (default: 30)
    Page    int    // Page number (default: 1)
}
```

### Project Structure

```go
type OrganizationProject struct {
    ID          *int64       `json:"id,omitempty"`
    NodeID      *string      `json:"node_id,omitempty"`
    Name        *string      `json:"name,omitempty"`
    Body        *string      `json:"body,omitempty"`
    Number      *int         `json:"number,omitempty"`
    State       *string      `json:"state,omitempty"`
    CreatedAt   *string      `json:"created_at,omitempty"`
    UpdatedAt   *string      `json:"updated_at,omitempty"`
    URL         *string      `json:"url,omitempty"`
    HTMLURL     *string      `json:"html_url,omitempty"`
    ColumnsURL  *string      `json:"columns_url,omitempty"`
    OwnerURL    *string      `json:"owner_url,omitempty"`
    Creator     *github.User `json:"creator,omitempty"`
}
```

## Error Handling

The function returns an error in the following cases:

1. **Empty organization name**: Returns `"organization name cannot be empty"`
2. **Network/API errors**: Returns API-specific error messages
3. **Authentication errors**: Returns authentication-related error messages
4. **Permission errors**: Returns permission-related error messages

## GitHub API Documentation

This function implements the GitHub REST API endpoint:
- **Endpoint**: `GET /orgs/{org}/projects`
- **Documentation**: [List organization projects](https://docs.github.com/en/rest/projects/projects#list-organization-projects)
- **Required Headers**: `Accept: application/vnd.github.inertia-preview+json`

## Limitations

1. **Projects (classic) only**: This API only returns classic projects, not the newer Projects (beta)
2. **Rate limiting**: Subject to GitHub API rate limits
3. **Permissions**: Requires appropriate access to the organization
4. **Deprecation**: Classic projects are deprecated by GitHub in favor of the new Projects (beta)

## Testing

To test the implementation, you can use the provided example:

```bash
# Set your GitHub token
export GITHUB_TOKEN="your_github_token_here"

# Run the example
cd examples
go run list_org_projects.go your-organization-name
```

The example will list all projects for the specified organization and provide detailed information about each project.
