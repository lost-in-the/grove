# Grove CLI Naming Options

## Preferred Names

### bough
*A main branch of a tree*

- Emphasizes the branching workflow
- Short, memorable, easy to type
- No known CLI conflicts

### trellis
*A framework of bars used as support for climbing plants*

- Suggests interconnected structure
- Evokes organized, supported growth
- No known CLI conflicts

---

## Alias Considerations

The current alias `w` conflicts with the Linux `w` command (shows who is logged in).

### Alias Options for "bough"

| Alias | Availability | Notes |
|-------|--------------|-------|
| `b` | Available | Single letter, fast |
| `bo` | Available | Two letters, unambiguous |
| `bg` | **Conflict** | Shell builtin (background job) |
| `bh` | Available | Bough abbreviated |

### Alias Options for "trellis"

| Alias | Availability | Notes |
|-------|--------------|-------|
| `t` | Available | Single letter, fast |
| `tl` | Available | Two letters |
| `tr` | **Conflict** | Unix `tr` (translate characters) |
| `trs` | Available | Three letters |

### Generic Worktree Aliases

| Alias | Availability | Notes |
|-------|--------------|-------|
| `wt` | Available | Direct "worktree" abbreviation |
| `gw` | Available | "git worktree" or "grove worktree" |
| `gt` | Available | "git trees" |
| `ctx` | Available | "context" - captures the switching concept |

---

## Additional Name Variations

### Tree/Branch Family

| Name | Meaning | Alias Ideas |
|------|---------|-------------|
| **bower** | Shelter formed by tree branches | `bw`, `bow` |
| **scion** | Shoot used for grafting; descendant | `sc`, `scn` |
| **sapling** | Young tree | `sp`, `sap` |
| **sprig** | Small stem with leaves | `sg`, `spr` |

### Short/Abbreviated Names

| Name | Rationale | Alias |
|------|-----------|-------|
| **wt** | Literal worktree abbreviation | (self) |
| **ctx** | Context switching is the core use case | (self) |
| **par** | Parallel development | `p` |
| **mux** | Multiplexing workspaces (like tmux) | `mx` |

### Compound Names

| Name | Meaning | Alias Ideas |
|------|---------|-------------|
| **gitree** | Git + tree | `gt` |
| **worq** | Work + queue (stylized) | `wq` |
| **forks** | Emphasizes forking workflow | `fk` |

---

## Command Examples

### With "bough"

```bash
# Alias: b
b ls                    # List worktrees
b to feature            # Switch to feature
b new hotfix            # Create new worktree
b fork experiment       # Fork current worktree
b apply feature --wip   # Apply changes
```

### With "trellis"

```bash
# Alias: t
t ls                    # List worktrees
t to feature            # Switch to feature
t new hotfix            # Create new worktree
t fork experiment       # Fork current worktree
t apply feature --wip   # Apply changes
```

### With "wt" (abbreviated)

```bash
# No alias needed - already short
wt ls
wt to feature
wt new hotfix
wt fork experiment
```

---

## Recommendation

**Primary choice:** `bough` with alias `b`
- Short, elegant, meaningful
- `b` is fast to type and available
- Commands read naturally: `b to main`, `b fork feature`, `b ls`

**Alternative:** `trellis` with alias `t`
- More evocative of structure/organization
- `t` is equally fast and available
- Slightly longer to type for full command

**Pragmatic option:** `wt` (no alias needed)
- Self-explanatory
- Matches git terminology directly
- Already short enough
