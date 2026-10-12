package tui

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/sahilm/fuzzy"

	"github.com/lost-in-the/grove/internal/mux"
	"github.com/lost-in-the/grove/plugins/tracker"
)

// omniScope limits what omni searches: everything (space) or actions (?).
type omniScope int

const (
	omniScopeAll omniScope = iota
	omniScopeActions
)

type omniKind int

const (
	omniWorktree omniKind = iota
	omniAction
	omniPR
	omniIssue
)

// omniItem is one result row.
type omniItem struct {
	Kind    omniKind
	Label   string // what the row is: alias, action label, PR title
	Matched string // the text the query matched, when it is not Label (a synonym, a branch)
	Hits    []int  // rune indexes of matched characters, in Matched if set, else Label
	Detail  string // dim trailing text: the action's key, the worktree's branch
	Ref     string // action ID, worktree path, or PR / issue number
	score   int
}

type omniGroup struct {
	Name  string
	Items []omniItem
}

// omniSource is everything omni can find (spec 9.2). PRs and issues are the
// session cache only; omni never fetches.
type omniSource struct {
	Worktrees  []WorktreeItem
	Actions    []Action
	PRs        []*tracker.PullRequest
	Issues     []*tracker.Issue
	Pins       pinSet
	AgentAware bool
}

const omniGroupMax = 5

// omniActionsFrom keeps the actions a result row can run: not the keys the
// list owns, not the strip openers, not the digit slots, and not no-ops.
func omniActionsFrom(all []Action) []Action {
	skip := map[string]bool{
		"goto.open": true, "pin.open": true, "pin.switch": true, "detail.close": true, "app.back": true,
	}
	labels := map[string]bool{}
	var out []Action
	for _, a := range all {
		if a.Run == nil || skip[a.ID] || labels[a.Label] {
			continue
		}
		labels[a.Label] = true
		out = append(out, a)
	}
	return out
}

// omniResults groups the matches for query (spec 9.1, 9.3).
func omniResults(query string, scope omniScope, src omniSource) []omniGroup {
	query = strings.TrimSpace(query)
	if query == "" {
		if scope == omniScopeActions {
			return nonEmpty(omniGroup{"actions", actionItems(src.Actions)})
		}
		return emptyQueryGroups(src)
	}
	actions := omniGroup{"actions", top(matchActions(query, src.Actions), scope == omniScopeAll)}
	if scope == omniScopeActions {
		return nonEmpty(actions)
	}
	return nonEmpty(
		actions,
		omniGroup{"worktrees", top(matchWorktrees(query, src.Worktrees), true)},
		omniGroup{"pull requests", top(matchPRs(query, src.PRs), true)},
		omniGroup{"issues", top(matchIssues(query, src.Issues), true)},
	)
}

func nonEmpty(groups ...omniGroup) []omniGroup {
	var out []omniGroup
	for _, g := range groups {
		if len(g.Items) > 0 {
			out = append(out, g)
		}
	}
	return out
}

// top sorts by score and keeps at most omniGroupMax when capped.
func top(items []omniItem, capped bool) []omniItem {
	sort.SliceStable(items, func(i, j int) bool { return items[i].score > items[j].score })
	if capped && len(items) > omniGroupMax {
		items = items[:omniGroupMax]
	}
	return items
}

// bestMatch matches query against each candidate and returns the best one,
// its byte-offset hits converted to rune indexes, and whether anything
// matched.
func bestMatch(query string, candidates []string) (string, []int, int, bool) {
	ms := fuzzy.Find(query, candidates)
	if len(ms) == 0 {
		return "", nil, 0, false
	}
	m := ms[0]
	return m.Str, highlightRunes(m.Str, m.MatchedIndexes), m.Score, true
}

// highlightRunes turns fuzzy's byte offsets into rune indexes.
func highlightRunes(s string, byteIdx []int) []int {
	out := make([]int, 0, len(byteIdx))
	for _, b := range byteIdx {
		if b >= 0 && b <= len(s) {
			out = append(out, utf8.RuneCountInString(s[:b]))
		}
	}
	return out
}

