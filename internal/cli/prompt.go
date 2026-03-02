package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// IsInteractive returns true if stdin is connected to a terminal.
func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// Confirm asks the user a yes/no question.
// Returns an error if stdin is not an interactive terminal.
func Confirm(question string, defaultYes bool) (bool, error) {
	if !IsInteractive() {
		return false, fmt.Errorf("not an interactive terminal")
	}

	hint := "y/N"
	if defaultYes {
		hint = "Y/n"
	}

	fmt.Fprintf(os.Stderr, "%s [%s]: ", question, hint)

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return defaultYes, scanner.Err()
	}
	input := strings.TrimSpace(strings.ToLower(scanner.Text()))

	switch input {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	case "":
		return defaultYes, nil
	default:
		return false, fmt.Errorf("invalid response %q: expected y or n", input)
	}
}

// ConfirmWithDetails shows information before asking for confirmation.
func ConfirmWithDetails(w *Writer, header string, details []string, question string, defaultYes bool) (bool, error) {
	if !IsInteractive() {
		return false, fmt.Errorf("not an interactive terminal")
	}

	// Print styled header and details
	Bold(w, "%s", header)
	for _, detail := range details {
		_, _ = fmt.Fprintf(w, "  %s\n", detail)
	}
	_, _ = fmt.Fprintln(w)

	return Confirm(question, defaultYes)
}

// Choose presents a numbered selection menu and returns the chosen option.
func Choose(title string, options []string) (string, error) {
	if !IsInteractive() {
		return "", fmt.Errorf("not an interactive terminal")
	}

	if len(options) == 0 {
		return "", fmt.Errorf("no options provided")
	}

	fmt.Fprintf(os.Stderr, "%s\n", title)
	for i, opt := range options {
		fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, opt)
	}
	fmt.Fprintf(os.Stderr, "Choice [1-%d]: ", len(options))

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return "", scanner.Err()
	}
	input := strings.TrimSpace(scanner.Text())

	var choice int
	if _, err := fmt.Sscanf(input, "%d", &choice); err != nil || choice < 1 || choice > len(options) {
		return "", fmt.Errorf("invalid choice %q: expected a number between 1 and %d", input, len(options))
	}

	return options[choice-1], nil
}
