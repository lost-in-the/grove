package claude

import (
	"os"
	"path/filepath"
	"strings"
)

const groveContextStart = "<!-- grove:agent-context -->"
const groveContextEnd = "<!-- /grove:agent-context -->"

const groveContextContent = `
## Grove Tooling

This project uses [Grove](https://github.com/lost-in-the/grove) for worktree management.
You are running inside a Grove-managed sandbox. Here are the key commands:

### Worktree Management
- ` + "`grove ls`" + ` — list all worktrees
- ` + "`grove new <name>`" + ` — create a new worktree (creates branch, tmux session, devcontainer)
- ` + "`grove to <name>`" + ` — switch to another worktree
- ` + "`grove here`" + ` — show current worktree info (name, branch, SHA, age)
- ` + "`grove rm <name>`" + ` — remove a worktree and its sandbox

### Sandbox Operations
- ` + "`grove sandbox status`" + ` — show running sandboxes and their state
- ` + "`grove sandbox status --json`" + ` — machine-readable sandbox status
- ` + "`grove sandbox exec <worktree> -- <cmd>`" + ` — run a command in a sibling sandbox

### Docker / Isolated Stacks
- ` + "`grove up`" + ` — start Docker containers for current worktree
- ` + "`grove up --isolated`" + ` — start an independent Docker stack (for parallel work)
- ` + "`grove agent-status --json`" + ` — show active Docker stacks

### Environment
- ` + "`GROVE_AGENT_MODE=1`" + ` — agent isolation mode (set automatically in sandbox)
- ` + "`GROVE_NONINTERACTIVE=1`" + ` — suppress prompts (set automatically in sandbox)
- ` + "`GROVE_SHELL=1`" + ` — shell integration active

### Multi-Agent Patterns
Each worktree is an independent workspace. To split work across agents:
1. Create task-specific worktrees: ` + "`grove new fix-auth`" + `, ` + "`grove new add-tests`" + `
2. Each agent works in its own worktree with isolated branch and containers
3. Use ` + "`grove agent-status --json`" + ` to see what's running where
`

// injectGroveContext appends grove tooling instructions to CLAUDE.md in the worktree.
// The content is idempotent — marked with delimiters to prevent duplication.
func injectGroveContext(worktreePath string) error {
	claudeMDPath := filepath.Join(worktreePath, "CLAUDE.md")

	existing, err := os.ReadFile(claudeMDPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	content := string(existing)

	// Already injected — update in place
	if strings.Contains(content, groveContextStart) {
		startIdx := strings.Index(content, groveContextStart)
		endIdx := strings.Index(content, groveContextEnd)
		if endIdx > startIdx {
			content = content[:startIdx] + groveContextBlock() + content[endIdx+len(groveContextEnd):]
			return os.WriteFile(claudeMDPath, []byte(content), 0o644)
		}
	}

	// Append new block
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "\n" + groveContextBlock() + "\n"

	return os.WriteFile(claudeMDPath, []byte(content), 0o644)
}

func groveContextBlock() string {
	return groveContextStart + "\n" + groveContextContent + groveContextEnd
}
