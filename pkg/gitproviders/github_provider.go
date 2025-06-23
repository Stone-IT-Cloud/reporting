package gitproviders

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/google/go-github/v72/github"
	"github.com/shurcooL/graphql"
	"golang.org/x/oauth2"
)

/*
Project Management Implementation Notes:
=======================================

This file implements GitHub Projects (beta) support via the GraphQL API.

Key Functions:
- ListOrganizationProjects: Lists all projects for an organization (basic info only)
- GetOrganizationProjectDetails: Gets detailed project info including items
- GetOrganizationProjectItemsSimple: Fetches project items using simplified HTTP approach
- ListOrganizationProjectsWithDetails: Combines basic listing with detailed fetching

The implementation handles:
- Projects (beta) - the new GitHub Projects experience
- Issues, Pull Requests, and Draft Issues as project items
- Basic column organization (simplified to "All Items" to avoid GraphQL complexity)
- Proper error handling and fallbacks

GraphQL Complexity Notes:
- Project field configurations use complex union types that are difficult to handle
- The simplified approach fetches items directly without complex field parsing
- This provides a robust solution that works with real project data

CLI Integration:
- Use the -projects flag with -org to list organization projects
- Projects are displayed with their items and content details
- Each item shows type, number, title, and repository information
*/

// GitHubClient represents a client for interacting with the GitHub API.
// It encapsulates the underlying GitHub client and the context for requests.
type GitHubClient struct {
	client *github.Client
	ctx    context.Context
}

// RepoMetadata contains the repository owner and name information.
// This struct is used to identify a specific repository when making API calls.
type RepoMetadata struct {
	Owner    string
	RepoName string
}

// NewGitHubClient creates and initializes a new GitHubClient instance.
// It authenticates using the token provided via the GITHUB_TOKEN environment
// variable. The function verifies the authentication by attempting to fetch
// the current user's information.
//
// If the provided context `ctx` is nil, `context.Background()` will be used.
// The `token` parameter in the function signature is currently unused;
// authentication relies solely on the GITHUB_TOKEN environment variable.
//
// Parameters:
//
//	ctx: The context.Context to use for requests. Defaults to context.Background() if nil.
//	token: An unused string parameter. Authentication uses the GITHUB_TOKEN environment variable.
//
// Returns:
//
//	A pointer to a new GitHubClient and a nil error if initialization and
//	authentication are successful.
//	An error if the GITHUB_TOKEN environment variable is not set or if
//	authentication with the provided token fails.
func NewGitHubClient(ctx context.Context) (*GitHubClient, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	authToken := os.Getenv("GITHUB_TOKEN")
	if authToken == "" {
		return nil, fmt.Errorf("la variable de entorno GITHUB_TOKEN no está configurada")
	}

	client := github.NewClient(nil).WithAuthToken(authToken)
	_, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("error al verificar la autenticación de GitHub: %w", err)
	}

	return &GitHubClient{
		client: client,
		ctx:    ctx,
	}, nil
}

// GetIssues retrieves all issues (excluding pull requests) from a GitHub repository
// specified in the metadata parameter. For each issue, it also fetches and includes
// the associated comments.
//
// The function returns a slice of Issue structs that contain normalized data from
// the GitHub API, including issue details (ID, Title, Body, URL, State) and all
// related comments with their metadata.
//
// If an error occurs when fetching issues or comments, the function will return nil
// for the issues slice and an error describing what went wrong.
//
// Parameters:
//   - metadata: RepoMetadata containing Owner and RepoName for the target repository
//
// Returns:
//   - []Issue: A slice of Issue structs containing the issues' data and their comments
//   - error: An error if the GitHub API requests fail, nil otherwise
func (gh *GitHubClient) GetIssues(metadata RepoMetadata) ([]Issue, error) {
	ghIssues, _, err := gh.client.Issues.ListByRepo(gh.ctx, metadata.Owner, metadata.RepoName, nil)
	if err != nil {
		return nil, fmt.Errorf("error al obtener los problemas de GitHub: %w", err)
	}

	var issues []Issue
	for _, issue := range ghIssues {
		if issue.IsPullRequest() {
			continue
		}
		var comments []Comment
		ghComments, _, err := gh.client.Issues.ListComments(gh.ctx, metadata.Owner, metadata.RepoName, issue.GetNumber(), nil)
		if err != nil {
			// Consider whether to return the error immediately or log it and continue
			return nil, fmt.Errorf("error al obtener los comentarios del problema #%d: %w", issue.GetNumber(), err)
		}

		for _, ghComment := range ghComments {
			comments = append(comments, Comment{
				ID:        fmt.Sprintf("%d", ghComment.GetID()),
				Body:      ghComment.GetBody(),
				CreatedAt: ghComment.GetCreatedAt().Time,
				Author:    ghComment.GetUser().GetLogin(),
				URL:       ghComment.GetHTMLURL(),
			})
		}

		issues = append(issues, Issue{
			ID:       fmt.Sprintf("%d", issue.GetNumber()),
			Title:    issue.GetTitle(),
			Body:     issue.GetBody(),
			URL:      issue.GetHTMLURL(),
			State:    issue.GetState(),
			Comments: comments,
		})
	}

	return issues, nil
}

