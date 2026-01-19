package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/LeahArmstrong/grove-cli/internal/worktree"
	timePlugin "github.com/LeahArmstrong/grove-cli/plugins/time"
	"github.com/spf13/cobra"
)

const (
	separatorWidth = 40 // Width of separator lines in output
)

var timeCmd = &cobra.Command{
	Use:   "time [subcommand]",
	Short: "Show time tracking information",
	Long: `Display time tracking data for worktrees.

With no arguments, shows time for the current worktree.
Use subcommands for different views:
  grove time week   - Show weekly summary across all worktrees`,
	RunE: runTime,
}

var (
	timeAll  bool
	timeJSON bool
)

func init() {
	timeCmd.Flags().BoolVar(&timeAll, "all", false, "Show time for all worktrees")
	timeCmd.Flags().BoolVar(&timeJSON, "json", false, "Output as JSON")

	// Add week subcommand
	timeCmd.AddCommand(&cobra.Command{
		Use:   "week",
		Short: "Show weekly time summary",
		Long:  "Display time tracking summary for the current week across all worktrees.",
		RunE:  runTimeWeek,
	})

	rootCmd.AddCommand(timeCmd)
}

func runTime(cmd *cobra.Command, args []string) error {
	// Get config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	stateDir := filepath.Join(homeDir, ".config", "grove", "state")
	tracker, err := timePlugin.NewTimeTracker(stateDir)
	if err != nil {
		return fmt.Errorf("failed to initialize time tracker: %w", err)
	}

	if timeAll {
		return showAllWorktreesTime(tracker)
	}

	// Show time for current worktree
	return showCurrentWorktreeTime(tracker)
}

func showCurrentWorktreeTime(tracker *timePlugin.TimeTracker) error {
	// Get current worktree
	mgr, err := worktree.NewManager("")
	if err != nil {
		return fmt.Errorf("failed to initialize worktree manager: %w", err)
	}

	currentTree, err := mgr.GetCurrent()
	if err != nil {
		return fmt.Errorf("failed to get current worktree: %w", err)
	}

	if currentTree == nil {
		return fmt.Errorf("not in a worktree")
	}

	// Get time for current worktree
	total, err := tracker.GetTotal(currentTree.Name)
	if err != nil {
		return fmt.Errorf("failed to get time data: %w", err)
	}

	if timeJSON {
		data := map[string]interface{}{
			"worktree": currentTree.Name,
			"duration": total.String(),
			"seconds":  int64(total.Seconds()),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}

	if total == 0 {
		fmt.Printf("No time recorded for '%s'\n", currentTree.Name)
		return nil
	}

	fmt.Printf("Time in '%s': %s\n", currentTree.Name, formatDuration(total))
	return nil
}

func showAllWorktreesTime(tracker *timePlugin.TimeTracker) error {
	worktrees := tracker.GetAllWorktrees()

	if len(worktrees) == 0 {
		fmt.Println("No time tracking data available")
		return nil
	}

	// Collect time data for all worktrees
	type worktreeTime struct {
		name     string
		duration time.Duration
	}

	var times []worktreeTime
	var grandTotal time.Duration

	for _, name := range worktrees {
		total, err := tracker.GetTotal(name)
		if err != nil {
			// Log error but continue
			fmt.Fprintf(os.Stderr, "warning: failed to get time for %s: %v\n", name, err)
			continue
		}

		if total > 0 {
			times = append(times, worktreeTime{name, total})
			grandTotal += total
		}
	}

	if len(times) == 0 {
		fmt.Println("No time recorded for any worktree")
		return nil
	}

	// Sort by duration (descending)
	sort.Slice(times, func(i, j int) bool {
		return times[i].duration > times[j].duration
	})

	if timeJSON {
		data := map[string]interface{}{
			"worktrees": times,
			"total":     grandTotal.String(),
			"seconds":   int64(grandTotal.Seconds()),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}

	// Display formatted output
	fmt.Println("Worktree Time Tracking")
	fmt.Println(strings.Repeat("─", separatorWidth))

	for _, wt := range times {
		fmt.Printf("%-20s %s\n", wt.name, formatDuration(wt.duration))
	}

	fmt.Println(strings.Repeat("─", separatorWidth))
	fmt.Printf("%-20s %s\n", "Total", formatDuration(grandTotal))

	return nil
}

func runTimeWeek(cmd *cobra.Command, args []string) error {
	// Get config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	stateDir := filepath.Join(homeDir, ".config", "grove", "state")
	tracker, err := timePlugin.NewTimeTracker(stateDir)
	if err != nil {
		return fmt.Errorf("failed to initialize time tracker: %w", err)
	}

	summary, err := tracker.GetWeeklySummary()
	if err != nil {
		return fmt.Errorf("failed to get weekly summary: %w", err)
	}

	if summary.EntryCount == 0 {
		fmt.Println("No time recorded this week")
		return nil
	}

	if timeJSON {
		data := map[string]interface{}{
			"week_start": summary.WeekStart.Format("2006-01-02"),
			"worktrees":  summary.ByWorktree,
			"total":      summary.Total.String(),
			"seconds":    int64(summary.Total.Seconds()),
			"entries":    summary.EntryCount,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}

	// Format week date
	weekStr := summary.WeekStart.Format("Jan 2")

	fmt.Printf("Time Tracking (Week of %s)\n", weekStr)
	fmt.Println(strings.Repeat("─", separatorWidth))

	// Sort worktrees by time (descending)
	type worktreeTime struct {
		name     string
		duration time.Duration
	}

	var times []worktreeTime
	for name, duration := range summary.ByWorktree {
		times = append(times, worktreeTime{name, duration})
	}

	sort.Slice(times, func(i, j int) bool {
		return times[i].duration > times[j].duration
	})

	for _, wt := range times {
		fmt.Printf("%-20s %s\n", wt.name, formatDuration(wt.duration))
	}

	fmt.Println(strings.Repeat("─", separatorWidth))
	fmt.Printf("%-20s %s\n", "Total", formatDuration(summary.Total))

	return nil
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	var parts []string
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 || hours > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if seconds > 0 && hours == 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}

	if len(parts) == 0 {
		return "0s"
	}

	return strings.Join(parts, " ")
}
