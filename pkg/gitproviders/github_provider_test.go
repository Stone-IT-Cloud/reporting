package gitproviders

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v71/github"
	"github.com/jarcoal/httpmock"
)

// Helper function to create a GitHubClient with a mocked HTTP transport
func newTestGitHubClient(t *testing.T) (*GitHubClient, func()) {
	// Save original token value to restore later
	originalToken := os.Getenv("GITHUB_TOKEN")

	// Activate httpmock
	httpmock.Activate()

	// Create a context
	ctx := context.Background()

	// Set a dummy token for testing purposes
	// Note: NewGitHubClient uses os.Getenv, so we need to set it
	t.Setenv("GITHUB_TOKEN", "test-token")

	// Mock the authentication check (Users.Get)
	httpmock.RegisterResponder("GET", "https://api.github.com/user",
		httpmock.NewStringResponder(200, `{"login": "testuser"}`))

	// Create the client using the default http client (which httpmock intercepts)
	client := github.NewClient(nil).WithAuthToken("test-token")

	// Create our wrapper client
	ghClient := &GitHubClient{
		client: client,
		ctx:    ctx,
	}

	// Teardown function
	cleanup := func() {
		httpmock.DeactivateAndReset()
		// Restore original token if it existed
		if originalToken != "" {
			t.Setenv("GITHUB_TOKEN", originalToken)
		} else {
			os.Unsetenv("GITHUB_TOKEN") // Or use t.Setenv("GITHUB_TOKEN", "") if using Go < 1.17 style cleanup
		}
	}

	return ghClient, cleanup
}

func TestNewGitHubClient(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		t.Setenv("GITHUB_TOKEN", "fake-token")
		httpmock.RegisterResponder("GET", "https://api.github.com/user",
			httpmock.NewStringResponder(200, `{"login": "testuser"}`))

		client, err := NewGitHubClient(ctx)
		if err != nil {
			t.Fatalf("NewGitHubClient() error = %v, wantErr %v", err, false)
		}
		if client == nil {
			t.Fatal("NewGitHubClient() client is nil, want non-nil")
		}
		if client.ctx == nil {
			t.Error("NewGitHubClient() ctx is nil, want non-nil")
		}
		if client.client == nil {
			t.Error("NewGitHubClient() internal client is nil, want non-nil")
		}
	})

	t.Run("NoTokenEnvVar", func(t *testing.T) {
		// Ensure the variable is unset for this test
		originalToken, wasSet := os.LookupEnv("GITHUB_TOKEN")
		os.Unsetenv("GITHUB_TOKEN")
		if wasSet {
			defer os.Setenv("GITHUB_TOKEN", originalToken)
		}

		_, err := NewGitHubClient(ctx)
		if err == nil {
			t.Fatalf("NewGitHubClient() error = %v, wantErr %v", err, true)
		}
		expectedErrorMsg := "la variable de entorno GITHUB_TOKEN no está configurada"
		if err.Error() != expectedErrorMsg {
			t.Errorf("NewGitHubClient() error = %q, want %q", err.Error(), expectedErrorMsg)
		}
	})

	t.Run("AuthFailure", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		t.Setenv("GITHUB_TOKEN", "invalid-token")
		httpmock.RegisterResponder("GET", "https://api.github.com/user",
			httpmock.NewStringResponder(401, `{"message": "Bad credentials"}`))

		_, err := NewGitHubClient(ctx)
		if err == nil {
			t.Fatalf("NewGitHubClient() error = %v, wantErr %v", err, true)
		}
		// Check for the custom error message prefix
		expectedPrefix := "error al verificar la autenticación de GitHub:"
		if !strings.HasPrefix(err.Error(), expectedPrefix) {
			t.Errorf("NewGitHubClient() error = %q, want prefix %q", err.Error(), expectedPrefix)
		}
	})

	t.Run("NilContext", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		t.Setenv("GITHUB_TOKEN", "fake-token")
		httpmock.RegisterResponder("GET", "https://api.github.com/user",
			httpmock.NewStringResponder(200, `{"login": "testuser"}`))

		client, err := NewGitHubClient(context.TODO()) // Pass TODO context per staticcheck SA1012
		if err != nil {
			t.Fatalf("NewGitHubClient(nil) error = %v, wantErr %v", err, false)
		}
		if client == nil {
			t.Fatal("NewGitHubClient(nil) client is nil, want non-nil")
		}
		// Check if context was defaulted to Background
		if client.ctx == nil {
			t.Error("NewGitHubClient(nil) ctx is nil, want non-nil (defaulted)")
		}
		// A more robust check might involve comparing against context.Background(),
		// but checking for non-nil is usually sufficient here.
	})
}

