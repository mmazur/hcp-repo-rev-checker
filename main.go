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
	Digest     string `json:"digest"`
	CommitDate string `json:"commit_date"`
}

type RepoRevisions struct {
	Int  []CommitInfo `json:"int"`
	Stg  []CommitInfo `json:"stg"`
	Prod []CommitInfo `json:"prod"`
}

var rootCmd = &cobra.Command{
	Use:   "repo-rev-checker [directory]",
	Short: "Check repository revisions across different branches",
	Long: `A tool that pulls the latest changes from main, release/hcp/public/stg and release/hcp/public/prod branches,
extracts ARO_HCP_REPO_REVISION values from ./hcp/Revision.mk and outputs them as JSON.`,
	Args: cobra.ExactArgs(1),
	Run:  runCommand,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runCommand(cmd *cobra.Command, args []string) {
	directory := args[0]

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

	// Initialize result structure
	var result RepoRevisions

	// Process each branch
	branches := map[string]*[]CommitInfo{
		"main":                      &result.Int,
		"release/hcp/public/stg":    &result.Stg,
		"release/hcp/public/prod":   &result.Prod,
	}

	for branch, arrayPtr := range branches {
		revision, commitDate, err := processBranch(branch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing branch '%s': %v\n", branch, err)
			continue
		}

		// Convert commit date to UTC
		utcDate, err := convertToUTC(commitDate)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error converting date to UTC for branch '%s': %v\n", branch, err)
			continue
		}

		// Add commit info to the array (currently just one commit per environment)
		*arrayPtr = []CommitInfo{
			{
				Digest:     revision,
				CommitDate: utcDate,
			},
		}
	}

	// Output JSON
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshalling JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(jsonData))
}

func processBranch(branch string) (string, string, error) {
	// First fetch to ensure we have latest remote refs
	fetchCmd := exec.Command("git", "fetch", "origin")
	if err := fetchCmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to fetch from origin: %v", err)
	}

	// Checkout the branch
	checkoutCmd := exec.Command("git", "checkout", branch)
	if err := checkoutCmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to checkout branch '%s': %v", branch, err)
	}

	// Reset to match the remote branch exactly
	resetCmd := exec.Command("git", "reset", "--hard", fmt.Sprintf("origin/%s", branch))
	if err := resetCmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to reset to origin/%s: %v", branch, err)
	}

	// Extract ARO_HCP_REPO_REVISION from ./hcp/Revision.mk
	revision, err := extractRevision("./hcp/Revision.mk")
	if err != nil {
		return "", "", fmt.Errorf("failed to extract revision from Revision.mk on branch '%s': %v", branch, err)
	}

	// Get the commit date of the current HEAD
	commitDateCmd := exec.Command("git", "log", "-1", "--format=%ci")
	commitDateOutput, err := commitDateCmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to get commit date for branch '%s': %v", branch, err)
	}
	commitDate := strings.TrimSpace(string(commitDateOutput))

	return revision, commitDate, nil
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