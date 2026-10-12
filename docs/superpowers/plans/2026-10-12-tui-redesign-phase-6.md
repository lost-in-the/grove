# TUI Redesign Phase 6 (Agent Triage and Poll) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Under herdr, the dashboard tells you which agents need you and keeps that current: a header summary, an attention sort, `g a` to jump to the next agent needing you, and a 3-second poll of agent state.

**Architecture:**

- Everything is gated on `agentAware`, decided by the first fetch (phase 4a). Under tmux nothing in this phase appears or runs.
- The poll is a `tea.Tick` loop. Each tick starts one command that calls the multiplexer's `List()` and returns sessions with a generation number. The handler drops stale generations, never runs two polls at once, and updates only `TmuxStatus`, `SessionBackend`, and `AgentStatus` on existing items.
- Session matching reuses the fetch's rule (`fetchContext.setTmuxStatus`), extracted into one function both call.

**Spec:** section 7, plus 5.5 (header) and 5.7 (sort).

## Global Constraints

As in phase 5b; the gate includes the integration suite. Expected golden churn: `TestGolden_Dashboard_Herdr` (header summary). No tmux golden changes.

## Decisions

| # | Decision | Why |
|---|---|---|
| 1 | **The poll starts when the first fetch sets `agentAware`**, and never under tmux | Spec 7.1, 7.4. |
| 2 | **The tick re-arms itself every 3s**; a tick that finds a poll in flight only re-arms | Spec 7.4 "no overlap". One loop, no second timer. |
| 3 | **`g a` flashes `no agents need you` as an info toast** | Toasts are the existing flash; spec 7.3 says footer, but the footer has no flash slot. |
| 4 | **Attention sort cycles only when `agentAware`**: name, age, dirty, attention | Spec 5.7. |
| 5 | **A poll that changes nothing does not re-sort or redraw.** The handler compares the three session fields per item first. | `applySortToList` calls `updateDetailContent`, which resets the pane's scroll; doing that every 3s would make the pane unusable. |
| 6 | **The poll handler owns re-sorting:** when something changed and the sort is attention, it re-sorts and restores the cursor by the path captured before the update | Rows move under attention sort; the cursor follows the worktree (spec 7.4). |
| 7 | **`pollAgentsCmd(cfg, gen)` captures the config by value at command creation** | The command runs on another goroutine; reading `m.cfg` there would race the model. |
| 8 | **Sort mode is not persisted**, so a session that starts under tmux can never be in attention sort | `NewModel` always starts on name; the cycle adds attention only when `agentAware`. |

## Review Focus

1. Under tmux: no tick, no header summary, no attention sort, no `g a`. Test: Task 2 to 5, each with a tmux model.
2. Stale and overlapping polls: an older generation is dropped; a tick during a poll does not start a second. Test: Task 5.
3. Cursor stability under attention sort: when a poll re-sorts, the cursor stays on the same worktree path. Test: Task 5.
4. A failed poll keeps the last known state and shows nothing. Test: Task 5.

---

### Task 1: Workspace and baseline

Workspace `2026-10-12-tui-redesign-phase-6`. Gate green.

### Task 2: Header triage summary

- [ ] Failing tests: `TestHeader_TriageSummary` (`! 2 blocked · ✓ 1 done` after the count, before the sort); at 44 columns `! 2 · ✓ 1`; zero counts show nothing; `buildDashboardHeader` counts only when `agentAware`.
- [ ] Implement `Header.Blocked`, `Header.Done`; the narrowing order drops their words with the count's.
- [ ] Gate. Commit `feat(tui): header summary of agents that need you`.

### Task 3: Attention sort

- [ ] Failing tests: `TestSort_AttentionOrder` (blocked, done, working, rest; ties by alias; main first); `TestSort_CycleIncludesAttentionOnlyWhenAgentAware`.
- [ ] Implement `SortByAttention`; `SortMode.Next(agentAware bool)`; the `o` action passes `m.agentAware`.
- [ ] Gate. Commit `feat(tui): sort by agents needing attention under herdr`.

### Task 4: `g a`

- [ ] Failing tests: `TestGA_JumpsToNextBlockedWrapping`, `TestGA_FallsBackToDone`, `TestGA_NoneFlashes`, `TestGA_ClearsFilterWhenTargetHidden`, `TestGA_AbsentUnderTmux` (not in the g strip; `g a` does nothing).
- [ ] Implement `goto.agent` (layer `g`, key `a`, `Enabled: agentAware`); the g strip lists only enabled entries. The search runs over `m.list.Items()` (the unfiltered list): after `ResetFilter`, `Select` takes an index into that list, not into `VisibleItems()`.
- [ ] Gate. Commit `feat(tui): g a jumps to the next agent needing you`.

### Task 5: The poll

- [ ] Extract `applySession(item *WorktreeItem, ix *mux.Index, projectName, backend string)` from `setTmuxStatus`; the fetch uses it (no behavior change; existing tests).
- [ ] Failing tests (messages driven directly, no timers):
  - `TestPoll_StartsOnlyWhenAgentAware`: the first fetch on herdr returns a tick command; on tmux it does not.
  - `TestPoll_TickStartsOnePoll`: a tick with no poll in flight returns a poll command and sets in-flight; a second tick while in flight returns only the re-arm.
  - `TestPoll_UpdatesAgentStatus`: a poll result marks `fix-bug` blocked; only the three session fields change.
  - `TestPoll_DropsStaleGeneration`.
  - `TestPoll_FailureKeepsState`: an error result leaves statuses as they were and adds no toast.
  - `TestPoll_AttentionSortKeepsCursorOnPath`: under attention sort, a result that re-sorts keeps the cursor on the same worktree.
  - `TestPoll_UnchangedResultLeavesPaneAlone`: a result identical to the current state leaves the detail pane's scroll offset where it was.
  - Session names in these tests are `test-project-<alias>` (`newTestModel` sets `projectName: "test-project"`); otherwise the lookup matches nothing and the tests pass on "no change".
- [ ] Implement `agentPollTickMsg`, `agentPollMsg{gen, sessions, err}`, `Model.pollGen`, `pollInFlight`; `pollAgentsCmd(cfg, gen)` uses `muxFor(m.cfg).List()`.
- [ ] Gate. Commit `feat(tui): poll herdr every 3s for agent state`.

### Task 6: Docs

`docs/TUI.md` (agent triage section: header, sort, `g a`, poll), changelog. Spec 19 item 3: record the measured `List()` latency. Gate. Commit.

### Task 7: Hands-on

Under herdr: the agent column and header summary with a real agent pane if one exists; measure `herdr workspace list` latency (10 runs, recorded in the ledger and spec 19 whether or not agents are present); `g a`; attention sort. Under tmux: none of it appears.

## Done when

- Gate (with integration) and the full suite (known `TestIsCommandRunning` only) pass.
- Task 7 matches.
