package gitcontributors_test // <-- El paquete de prueba para 'gitcontributors'

import (
	// "errors" // Descomentar si se usa para chequeos de error específicos
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort" // Asegurarse de que 'sort' está importado para el helper de ordenamiento
	"strings"
	"testing"
	"time"

	"github.com/Stone-IT-Cloud/reporting/pkg/gitcontributors"
	// --- ★★★ IMPORTANTE: Actualizar la ruta de importación ★★★ ---
	// <-- Ruta al paquete DENTRO del módulo 'reporting'
)

// --- Test Suite Setup (Helpers: setupGitRepo, runGitCommand, gitCommit, testTime, PtrTime, sortContributorsForTest) ---

// Helper function to set up a temporary Git repository
func setupGitRepo(t *testing.T) string {
	t.Helper()
	repoPath := t.TempDir()
	runGitCommand(t, repoPath, "init", "-b", "main")
	runGitCommand(t, repoPath, "config", "user.name", "Test User")
	runGitCommand(t, repoPath, "config", "user.email", "test@example.com")
	runGitCommand(t, repoPath, "commit", "--allow-empty", "-m", "Initial empty commit")
	return repoPath
}

func runGitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git command failed (args: %v): %v\nOutput:\n%s", args, err, string(output))
	}
}

func gitCommit(t *testing.T, repoPath, message, authorName, authorEmail string, commitDate time.Time) {
	t.Helper()
	dummyFile := filepath.Join(repoPath, fmt.Sprintf("file-%d.txt", time.Now().UnixNano()))
	content := fmt.Sprintf("%s\n%s\n%s", message, authorName, commitDate.String())
	if err := os.WriteFile(dummyFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to write dummy file for commit: %v", err)
	}
	runGitCommand(t, repoPath, "add", dummyFile)
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoPath
	isoDate := commitDate.Format(time.RFC3339)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME="+authorName,
		"GIT_AUTHOR_EMAIL="+authorEmail,
		"GIT_AUTHOR_DATE="+isoDate,
		"GIT_COMMITTER_NAME="+authorName,
		"GIT_COMMITTER_EMAIL="+authorEmail,
		"GIT_COMMITTER_DATE="+isoDate,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "nothing to commit") {
			return
		}
		t.Fatalf("git commit failed for %q: %v\nOutput: %s", message, err, string(output))
	}
}

func testTime(year int, month time.Month, day, hour int) time.Time {
	return time.Date(year, month, day, hour, 0, 0, 0, time.UTC)
}

func PtrTime(t time.Time) *time.Time { return &t }

func sortContributorsForTest(contributors []gitcontributors.Contributor) {
	sort.SliceStable(contributors, func(i, j int) bool {
		nameI := strings.ToLower(contributors[i].Name)
		nameJ := strings.ToLower(contributors[j].Name)
		if nameI != nameJ {
			return nameI < nameJ
		}
		emailI := strings.ToLower(contributors[i].Email)
		emailJ := strings.ToLower(contributors[j].Email)
		return emailI < emailJ
	})
}

// --- Test Cases ---
const ( // Asegurarse que las constantes estén definidas si no se copiaron antes
	author1Name  = "Alice Alpha"
	author1Email = "alice@example.com"
	author2Name  = "Bob Bravo"
	author2Email = "bob@example.com"
	author3Name  = "Alice Alpha"
	author3Email = "alice.alt@example.com"
	author4Name  = "Charlie Charlie"
	author4Email = "bob@example.com"
)

