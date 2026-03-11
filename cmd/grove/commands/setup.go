package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up shell integration interactively",
	Long: `Detect your shell, find the appropriate config file, and add the grove
shell integration line. Idempotent — safe to run multiple times.

This adds the following to your shell config:

  eval "$(grove install zsh)"   # or bash

The shell integration enables directory switching, tab completion, and
environment variable export for grove commands.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := cli.NewStdout()
		cli.Header(w, "grove setup")

		shellName, rcFile := detectShellAndRC()
		if shellName == "" {
			cli.Error(cli.NewStderr(), "Could not detect shell (SHELL=%s)", os.Getenv("SHELL"))
			fmt.Fprintln(os.Stderr, "\nManual setup:")
			fmt.Fprintln(os.Stderr, "  # For zsh, add to ~/.zshrc:")
			fmt.Fprintln(os.Stderr, "  eval \"$(grove install zsh)\"")
			fmt.Fprintln(os.Stderr, "  # For bash, add to ~/.bashrc:")
			fmt.Fprintln(os.Stderr, "  eval \"$(grove install bash)\"")
			return nil
		}

		evalLine := fmt.Sprintf(`eval "$(grove install %s)"`, shellName)

		_, _ = fmt.Fprintf(w, "  Shell:   %s\n", shellName)
		_, _ = fmt.Fprintf(w, "  Config:  %s\n", rcFile)
		_, _ = fmt.Fprintf(w, "  Line:    %s\n\n", evalLine)

		// Check if already present
		if rcFileContains(rcFile, evalLine) {
			cli.Success(w, "Shell integration already configured in %s", rcFile)
			_, _ = fmt.Fprintf(w, "\n  To apply changes now: source %s\n", rcFile)
			return nil
		}

		// Also check for the old grove init pattern
		oldLine := fmt.Sprintf(`eval "$(grove init %s)"`, shellName)
		if rcFileContains(rcFile, oldLine) {
			cli.Warning(w, "Found deprecated 'grove init %s' in %s", shellName, rcFile)
			_, _ = fmt.Fprintln(w, "  Please replace it with:")
			_, _ = fmt.Fprintf(w, "  %s\n", evalLine)
			return nil
		}

		// Append the eval line
		f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", rcFile, err)
		}
		defer func() { _ = f.Close() }()

		// Ensure we start on a new line
		if info, err := f.Stat(); err == nil && info.Size() > 0 {
			_, _ = f.WriteString("\n")
		}
		_, err = fmt.Fprintf(f, "# Grove shell integration\n%s\n", evalLine)
		if err != nil {
			return fmt.Errorf("failed to write to %s: %w", rcFile, err)
		}

		cli.Success(w, "Added shell integration to %s", rcFile)

		_, _ = fmt.Fprintf(w, "\n  To activate now: source %s\n", rcFile)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

// detectShellAndRC returns the shell name ("zsh" or "bash") and the
// appropriate rc file path. Returns ("", "") if detection fails.
func detectShellAndRC() (string, string) {
	shellPath := os.Getenv("SHELL")
	shellName := filepath.Base(shellPath)

	home, err := os.UserHomeDir()
	if err != nil {
		return "", ""
	}

	switch shellName {
	case "zsh":
		return "zsh", filepath.Join(home, ".zshrc")
	case "bash":
		// On macOS, bash uses .bash_profile for login shells
		if runtime.GOOS == "darwin" {
			profile := filepath.Join(home, ".bash_profile")
			if _, err := os.Stat(profile); err == nil {
				return "bash", profile
			}
		}
		return "bash", filepath.Join(home, ".bashrc")
	default:
		return "", ""
	}
}

// rcFileContains checks if the rc file contains the given line.
func rcFileContains(path, line string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == strings.TrimSpace(line) {
			return true
		}
	}
	return false
}
