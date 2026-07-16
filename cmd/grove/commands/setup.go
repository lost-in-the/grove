package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/shell"
)

var setupAliasFlag string

var setupCmd = &cobra.Command{
	Use:   "setup",
	Args:  cobra.NoArgs,
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

		// Preserve a previously opted-in alias: plain `grove setup` (e.g.
		// re-run because `grove doctor` flagged an outdated ShellVersion)
		// must not silently strip the user's --alias choice from the rc.
		alias := setupAliasFlag
		if alias == "" {
			if existing := existingRCAlias(rcFile); existing != "" && shell.ValidateAlias(existing) == nil {
				alias = existing
			}
		}

		evalLine := fmt.Sprintf(`eval "$(grove install %s)"`, shellName)
		if alias != "" {
			evalLine = fmt.Sprintf(`eval "$(grove install %s --alias=%s)"`, shellName, alias)
		}

		_, _ = fmt.Fprintf(w, "  Shell:   %s\n", shellName)
		_, _ = fmt.Fprintf(w, "  Config:  %s\n", rcFile)
		_, _ = fmt.Fprintf(w, "  Line:    %s\n\n", evalLine)

		// Self-heal every stale grove line (deprecated `grove init` form or
		// an out-of-date `grove install` variant): the first is replaced by
		// the canonical line (or deleted when the canonical line already
		// exists), the rest are deleted. The rest of the file is untouched.
		alreadyPresent := rcFileContains(rcFile, evalLine)
		healed, err := healGroveRCLines(rcFile, evalLine)
		if err != nil {
			return err
		}
		if healed {
			cli.Success(w, "Updated grove line(s) in %s", rcFile)
			_, _ = fmt.Fprintf(w, "\n  To apply changes now: source %s\n", rcFile)
			return nil
		}
		if alreadyPresent {
			cli.Success(w, "Shell integration already configured in %s", rcFile)
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
// `eval "$(grove install ...)"` variant (callers exclude the canonical line).
func isStaleGroveEvalLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, `eval "$(grove init `) ||
		strings.HasPrefix(trimmed, `eval "$(grove install `)
}

// rcAliasPattern extracts the alias name from a grove install rc line.
// A bare `--alias` means "w" (the flag's NoOptDefVal).
var rcAliasPattern = regexp.MustCompile(`--alias(?:=([^)"\s]+))?`)

// existingRCAlias returns the alias carried by an existing grove install
// line in the rc file, or "" if none.
func existingRCAlias(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, `eval "$(grove install `) {
			continue
		}
		if m := rcAliasPattern.FindStringSubmatch(line); m != nil {
			if m[1] == "" {
				return "w" // bare --alias
			}
			return m[1]
		}
	}
	return ""
}

// healGroveRCLines rewrites the rc file so that exactly one canonical grove
// line remains: the first stale grove line is replaced with canonical (or
// deleted when canonical is already present), and any further stale lines
// are deleted. Every other byte of the file is unchanged. Returns false
// (and does not create the file) when it doesn't exist or nothing is stale.
func healGroveRCLines(path, canonical string) (bool, error) {
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

	canonicalTrim := strings.TrimSpace(canonical)
	lines := strings.Split(string(data), "\n")

	replaced := false
	for _, line := range lines {
		if strings.TrimSpace(line) == canonicalTrim {
			replaced = true // canonical already present — stale lines are just deleted
			break
		}
	}

	out := make([]string, 0, len(lines))
	changed := false
	for _, line := range lines {
		if strings.TrimSpace(line) != canonicalTrim && isStaleGroveEvalLine(line) {
			changed = true
			if !replaced {
				out = append(out, canonical)
				replaced = true
			}
			continue // drop the stale line
		}
		out = append(out, line)
	}
	if !changed {
		return false, nil
	}

	if err := os.WriteFile(path, []byte(strings.Join(out, "\n")), info.Mode().Perm()); err != nil {
		return false, fmt.Errorf("failed to update %s: %w", path, err)
	}
	return true, nil
}