// GetPullRequests retrieves all pull requests for a given repository from GitHub.
// It fetches the pull requests and for each pull request, it also fetches its associated comments and reviewers.
//
// Parameters:
//   - owner: The username or organization name that owns the repository.
//   - repo: The name of the repository.
//
// Returns:
//   - A slice of PullRequest structs, each populated with details fetched from the GitHub API,
//     including comments and reviewers associated with the pull request.
//   - An error if any occurred during the API calls to GitHub (e.g., fetching pull requests, comments, or reviewers).
func (gh *GitHubClient) GetPullRequests(metadata RepoMetadata) ([]PullRequest, error) {
	ghPullRequests, _, err := gh.client.PullRequests.List(gh.ctx, metadata.Owner, metadata.RepoName, nil)
	if err != nil {
		return nil, fmt.Errorf("error al obtener las solicitudes de extracción de GitHub: %w", err)
	}

	var pullRequests []PullRequest
	for _, pr := range ghPullRequests {
		var comments []Comment
		ghComments, _, err := gh.client.PullRequests.ListComments(gh.ctx, metadata.Owner, metadata.RepoName, pr.GetNumber(), nil)
		if err != nil {
			return nil, fmt.Errorf("error al obtener los comentarios de la solicitud de extracción: %w", err)
		}

		for _, ghComment := range ghComments {
			comments = append(comments, Comment{
				ID:        fmt.Sprintf("%d", ghComment.GetID()),
				Body:      ghComment.GetBody(),
				CreatedAt: ghComment.GetCreatedAt().Time,
				Author:    ghComment.GetUser().GetLogin(),
				URL:       ghComment.GetHTMLURL(),
			})
		}
		var reviewers []Reviewer
		ghReviewers, _, err := gh.client.PullRequests.ListReviews(gh.ctx, metadata.Owner, metadata.RepoName, pr.GetNumber(), nil)
		if err != nil {
			return nil, fmt.Errorf("error al obtener los revisores de la solicitud de extracción: %w", err)
		}
		for _, ghReviewer := range ghReviewers {
			reviewers = append(reviewers, Reviewer{
				ID:         fmt.Sprintf("%d", ghReviewer.GetUser().GetID()),
				Name:       ghReviewer.GetUser().GetLogin(),
				ProfileURL: ghReviewer.GetUser().GetHTMLURL(),
				Email:      ghReviewer.GetUser().GetEmail(),
			})
		}
		pullRequests = append(pullRequests, PullRequest{
			ID:           fmt.Sprintf("%d", pr.GetNumber()),
			Title:        pr.GetTitle(),
			Body:         pr.GetBody(),
			CreatedAt:    pr.GetCreatedAt().Time,
			SourceBranch: pr.GetHead().GetRef(),
			TargetBranch: pr.GetBase().GetRef(),
			Author:       pr.GetUser().GetLogin(),
			Assignee:     pr.GetAssignee().GetLogin(),
			Comments:     comments,
			Reviewers:    reviewers,
		})
	}
	return pullRequests, nil
}

// GetRepository retrieves repository information for a specific GitHub repository.
//
// Parameters:
//
//	owner: The username or organization name that owns the repository.
//	repo: The name of the repository.
//
// Returns:
//
//	Repository: A struct containing the basic details of the repository (ID, Name, Owner, CreatedAt).
//	error: An error if the repository could not be retrieved from GitHub.
func (gh *GitHubClient) GetRepository(owner, repo string) (Repository, error) {
	ghRepo, _, err := gh.client.Repositories.Get(gh.ctx, owner, repo)
	if err != nil {
		return Repository{}, fmt.Errorf("error al obtener el repositorio de GitHub: %w", err)
	}

	repository := Repository{
		ID:        fmt.Sprintf("%d", ghRepo.GetID()),
		Name:      ghRepo.GetName(),
		Owner:     ghRepo.GetOwner().GetLogin(),
		CreatedAt: ghRepo.GetCreatedAt().Time,
	}

	return repository, nil
}

// GetPullRequest retrieves a specific pull request from a GitHub repository,
// including its comments and reviewers.
//
// Parameters:
//   - owner: The owner of the repository.
//   - repo: The name of the repository.
//   - prID: The ID (number) of the pull request as a string.
//
// Returns:
//   - PullRequest: A struct containing details of the pull request, its comments, and reviewers.
//   - error: An error if the pull request ID is invalid, or if there's an issue fetching data from GitHub.
func (gh *GitHubClient) GetPullRequest(owner, repo, prID string) (PullRequest, error) {
	// Convert prID to int
	prNumber, err := strconv.Atoi(prID)
	if err != nil {
		return PullRequest{}, fmt.Errorf("error al convertir el ID de la solicitud de extracción a int: %w", err)
	}
	ghPR, _, err := gh.client.PullRequests.Get(gh.ctx, owner, repo, prNumber)
	if err != nil {
		return PullRequest{}, fmt.Errorf("error al obtener la solicitud de extracción de GitHub: %w", err)
	}

	var comments []Comment
	ghComments, _, err := gh.client.PullRequests.ListComments(gh.ctx, owner, repo, ghPR.GetNumber(), nil)
	if err != nil {
		return PullRequest{}, fmt.Errorf("error al obtener los comentarios de la solicitud de extracción: %w", err)
	}

	for _, ghComment := range ghComments {
		comments = append(comments, Comment{
			ID:        fmt.Sprintf("%d", ghComment.GetID()),
			Body:      ghComment.GetBody(),
			CreatedAt: ghComment.GetCreatedAt().Time,
			Author:    ghComment.GetUser().GetLogin(),
			URL:       ghComment.GetHTMLURL(),
		})
	}
	var reviewers []Reviewer
	ghReviewers, _, err := gh.client.PullRequests.ListReviews(gh.ctx, owner, repo, ghPR.GetNumber(), nil)
	if err != nil {
		return PullRequest{}, fmt.Errorf("error al obtener los revisores de la solicitud de extracción: %w", err)
	}
	for _, ghReviewer := range ghReviewers {
		reviewers = append(reviewers, Reviewer{
			ID:         fmt.Sprintf("%d", ghReviewer.GetUser().GetID()),
			Name:       ghReviewer.GetUser().GetLogin(),
			ProfileURL: ghReviewer.GetUser().GetHTMLURL(),
			Email:      ghReviewer.GetUser().GetEmail(),
		})
	}
	pullRequest := PullRequest{
		ID:           fmt.Sprintf("%d", ghPR.GetNumber()),
		Title:        ghPR.GetTitle(),
		Body:         ghPR.GetBody(),
		CreatedAt:    ghPR.GetCreatedAt().Time,
		SourceBranch: ghPR.GetHead().GetRef(),
		TargetBranch: ghPR.GetBase().GetRef(),
		Author:       ghPR.GetUser().GetLogin(),
		Assignee:     ghPR.GetAssignee().GetLogin(),
		Comments:     comments,
		Reviewers:    reviewers,
	}
	return pullRequest, nil
}

