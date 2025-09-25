package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

type RepoRevisions struct {
	Int struct {
		RepoTip           string `json:"repo_tip"`
		RepoTipCommitDate string `json:"repo_tip_commit_date"`
	} `json:"int"`
	Stg struct {
		RepoTip           string `json:"repo_tip"`
		RepoTipCommitDate string `json:"repo_tip_commit_date"`
	} `json:"stg"`
	Prod struct {
		RepoTip           string `json:"repo_tip"`
		RepoTipCommitDate string `json:"repo_tip_commit_date"`
	} `json:"prod"`
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
	branches := map[string]struct {
		repoTip    *string
		commitDate *string
	}{
		"main":                      {&result.Int.RepoTip, &result.Int.RepoTipCommitDate},
		"release/hcp/public/stg":    {&result.Stg.RepoTip, &result.Stg.RepoTipCommitDate},
		"release/hcp/public/prod":   {&result.Prod.RepoTip, &result.Prod.RepoTipCommitDate},
	}

	for branch, ptrs := range branches {
		revision, commitDate, err := processBranch(branch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing branch '%s': %v\n", branch, err)
			continue
		}
		*ptrs.repoTip = revision
		*ptrs.commitDate = commitDate
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