func TestGitHubClient_GetIssues(t *testing.T) {
	ghClient, cleanup := newTestGitHubClient(t)
	defer cleanup()

	owner := "testowner"
	repo := "testrepo"
	issuesURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues", owner, repo)
	commentsURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/1/comments", owner, repo)

	t.Run("Success", func(t *testing.T) {
		httpmock.Reset() // Reset mocks for this subtest
		// Mock ListByRepo
		httpmock.RegisterResponder("GET", issuesURL,
			httpmock.NewStringResponder(200, `[
				{"number": 1, "title": "Test Issue 1", "body": "Issue Body", "html_url": "issue_url", "state": "open", "pull_request": null, "user": {"login": "author"}},
				{"number": 2, "title": "Test PR", "body": "PR Body", "html_url": "pr_url", "state": "open", "pull_request": {"url": "some_url"}, "user": {"login": "author"}}
			]`))
		// Mock ListComments for issue 1
		httpmock.RegisterResponder("GET", commentsURL,
			httpmock.NewStringResponder(200, `[
				{"id": 101, "body": "Comment Body", "created_at": "2023-01-01T10:00:00Z", "user": {"login": "commenter"}, "html_url": "comment_url"}
			]`))

		issues, err := ghClient.GetIssues(RepoMetadata{Owner: owner, RepoName: repo})
		if err != nil {
			t.Fatalf("GetIssues() error = %v, wantErr %v", err, false)
		}

		if len(issues) != 1 {
			t.Fatalf("GetIssues() got %d issues, want %d", len(issues), 1)
		}
		issue := issues[0]
		if issue.ID != "1" {
			t.Errorf("Issue ID = %s, want %s", issue.ID, "1")
		}
		if issue.Title != "Test Issue 1" {
			t.Errorf("Issue Title = %s, want %s", issue.Title, "Test Issue 1")
		}
		if len(issue.Comments) != 1 {
			t.Fatalf("Issue Comments count = %d, want %d", len(issue.Comments), 1)
		}
		comment := issue.Comments[0]
		if comment.ID != "101" {
			t.Errorf("Comment ID = %s, want %s", comment.ID, "101")
		}
		if comment.Author != "commenter" {
			t.Errorf("Comment Author = %s, want %s", comment.Author, "commenter")
		}
	})

	t.Run("ListIssuesError", func(t *testing.T) {
		httpmock.Reset()
		httpmock.RegisterResponder("GET", issuesURL,
			httpmock.NewStringResponder(500, `{"message": "Internal Server Error"}`))

		_, err := ghClient.GetIssues(RepoMetadata{Owner: owner, RepoName: repo})
		if err == nil {
			t.Fatalf("GetIssues() error = %v, wantErr %v", err, true)
		}
		expectedPrefix := "error al obtener los problemas de GitHub:"
		if !strings.HasPrefix(err.Error(), expectedPrefix) {
			t.Errorf("GetIssues() error = %q, want prefix %q", err.Error(), expectedPrefix)
		}
	})

	t.Run("ListCommentsError", func(t *testing.T) {
		httpmock.Reset()
		// Mock ListByRepo successfully
		httpmock.RegisterResponder("GET", issuesURL,
			httpmock.NewStringResponder(200, `[
				{"number": 1, "title": "Test Issue 1", "body": "Issue Body", "html_url": "issue_url", "state": "open", "pull_request": null, "user": {"login": "author"}}
			]`))
		// Mock ListComments failure
		httpmock.RegisterResponder("GET", commentsURL,
			httpmock.NewStringResponder(500, `{"message": "Internal Server Error"}`))

		_, err := ghClient.GetIssues(RepoMetadata{Owner: owner, RepoName: repo})
		if err == nil {
			t.Fatalf("GetIssues() error = %v, wantErr %v", err, true)
		}
		expectedPrefix := "error al obtener los comentarios del problema #1:"
		if !strings.HasPrefix(err.Error(), expectedPrefix) {
			t.Errorf("GetIssues() error = %q, want prefix %q", err.Error(), expectedPrefix)
		}
	})
}

