package gitproviders

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/google/go-github/v71/github"
)

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
