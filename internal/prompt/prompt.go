package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// Confirm asks the user a yes/no question and returns their response
// defaultNo: if true, pressing Enter defaults to No; otherwise defaults to Yes
func Confirm(question string, defaultNo bool) (bool, error) {
	// Check if we're in an interactive terminal
	if !IsInteractive() {
		return false, fmt.Errorf("not an interactive terminal; use --keep-branch or --delete-branch flag")
	}

	prompt := question
	if defaultNo {
		prompt += " [y/N]: "
	} else {
		prompt += " [Y/n]: "
	}

	fmt.Print(prompt)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))

	switch response {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	case "":
		// Default response
		return !defaultNo, nil
	default:
		return false, nil
	}
}

// ConfirmWithDetails shows information before asking for confirmation
func ConfirmWithDetails(header string, details []string, question string, defaultNo bool) (bool, error) {
	if !IsInteractive() {
		return false, fmt.Errorf("not an interactive terminal; use --keep-branch or --delete-branch flag")
	}

	// Print header
	fmt.Println(header)

	// Print details
	for _, detail := range details {
		fmt.Printf("  %s\n", detail)
	}
	fmt.Println()

	return Confirm(question, defaultNo)
}

// IsInteractive returns true if stdin is connected to a terminal
func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// ConfirmBatch asks for confirmation to process multiple items
// Returns: keepAll, deleteAll, cancelled, error
func ConfirmBatch(items []string, itemType string) (keepAll bool, deleteAll bool, err error) {
	if !IsInteractive() {
		return true, false, fmt.Errorf("not an interactive terminal; use explicit flags")
	}

	if len(items) == 0 {
		return true, false, nil
	}

	fmt.Printf("\nAssociated %ss:\n", itemType)
	for _, item := range items {
		fmt.Printf("  • %s\n", item)
	}
	fmt.Println()

	prompt := fmt.Sprintf("Delete %d associated %s", len(items), itemType)
	if len(items) > 1 {
		prompt += "es"
	}
	prompt += "?"

	confirmed, err := Confirm(prompt, true)
	if err != nil {
		return true, false, err
	}

	return !confirmed, confirmed, nil
}
