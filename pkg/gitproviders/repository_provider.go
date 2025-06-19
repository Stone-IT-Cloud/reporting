package gitproviders

import "time"

// Repository represents a code repository hosted on a Git provider.
// It contains basic information about the repository, such as its unique ID,
// name, owner, description, and creation timestamp.
type Repository struct {
	ID          string
	Name        string
	Owner       string
	Description string
	CreatedAt   time.Time
}

// Issue represents an issue within a version control system repository.
// It encapsulates details commonly found in issue tracking systems.
//
// Fields:
//
//	ID:        Unique identifier for the issue (e.g., issue number or UUID).
//	Title:     The title or summary of the issue.
//	Body:      The detailed description or content of the issue.
//	CreatedAt: The timestamp indicating when the issue was created.
//	URL:       The web URL pointing directly to the issue page.
//	State:     The current status of the issue (e.g., "open", "closed").
//	Comments:  A slice containing comments made on the issue.
type Issue struct {
	ID        string
	Title     string
	Body      string
	CreatedAt time.Time
	URL       string
	State     string
	Comments  []Comment
}

// Comment represents a comment on a pull request or issue.
type Comment struct {
	ID        string
	Body      string
	CreatedAt time.Time
	Author    string
	URL       string
}

// PullRequest represents a pull request in a version control system.
// It encapsulates details such as its unique identifier, title, description,
// current state, creation timestamp, source and target branches, author,
// assignee, associated comments, and assigned reviewers.
//
// Fields:
//
//	ID: Unique identifier for the pull request.
//	Title: The title of the pull request.
//	Body: The description or body content of the pull request.
//	State: The current state of the pull request (e.g., "open", "closed", "merged").
//	CreatedAt: The timestamp indicating when the pull request was created.
//	SourceBranch: The name of the branch where the changes originated.
//	TargetBranch: The name of the branch the changes are intended to be merged into.
//	Author: The username or identifier of the user who created the pull request.
//	Assignee: The username or identifier of the user assigned to handle the pull request.
//	Comments: A slice containing comments made on the pull request. Assumes a Comment struct exists.
//	Reviewers: A slice containing information about the reviewers assigned to the pull request. Assumes a Reviewer struct exists.
type PullRequest struct {
	ID           string
	Title        string
	Body         string
	State        string
	CreatedAt    time.Time
	SourceBranch string
	TargetBranch string
	Author       string
	Assignee     string
	Comments     []Comment
	Reviewers    []Reviewer
}

// Reviewer represents a code reviewer in a version control system.
// It contains basic information about the reviewer, such as their unique identifier,
// name, profile URL, and email address.
type Reviewer struct {
	ID         string
	Name       string
	ProfileURL string
	Email      string
}

// GitServiceProvider defines the interface for interacting with a Git service provider
// like GitHub or GitLab. It provides methods to retrieve information about
// repositories, pull requests, and issues.
type GitServiceProvider interface {
	GetRepository(owner, repo string) (Repository, error)
	GetPullRequest(owner, repo, prID string) (PullRequest, error)
	GetPullRequests(metadata RepoMetadata) ([]PullRequest, error)
	GetIssues(metadata RepoMetadata) ([]Issue, error)
	GetIssue(owner, repo, issueID string) (Issue, error)
}
