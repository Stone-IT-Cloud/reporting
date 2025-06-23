#!/bin/bash

# Test script for the projects functionality
# Usage: ./test_projects.sh <org-name>

if [ $# -eq 0 ]; then
    echo "Usage: $0 <organization-name>"
    echo "Example: $0 microsoft"
    exit 1
fi

ORG_NAME="$1"

echo "Testing the projects functionality with organization: $ORG_NAME"
echo "============================================================"

# Check if GITHUB_TOKEN is set
if [ -z "$GITHUB_TOKEN" ]; then
    echo "Warning: GITHUB_TOKEN environment variable is not set."
    echo "You'll need to set it to access private repositories or avoid rate limiting."
    echo ""
    echo "To set it:"
    echo "export GITHUB_TOKEN='your_github_token_here'"
    echo ""
fi

# Build the CLI if it doesn't exist
if [ ! -f "./reporting_cli" ]; then
    echo "Building the CLI..."
    go build ./cmd/reporting_cli
    if [ $? -ne 0 ]; then
        echo "Failed to build the CLI"
        exit 1
    fi
fi

echo "Running: ./reporting_cli -projects -org $ORG_NAME ."
echo "============================================================"

# Run the CLI with projects flag
./reporting_cli -projects -org "$ORG_NAME" .
