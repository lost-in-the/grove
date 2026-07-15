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
	"github.com/lost-in-the/grove/internal/shell"
)

var setupAliasFlag string

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up shell integration interactively",
	Long: `Detect your shell, find the appropriate config file, and add the grove
shell integration line. Idempotent — safe to run multiple times, and it
migrates deprecated or out-of-date grove lines in place, so you never need
to edit the file by hand.

This adds the following to your shell config:

  eval "$(grove install zsh)"   # or bash

Pass --alias for a shorthand alias (bare --alias means 'w'):

  grove setup --alias      # writes: eval "$(grove install zsh --alias=w)"
  grove setup --alias=g    # writes: eval "$(grove install zsh --alias=g)"

The shell integration enables directory switching, tab completion, and
environment variable export for grove commands. Because the rc line evals
the current binary's output, grove updates take effect on the next shell —
no rc file edits needed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := cli.NewStdout()
		cli.Header(w, "grove setup")

		if setupAliasFlag != "" {
			if err := shell.ValidateAlias(setupAliasFlag); err != nil {
				return err
			}
		}

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
		if setupAliasFlag != "" {
			evalLine = fmt.Sprintf(`eval "$(grove install %s --alias=%s)"`, shellName, setupAliasFlag)
		}

		_, _ = fmt.Fprintf(w, "  Shell:   %s\n", shellName)
		_, _ = fmt.Fprintf(w, "  Config:  %s\n", rcFile)
		_, _ = fmt.Fprintf(w, "  Line:    %s\n\n", evalLine)

		// Check if already present
		if rcFileContains(rcFile, evalLine) {
			cli.Success(w, "Shell integration already configured in %s", rcFile)
			_, _ = fmt.Fprintf(w, "\n  To apply changes now: source %s\n", rcFile)
			return nil
		}

		// Self-heal: replace a deprecated `grove init` line or an
		// out-of-date `grove install` line in place, keeping the rest of
		// the rc file untouched.
		migrated, err := migrateRCLine(rcFile, isStaleGroveEvalLine, evalLine)
		if err != nil {
			return err
		}
		if migrated {
			cli.Success(w, "Updated existing grove line in %s", rcFile)
			_, _ = fmt.Fprintf(w, "\n  To apply changes now: source %s\n", rcFile)
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
	setupCmd.Flags().StringVar(&setupAliasFlag, "alias", "", "define a shell alias for grove (bare --alias means 'w')")
	setupCmd.Flags().Lookup("alias").NoOptDefVal = "w"
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
	case shellZsh:
		return shellZsh, filepath.Join(home, ".zshrc")
	case shellBash:
		// On macOS, bash uses .bash_profile for login shells
		if runtime.GOOS == "darwin" {
			profile := filepath.Join(home, ".bash_profile")
			if _, err := os.Stat(profile); err == nil {
				return shellBash, profile
			}
		}
		return shellBash, filepath.Join(home, ".bashrc")
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

// isStaleGroveEvalLine matches rc lines that carry an old grove integration:
// the deprecated `eval "$(grove init <shell>)"` form, or any
// `eval "$(grove install ...)"` variant (the caller has already ruled out
// the exact canonical line).
func isStaleGroveEvalLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, `eval "$(grove init `) ||
		strings.HasPrefix(trimmed, `eval "$(grove install `)
}

// migrateRCLine replaces the first line of the rc file matching the
// predicate with newLine, leaving every other byte of the file unchanged.
// Returns false (and does not create the file) when it doesn't exist or no
// line matches.
func migrateRCLine(path string, match func(string) bool, newLine string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read %s: %w", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("failed to stat %s: %w", path, err)
	}

	lines := strings.Split(string(data), "\n")
	replaced := false
	for i, line := range lines {
		if match(line) {
			lines[i] = newLine
			replaced = true
			break
		}
	}
	if !replaced {
		return false, nil
	}

	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), info.Mode().Perm()); err != nil {
		return false, fmt.Errorf("failed to update %s: %w", path, err)
	}
	return true, nil
}
