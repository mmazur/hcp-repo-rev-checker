package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type CommitInfo struct {
	RepoRevision string `json:"repo_revision"`
	CommitDate   string `json:"commit_date"`
}

var (
	quickMode bool
	envList   string
	days      int
)

var rootCmd = &cobra.Command{
	Use:   "repo-rev-checker [directory]",
	Short: "Check repository revisions across different branches",
	Long: `A tool that pulls the latest changes from main, release/hcp/public/stg and release/hcp/public/prod branches,
extracts ARO_HCP_REPO_REVISION values from ./hcp/Revision.mk and outputs them as JSON.`,
	Args: cobra.ExactArgs(1),
	Run:  runCommand,
}

func init() {
	rootCmd.Flags().BoolVarP(&quickMode, "quick", "q", false, "Skip git fetch/reset operations and use repository as-is")
	rootCmd.Flags().StringVarP(&envList, "envs", "e", "", "Comma-separated list of environments to analyze (int,stg,prod). If not specified, all environments are processed.")
	rootCmd.Flags().IntVarP(&days, "days", "d", 0, "Number of days to look back in commit history for Revision.mk changes. If 0, only checks the tip commit.")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func parseEnvironments(envStr string) ([]string, error) {
	if envStr == "" {
		// Default to all environments
		return []string{"int", "stg", "prod"}, nil
	}

	// Split by comma and trim spaces
	envs := strings.Split(envStr, ",")
	var validEnvs []string
	validEnvNames := map[string]bool{
		"int":  true,
		"stg":  true,
		"prod": true,
	}

	for _, env := range envs {
		env = strings.TrimSpace(env)
		if env == "" {
			continue
		}
		if !validEnvNames[env] {
			return nil, fmt.Errorf("invalid environment '%s'. Valid environments are: int, stg, prod", env)
		}
		validEnvs = append(validEnvs, env)
	}

	if len(validEnvs) == 0 {
		return nil, fmt.Errorf("no valid environments specified")
	}

	return validEnvs, nil
}

func runCommand(cmd *cobra.Command, args []string) {
	directory := args[0]

	// Parse and validate environments
	selectedEnvs, err := parseEnvironments(envList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Check if directory exists
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Directory '%s' does not exist\n", directory)
		os.Exit(1)
	}

	// Change to the directory
	originalDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	if err := os.Chdir(directory); err != nil {
		fmt.Fprintf(os.Stderr, "Error changing to directory '%s': %v\n", directory, err)
		os.Exit(1)
	}
	defer os.Chdir(originalDir)

	// Initialize result map
	result := make(map[string][]CommitInfo)

	// Map of all possible branches
	allBranches := map[string]string{
		"main":                      "int",
		"release/hcp/public/stg":    "stg",
		"release/hcp/public/prod":   "prod",
	}

	// Filter branches based on selected environments
	selectedEnvsMap := make(map[string]bool)
	for _, env := range selectedEnvs {
		selectedEnvsMap[env] = true
	}

	for branch, envName := range allBranches {
		if !selectedEnvsMap[envName] {
			continue // Skip this environment if not selected
		}

		commits, err := processBranch(branch, quickMode, days)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing branch '%s': %v\n", branch, err)
			continue
		}

		// Convert all commit dates to UTC and add to result
		var commitInfos []CommitInfo
		for _, commit := range commits {
			utcDate, err := convertToUTC(commit.CommitDate)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error converting date to UTC for branch '%s', commit '%s': %v\n", branch, commit.RepoRevision, err)
				continue
			}

			commitInfos = append(commitInfos, CommitInfo{
				RepoRevision: commit.RepoRevision,
				CommitDate:   utcDate,
			})
		}

		result[envName] = commitInfos
	}

	// Output JSON
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshalling JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(jsonData))
}