// item builds a row; matched is shown only when it differs from label.
func item(kind omniKind, label, matched string, hits []int, detail, ref string, score int) omniItem {
	it := omniItem{Kind: kind, Label: label, Hits: hits, Detail: detail, Ref: ref, score: score}
	if matched != label {
		it.Matched = matched
	}
	return it
}

func actionItems(actions []Action) []omniItem {
	out := make([]omniItem, 0, len(actions))
	for _, a := range actions {
		out = append(out, item(omniAction, a.Label, a.Label, nil, actionKey(a), a.ID, 0))
	}
	return out
}

// actionKey is how a row shows an action's key: "R", "g u", or "".
func actionKey(a Action) string {
	k := a.display()
	if k != "" && a.Layer != "" {
		return a.Layer + " " + k
	}
	return k
}

func matchActions(q string, actions []Action) []omniItem {
	var out []omniItem
	for _, a := range actions {
		candidates := append([]string{a.Label}, a.Synonyms...)
		if m, hits, score, ok := bestMatch(q, candidates); ok {
			out = append(out, item(omniAction, a.Label, m, hits, actionKey(a), a.ID, score))
		}
	}
	return out
}

func matchWorktrees(q string, wts []WorktreeItem) []omniItem {
	var out []omniItem
	for _, w := range wts {
		candidates := []string{w.ShortName}
		if w.Branch != "" && w.Branch != w.ShortName {
			candidates = append(candidates, w.Branch)
		}
		if m, hits, score, ok := bestMatch(q, candidates); ok {
			out = append(out, item(omniWorktree, w.ShortName, m, hits, "", w.Path, score))
		}
	}
	return out
}

func matchPRs(q string, prs []*tracker.PullRequest) []omniItem {
	var out []omniItem
	for _, pr := range prs {
		num := fmt.Sprintf("#%d", pr.Number)
		candidates := []string{pr.Title, num, pr.Branch}
		if m, hits, score, ok := bestMatch(q, candidates); ok {
			out = append(out, item(omniPR, pr.Title, m, hits, num, fmt.Sprint(pr.Number), score))
		}
	}
	return out
}

func matchIssues(q string, issues []*tracker.Issue) []omniItem {
	var out []omniItem
	for _, is := range issues {
		num := fmt.Sprintf("#%d", is.Number)
		if m, hits, score, ok := bestMatch(q, []string{is.Title, num}); ok {
			out = append(out, item(omniIssue, is.Title, m, hits, num, fmt.Sprint(is.Number), score))
		}
	}
	return out
}

// emptyQueryGroups is what omni shows before you type (spec 9.1): agents
// that need you, pinned worktrees, and recently used ones.
func emptyQueryGroups(src omniSource) []omniGroup {
	wt := func(w WorktreeItem) omniItem { return item(omniWorktree, w.ShortName, w.ShortName, nil, "", w.Path, 0) }

	var needs []omniItem
	if src.AgentAware {
		for _, want := range []mux.AgentStatus{mux.AgentBlocked, mux.AgentDone} {
			for _, w := range src.Worktrees {
				if w.AgentStatus == want {
					needs = append(needs, wt(w))
				}
			}
		}
	}

	var pinned []omniItem
	pinnedAlias := map[string]bool{}
	for d := '1'; d <= '9'; d++ {
		alias := src.Pins[string(d)]
		if alias == "" {
			continue
		}
		for _, w := range src.Worktrees {
			if w.ShortName == alias {
				it := wt(w)
				it.Detail = string(d)
				pinned = append(pinned, it)
				pinnedAlias[alias] = true
			}
		}
	}

	recent := slices.Clone(src.Worktrees)
	sort.SliceStable(recent, func(i, j int) bool { return recent[i].LastAccessed.After(recent[j].LastAccessed) })
	var recentItems []omniItem
	for _, w := range recent {
		if w.IsCurrent || pinnedAlias[w.ShortName] || w.LastAccessed.IsZero() {
			continue
		}
		recentItems = append(recentItems, wt(w))
		if len(recentItems) == omniGroupMax {
			break
		}
	}
	return nonEmpty(omniGroup{"needs you", needs}, omniGroup{"pinned", pinned}, omniGroup{"recent", recentItems})
}