// GetIssue retrieves a specific issue and its comments from a GitHub repository.
// It fetches the issue details using the provided owner, repository name, and issue ID.
// It then fetches all comments associated with that issue.
// The issue ID string is converted to an integer internally for the GitHub API call.
// It returns a populated Issue struct containing the issue's details and its comments,
// or an error if the issue ID is invalid, or if there's an error communicating with the GitHub API
// while fetching the issue or its comments.
func (gh *GitHubClient) GetIssue(owner, repo, issueID string) (Issue, error) {
	// Convert issueID to int
	issueNumber, err := strconv.Atoi(issueID)
	if err != nil {
		return Issue{}, fmt.Errorf("error al convertir el ID del problema a int: %w", err)
	}
	ghIssue, _, err := gh.client.Issues.Get(gh.ctx, owner, repo, issueNumber)
	if err != nil {
		return Issue{}, fmt.Errorf("error al obtener el problema de GitHub: %w", err)
	}

	var comments []Comment
	ghComments, _, err := gh.client.Issues.ListComments(gh.ctx, owner, repo, ghIssue.GetNumber(), nil)
	if err != nil {
		return Issue{}, fmt.Errorf("error al obtener los comentarios del problema: %w", err)
	}

	for _, ghComment := range ghComments {
		comments = append(comments, Comment{
			ID:        fmt.Sprintf("%d", ghComment.GetID()),
			Body:      ghComment.GetBody(),
			CreatedAt: ghComment.GetCreatedAt().Time,
			Author:    ghComment.GetUser().GetLogin(),
			URL:       ghComment.GetHTMLURL(),
		})
	}
	issue := Issue{
		ID:       fmt.Sprintf("%d", ghIssue.GetNumber()),
		Title:    ghIssue.GetTitle(),
		Body:     ghIssue.GetBody(),
		URL:      ghIssue.GetHTMLURL(),
		State:    ghIssue.GetState(),
		Comments: comments,
	}
	return issue, nil
}

// ExtractRepoMetadata extracts repository owner and name from a local git repository.
// It reads the remote URL from the git repository at the specified path and parses
// it to extract the owner and repository name. Supports both SSH and HTTPS formats.
//
// Parameters:
//   - ctx: The context for the git command execution
//   - repoPath: The local path to the git repository
//
// Returns:
//   - RepoMetadata: A struct containing the owner and repository name
//   - error: An error if the repository path is invalid or URL parsing fails
func ExtractRepoMetadata(ctx context.Context, repoPath string) (RepoMetadata, error) {
	// Placeholder for the actual implementation
	// Use git to get the remote URL for 'origin'
	cmd := exec.CommandContext(ctx, "git", "remote", "get-url", "origin")
	cmd.Dir = repoPath // Run the command in the repository directory

	output, err := cmd.Output()
	if err != nil {
		return RepoMetadata{}, fmt.Errorf("failed to get git remote URL for %s: %w", repoPath, err)
	}

	remoteURL := strings.TrimSpace(string(output))

	var owner string
	var repoName string

	switch {
	case strings.Contains(remoteURL, "@"): // SSH format: git@github.com:Owner/Repo.git
		// Split at ":"
		parts := strings.SplitN(remoteURL, ":", 2)
		if len(parts) != 2 {
			return RepoMetadata{}, fmt.Errorf("invalid SSH remote URL format: %s", remoteURL)
		}
		pathPart := parts[1] // Owner/Repo.git

		// Split path at "/"
		pathParts := strings.SplitN(pathPart, "/", 2)
		if len(pathParts) != 2 { // Expecting Owner and Repo parts
			return RepoMetadata{}, fmt.Errorf("could not extract owner/repo from SSH path: %s", pathPart)
		}
		owner = pathParts[0]
		repoName = strings.TrimSuffix(pathParts[1], ".git")

	case strings.Contains(remoteURL, "://"): // HTTPS format: https://github.com/Owner/Repo.git
		// Find the end of the schema part "://"
		schemaEndIndex := strings.Index(remoteURL, "://")
		if schemaEndIndex == -1 {
			return RepoMetadata{}, fmt.Errorf("invalid HTTPS remote URL format (missing ://): %s", remoteURL)
		}
		// Find the first '/' after the domain part (e.g., after github.com)
		// Start searching after "://"
		pathStartIndex := strings.Index(remoteURL[schemaEndIndex+3:], "/")
		if pathStartIndex == -1 {
			return RepoMetadata{}, fmt.Errorf("invalid HTTPS remote URL format (missing path separator after domain): %s", remoteURL)
		}
		// Adjust pathStartIndex to be relative to the original string start
		pathStartIndex += schemaEndIndex + 3

		// The path part starts right after this slash
		pathPart := remoteURL[pathStartIndex+1:] // Owner/Repo.git

		// Split path at "/"
		pathParts := strings.SplitN(pathPart, "/", 2)
		if len(pathParts) != 2 { // Expecting Owner and Repo parts
			return RepoMetadata{}, fmt.Errorf("could not extract owner/repo from HTTPS path: %s", pathPart)
		}
		owner = pathParts[0]
		repoName = strings.TrimSuffix(pathParts[1], ".git")

	default:
		// Could be a local path or other unsupported format
		return RepoMetadata{}, fmt.Errorf("unsupported remote URL format (neither SSH nor HTTPS): %s", remoteURL)
	}

	// Basic validation
	return RepoMetadata{
		Owner:    owner,
		RepoName: repoName,
	}, nil
}