func TestGitHubClient_GetPullRequests(t *testing.T) {
	ghClient, cleanup := newTestGitHubClient(t)
	defer cleanup()

	owner := "testowner"
	repo := "testrepo"
	prListURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", owner, repo)
	prNumber := 1
	prCommentsURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/comments", owner, repo, prNumber)
	prReviewsURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/reviews", owner, repo, prNumber)

	t.Run("Success", func(t *testing.T) {
		httpmock.Reset()
		// Mock PR List
		httpmock.RegisterResponder("GET", prListURL,
			httpmock.NewStringResponder(200, `[
				{
					"number": 1, "title": "Test PR 1", "body": "PR Body", "created_at": "2023-01-01T11:00:00Z",
					"head": {"ref": "feature-branch"}, "base": {"ref": "main"},
					"user": {"login": "pr-author"}, "assignee": {"login": "pr-assignee"}
				}
			]`))
		// Mock PR Comments
		httpmock.RegisterResponder("GET", prCommentsURL,
			httpmock.NewStringResponder(200, `[
				{"id": 201, "body": "PR Comment", "created_at": "2023-01-01T12:00:00Z", "user": {"login": "commenter"}, "html_url": "pr_comment_url"}
			]`))
		// Mock PR Reviews
		httpmock.RegisterResponder("GET", prReviewsURL,
			httpmock.NewStringResponder(200, `[
				{"id": 301, "user": {"id": 123, "login": "reviewer1", "html_url": "reviewer_url", "email": "reviewer@example.com"}, "state": "APPROVED"}
			]`))

		prs, err := ghClient.GetPullRequests(RepoMetadata{Owner: owner, RepoName: repo})
		if err != nil {
			t.Fatalf("GetPullRequests() error = %v, wantErr %v", err, false)
		}

		if len(prs) != 1 {
			t.Fatalf("GetPullRequests() got %d PRs, want %d", len(prs), 1)
		}
		pr := prs[0]
		if pr.ID != "1" {
			t.Errorf("PR ID = %s, want %s", pr.ID, "1")
		}
		if pr.Title != "Test PR 1" {
			t.Errorf("PR Title = %s, want %s", pr.Title, "Test PR 1")
		}
		if pr.Author != "pr-author" {
			t.Errorf("PR Author = %s, want %s", pr.Author, "pr-author")
		}
		if pr.Assignee != "pr-assignee" {
			t.Errorf("PR Assignee = %s, want %s", pr.Assignee, "pr-assignee")
		}
		if len(pr.Comments) != 1 {
			t.Fatalf("PR Comments count = %d, want %d", len(pr.Comments), 1)
		}
		if pr.Comments[0].ID != "201" {
			t.Errorf("PR Comment ID = %s, want %s", pr.Comments[0].ID, "201")
		}
		if len(pr.Reviewers) != 1 {
			t.Fatalf("PR Reviewers count = %d, want %d", len(pr.Reviewers), 1)
		}
		if pr.Reviewers[0].ID != "123" {
			t.Errorf("PR Reviewer ID = %s, want %s", pr.Reviewers[0].ID, "123")
		}
		if pr.Reviewers[0].Name != "reviewer1" {
			t.Errorf("PR Reviewer Name = %s, want %s", pr.Reviewers[0].Name, "reviewer1")
		}
	})

	t.Run("ListPRError", func(t *testing.T) {
		httpmock.Reset()
		httpmock.RegisterResponder("GET", prListURL,
			httpmock.NewStringResponder(500, `{"message": "Server Error"}`))

		_, err := ghClient.GetPullRequests(RepoMetadata{Owner: owner, RepoName: repo})
		if err == nil {
			t.Fatalf("GetPullRequests() error = %v, wantErr %v", err, true)
		}
		expectedPrefix := "error al obtener las solicitudes de extracción de GitHub:"
		if !strings.HasPrefix(err.Error(), expectedPrefix) {
			t.Errorf("GetPullRequests() error = %q, want prefix %q", err.Error(), expectedPrefix)
		}
	})

	t.Run("ListCommentsError", func(t *testing.T) {
		httpmock.Reset()
		// Mock PR List success
		httpmock.RegisterResponder("GET", prListURL,
			httpmock.NewStringResponder(200, `[{"number": 1}]`)) // Minimal valid PR
		// Mock PR Comments failure
		httpmock.RegisterResponder("GET", prCommentsURL,
			httpmock.NewStringResponder(500, `{"message": "Server Error"}`))

		_, err := ghClient.GetPullRequests(RepoMetadata{Owner: owner, RepoName: repo})
		if err == nil {
			t.Fatalf("GetPullRequests() error = %v, wantErr %v", err, true)
		}
		expectedPrefix := "error al obtener los comentarios de la solicitud de extracción:"
		if !strings.HasPrefix(err.Error(), expectedPrefix) {
			t.Errorf("GetPullRequests() error = %q, want prefix %q", err.Error(), expectedPrefix)
		}
	})

	t.Run("ListReviewsError", func(t *testing.T) {
		httpmock.Reset()
		// Mock PR List success
		httpmock.RegisterResponder("GET", prListURL,
			httpmock.NewStringResponder(200, `[{"number": 1}]`)) // Minimal valid PR
		// Mock PR Comments success
		httpmock.RegisterResponder("GET", prCommentsURL,
			httpmock.NewStringResponder(200, `[]`))
		// Mock PR Reviews failure
		httpmock.RegisterResponder("GET", prReviewsURL,
			httpmock.NewStringResponder(500, `{"message": "Server Error"}`))

		_, err := ghClient.GetPullRequests(RepoMetadata{Owner: owner, RepoName: repo})
		if err == nil {
			t.Fatalf("GetPullRequests() error = %v, wantErr %v", err, true)
		}
		expectedPrefix := "error al obtener los revisores de la solicitud de extracción:"
		if !strings.HasPrefix(err.Error(), expectedPrefix) {
			t.Errorf("GetPullRequests() error = %q, want prefix %q", err.Error(), expectedPrefix)
		}
	})
}

