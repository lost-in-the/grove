package notify

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Send sends a notification with the given title and message
func Send(title, message string) error {
	if title == "" {
		return fmt.Errorf("title cannot be empty")
	}

	if !IsAvailable() {
		// Notifications not available on this platform - not an error
		return nil
	}

	switch runtime.GOOS {
	case "darwin":
		return sendMacOS(title, message)
	case "linux":
		return sendLinux(title, message)
	default:
		// Platform not supported - not an error, just skip
		return nil
	}
}

// IsAvailable checks if notifications are available on this platform
func IsAvailable() bool {
	switch runtime.GOOS {
	case "darwin":
		return isMacOSNotifyAvailable()
	case "linux":
		return isLinuxNotifyAvailable()
	default:
		return false
	}
}

// sendMacOS sends a notification on macOS using osascript
func sendMacOS(title, message string) error {
	script := fmt.Sprintf(`display notification "%s" with title "%s"`, message, title)
	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}

// sendLinux sends a notification on Linux using notify-send
func sendLinux(title, message string) error {
	cmd := exec.Command("notify-send", title, message)
	return cmd.Run()
}

// isMacOSNotifyAvailable checks if osascript is available
func isMacOSNotifyAvailable() bool {
	_, err := exec.LookPath("osascript")
	return err == nil
}

// isLinuxNotifyAvailable checks if notify-send is available
func isLinuxNotifyAvailable() bool {
	_, err := exec.LookPath("notify-send")
	return err == nil
}
