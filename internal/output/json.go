// Package output provides standardized output formatting for Grove CLI commands.
package output

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/LeahArmstrong/grove-cli/internal/exitcode"
)

// JSONError represents an error response in JSON format.
type JSONError struct {
	Error       bool     `json:"error"`
	Code        int      `json:"code"`
	Message     string   `json:"message"`
	Suggestions []string `json:"suggestions,omitempty"`
}

// SwitchResult represents the result of a command that changes directories.
// The switch_to field is used by shell integration to change the working directory.
type SwitchResult struct {
	SwitchTo string `json:"switch_to,omitempty"`
	Name     string `json:"name"`
	Branch   string `json:"branch"`
	Path     string `json:"path"`
}

// NewWorktreeResult represents the result of creating a new worktree.
type NewWorktreeResult struct {
	SwitchTo string `json:"switch_to,omitempty"`
	Name     string `json:"name"`
	Branch   string `json:"branch"`
	Path     string `json:"path"`
	Created  bool   `json:"created"`
}

// ForkResult represents the result of forking a worktree.
type ForkResult struct {
	SwitchTo string `json:"switch_to,omitempty"`
	Name     string `json:"name"`
	Branch   string `json:"branch"`
	Path     string `json:"path"`
	Parent   string `json:"parent"`
}

// AttachResult represents the result of attaching to a tmux session.
type AttachResult struct {
	Name    string `json:"name"`
	Session string `json:"session"`
	Path    string `json:"path"`
	Created bool   `json:"created"`
}

// PrintJSON marshals the given value to JSON and prints it to stdout.
func PrintJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// PrintJSONError prints an error in JSON format to stderr.
func PrintJSONError(code int, message string, suggestions ...string) {
	errOutput := JSONError{
		Error:       true,
		Code:        code,
		Message:     message,
		Suggestions: suggestions,
	}
	data, _ := json.MarshalIndent(errOutput, "", "  ")
	fmt.Fprintln(os.Stderr, string(data))
}

// ExitWithJSONError prints an error in JSON format and exits with the given code.
func ExitWithJSONError(code int, message string, suggestions ...string) {
	PrintJSONError(code, message, suggestions...)
	os.Exit(code)
}

// ErrorSuggestions returns standard suggestions for common error codes.
func ErrorSuggestions(code int) []string {
	switch code {
	case exitcode.NotGroveProject:
		return []string{"run grove setup", "change to a directory containing a .grove folder"}
	case exitcode.ResourceNotFound:
		return []string{"run grove ls to see available worktrees"}
	case exitcode.ResourceExists:
		return []string{"use a different name", "remove the existing resource first"}
	case exitcode.GitOperationFailed:
		return []string{"check git status", "ensure you have a clean working tree"}
	default:
		return nil
	}
}
