package activityreport

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	gp "github.com/Stone-IT-Cloud/reporting/pkg/gitproviders"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
	"gopkg.in/yaml.v3"
)

// Config contains the configuration parameters for the activity report generation.
type Config struct {
	ChunkSize       int    `yaml:"chunk_size"`
	ProjectID       string `yaml:"project_id"`
	Location        string `yaml:"location"`
	GeminiModel     string `yaml:"gemini_model"`
	CredentialsFile string `yaml:"credentials_file"`
}

// LoadConfig reads and parses the YAML configuration file.
func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		return nil, fmt.Errorf("config path cannot be empty")
	}

	// Clean the path to prevent directory traversal issues somewhat
	cleanedPath := filepath.Clean(configPath)

	// Check if file exists
	if _, err := os.Stat(cleanedPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found at path: %s", cleanedPath)
	}

	// #nosec G304 -- User provides the config path via flag, accept the risk for CLI tool.
	yamlFile, err := os.ReadFile(cleanedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", cleanedPath, err)
	}

	var cfg Config
	err = yaml.Unmarshal(yamlFile, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config YAML from %s: %w", cleanedPath, err)
	}

	// Basic validation
	if cfg.ChunkSize <= 0 {
		return nil, fmt.Errorf("chunk_size must be positive in config")
	}
	if cfg.ProjectID == "" {
		return nil, fmt.Errorf("project_id cannot be empty in config")
	}
	if cfg.Location == "" {
		return nil, fmt.Errorf("location cannot be empty in config")
	}
	if cfg.GeminiModel == "" {
		return nil, fmt.Errorf("gemini_model cannot be empty in config")
	}

	return &cfg, nil
}

// CommitLog represents the structure expected for each commit in the input JSON array.
// Using map[string]interface{} for flexibility from gitlogs output.
type CommitLog map[string]interface{}

// #nosec G101 -- This is the name of an environment variable, not a credential itself.
const apiKeyEnvVar = "VERTEX_AI_API_KEY" // Environment variable for the API key

// #nosec G101 -- This is the name of an environment variable, not a credential itself.
const credentialsFileEnvVar = "GOOGLE_APPLICATION_CREDENTIALS" // Environment variable for credentials file

// GenerateReport takes JSON commit logs, processes them in chunks, generates an AI report,
// saves it to a file, and prints it to stdout.