func TestGetContributors(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name                 string
		setupRepo            func(t *testing.T, repoPath string)
		repoPathOverride     string
		opts                 *gitcontributors.Options // <-- ★ Asegurarse que usa el tipo importado correctamente
		expectedError        bool
		expectedErrorSubstr  string
		expectedContributors []gitcontributors.Contributor // <-- ★ Tipo importado
	}{
		// ... (Copiar TODOS los test cases de la respuesta anterior, incluyendo los de errores y los de filtros de fecha) ...
		// --- Error Cases ---
		{name: "Error: Empty path", repoPathOverride: "", expectedError: true},
		{name: "Error: Non-existent path", repoPathOverride: filepath.Join(os.TempDir(), "nonexistent-path-for-test-"+fmt.Sprintf("%d", time.Now().UnixNano())), expectedError: true},
		{name: "Error: Path is a file", repoPathOverride: func() string {
			f, err := os.CreateTemp("", "test-file-*.txt")
			if err != nil {
				t.Fatalf("Failed to create temp file for test case: %v", err)
			}
			f.Close()
			t.Cleanup(func() { os.Remove(f.Name()) })
			return f.Name()
		}(), expectedError: true},
		{name: "Error: Not a git repository", repoPathOverride: t.TempDir(), expectedError: true},
		// --- Success Cases ---
		{
			name:      "Success: Empty repository (only initial commit)",
			setupRepo: func(t *testing.T, repoPath string) {},
			expectedContributors: []gitcontributors.Contributor{
				{
					Name:    "Test User",
					Email:   "test@example.com",
					Commits: 1,
					// No especificamos fechas exactas, las verificaremos luego
				},
			},
			expectedError: false,
		},
		{name: "Success: Single contributor, single commit", setupRepo: func(t *testing.T, repoPath string) {
			gitCommit(t, repoPath, "C1", author1Name, author1Email, testTime(2023, 1, 1, 10))
		}, expectedContributors: []gitcontributors.Contributor{{Name: "Test User", Email: "test@example.com", Commits: 1}, {Name: author1Name, Email: author1Email, Commits: 1, FirstCommitDate: testTime(2023, 1, 1, 10), LastCommitDate: testTime(2023, 1, 1, 10)}}, expectedError: false},
		{name: "Success: Multiple contributors, sorted correctly", setupRepo: func(t *testing.T, repoPath string) {
			gitCommit(t, repoPath, "B C1", author2Name, author2Email, testTime(2023, 1, 2, 9))
			gitCommit(t, repoPath, "A C1", author1Name, author1Email, testTime(2023, 1, 1, 10))
			gitCommit(t, repoPath, "A C2", author1Name, author1Email, testTime(2023, 1, 5, 12))
			gitCommit(t, repoPath, "C C1", author3Name, author3Email, testTime(2023, 1, 3, 11))
			gitCommit(t, repoPath, "D C1", author4Name, author4Email, testTime(2023, 1, 4, 14))
		}, expectedContributors: []gitcontributors.Contributor{{Name: "Test User", Email: "test@example.com", Commits: 1}, {Name: author1Name, Email: author1Email, Commits: 2, FirstCommitDate: testTime(2023, 1, 1, 10), LastCommitDate: testTime(2023, 1, 5, 12)}, {Name: author3Name, Email: author3Email, Commits: 1, FirstCommitDate: testTime(2023, 1, 3, 11), LastCommitDate: testTime(2023, 1, 3, 11)}, {Name: author2Name, Email: author2Email, Commits: 1, FirstCommitDate: testTime(2023, 1, 2, 9), LastCommitDate: testTime(2023, 1, 2, 9)}, {Name: author4Name, Email: author4Email, Commits: 1, FirstCommitDate: testTime(2023, 1, 4, 14), LastCommitDate: testTime(2023, 1, 4, 14)}}, expectedError: false},
		// ... (Añadir más casos de prueba de la respuesta anterior, especialmente los de merge y filtros de fecha) ...
		// --- Date Filter Cases ---
		{name: "Success: Date Filter - Start and End Date", setupRepo: func(t *testing.T, repoPath string) {
			gitCommit(t, repoPath, "Way Before", author1Name, author1Email, testTime(2023, 5, 1, 10))
			gitCommit(t, repoPath, "Start", author1Name, author1Email, testTime(2023, 5, 10, 12))
			gitCommit(t, repoPath, "Middle", author2Name, author2Email, testTime(2023, 5, 15, 14))
			gitCommit(t, repoPath, "End", author1Name, author1Email, testTime(2023, 5, 20, 16))
			gitCommit(t, repoPath, "Way After", author2Name, author2Email, testTime(2023, 5, 25, 18))
		}, opts: &gitcontributors.Options{StartDate: PtrTime(testTime(2023, 5, 10, 0)), EndDate: PtrTime(time.Date(2023, 5, 20, 23, 59, 59, 0, time.UTC))}, expectedContributors: []gitcontributors.Contributor{{Name: author1Name, Email: author1Email, Commits: 2, FirstCommitDate: testTime(2023, 5, 10, 12), LastCommitDate: testTime(2023, 5, 20, 16)}, {Name: author2Name, Email: author2Email, Commits: 1, FirstCommitDate: testTime(2023, 5, 15, 14), LastCommitDate: testTime(2023, 5, 15, 14)}}, expectedError: false},
	}

	// --- Run Test Cases ---
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repoPath := tc.repoPathOverride
			if tc.setupRepo != nil && repoPath == "" {
				repoPath = setupGitRepo(t)
				tc.setupRepo(t, repoPath)
			} else if repoPath == "" && !tc.expectedError {
				t.Fatal("Test case needs either setupRepo or repoPathOverride")
			}

			// --- ★★★ Usa el paquete importado ★★★ ---
			actualContributors, err := gitcontributors.GetContributors(repoPath, tc.opts)

			// Check for errors
			if tc.expectedError {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				} else if tc.expectedErrorSubstr != "" && !strings.Contains(err.Error(), tc.expectedErrorSubstr) {
					t.Errorf("Expected error containing %q, got: %v", tc.expectedErrorSubstr, err)
				}
				return // No más validaciones si esperábamos un error
			}

			if err != nil {
				t.Errorf("Expected no error, but got: %v", err)
				return // No más validaciones si no esperábamos un error pero lo recibimos
			}

			// Para casos de prueba con un repositorio vacío o inicial, solo verificamos nombre, email y commits
			// ignorando las fechas porque siempre serán las fechas reales
			if strings.Contains(tc.name, "Empty repository") {
				if len(actualContributors) != len(tc.expectedContributors) {
					t.Fatalf("Expected %d contributors, got %d", len(tc.expectedContributors), len(actualContributors))
				}

				// Ordenar los contribuidores para que la comparación sea consistente
				sortContributorsForTest(actualContributors)
				sortContributorsForTest(tc.expectedContributors)

				for i, expected := range tc.expectedContributors {
					if i >= len(actualContributors) {
						t.Errorf("Missing expected contributor at index %d: %+v", i, expected)
						continue
					}

					actual := actualContributors[i]
					if actual.Name != expected.Name || actual.Email != expected.Email || actual.Commits != expected.Commits {
						t.Errorf("Mismatch at index %d:\nExpected: {Name:%s Email:%s Commits:%d}\nActual:   {Name:%s Email:%s Commits:%d}",
							i, expected.Name, expected.Email, expected.Commits, actual.Name, actual.Email, actual.Commits)
					}
					// Ignoramos FirstCommitDate y LastCommitDate
				}
				return
			}

			// Normalize timezones and compare
			for i := range actualContributors {
				actualContributors[i].FirstCommitDate = actualContributors[i].FirstCommitDate.UTC()
				actualContributors[i].LastCommitDate = actualContributors[i].LastCommitDate.UTC()
			}

			// Para el resto de casos, necesitamos verificar los datos según el tipo de test

			// Ordenar los contribuidores para una comparación consistente
			sortContributorsForTest(actualContributors)
			sortContributorsForTest(tc.expectedContributors)

			// Verificar si los datos esperados y actuales coinciden en longitud
			if len(actualContributors) != len(tc.expectedContributors) {
				t.Errorf("Contributors count mismatch: Expected %d, got %d",
					len(tc.expectedContributors), len(actualContributors))
				return
			}

			// Para tests de filtros de fecha, verificamos todos los campos incluidas las fechas
			if strings.Contains(tc.name, "Date Filter") {
				if !reflect.DeepEqual(actualContributors, tc.expectedContributors) {
					t.Errorf("Contributor mismatch for date filter test:\nExpected: %+v\nActual:   %+v", tc.expectedContributors, actualContributors)
					for i := range tc.expectedContributors {
						if !reflect.DeepEqual(actualContributors[i], tc.expectedContributors[i]) {
							t.Errorf("Mismatch at index %d:\nExpected: %+v\nActual:   %+v", i, tc.expectedContributors[i], actualContributors[i])
						}
					}
				}
				return
			}

			// Para el resto de casos, verificamos nombre, email y commits, pero ignoramos las fechas del usuario "Test User"
			for i, expected := range tc.expectedContributors {
				if i >= len(actualContributors) {
					t.Errorf("Missing expected contributor at index %d: %+v", i, expected)
					continue
				}

				actual := actualContributors[i]
				if actual.Name != expected.Name || actual.Email != expected.Email || actual.Commits != expected.Commits {
					t.Errorf("Mismatch at index %d:\nExpected: {Name:%s Email:%s Commits:%d}\nActual:   {Name:%s Email:%s Commits:%d}",
						i, expected.Name, expected.Email, expected.Commits, actual.Name, actual.Email, actual.Commits)
				}

				// Solo verificamos las fechas para contribuidores que no sean "Test User"
				if actual.Name != "Test User" && (!actual.FirstCommitDate.Equal(expected.FirstCommitDate) ||
					!actual.LastCommitDate.Equal(expected.LastCommitDate)) {
					t.Errorf("Date mismatch at index %d:\nExpected: {FirstCommitDate:%v LastCommitDate:%v}\nActual:   {FirstCommitDate:%v LastCommitDate:%v}",
						i, expected.FirstCommitDate, expected.LastCommitDate, actual.FirstCommitDate, actual.LastCommitDate)
				}
			}
		})
	}
}