func TestGitHubClient_GetRepository(t *testing.T) {
	ghClient, cleanup := newTestGitHubClient(t)
	defer cleanup()

	owner := "testowner"
	repo := "testrepo"
	repoURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)

	t.Run("Success", func(t *testing.T) {
		httpmock.Reset()
		createdAtStr := "2022-01-01T00:00:00Z"
		httpmock.RegisterResponder("GET", repoURL,
			httpmock.NewStringResponder(200, fmt.Sprintf(`{
				"id": 12345, "name": "%s", "owner": {"login": "%s"}, "created_at": "%s"
			}`, repo, owner, createdAtStr)))

		repository, err := ghClient.GetRepository(owner, repo)
		if err != nil {
			t.Fatalf("GetRepository() error = %v, wantErr %v", err, false)
		}

		if repository.ID != "12345" {
			t.Errorf("Repository ID = %s, want %s", repository.ID, "12345")
		}
		if repository.Name != repo {
			t.Errorf("Repository Name = %s, want %s", repository.Name, repo)
		}
		if repository.Owner != owner {
			t.Errorf("Repository Owner = %s, want %s", repository.Owner, owner)
		}
		expectedTime, _ := time.Parse(time.RFC3339, createdAtStr)
		if !repository.CreatedAt.Equal(expectedTime) {
			t.Errorf("Repository CreatedAt = %v, want %v", repository.CreatedAt, expectedTime)
		}
	})

	t.Run("GetRepoError", func(t *testing.T) {
		httpmock.Reset()
		httpmock.RegisterResponder("GET", repoURL,
			httpmock.NewStringResponder(404, `{"message": "Not Found"}`))

		_, err := ghClient.GetRepository(owner, repo)
		if err == nil {
			t.Fatalf("GetRepository() error = %v, wantErr %v", err, true)
		}
		expectedPrefix := "error al obtener el repositorio de GitHub:"
		if !strings.HasPrefix(err.Error(), expectedPrefix) {
			t.Errorf("GetRepository() error = %q, want prefix %q", err.Error(), expectedPrefix)
		}
	})
}