// OrganizationProject represents a GitHub organization project (classic)
type OrganizationProject struct {
	ID         *int64       `json:"id,omitempty"`
	NodeID     *string      `json:"node_id,omitempty"`
	Name       *string      `json:"name,omitempty"`
	Body       *string      `json:"body,omitempty"`
	Number     *int         `json:"number,omitempty"`
	State      *string      `json:"state,omitempty"`
	CreatedAt  *string      `json:"created_at,omitempty"`
	UpdatedAt  *string      `json:"updated_at,omitempty"`
	URL        *string      `json:"url,omitempty"`
	HTMLURL    *string      `json:"html_url,omitempty"`
	ColumnsURL *string      `json:"columns_url,omitempty"`
	OwnerURL   *string      `json:"owner_url,omitempty"`
	Creator    *github.User `json:"creator,omitempty"`
}

// OrganizationProjectV2 represents a GitHub organization project (beta/new) from GraphQL API
type OrganizationProjectV2 struct {
	ID               string  `json:"id"`
	Number           int     `json:"number"`
	Title            string  `json:"title"`
	ShortDescription *string `json:"shortDescription"`
	Public           bool    `json:"public"`
	Closed           bool    `json:"closed"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
	URL              string  `json:"url"`
	Creator          *struct {
		Login string `json:"login"`
	} `json:"creator"`
	Columns []ProjectColumn `json:"columns,omitempty"` // Only populated when getting detailed project info
}

// ProjectColumn represents a column in a GitHub project (beta)
type ProjectColumn struct {
	ID    string        `json:"id"`
	Name  string        `json:"name"`
	Items []ProjectItem `json:"items"`
}

// ProjectItem represents an item (issue/pull request) in a project column
type ProjectItem struct {
	ID      string              `json:"id"`
	Type    string              `json:"type"` // "ISSUE", "PULL_REQUEST", "DRAFT_ISSUE"
	Content *ProjectItemContent `json:"content,omitempty"`
}

// ProjectItemContent represents the actual content of a project item
type ProjectItemContent struct {
	ID         string `json:"id"`
	Number     int    `json:"number,omitempty"`
	Title      string `json:"title"`
	Body       string `json:"body,omitempty"`
	State      string `json:"state,omitempty"`
	URL        string `json:"url,omitempty"`
	Repository *struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository,omitempty"`
	Author *struct {
		Login string `json:"login"`
	} `json:"author,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// BasicOrganizationProjectV2 represents a basic GitHub organization project without detailed fields
type BasicOrganizationProjectV2 struct {
	ID               string  `graphql:"id"`
	Number           int     `graphql:"number"`
	Title            string  `graphql:"title"`
	ShortDescription *string `graphql:"shortDescription"`
	Public           bool    `graphql:"public"`
	Closed           bool    `graphql:"closed"`
	CreatedAt        string  `graphql:"createdAt"`
	UpdatedAt        string  `graphql:"updatedAt"`
	URL              string  `graphql:"url"`
	Creator          *struct {
		Login string `graphql:"login"`
	} `graphql:"creator"`
}

// GraphQLResponse represents the GraphQL response structure for projects
type GraphQLResponse struct {
	Organization struct {
		ProjectsV2 struct {
			Nodes    []BasicOrganizationProjectV2 `graphql:"nodes"`
			PageInfo struct {
				HasNextPage bool   `graphql:"hasNextPage"`
				EndCursor   string `graphql:"endCursor"`
			} `graphql:"pageInfo"`
			TotalCount int `graphql:"totalCount"`
		} `graphql:"projectsV2(first: $first, after: $after)"`
	} `graphql:"organization(login: $login)"`
}

// GraphQLProjectDetailsResponse represents the GraphQL response for detailed project information
type GraphQLProjectDetailsResponse struct {
	Organization struct {
		ProjectV2 struct {
			ID               string  `graphql:"id"`
			Number           int     `graphql:"number"`
			Title            string  `graphql:"title"`
			ShortDescription *string `graphql:"shortDescription"`
			Public           bool    `graphql:"public"`
			Closed           bool    `graphql:"closed"`
			CreatedAt        string  `graphql:"createdAt"`
			UpdatedAt        string  `graphql:"updatedAt"`
			URL              string  `graphql:"url"`
			Creator          *struct {
				Login string `graphql:"login"`
			} `graphql:"creator"`
			Items struct {
				Nodes []struct {
					ID      string `graphql:"id"`
					Type    string `graphql:"type"`
					Content struct {
						TypeName string `graphql:"__typename"`
						Issue    *struct {
							Number     int    `graphql:"number"`
							Title      string `graphql:"title"`
							Body       string `graphql:"body"`
							State      string `graphql:"state"`
							URL        string `graphql:"url"`
							CreatedAt  string `graphql:"createdAt"`
							UpdatedAt  string `graphql:"updatedAt"`
							Repository struct {
								Name  string `graphql:"name"`
								Owner struct {
									Login string `graphql:"login"`
								} `graphql:"owner"`
							} `graphql:"repository"`
							Author *struct {
								Login string `graphql:"login"`
							} `graphql:"author"`
						} `graphql:"... on Issue"`
						PullRequest *struct {
							Number     int    `graphql:"number"`
							Title      string `graphql:"title"`
							Body       string `graphql:"body"`
							State      string `graphql:"state"`
							URL        string `graphql:"url"`
							CreatedAt  string `graphql:"createdAt"`
							UpdatedAt  string `graphql:"updatedAt"`
							Repository struct {
								Name  string `graphql:"name"`
								Owner struct {
									Login string `graphql:"login"`
								} `graphql:"owner"`
							} `graphql:"repository"`
							Author *struct {
								Login string `graphql:"login"`
							} `graphql:"author"`
						} `graphql:"... on PullRequest"`
						DraftIssue *struct {
							Title     string `graphql:"title"`
							Body      string `graphql:"body"`
							CreatedAt string `graphql:"createdAt"`
							UpdatedAt string `graphql:"updatedAt"`
							Creator   *struct {
								Login string `graphql:"login"`
							} `graphql:"creator"`
						} `graphql:"... on DraftIssue"`
					} `graphql:"content"`
				} `graphql:"nodes"`
				PageInfo struct {
					HasNextPage bool   `graphql:"hasNextPage"`
					EndCursor   string `graphql:"endCursor"`
				} `graphql:"pageInfo"`
			} `graphql:"items(first: 100, after: $after)"`
		} `graphql:"projectV2(number: $projectNumber)"`
	} `graphql:"organization(login: $login)"`
}

// ListOrganizationProjectsOptions specifies the optional parameters for listing organization projects
type ListOrganizationProjectsOptions struct {
	// State filters projects by their state. Can be either "open", "closed", or "all". Default: "open"
	State string `url:"state,omitempty"`

	// PerPage is the number of results per page (max 100). Default: 30
	PerPage int `url:"per_page,omitempty"`

	// Page is the page number of results to fetch. Default: 1
	Page int `url:"page,omitempty"`
}

// ListOrganizationProjects lists all projects (beta) for an organization using GraphQL.
// This function uses the GitHub GraphQL API to fetch "Projects (beta)" which replaced
// the deprecated "Projects (classic)".
//
// The GitHub GraphQL API requires authentication and the user must have
// read access to the organization.
//
// GitHub GraphQL docs: https://docs.github.com/en/graphql/reference/objects#projectv2
//
// Parameters:
//
//	org: The organization name
//	opts: Optional parameters for filtering and pagination (currently unused in GraphQL implementation)
//
// Returns:
//
//	A slice of OrganizationProjectV2 structs and an error if the request fails
func (gh *GitHubClient) ListOrganizationProjects(org string, opts *ListOrganizationProjectsOptions) ([]*OrganizationProjectV2, error) {
	if org == "" {
		return nil, fmt.Errorf("organization name cannot be empty")
	}

	// Get the GitHub token from environment
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable is required for GraphQL API")
	}

	// Create GraphQL client
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(gh.ctx, src)
	client := graphql.NewClient("https://api.github.com/graphql", httpClient)

	// Set up the GraphQL query variables
	variables := map[string]interface{}{
		"login": graphql.String(org),
		"first": graphql.Int(100), // Fetch up to 100 projects
		"after": (*graphql.String)(nil),
	}

	var query GraphQLResponse
	var allProjects []*OrganizationProjectV2

	// Execute the GraphQL query (with pagination support)
	for {
		err := client.Query(gh.ctx, &query, variables)
		if err != nil {
			return nil, fmt.Errorf("failed to execute GraphQL query: %w", err)
		}

		// Convert to OrganizationProjectV2 and append to results
		for i := range query.Organization.ProjectsV2.Nodes {
			basicProject := &query.Organization.ProjectsV2.Nodes[i]
			project := &OrganizationProjectV2{
				ID:               basicProject.ID,
				Number:           basicProject.Number,
				Title:            basicProject.Title,
				ShortDescription: basicProject.ShortDescription,
				Public:           basicProject.Public,
				Closed:           basicProject.Closed,
				CreatedAt:        basicProject.CreatedAt,
				UpdatedAt:        basicProject.UpdatedAt,
				URL:              basicProject.URL,
				Columns:          []ProjectColumn{}, // Empty for basic query
			}
			// Convert creator if present
			if basicProject.Creator != nil {
				project.Creator = &struct {
					Login string `json:"login"`
				}{Login: basicProject.Creator.Login}
			}
			allProjects = append(allProjects, project)
		}

		// Check if there are more pages
		if !query.Organization.ProjectsV2.PageInfo.HasNextPage {
			break
		}

		// Set the cursor for the next page
		variables["after"] = graphql.String(query.Organization.ProjectsV2.PageInfo.EndCursor)
	}

	return allProjects, nil
}

// GetOrganizationProjectDetails retrieves detailed information about a specific organization project,
// including all its items (issues/PRs).
// This version uses the simplified items fetching to get real project data.
//
// Parameters:
//
//	org: The organization name
//	projectNumber: The project number (visible in the project URL)
//
// Returns:
//
//	A pointer to OrganizationProjectV2 struct with detailed information and an error if the request fails
func (gh *GitHubClient) GetOrganizationProjectDetails(org string, projectNumber int) (*OrganizationProjectV2, error) {
	if org == "" {
		return nil, fmt.Errorf("organization name cannot be empty")
	}
	if projectNumber <= 0 {
		return nil, fmt.Errorf("project number must be positive")
	}

	// Get basic project information
	basicProjects, err := gh.ListOrganizationProjects(org, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get basic project info: %w", err)
	}

	// Find the requested project
	var targetProject *OrganizationProjectV2
	for _, project := range basicProjects {
		if project.Number == projectNumber {
			targetProject = project
			break
		}
	}

	if targetProject == nil {
		return nil, fmt.Errorf("project with number %d not found", projectNumber)
	}

	// Try to get items with their actual columns for this project
	columns, err := gh.GetOrganizationProjectItemsWithColumns(org, projectNumber)
	if err != nil {
		// If we can't get items with columns, fall back to simple method
		fmt.Printf("Warning: Could not fetch items with columns for project %d: %v\n", projectNumber, err)
		fmt.Printf("Falling back to simple item fetching...\n")

		items, err := gh.GetOrganizationProjectItemsSimple(org, projectNumber)
		if err != nil {
			// If we can't get items at all, return the basic project with empty items
			fmt.Printf("Warning: Could not fetch items for project %d: %v\n", projectNumber, err)
			targetProject.Columns = []ProjectColumn{
				{
					ID:    "placeholder",
					Name:  "Items",
					Items: []ProjectItem{},
				},
			}
			return targetProject, nil
		}

		// Create a column with the fetched items
		if len(items) > 0 {
			targetProject.Columns = []ProjectColumn{
				{
					ID:    "all-items",
					Name:  "All Items",
					Items: items,
				},
			}
		} else {
			targetProject.Columns = []ProjectColumn{
				{
					ID:    "empty",
					Name:  "Items",
					Items: []ProjectItem{},
				},
			}
		}
	} else {
		// Successfully got columns
		targetProject.Columns = columns
	}

	return targetProject, nil
}

// ListOrganizationProjectsWithDetails lists all projects for an organization with detailed information
// including columns and items. This is a convenience function that combines ListOrganizationProjects
// and GetOrganizationProjectDetails.
//
// Parameters:
//
//	org: The organization name
//	opts: Optional parameters for filtering and pagination (currently unused in GraphQL implementation)
//	includeDetails: Whether to fetch detailed information (columns and items) for each project
//
// Returns:
//
//	A slice of OrganizationProjectV2 structs with detailed information and an error if the request fails
func (gh *GitHubClient) ListOrganizationProjectsWithDetails(org string, opts *ListOrganizationProjectsOptions, includeDetails bool) ([]*OrganizationProjectV2, error) {
	// First, get the basic list of projects
	projects, err := gh.ListOrganizationProjects(org, opts)
	if err != nil {
		return nil, err
	}

	// If details are not requested, return the basic list
	if !includeDetails {
		return projects, nil
	}

	// Fetch detailed information for each project
	var detailedProjects []*OrganizationProjectV2
	for _, project := range projects {
		detailedProject, err := gh.GetOrganizationProjectDetails(org, project.Number)
		if err != nil {
			// Log the error but continue with other projects
			fmt.Printf("Warning: Failed to get details for project %d: %v\n", project.Number, err)
			// Add the basic project without details
			detailedProjects = append(detailedProjects, project)
			continue
		}
		detailedProjects = append(detailedProjects, detailedProject)
	}

	return detailedProjects, nil
}

// GetOrganizationProjectItemsSimple retrieves items for a specific project using a simplified approach
// that manually constructs and executes GraphQL queries to avoid complex type union issues.
//
// Parameters:
//
//	org: The organization name
//	projectNumber: The project number
//
// Returns:
//
//	A slice of ProjectItem structs and an error if the request fails
func (gh *GitHubClient) GetOrganizationProjectItemsSimple(org string, projectNumber int) ([]ProjectItem, error) {
	if org == "" {
		return nil, fmt.Errorf("organization name cannot be empty")
	}
	if projectNumber <= 0 {
		return nil, fmt.Errorf("project number must be positive")
	}

	// Get the GitHub token from environment
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable is required for GraphQL API")
	}

	// Create HTTP client
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(gh.ctx, src)

	// Use manual HTTP request to avoid GraphQL library issues
	queryTemplate := `{
		"query": "query($org: String!, $number: Int!) { organization(login: $org) { projectV2(number: $number) { items(first: 100) { nodes { id type content { __typename ... on Issue { number title state url repository { name owner { login } } author { login } } ... on PullRequest { number title state url repository { name owner { login } } author { login } } ... on DraftIssue { title body creator { login } } } } } } } }",
		"variables": {"org": "%s", "number": %d}
	}`

	queryBody := fmt.Sprintf(queryTemplate, org, projectNumber)

	// Make HTTP request
	resp, err := httpClient.Post("https://api.github.com/graphql", "application/json", strings.NewReader(queryBody))
	if err != nil {
		return nil, fmt.Errorf("failed to make GraphQL request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GraphQL request failed with status %d", resp.StatusCode)
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode GraphQL response: %w", err)
	}

	// Check for GraphQL errors
	if errors, ok := result["errors"]; ok {
		return nil, fmt.Errorf("GraphQL errors: %v", errors)
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format: missing data")
	}

	orgData, ok := data["organization"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format: missing organization")
	}

	projectData, ok := orgData["projectV2"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format: missing projectV2")
	}

	itemsData, ok := projectData["items"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format: missing items")
	}

	nodes, ok := itemsData["nodes"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format: missing nodes")
	}

	var items []ProjectItem
	for _, nodeInterface := range nodes {
		node, ok := nodeInterface.(map[string]interface{})
		if !ok {
			continue
		}

		item := ProjectItem{
			ID:   getStringFromMap(node, "id"),
			Type: getStringFromMap(node, "type"),
		}

		contentData, ok := node["content"].(map[string]interface{})
		if !ok || contentData == nil {
			continue
		}

		typename := getStringFromMap(contentData, "__typename")
		content := &ProjectItemContent{
			ID:    item.ID,
			Title: getStringFromMap(contentData, "title"),
		}

		switch typename {
		case "Issue", "PullRequest":
			content.Number = getIntFromMap(contentData, "number")
			content.State = getStringFromMap(contentData, "state")
			content.URL = getStringFromMap(contentData, "url")

			if repoData, ok := contentData["repository"].(map[string]interface{}); ok && repoData != nil {
				content.Repository = &struct {
					Name  string `json:"name"`
					Owner struct {
						Login string `json:"login"`
					} `json:"owner"`
				}{
					Name: getStringFromMap(repoData, "name"),
				}
				if ownerData, ok := repoData["owner"].(map[string]interface{}); ok && ownerData != nil {
					content.Repository.Owner.Login = getStringFromMap(ownerData, "login")
				}
			}

			if authorData, ok := contentData["author"].(map[string]interface{}); ok && authorData != nil {
				content.Author = &struct {
					Login string `json:"login"`
				}{Login: getStringFromMap(authorData, "login")}
			}

		case "DraftIssue":
			content.Body = getStringFromMap(contentData, "body")

			if creatorData, ok := contentData["creator"].(map[string]interface{}); ok && creatorData != nil {
				content.Author = &struct {
					Login string `json:"login"`
				}{Login: getStringFromMap(creatorData, "login")}
			}
		}

		item.Content = content
		items = append(items, item)
	}

	return items, nil
}

// GetOrganizationProjectItemsWithColumns retrieves items for a specific project grouped by their actual columns.
// This function fetches project field values to determine which column each item belongs to.
//
// Parameters:
//
//	org: The organization name
//	projectNumber: The project number
//
// Returns:
//
//	A slice of ProjectColumn structs with items grouped by column and an error if the request fails
func (gh *GitHubClient) GetOrganizationProjectItemsWithColumns(org string, projectNumber int) ([]ProjectColumn, error) {
	if org == "" {
		return nil, fmt.Errorf("organization name cannot be empty")
	}
	if projectNumber <= 0 {
		return nil, fmt.Errorf("project number must be positive")
	}

	// Get the GitHub token from environment
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable is required for GraphQL API")
	}

	// Create HTTP client
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(gh.ctx, src)

	// First, get the project fields to understand column structure
	fieldsQueryTemplate := `{
		"query": "query($org: String!, $number: Int!) { organization(login: $org) { projectV2(number: $number) { fields(first: 20) { nodes { ... on ProjectV2SingleSelectField { id name options { id name } } } } } } }",
		"variables": {"org": "%s", "number": %d}
	}`

	fieldsQueryBody := fmt.Sprintf(fieldsQueryTemplate, org, projectNumber)

	// Make HTTP request for fields
	resp, err := httpClient.Post("https://api.github.com/graphql", "application/json", strings.NewReader(fieldsQueryBody))
	if err != nil {
		return nil, fmt.Errorf("failed to make GraphQL request for fields: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GraphQL request for fields failed with status %d", resp.StatusCode)
	}

	// Parse fields response
	var fieldsResult map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&fieldsResult); err != nil {
		return nil, fmt.Errorf("failed to decode GraphQL fields response: %w", err)
	}

	// Extract column information from fields
	columnMap := make(map[string]string) // optionID -> columnName
	allColumns := make(map[string]bool)  // columnName -> exists (to track unique column names)
	statusFieldID := ""

	if data, ok := fieldsResult["data"].(map[string]interface{}); ok {
		if orgData, ok := data["organization"].(map[string]interface{}); ok {
			if projectData, ok := orgData["projectV2"].(map[string]interface{}); ok {
				if fieldsData, ok := projectData["fields"].(map[string]interface{}); ok {
					if nodes, ok := fieldsData["nodes"].([]interface{}); ok {
						for _, nodeInterface := range nodes {
							if node, ok := nodeInterface.(map[string]interface{}); ok {
								fieldName := getStringFromMap(node, "name")
								if fieldName == "Status" { // Common name for status column
									statusFieldID = getStringFromMap(node, "id")
									if options, ok := node["options"].([]interface{}); ok {
										for _, optInterface := range options {
											if opt, ok := optInterface.(map[string]interface{}); ok {
												optionID := getStringFromMap(opt, "id")
												optionName := getStringFromMap(opt, "name")
												columnMap[optionID] = optionName
												allColumns[optionName] = true
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Now get items with their field values
	itemsQueryTemplate := `{
		"query": "query($org: String!, $number: Int!) { organization(login: $org) { projectV2(number: $number) { items(first: 100) { nodes { id type fieldValues(first: 20) { nodes { ... on ProjectV2ItemFieldSingleSelectValue { field { ... on ProjectV2SingleSelectField { id name } } name optionId } ... on ProjectV2ItemFieldTextValue { field { ... on ProjectV2Field { id name } } text } ... on ProjectV2ItemFieldNumberValue { field { ... on ProjectV2Field { id name } } number } ... on ProjectV2ItemFieldDateValue { field { ... on ProjectV2Field { id name } } date } } } content { __typename ... on Issue { number title state url repository { name owner { login } } author { login } } ... on PullRequest { number title state url repository { name owner { login } } author { login } } ... on DraftIssue { title body creator { login } } } } } } } }",
		"variables": {"org": "%s", "number": %d}
	}`

	itemsQueryBody := fmt.Sprintf(itemsQueryTemplate, org, projectNumber)

	// Make HTTP request for items
	resp2, err := httpClient.Post("https://api.github.com/graphql", "application/json", strings.NewReader(itemsQueryBody))
	if err != nil {
		return nil, fmt.Errorf("failed to make GraphQL request for items: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != 200 {
		return nil, fmt.Errorf("GraphQL request for items failed with status %d", resp2.StatusCode)
	}

	// Parse items response
	var itemsResult map[string]interface{}
	if err := json.NewDecoder(resp2.Body).Decode(&itemsResult); err != nil {
		return nil, fmt.Errorf("failed to decode GraphQL items response: %w", err)
	}

	// Check for GraphQL errors
	if errors, ok := itemsResult["errors"]; ok {
		return nil, fmt.Errorf("GraphQL errors: %v", errors)
	}

	data, ok := itemsResult["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format: missing data")
	}

	orgData, ok := data["organization"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format: missing organization")
	}

	projectData, ok := orgData["projectV2"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format: missing projectV2")
	}

	itemsData, ok := projectData["items"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format: missing items")
	}

	nodes, ok := itemsData["nodes"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format: missing nodes")
	}

	// Group items by column
	columnItems := make(map[string][]ProjectItem)

	for _, nodeInterface := range nodes {
		node, ok := nodeInterface.(map[string]interface{})
		if !ok {
			continue
		}

		item := ProjectItem{
			ID:   getStringFromMap(node, "id"),
			Type: getStringFromMap(node, "type"),
		}

		// Extract content
		contentData, ok := node["content"].(map[string]interface{})
		if ok && contentData != nil {
			typename := getStringFromMap(contentData, "__typename")
			content := &ProjectItemContent{
				ID:    item.ID,
				Title: getStringFromMap(contentData, "title"),
			}

			switch typename {
			case "Issue", "PullRequest":
				content.Number = getIntFromMap(contentData, "number")
				content.State = getStringFromMap(contentData, "state")
				content.URL = getStringFromMap(contentData, "url")

				if repoData, ok := contentData["repository"].(map[string]interface{}); ok && repoData != nil {
					content.Repository = &struct {
						Name  string `json:"name"`
						Owner struct {
							Login string `json:"login"`
						} `json:"owner"`
					}{
						Name: getStringFromMap(repoData, "name"),
					}
					if ownerData, ok := repoData["owner"].(map[string]interface{}); ok && ownerData != nil {
						content.Repository.Owner.Login = getStringFromMap(ownerData, "login")
					}
				}

				if authorData, ok := contentData["author"].(map[string]interface{}); ok && authorData != nil {
					content.Author = &struct {
						Login string `json:"login"`
					}{Login: getStringFromMap(authorData, "login")}
				}

			case "DraftIssue":
				content.Body = getStringFromMap(contentData, "body")

				if creatorData, ok := contentData["creator"].(map[string]interface{}); ok && creatorData != nil {
					content.Author = &struct {
						Login string `json:"login"`
					}{Login: getStringFromMap(creatorData, "login")}
				}
			}

			item.Content = content
		}

		// Determine which column this item belongs to
		columnName := "Uncategorized" // Default column

		if fieldValues, ok := node["fieldValues"].(map[string]interface{}); ok {
			if nodes, ok := fieldValues["nodes"].([]interface{}); ok {
				for _, fieldValueInterface := range nodes {
					if fieldValue, ok := fieldValueInterface.(map[string]interface{}); ok {
						if field, ok := fieldValue["field"].(map[string]interface{}); ok {
							fieldID := getStringFromMap(field, "id")
							if fieldID == statusFieldID {
								optionID := getStringFromMap(fieldValue, "optionId")
								if colName, exists := columnMap[optionID]; exists {
									columnName = colName
								}
								break
							}
						}
					}
				}
			}
		}

		columnItems[columnName] = append(columnItems[columnName], item)
	}

	// Convert to ProjectColumn slice, ensuring all defined columns are included
	var columns []ProjectColumn

	// First, add all defined columns (even if empty)
	for columnName := range allColumns {
		columns = append(columns, ProjectColumn{
			ID:    fmt.Sprintf("column-%s", columnName),
			Name:  columnName,
			Items: columnItems[columnName], // Will be empty slice if no items
		})
	}

	// Add any items that don't belong to a defined column
	if uncategorizedItems, exists := columnItems["Uncategorized"]; exists && len(uncategorizedItems) > 0 {
		columns = append(columns, ProjectColumn{
			ID:    "column-uncategorized",
			Name:  "Uncategorized",
			Items: uncategorizedItems,
		})
	}

	return columns, nil
}

// Helper functions for map parsing
func getStringFromMap(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok && val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getIntFromMap(data map[string]interface{}, key string) int {
	if val, ok := data[key]; ok && val != nil {
		if num, ok := val.(float64); ok {
			return int(num)
		}
		if num, ok := val.(int); ok {
			return num
		}
	}
	return 0
}