// GenerateReport generates a weekly activity report based on provided Git commit logs.
// The report is generated using a Gemini AI model and saved in Markdown format.
//
// Parameters:
//   - ctx: The context for managing request deadlines and cancellations.
//   - gitLogsJSON: A JSON string containing a list of Git commit logs.
//   - configPath: The file path to the configuration file containing settings for the report generation.
//   - outputPath: The file path where the generated report will be saved.
//
// Behavior:
//  1. Loads the configuration from the specified configPath.
//  2. Sets up authentication using either a credentials file or an API key.
//  3. Parses the provided gitLogsJSON into a list of commit logs.
//  4. Initializes a Gemini AI client using the generative-ai-go library.
//  5. Sends an initial prompt to the Gemini AI model to set the context for report generation.
//  6. Processes the commit logs in chunks, sending them to the AI model for report generation.
//  7. Extracts the final AI-generated response and formats it as a Markdown report.
//  8. Saves the generated report to the specified outputPath and prints it to the console.
//
// Returns:
//   - An error if any step in the process fails, or nil if the report is successfully generated.
//
// Notes:
//   - If no commit logs are provided or the AI model does not generate a response, an empty report is created.
//   - The function ensures that non-technical stakeholders can understand the report by avoiding technical jargon.
func GenerateReport(ctx context.Context, gitLogsJSON string, issues []gp.Issue, configPath string, outputPath string) (string, error) {
	// --- 1. Load Configuration ---
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to load configuration: %w", err)
	}

	// --- 2. Setup Authentication ---
	var clientOpts []option.ClientOption

	// Check for credentials file in config
	if cfg.CredentialsFile != "" {
		// Use credentials file from config
		clientOpts = append(clientOpts, option.WithCredentialsFile(cfg.CredentialsFile))
	} else {
		// Check for credentials file in environment variable
		credentialsPath := os.Getenv(credentialsFileEnvVar)
		if credentialsPath != "" {
			clientOpts = append(clientOpts, option.WithCredentialsFile(credentialsPath))
		} else {
			// Fall back to API key as last resort
			apiKey := os.Getenv(apiKeyEnvVar)
			if apiKey == "" {
				return "", fmt.Errorf("no authentication method available: neither credentials file specified in config/environment nor %s env var set", apiKeyEnvVar)
			}
			clientOpts = append(clientOpts, option.WithAPIKey(apiKey))
		}
	}

	// --- 3. Parse Input JSON ---
	var logs []CommitLog
	// Use json.Unmarshal directly on the string converted to bytes
	if err := json.Unmarshal([]byte(gitLogsJSON), &logs); err != nil {
		// Handle empty JSON array specifically - not an error, just no logs
		if strings.TrimSpace(gitLogsJSON) == "[]" {
			fmt.Println("No commit logs provided or found in the input JSON. Skipping report generation.")
			if outputPath != "" {
				// Optionally write an empty report file or do nothing
				if err := os.WriteFile(outputPath, []byte("# Activity Report\n\nNo activity found in the provided logs.\n"), 0o600); err != nil {
					return "", fmt.Errorf("failed to write empty report file %s: %w", outputPath, err)
				}
				fmt.Println("Generated empty report file:", outputPath)
				return "", nil
			}
		}
		return "", fmt.Errorf("failed to unmarshal git logs JSON: %w", err)
	}

	if len(logs) == 0 {
		fmt.Println("No commit logs found after parsing. Skipping report generation.")
		if outputPath != "" {
			_ = os.WriteFile(outputPath, []byte("# Activity Report\n\nNo activity found in the provided logs.\n"), 0o600)
			fmt.Println("Generated empty report file:", outputPath)
			return "", nil
		}
	}

	// --- 5. Initialize Gemini Client ---
	// Creating a new client with the generative-ai-go library
	client, err := genai.NewClient(ctx, clientOpts...)
	if err != nil {
		return "", fmt.Errorf("failed to initialize Gemini AI client: %w", err)
	}
	defer client.Close()

	// Get the model
	model := client.GenerativeModel(cfg.GeminiModel)

	fmt.Printf("Initialized Gemini model %s\n", cfg.GeminiModel)

	// --- 5. Start Chat Session & Send Initial Prompt ---
	// Create a new chat session
	cs := model.StartChat()

	initialPrompt := `
act as a project manager, expert on IT project. 
After this prompt you will receive one or more json lists of objects with the commits sent to a git repository in separate prompts. 
And right after that you migth receive a json list of issues related to the project in different prompts.
Read each json and prepare a weekly activity report that will be sent to the client and other stackholders. 
Some of them are not technical persons, so keep a formal tone avoiding jargons. 
Please write the report in markdown format. 
Only return the report without any other text or explanation.
Use the following example as a template for the report. If you don't have enough information to fill all the fields, just ommit the field or section, do not show any placeholder or empty field:
# Weekly Project Status Report Data

## Project Identification
Project Name: [Insert Project Name Here]
Reporting Period: [Start Date of Week] - [End Date of Week]
Report Date: [Date Report is Issued]

## Project Health
Overall Project Status: [Green / Yellow / Red]
Status Rationale: [Brief explanation, especially for Yellow/Red status]

## Executive Summary / Highlights
- [Highlight 1]
- [Highlight 2]
- [Highlight 3]

## Accomplishments This Period
- [Accomplishment 1: Developed...]
- [Accomplishment 2: Deployed...]
- [Accomplishment 3: Conducted...]
- [Accomplishment 4: Resolved...]

## Planned Activities Next Period
- [Planned Activity 1: Begin...]
- [Planned Activity 2: Prepare...]
- [Planned Activity 3: Schedule...]
- [Planned Activity 4: Integrate...]

## Key Risks, Issues, and Blockers
### Item 1
- Type: [Risk / Issue / Blocker]
- Description: [Briefly describe]
- Impact: [Effect on schedule, scope, quality]
- Mitigation/Action Plan: [What's being done, who owns it]
- Status: [New, In Progress, Monitoring, Resolved, Escalated]
### Item 2 (if applicable)
- Type: [Risk / Issue / Blocker]
- Description: [Briefly describe]
- Impact: [Effect on schedule, scope, quality]
- Mitigation/Action Plan: [What's being done, who owns it]
- Status: [New, In Progress, Monitoring, Resolved, Escalated]
### (Add more items as needed)

## Scope Changes 
- [Change 1: CR#, Description, Status - e.g., Approved/Submitted]
- [Change 2: ...]

## Milestone Tracking
### Milestone 1
- Name: [e.g., Phase 1 Delivery]
- Target Date: [e.g., June 15th, 2025]
- Status: [On Track / At Risk / Delayed]
- Notes: [Brief relevant notes]
### Milestone 2
- Name: [e.g., UAT Sign-off]
- Target Date: [e.g., July 1st, 2025]
- Status: [On Track / At Risk / Delayed]
- Notes: [Brief relevant notes]
### (Add more milestones as needed)
`

	fmt.Println("Sending initial prompt to Gemini...")

	// Send the initial prompt
	if _, err := cs.SendMessage(ctx, genai.Text(initialPrompt)); err != nil {
		return "", fmt.Errorf("failed to send initial prompt to Gemini: %w", err)
	}

	// --- 6. Chunk Data and Send Prompts ---
	fmt.Printf("Processing %d logs in chunks of %d...\n", len(logs), cfg.ChunkSize)
	totalChunks := int(math.Ceil(float64(len(logs)) / float64(cfg.ChunkSize)))

	var commitsFinalResp *genai.GenerateContentResponse
	for i := 0; i < len(logs); i += cfg.ChunkSize {
		end := i + cfg.ChunkSize
		if end > len(logs) {
			end = len(logs)
		}
		chunk := logs[i:end]

		// Marshal chunk back to JSON
		chunkJSONBytes, err := json.MarshalIndent(chunk, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal commit chunk %d/%d to JSON: %w", (i/cfg.ChunkSize)+1, totalChunks, err)
		}
		chunkJSONString := string(chunkJSONBytes)

		fmt.Printf("Sending chunk %d/%d (%d commits) to Gemini...\n", (i/cfg.ChunkSize)+1, totalChunks, len(chunk))

		// Send chunk JSON as the next prompt in the chat session
		tempResp, err := cs.SendMessage(ctx, genai.Text(chunkJSONString))
		if err != nil {
			return "", fmt.Errorf("failed to send chunk %d/%d to Gemini: %w", (i/cfg.ChunkSize)+1, totalChunks, err)
		}
		commitsFinalResp = tempResp // Store the last response
		if commitsFinalResp != nil {
			log.Println("Response from Gemini after sending latest commit chunk:", extractTextFromResponse(commitsFinalResp))
		} else {
			log.Println("No response received from Gemini after sending commit chunks.")
		}
	}

	var issuesFinalResp *genai.GenerateContentResponse
	if len(issues) > 0 {
		fmt.Printf("Sending list of [%d] issues to Gemini...\n", len(issues))
		_, err := cs.SendMessage(ctx, genai.Text("Now you will receive a json list of issues related to the project."))
		if err != nil {
			return "", fmt.Errorf("failed to send issues introduction message to Gemini: %w", err)
		}

		for i := 0; i < len(issues); i += cfg.ChunkSize {
			end := i + cfg.ChunkSize
			if end > len(issues) {
				end = len(issues)
			}
			chunk := issues[i:end]
			// Marshal chunk back to JSON
			chunkJSONBytes, err := json.MarshalIndent(chunk, "", "  ")
			if err != nil {
				return "", fmt.Errorf("failed to marshal issues chunk %d/%d to JSON: %w", (i/cfg.ChunkSize)+1, totalChunks, err)
			}
			chunkJSONString := string(chunkJSONBytes)
			fmt.Printf("Sending issues chunk %d/%d (%d issues) to Gemini...\n", (i/cfg.ChunkSize)+1, totalChunks, len(chunk))
			// Send chunk JSON as the next prompt in the chat session
			tempResp, err := cs.SendMessage(ctx, genai.Text(chunkJSONString))
			if err != nil {
				return "", fmt.Errorf("failed to send issues chunk %d/%d to Gemini: %w", (i/cfg.ChunkSize)+1, totalChunks, err)
			}
			issuesFinalResp = tempResp // Store the last response
		}
		if issuesFinalResp != nil {
			log.Println("Response from Gemini after sending latest issues chunk:", extractTextFromResponse(issuesFinalResp))
		} else {
			log.Println("No response received from Gemini after sending issues chunks.")
		}
	}

	// --- 7. Send Final Prompt ---
	fmt.Println("Sending final prompt to Gemini...")
	finalPrompt := `
Now that you have received the commits and issues, please prepare a weekly activity report that will be sent to the client and other stakeholders.
Use the issues list, if provided, for calculating project's lifecycle time of each issue and the velocity of the team.
Some of them are not technical persons, so keep a formal tone avoiding jargons. Please analyze the commits and issues and summarize the key points, challenges, and resolutions.
Please write the report in markdown format.`
	finalResp, err := cs.SendMessage(ctx, genai.Text(finalPrompt))
	if err != nil {
		return "", fmt.Errorf("failed to send final prompt to Gemini: %w", err)
	}

	// --- 8. Extract Final AI Response ---
	if finalResp == nil {
		fmt.Println("No response received from Gemini after sending chunks (logs might have been empty initially).")
		if outputPath != "" {
			_ = os.WriteFile(outputPath, []byte("# Activity Report\n\nNo response generated by AI.\n"), 0o600)
			fmt.Println("Generated empty report file:", outputPath)
			return "", nil
		}
	}

	// Extract text content from the final response
	reportContent := extractTextFromResponse(finalResp)
	if reportContent == "" {
		fmt.Println("Warning: Received response from Gemini, but could not extract text content.")
		reportContent = "# Activity Report\n\nError: Could not extract text content from AI response.\n"
	}

	// --- 8. Save and Print Report ---
	if outputPath != "" {
		fmt.Printf("Saving report to %s...\n", outputPath)
		err = os.WriteFile(outputPath, []byte(reportContent), 0o600)
		if err != nil {
			return "", fmt.Errorf("failed to write report file %s: %w", outputPath, err)
		}
		fmt.Printf("Report successfully saved to %s\n", outputPath)
	}

	return reportContent, nil
}

// extractTextFromResponse safely extracts the text content from the Gemini response.
func extractTextFromResponse(resp *genai.GenerateContentResponse) string {
	var builder strings.Builder
	if resp == nil {
		return ""
	}

	// Extract text from the response
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				if textPart, ok := part.(genai.Text); ok {
					builder.WriteString(string(textPart))
				}
			}
		}
	}

	return builder.String()
}
