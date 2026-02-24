// fakegrove is a test binary that simulates grove's behavior for shell wrapper testing.
//
// Behavior:
//   - No args: prints "TUI_RENDERED" to stdout (simulates TUI output visible on terminal)
//   - "ls": prints regular output lines
//   - "to <name>": prints "cd:/tmp/fakegrove-<name>" directive + optional output
//   - Any arg starting with "fail": exits with code 1
//
// Debug logging: when GROVE_DEBUG=1, prints diagnostics to stderr.
package main

import (
	"fmt"
	"os"
)

func main() {
	debug := os.Getenv("GROVE_DEBUG") == "1"
	shell := os.Getenv("GROVE_SHELL")

	if debug {
		fmt.Fprintf(os.Stderr, "[fakegrove] args=%v GROVE_SHELL=%s\n", os.Args[1:], shell)
	}

	if len(os.Args) < 2 {
		// Bare invocation — simulate TUI rendering directly to stdout
		fmt.Println("TUI_RENDERED")
		if debug {
			fmt.Fprintf(os.Stderr, "[fakegrove] TUI mode: printed TUI_RENDERED to stdout\n")
		}
		// If GROVE_CD_FILE is set, write a test path to it (simulates TUI selection)
		if cdFile := os.Getenv("GROVE_CD_FILE"); cdFile != "" {
			if cdTarget := os.Getenv("FAKEGROVE_CD_TARGET"); cdTarget != "" {
				os.WriteFile(cdFile, []byte(cdTarget), 0600)
				if debug {
					fmt.Fprintf(os.Stderr, "[fakegrove] wrote %s to cd file %s\n", cdTarget, cdFile)
				}
			}
		}
		return
	}

	cmd := os.Args[1]

	switch cmd {
	case "ls":
		fmt.Println("root")
		fmt.Println("feature-auth")
		fmt.Println("testing")

	case "to":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: grove to <name>")
			os.Exit(1)
		}
		name := os.Args[2]
		// Emit cd directive that shell wrapper should intercept
		fmt.Printf("cd:/tmp/fakegrove-%s\n", name)
		if debug {
			fmt.Fprintf(os.Stderr, "[fakegrove] emitted cd:/tmp/fakegrove-%s\n", name)
		}

	case "fork":
		// Simulates fork output: regular lines mixed with a cd: directive
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: grove fork <name>")
			os.Exit(1)
		}
		name := os.Args[2]
		fmt.Println("some output before")
		fmt.Printf("cd:/tmp/fakegrove-%s\n", name)
		fmt.Println("some output after")

	case "mixed":
		// Legacy test case — mixed output with directives (no longer a directive command)
		fmt.Println("some output before")
		fmt.Println("cd:/tmp/fakegrove-mixed")
		fmt.Println("some output after")

	case "version":
		// Simple output command — used to test passthrough (no directives)
		fmt.Println("grove v1.0.0-test")

	case "logs":
		// Streaming-style output — used to test direct passthrough
		fmt.Println("line1: starting up")
		fmt.Println("line2: ready")
		fmt.Fprintln(os.Stderr, "stderr: debug info")
		fmt.Println("line3: listening")

	default:
		if len(cmd) >= 4 && cmd[:4] == "fail" {
			fmt.Fprintln(os.Stderr, "error: simulated failure")
			os.Exit(1)
		}
		fmt.Printf("unknown command: %s\n", cmd)
	}
}
