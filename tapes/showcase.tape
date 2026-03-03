Output docs/demo-showcase.gif
Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms

Hide
Type "cd /tmp/grove-demo/rails-app"
Enter
Sleep 500ms
Show

# Launch dashboard — hero shot
Type "./bin/grove"
Enter
Sleep 2.5s

# Navigate through worktrees — detail panel updates
Down
Sleep 800ms
Down
Sleep 800ms
Down
Sleep 800ms

# Toggle to compact view
Type "v"
Sleep 1.2s

# Toggle back to rich view
Type "v"
Sleep 1.2s

# Filter by name
Type "/"
Sleep 600ms
Type "feat"
Sleep 1.2s

# Clear filter
Escape
Sleep 1.2s

# Sort cycle — dirty first
Type "o"
Sleep 1.2s

# Sort cycle — back to default
Type "o"
Sleep 1.2s

# Create new worktree — type a new branch name to trigger "Create new branch"
Type "n"
Sleep 1.2s
Type "my-feature"
Sleep 600ms
Enter
Sleep 800ms
Enter
Sleep 800ms
Enter
Sleep 3s

# Quit
Type "q"
