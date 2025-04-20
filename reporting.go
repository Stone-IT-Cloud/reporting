package reporting // <-- Paquete raíz

import (
	"context"
	"fmt"
	"time"

	// --- ★★★ Importa los sub-paquetes usando la ruta correcta desde la raíz del módulo ★★★ ---
	"github.com/Stone-IT-Cloud/reporting/internal/activityreport" // Correct path
	"github.com/Stone-IT-Cloud/reporting/pkg/gitlogs"             // Correct path
)

// GenerateAIActivityReport orchestates the process of getting logs and generating the AI report.
// This is the main function exposed by the 'reporting' package for this task.
func GenerateAIActivityReport(ctx context.Context, repoPath, configPath string, startDate, endDate *time.Time) error {
	fmt.Println("Orchestration: Starting AI Activity Report Generation")

	// Step 1: Get Git Logs as JSON using the gitlogs sub-package
	fmt.Println("Orchestration: Fetching git logs...")
	logOpts := &gitlogs.Options{
		StartDate: startDate,
		EndDate:   endDate,
	}
	gitLogsJSON, err := gitlogs.GetLogsJSON(repoPath, logOpts)
	if err != nil {
		return fmt.Errorf("orchestration failed during git log retrieval: %w", err)
	}
	fmt.Println("Orchestration: Git logs fetched successfully.")

	// Step 2: Generate the report using the activityreport sub-package
	fmt.Println("Orchestration: Generating AI report...")
	err = activityreport.GenerateReport(ctx, gitLogsJSON, configPath)
	if err != nil {
		return fmt.Errorf("orchestration failed during AI report generation: %w", err)
	}

	fmt.Println("Orchestration: AI Activity Report Generation Finished Successfully.")
	return nil
}

// Placeholder for other report types if needed in the future
// func GenerateContributorReport(...) error { ... }
// func GetRawLogsJSON(...) (string, error) { ... }