func processBranch(branch string, quick bool, daysBack int) ([]CommitInfo, error) {
	if !quick {
		// First fetch to ensure we have latest remote refs
		fetchCmd := exec.Command("git", "fetch", "origin")
		if err := fetchCmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to fetch from origin: %v", err)
		}

		// Checkout the branch
		checkoutCmd := exec.Command("git", "checkout", branch)
		if err := checkoutCmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to checkout branch '%s': %v", branch, err)
		}

		// Reset to match the remote branch exactly
		resetCmd := exec.Command("git", "reset", "--hard", fmt.Sprintf("origin/%s", branch))
		if err := resetCmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to reset to origin/%s: %v", branch, err)
		}
	} else {
		// In quick mode, just checkout the branch without fetching/resetting
		checkoutCmd := exec.Command("git", "checkout", branch)
		if err := checkoutCmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to checkout branch '%s': %v", branch, err)
		}
	}

	var commits []CommitInfo

	// Always get the tip commit first
	tipRevision, err := extractRevision("./hcp/Revision.mk")
	if err != nil {
		return nil, fmt.Errorf("failed to extract revision from Revision.mk on branch '%s': %v", branch, err)
	}

	// Get the commit date of the last change to Revision.mk
	commitDateCmd := exec.Command("git", "log", "-1", "--format=%ci", "--", "./hcp/Revision.mk")
	commitDateOutput, err := commitDateCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get commit date for Revision.mk on branch '%s': %v", branch, err)
	}
	tipCommitDate := strings.TrimSpace(string(commitDateOutput))

	// Add tip commit as first entry
	commits = append(commits, CommitInfo{
		RepoRevision: tipRevision,
		CommitDate:   tipCommitDate,
	})

	// If days is specified, get historical commits
	if daysBack > 0 {
		historicalCommits, err := getHistoricalCommits("./hcp/Revision.mk", daysBack)
		if err != nil {
			return nil, fmt.Errorf("failed to get historical commits for Revision.mk on branch '%s': %v", branch, err)
		}

		// Add historical commits (excluding tip if it's already included)
		tipCommitHash, err := getLastCommitHashForFile("./hcp/Revision.mk")
		if err == nil {
			for _, commit := range historicalCommits {
				if commit.CommitHash != tipCommitHash {
					commits = append(commits, CommitInfo{
						RepoRevision: commit.RepoRevision,
						CommitDate:   commit.CommitDate,
					})
				}
			}
		} else {
			// If we can't get tip hash, just add all historical commits
			for _, commit := range historicalCommits {
				commits = append(commits, CommitInfo{
					RepoRevision: commit.RepoRevision,
					CommitDate:   commit.CommitDate,
				})
			}
		}
	}

	return commits, nil
}

func extractRevision(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file '%s': %v", filePath, err)
	}

	// Look for ARO_HCP_REPO_REVISION= pattern
	re := regexp.MustCompile(`ARO_HCP_REPO_REVISION\s*=\s*(.+)`)
	matches := re.FindStringSubmatch(string(content))

	if len(matches) < 2 {
		return "", fmt.Errorf("ARO_HCP_REPO_REVISION not found in '%s'", filePath)
	}

	// Clean up the value (remove quotes if present and trim whitespace)
	revision := strings.TrimSpace(matches[1])
	revision = strings.Trim(revision, "\"'")

	return revision, nil
}

func extractRevisionFromContent(content string) (string, error) {
	// Look for ARO_HCP_REPO_REVISION= pattern
	re := regexp.MustCompile(`ARO_HCP_REPO_REVISION\s*=\s*(.+)`)
	matches := re.FindStringSubmatch(content)

	if len(matches) < 2 {
		return "", fmt.Errorf("ARO_HCP_REPO_REVISION not found in content")
	}

	// Clean up the value (remove quotes if present and trim whitespace)
	revision := strings.TrimSpace(matches[1])
	revision = strings.Trim(revision, "\"'")

	return revision, nil
}

func convertToUTC(dateStr string) (string, error) {
	// Parse the git commit date (format: "2006-01-02 15:04:05 -0700")
	parsedTime, err := time.Parse("2006-01-02 15:04:05 -0700", dateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse date '%s': %v", dateStr, err)
	}

	// Convert to UTC and format
	utcTime := parsedTime.UTC()
	return utcTime.Format("2006-01-02 15:04:05 +0000"), nil
}

type HistoricalCommit struct {
	CommitHash   string
	CommitDate   string
	RepoRevision string
}

func getCurrentCommitHash() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func getLastCommitHashForFile(filePath string) (string, error) {
	cmd := exec.Command("git", "log", "-1", "--format=%H", "--", filePath)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func getHistoricalCommits(filePath string, daysBack int) ([]HistoricalCommit, error) {
	// Get commits that modified the file in the last N days
	sinceDate := time.Now().AddDate(0, 0, -daysBack).Format("2006-01-02")

	cmd := exec.Command("git", "log", "--since="+sinceDate, "--format=%H|%ci", "--", filePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git log: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var commits []HistoricalCommit

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) != 2 {
			continue
		}

		commitHash := parts[0]
		commitDate := parts[1]

		// Get the file content at this specific commit
		showCmd := exec.Command("git", "show", commitHash+":"+filePath)
		fileContent, err := showCmd.Output()
		if err != nil {
			continue // Skip this commit if we can't get the file content
		}

		// Extract revision from the file content at this commit
		revision, err := extractRevisionFromContent(string(fileContent))
		if err != nil {
			continue // Skip this commit if we can't extract revision
		}

		commits = append(commits, HistoricalCommit{
			CommitHash:   commitHash,
			CommitDate:   commitDate,
			RepoRevision: revision,
		})
	}

	return commits, nil
}