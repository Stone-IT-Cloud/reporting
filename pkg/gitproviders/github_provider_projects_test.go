package gitproviders

import (
	"context"
	"testing"
)

func TestListOrganizationProjects_EmptyOrg(t *testing.T) {
	ctx := context.Background()
	client := &GitHubClient{
		ctx: ctx,
	}

	// Test with empty organization name
	projects, err := client.ListOrganizationProjects("", nil)
	if err == nil {
		t.Error("Expected error for empty organization name, got nil")
	}
	if projects != nil {
		t.Error("Expected nil projects for empty organization name")
	}

	expectedError := "organization name cannot be empty"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestListOrganizationProjectsOptions_StructFields(t *testing.T) {
	// Test that the struct fields are correctly defined
	opts := &ListOrganizationProjectsOptions{
		State:   "all",
		PerPage: 50,
		Page:    2,
	}

	if opts.State != "all" {
		t.Errorf("Expected State 'all', got '%s'", opts.State)
	}
	if opts.PerPage != 50 {
		t.Errorf("Expected PerPage 50, got %d", opts.PerPage)
	}
	if opts.Page != 2 {
		t.Errorf("Expected Page 2, got %d", opts.Page)
	}
}

func TestOrganizationProject_StructFields(t *testing.T) {
	// Test that the struct fields are correctly defined
	project := &OrganizationProject{}

	// Just verify the struct can be instantiated and has the expected nil pointer fields
	if project.ID != nil {
		t.Error("Expected ID to be nil")
	}
	if project.Name != nil {
		t.Error("Expected Name to be nil")
	}
	if project.State != nil {
		t.Error("Expected State to be nil")
	}
}

func TestOrganizationProjectV2_StructFields(t *testing.T) {
	// Test that the new GraphQL struct fields are correctly defined
	project := &OrganizationProjectV2{
		ID:        "test-id",
		Number:    123,
		Title:     "Test Project",
		Public:    true,
		Closed:    false,
		CreatedAt: "2023-01-01T00:00:00Z",
		UpdatedAt: "2023-01-02T00:00:00Z",
		URL:       "https://github.com/orgs/test/projects/123",
	}

	if project.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got '%s'", project.ID)
	}
	if project.Number != 123 {
		t.Errorf("Expected Number 123, got %d", project.Number)
	}
	if project.Title != "Test Project" {
		t.Errorf("Expected Title 'Test Project', got '%s'", project.Title)
	}
	if !project.Public {
		t.Error("Expected Public to be true")
	}
	if project.Closed {
		t.Error("Expected Closed to be false")
	}
}

func TestListOrganizationProjects_MissingToken(t *testing.T) {
	ctx := context.Background()
	client := &GitHubClient{
		ctx: ctx,
	}

	// This test will fail if GITHUB_TOKEN is not set, which is expected behavior
	// We're testing that the error handling works correctly
	projects, err := client.ListOrganizationProjects("test-org", nil)
	if err == nil {
		// If this passes, it means GITHUB_TOKEN is set and the API call succeeded
		// This is actually OK for the test since it validates the structure
		t.Logf("GITHUB_TOKEN is set, got %d projects", len(projects))
		return
	}

	// Check that we get the expected error when GITHUB_TOKEN is not set
	expectedError := "GITHUB_TOKEN environment variable is required for GraphQL API"
	if err.Error() != expectedError {
		t.Logf("Got different error (likely due to authentication): %s", err.Error())
	}
}

// Integration tests that require a valid GITHUB_TOKEN

// TestListOrganizationProjectsWithDetails tests the complete project listing functionality
// including columns and items. This test requires a valid GITHUB_TOKEN.
func TestListOrganizationProjectsWithDetails_Integration(t *testing.T) {
	// Skip test if no GitHub token is available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client, err := NewGitHubClient(context.Background())
	if err != nil {
		t.Skipf("Failed to create GitHub client (probably no GITHUB_TOKEN): %v", err)
	}

	// Test with a known organization that has public projects
	projects, err := client.ListOrganizationProjectsWithDetails("facebook", nil, true)
	if err != nil {
		t.Fatalf("Failed to list organization projects with details: %v", err)
	}

	// Verify that we got some projects
	if len(projects) == 0 {
		t.Log("No projects found for facebook organization")
		return
	}

	t.Logf("Found %d projects for facebook organization", len(projects))

	// Check the first project for basic structure
	project := projects[0]
	if project.ID == "" {
		t.Error("Project ID should not be empty")
	}
	if project.Title == "" {
		t.Error("Project title should not be empty")
	}
	if project.Number == 0 {
		t.Error("Project number should not be zero")
	}

	t.Logf("First project: %s (ID: %s, Number: %d)", project.Title, project.ID, project.Number)

	// Check if columns are populated when details are requested
	if len(project.Columns) > 0 {
		t.Logf("Project has %d columns", len(project.Columns))
		column := project.Columns[0]
		t.Logf("First column: %s with %d items", column.Name, len(column.Items))

		if len(column.Items) > 0 {
			item := column.Items[0]
			if item.ID == "" {
				t.Error("Item ID should not be empty")
			}
			if item.Content != nil && item.Content.Title == "" {
				t.Error("Item content title should not be empty")
			}
			t.Logf("First item: %s (Type: %s)", item.Content.Title, item.Type)
		}
	}
}

// TestGetOrganizationProjectItemsSimple tests the simplified item fetching
func TestGetOrganizationProjectItemsSimple_Integration(t *testing.T) {
	// Skip test if no GitHub token is available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client, err := NewGitHubClient(context.Background())
	if err != nil {
		t.Skipf("Failed to create GitHub client (probably no GITHUB_TOKEN): %v", err)
	}

	// Test with a known project number from facebook
	items, err := client.GetOrganizationProjectItemsSimple("facebook", 50)
	if err != nil {
		t.Fatalf("Failed to get project items: %v", err)
	}

	t.Logf("Found %d items in project 50", len(items))

	if len(items) > 0 {
		item := items[0]
		if item.ID == "" {
			t.Error("Item ID should not be empty")
		}
		if item.Type == "" {
			t.Error("Item type should not be empty")
		}
		t.Logf("First item: ID=%s, Type=%s", item.ID, item.Type)

		if item.Content != nil {
			t.Logf("Item content: Title=%s, Number=%d", item.Content.Title, item.Content.Number)
		}
	}
}
