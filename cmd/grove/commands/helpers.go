package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func isGitRepo(dir string) bool {
	gitDir := filepath.Join(dir, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

func detectProjectName(dir string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err == nil {
		url := strings.TrimSpace(string(output))
		url = strings.TrimSuffix(url, ".git")
		parts := strings.Split(url, "/")
		if len(parts) > 0 {
			name := parts[len(parts)-1]
			if idx := strings.LastIndex(name, ":"); idx != -1 {
				name = name[idx+1:]
			}
			if name != "" {
				return name
			}
		}
	}

	return filepath.Base(dir)
}

func detectMainBranch(dir string) string {
	for _, branch := range []string{"main", "master"} {
		cmd := exec.Command("git", "rev-parse", "--verify", branch)
		cmd.Dir = dir
		if err := cmd.Run(); err == nil {
			return branch
		}
	}

	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}

	return "main"
}

func updateGitignore(dir string) error {
	gitignorePath := filepath.Join(dir, ".gitignore")

	content, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if strings.Contains(string(content), ".grove/state.json") {
		return nil
	}

	entry := "\n# Grove (worktree manager)\n.grove/state.json\n.grove/state.json.bak\n.grove/.envrc\n"

	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(entry)
	return err
}

func createWorktree(repoDir, projectName, name string) error {
	parentDir := filepath.Dir(repoDir)
	worktreeDir := filepath.Join(parentDir, fmt.Sprintf("%s-%s", projectName, name))

	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	baseBranch := strings.TrimSpace(string(output))

	branchName := name
	cmd = exec.Command("git", "worktree", "add", "-b", branchName, worktreeDir, baseBranch)
	cmd.Dir = repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
