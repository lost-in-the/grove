package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SandboxInfo represents the status of a single sandbox.
type SandboxInfo struct {
	Worktree  string `json:"worktree"`
	Status    string `json:"status"` // "running", "stopped", "not-created"
	Container string `json:"container,omitempty"`
}

// buildSandbox creates (builds) the devcontainer for a worktree.
func buildSandbox(worktreePath string) error {
	devcontainerDir := filepath.Join(worktreePath, ".devcontainer")
	devcontainerFile := filepath.Join(devcontainerDir, "devcontainer.json")

	// Check that devcontainer.json exists
	if _, err := lookPath("devcontainer"); err != nil {
		return fmt.Errorf("devcontainer CLI not found in PATH")
	}

	cmd := exec.Command("devcontainer", "build", "--workspace-folder", worktreePath, "--config", devcontainerFile)
	cmd.Dir = worktreePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("devcontainer build failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// startSandbox starts the devcontainer for a worktree.
func startSandbox(worktreePath string) error {
	devcontainerFile := filepath.Join(worktreePath, ".devcontainer", "devcontainer.json")

	cmd := exec.Command("devcontainer", "up", "--workspace-folder", worktreePath, "--config", devcontainerFile)
	cmd.Dir = worktreePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("devcontainer up failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// stopSandbox stops the devcontainer for a worktree.
func stopSandbox(worktreePath string) error {
	containerName := sandboxContainerName(worktreePath)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "stop", containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Container might not exist — not an error
		if strings.Contains(string(output), "No such container") {
			return nil
		}
		return fmt.Errorf("failed to stop sandbox: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// removeSandbox stops and removes the devcontainer and its volumes.
func removeSandbox(worktreePath string) error {
	if err := stopSandbox(worktreePath); err != nil {
		return err
	}

	containerName := sandboxContainerName(worktreePath)
	cmd := exec.Command("docker", "rm", "-v", containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "No such container") {
			return nil
		}
		return fmt.Errorf("failed to remove sandbox: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// execInSandbox runs a command inside the sandbox.
func execInSandbox(worktreePath string, command []string) error {
	devcontainerFile := filepath.Join(worktreePath, ".devcontainer", "devcontainer.json")

	args := []string{"exec", "--workspace-folder", worktreePath, "--config", devcontainerFile}
	args = append(args, command...)

	cmd := exec.Command("devcontainer", args...)
	cmd.Dir = worktreePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sandbox exec failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	fmt.Print(string(output))
	return nil
}

// sandboxStatus returns status info for all sandboxes based on known worktree paths.
func sandboxStatus(worktreePaths []string) []SandboxInfo {
	var infos []SandboxInfo
	for _, wt := range worktreePaths {
		info := SandboxInfo{
			Worktree: filepath.Base(wt),
			Status:   getSandboxState(wt),
		}
		if info.Status != "not-created" {
			info.Container = sandboxContainerName(wt)
		}
		infos = append(infos, info)
	}
	return infos
}

// getSandboxState checks if a devcontainer exists and its running state.
func getSandboxState(worktreePath string) string {
	containerName := sandboxContainerName(worktreePath)

	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		return "not-created"
	}

	state := strings.TrimSpace(string(output))
	if state == "running" {
		return "running"
	}
	return "stopped"
}

// sandboxContainerName derives the container name for a worktree's sandbox.
func sandboxContainerName(worktreePath string) string {
	return fmt.Sprintf("grove-sandbox-%s", filepath.Base(worktreePath))
}

// sandboxStatusJSON returns sandbox status as JSON bytes.
func sandboxStatusJSON(worktreePaths []string) ([]byte, error) {
	infos := sandboxStatus(worktreePaths)
	return json.MarshalIndent(infos, "", "  ")
}

// lookPath is a package-level variable for testability.
var lookPath = exec.LookPath

// Exported API for use by sandbox commands.

// BuildSandbox creates (builds) the devcontainer for a worktree.
func BuildSandbox(worktreePath string) error {
	return buildSandbox(worktreePath)
}

// StartSandbox starts the devcontainer for a worktree.
func StartSandbox(worktreePath string) error {
	return startSandbox(worktreePath)
}

// StopSandbox stops the devcontainer for a worktree.
func StopSandbox(worktreePath string) error {
	return stopSandbox(worktreePath)
}

// RemoveSandbox stops and removes the devcontainer and its volumes.
func RemoveSandbox(worktreePath string) error {
	return removeSandbox(worktreePath)
}

// ExecInSandbox runs a command inside the sandbox.
func ExecInSandbox(worktreePath string, command []string) error {
	return execInSandbox(worktreePath, command)
}

// SandboxStatus returns status info for all sandboxes.
func SandboxStatus(worktreePaths []string) []SandboxInfo {
	return sandboxStatus(worktreePaths)
}

// SandboxStatusJSON returns sandbox status as JSON bytes.
func SandboxStatusJSON(worktreePaths []string) ([]byte, error) {
	return sandboxStatusJSON(worktreePaths)
}