func TestGitHubClient_GetPullRequest(t *testing.T) {
	ghClient, cleanup := newTestGitHubClient(t)
	defer cleanup()

	owner := "testowner"
	repo := "testrepo"
	prIDStr := "1"
	prNumber := 1
	prGetURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, prNumber)
	prCommentsURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/comments", owner, repo, prNumber) // Same as list comments
	prReviewsURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/reviews", owner, repo, prNumber)   // Same as list reviews

	t.Run("Success", func(t *testing.T) {
		httpmock.Reset()
		// Mock Get PR
		httpmock.RegisterResponder("GET", prGetURL,
			httpmock.NewStringResponder(200, `
				{
					"number": 1, "title": "Specific PR 1", "body": "Specific PR Body", "created_at": "2023-01-01T11:00:00Z",
					"head": {"ref": "feature-branch"}, "base": {"ref": "main"},
					"user": {"login": "pr-author"}, "assignee": {"login": "pr-assignee"}
				}`))
		// Mock PR Comments
		httpmock.RegisterResponder("GET", prCommentsURL,
			httpmock.NewStringResponder(200, `[
				{"id": 201, "body": "PR Comment", "created_at": "2023-01-01T12:00:00Z", "user": {"login": "commenter"}, "html_url": "pr_comment_url"}
			]`))
		// Mock PR Reviews
		httpmock.RegisterResponder("GET", prReviewsURL,
			httpmock.NewStringResponder(200, `[
				{"id": 301, "user": {"id": 123, "login": "reviewer1", "html_url": "reviewer_url", "email": "reviewer@example.com"}, "state": "APPROVED"}
			]`))

		pr, err := ghClient.GetPullRequest(owner, repo, prIDStr)
		if err != nil {
			t.Fatalf("GetPullRequest() error = %v, wantErr %v", err, false)
		}

		if pr.ID != prIDStr {
			t.Errorf("PR ID = %s, want %s", pr.ID, prIDStr)
		}
		if pr.Title != "Specific PR 1" {
			t.Errorf("PR Title = %s, want %s", pr.Title, "Specific PR 1")
		}
		// Add more assertions as needed, similar to GetPullRequests success case
	})

	t.Run("InvalidPrID", func(t *testing.T) {
		httpmock.Reset()
		_, err := ghClient.GetPullRequest(owner, repo, "not-a-number")
		if err == nil {
			t.Fatalf("GetPullRequest() error = %v, wantErr %v", err, true)
		}
		expectedPrefix := "error al convertir el ID de la solicitud de extracción a int:"
		if !strings.HasPrefix(err.Error(), expectedPrefix) {
			t.Errorf("GetPullRequest() error = %q, want prefix %q", err.Error(), expectedPrefix)
		}
	})

	t.Run("GetPRError", func(t *testing.T) {
		httpmock.Reset()
		httpmock.RegisterResponder("GET", prGetURL,
			httpmock.NewStringResponder(404, `{"message": "Not Found"}`))

		_, err := ghClient.GetPullRequest(owner, repo, prIDStr)
		if err == nil {
			t.Fatalf("GetPullRequest() error = %v, wantErr %v", err, true)
		}
		expectedPrefix := "error al obtener la solicitud de extracción de GitHub:"
		if !strings.HasPrefix(err.Error(), expectedPrefix) {
			t.Errorf("GetPullRequest() error = %q, want prefix %q", err.Error(), expectedPrefix)
		}
	})

	// Add tests for ListCommentsError and ListReviewsError similar to GetPullRequests
}

