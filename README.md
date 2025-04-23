# Reporting Tool

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Stone-IT-Cloud/reporting)](/go.mod)

A command-line tool written in Go to analyze Git repositories and generate various reports, including contributor statistics, detailed commit logs, and AI-powered weekly activity summaries suitable for non-technical stakeholders using Google Gemini.

## Description

This project provides a CLI utility (`reporting_cli`) designed to extract valuable information from Git repositories. It can:

1.  **List Contributors:** Generate a summary of contributors based on commit history, including commit counts and first/last commit dates within a specified range.
2.  **Extract Git Logs:** Retrieve detailed commit logs (excluding merges by default) across all branches in chronological order, outputting the results as a JSON array. Each entry includes commit timestamp, author details, commit message, and modified files.
3.  **Generate AI Activity Reports:** Leverage Google's Gemini AI models to process the extracted commit logs and generate a human-readable weekly activity report in Markdown format. This report is tailored for project managers and stakeholders, avoiding technical jargon.

The tool is built using Go and utilizes standard library features along with external packages for interacting with Git and the Google Generative AI API.

## Table of Contents

*   [Features](#features)
*   [Installation](#installation)
*   [Usage](#usage)
    *   [Contributor Report](#contributor-report)
    *   [Git Log JSON Report](#git-log-json-report)
    *   [AI Activity Report](#ai-activity-report)
*   [Configuration (AI Activity Report)](#configuration-ai-activity-report)
    *   [Configuration File](#configuration-file)
    *   [Authentication](#authentication)
*   [Development & Contributing](#development--contributing)
*   [License](#license)

## Features

*   Generate contributor reports with commit counts and date ranges.
*   Filter contributor reports by date and optionally include merge commits.
*   Extract detailed Git commit logs as JSON.
*   Filter log reports by date range.
*   Generate AI-powered weekly activity summaries using Google Gemini.
*   Configure AI report generation parameters (chunk size, GCP project, location, model).
*   Support multiple authentication methods for Google Cloud AI Platform (Credentials File, API Key).
*   Command-line interface for easy integration into scripts.

## Installation

1.  **Prerequisites:**
    *   Go (version 1.24 or later, as specified in `go.mod`).
    *   Git command-line tool installed and accessible in your PATH.
    *   (Optional) `pre-commit` if you plan to contribute.

2.  **Build the CLI:**
    Clone the repository and build the executable:
    ```bash
    git clone https://github.com/Stone-IT-Cloud/reporting.git
    cd reporting
    go build -o reporting_cli ./cmd/reporting_cli
    ```
    This will create an executable file named `reporting_cli` (or `reporting_cli.exe` on Windows) in the current directory. You can move this executable to a directory in your system's PATH for easier access.

## Usage

The tool operates via the `reporting_cli` executable. The general syntax is:

```bash
./reporting_cli [options] <path-to-git-repo>
```

By default (without `-log` or `-generate-report`), it generates the contributor report.

### Contributor Report

Generates a list of contributors, their commit counts, and first/last commit dates.

**Command:**

```bash
./reporting_cli [flags] <path-to-git-repo>
```

**Flags:**

*   `-m`: Include merge commits in the count (default: false).
*   `-start <YYYY-MM-DD>`: Filter commits made on or after this date.
*   `-end <YYYY-MM-DD>`: Filter commits made on or before this date.

**Example:**

```bash
# Get contributors for the current directory, excluding merges
./reporting_cli .

# Get contributors for ../my-project, including merges, from 2024-01-01 onwards
./reporting_cli -m -start 2024-01-01 ../my-project
```

### Git Log JSON Report

Generates a JSON array containing detailed commit information.

**Command:**

```bash
./reporting_cli -log [flags] <path-to-git-repo>
```

**Flags:**

*   `-start <YYYY-MM-DD>`: Filter commits made on or after this date.
*   `-end <YYYY-MM-DD>`: Filter commits made on or before this date.

**Example:**

```bash
# Get all commit logs as JSON for the current directory
./reporting_cli -log .

# Get commit logs between 2024-03-01 and 2024-03-31
./reporting_cli -log -start 2024-03-01 -end 2024-03-31 .
```

### AI Activity Report

Generates a weekly activity report using Google Gemini based on commit logs.

**Command:**

```bash
./reporting_cli -generate-report [flags] <path-to-git-repo>
```

**Flags:**

*   `-config <path>`: Path to the YAML configuration file (default: `configs/activity_report_config.yaml`).
*   `-report-path <path>`: Path to save the generated Markdown report file (optional, prints to console if not specified).
*   `-start <YYYY-MM-DD>`: Filter commits made on or after this date (used for log fetching).
*   `-end <YYYY-MM-DD>`: Filter commits made on or before this date (used for log fetching).

**Example:**

```bash
# Generate report for the current repo using default config, print to console
./reporting_cli -generate-report .

# Generate report for ../my-project for last week, using custom config, save to file
./reporting_cli -generate-report -start 2025-04-16 -end 2025-04-22 -config my_config.yaml -report-path weekly_report.md ../my-project
```

## Configuration (AI Activity Report)

The AI Activity Report generation requires configuration, primarily provided via a YAML file.

### Configuration File

Create a configuration file (e.g., `configs/activity_report_config.yaml`) based on the provided `configs/activity_report_config.yaml.example`.

```yaml
# Configuration for the activity report generation
chunk_size: 100                  # Max number of commits per chunk sent to AI
project_id: "your-gcp-project-id" # ★★★ Replace with your Google Cloud Project ID ★★★
location: "us-central1"          # Vertex AI region (e.g., us-central1, europe-west1)
gemini_model: "gemini-1.5-flash-001" # Gemini model to use
# Optional: Specify credentials file path directly (overrides environment variables)
# credentials_file: "/path/to/your/service-account-key.json"
```

*   `chunk_size`: How many commits to send to the AI model in each request. Adjust based on model context limits and desired granularity.
*   `project_id`: Your Google Cloud Project ID where Vertex AI is enabled.
*   `location`: The Google Cloud region for your Vertex AI endpoint.
*   `gemini_model`: The specific Gemini model identifier to use (e.g., `gemini-1.5-flash-001`, `gemini-1.0-pro`).
*   `credentials_file` (Optional): Explicit path to your Google Cloud service account key file. If provided, this takes precedence over environment variables.

### Authentication

The tool needs to authenticate with Google Cloud to use the Gemini API. It uses the following methods in order of precedence:

1.  **`credentials_file` in Config:** If `credentials_file` is specified in the YAML configuration, that file will be used.
2.  **`GOOGLE_APPLICATION_CREDENTIALS` Environment Variable:** If the config field is not set, the tool checks for the standard `GOOGLE_APPLICATION_CREDENTIALS` environment variable pointing to your service account key file.
3.  **`VERTEX_AI_API_KEY` Environment Variable:** If neither of the above is found, the tool looks for the `VERTEX_AI_API_KEY` environment variable containing a valid Vertex AI API key.

Ensure you have appropriate permissions (e.g., Vertex AI User role) for the service account or API key used.

## Development & Contributing

This project uses Go modules for dependency management and `pre-commit` for code quality checks.

1.  **Setup:**
    *   Install Go and Git.
    *   Install `pre-commit`: `pip install pre-commit` (or `brew install pre-commit`).
    *   Install pre-commit hooks: `pre-commit install` in the repository root.

2.  **Running Tests:**
    ```bash
    go test ./...
    ```

3.  **Code Formatting & Linting:**
    Pre-commit hooks handle formatting (`gofumpt`, `goimports`) and linting (`golangci-lint`, `go vet`, `revive`, `staticcheck`, etc.) automatically before each commit. You can also run them manually:
    ```bash
    pre-commit run --all-files
    # Or specific Go tools
    go fmt ./...
    go vet ./...
    golangci-lint run
    ```

4.  **Contributing:**
    Contributions are welcome! Please ensure your code passes all pre-commit checks and tests before submitting a pull request. Follow standard Go best practices.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