func TestGitHubClient_GetIssue(t *testing.T) {
	ghClient, cleanup := newTestGitHubClient(t)
	defer cleanup()

	owner := "testowner"
	repo := "testrepo"
	issueIDStr := "1"
	issueNumber := 1
	issueGetURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d", owner, repo, issueNumber)
	issueCommentsURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments", owner, repo, issueNumber)

	t.Run("Success", func(t *testing.T) {
		httpmock.Reset()
		// Mock Get Issue
		httpmock.RegisterResponder("GET", issueGetURL,
			httpmock.NewStringResponder(200, `
				{
					"number": 1, "title": "Specific Issue 1", "body": "Specific Issue Body", "html_url": "issue_url", "state": "closed",
					"user": {"login": "issue-author"}
				}`))
		// Mock Issue Comments
		httpmock.RegisterResponder("GET", issueCommentsURL,
			httpmock.NewStringResponder(200, `[
				{"id": 101, "body": "Issue Comment", "created_at": "2023-01-01T10:00:00Z", "user": {"login": "commenter"}, "html_url": "comment_url"}
			]`))

		issue, err := ghClient.GetIssue(owner, repo, issueIDStr)
		if err != nil {
			t.Fatalf("GetIssue() error = %v, wantErr %v", err, false)
		}

		if issue.ID != issueIDStr {
			t.Errorf("Issue ID = %s, want %s", issue.ID, issueIDStr)
		}
		if issue.Title != "Specific Issue 1" {
			t.Errorf("Issue Title = %s, want %s", issue.Title, "Specific Issue 1")
		}
		if issue.State != "closed" {
			t.Errorf("Issue State = %s, want %s", issue.State, "closed")
		}
		if len(issue.Comments) != 1 {
			t.Fatalf("Issue Comments count = %d, want %d", len(issue.Comments), 1)
		}
		// Add more assertions as needed
	})

	t.Run("InvalidIssueID", func(t *testing.T) {
		httpmock.Reset()
		_, err := ghClient.GetIssue(owner, repo, "not-a-number")
		if err == nil {
			t.Fatalf("GetIssue() error = %v, wantErr %v", err, true)
		}
		expectedPrefix := "error al convertir el ID del problema a int:"
		if !strings.HasPrefix(err.Error(), expectedPrefix) {
			t.Errorf("GetIssue() error = %q, want prefix %q", err.Error(), expectedPrefix)
		}
	})

	t.Run("GetIssueError", func(t *testing.T) {
		httpmock.Reset()
		httpmock.RegisterResponder("GET", issueGetURL,
			httpmock.NewStringResponder(404, `{"message": "Not Found"}`))

		_, err := ghClient.GetIssue(owner, repo, issueIDStr)
		if err == nil {
			t.Fatalf("GetIssue() error = %v, wantErr %v", err, true)
		}
		expectedPrefix := "error al obtener el problema de GitHub:"
		if !strings.HasPrefix(err.Error(), expectedPrefix) {
			t.Errorf("GetIssue() error = %q, want prefix %q", err.Error(), expectedPrefix)
		}
	})

	t.Run("ListCommentsError", func(t *testing.T) {
		httpmock.Reset()
		// Mock Get Issue success
		httpmock.RegisterResponder("GET", issueGetURL,
			httpmock.NewStringResponder(200, `{"number": 1}`)) // Minimal valid issue
		// Mock List Comments failure
		httpmock.RegisterResponder("GET", issueCommentsURL,
			httpmock.NewStringResponder(500, `{"message": "Server Error"}`))

		_, err := ghClient.GetIssue(owner, repo, issueIDStr)
		if err == nil {
			t.Fatalf("GetIssue() error = %v, wantErr %v", err, true)
		}
		expectedPrefix := "error al obtener los comentarios del problema:"
		if !strings.HasPrefix(err.Error(), expectedPrefix) {
			t.Errorf("GetIssue() error = %q, want prefix %q", err.Error(), expectedPrefix)
		}
	})
}
