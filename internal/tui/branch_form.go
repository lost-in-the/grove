package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/git"
	"github.com/lost-in-the/grove/internal/tuilog"
	"github.com/lost-in-the/grove/internal/worktree"
)

type formMode int

const (
	formModeNew formMode = iota
	formModeCopy
	formModeSwitch
)

type formField int

const (
	fieldBranch formField = iota
	fieldBase
	fieldAlias
	fieldChanges
)

type formResult int

const (
	formContinue formResult = iota
	formSubmit
	formCancel
)

type formIntentKind int

const (
	intentNone formIntentKind = iota
	intentSwitchTo
	intentCreateExisting
	intentCreateNew
	intentCopy
	intentSwitchBranch
)

type formIntent struct {
	Kind    formIntentKind
	Alias   string
	Branch  string
	FromRef string
	Target  *WorktreeItem
	WIP     WIPStrategy
	Stash   bool
}

type formStatusKind int

const (
	formStatusHint formStatusKind = iota
	formStatusReady
	formStatusInfo
	formStatusError
)

type formSuggestion struct {
	Branch string
	Note   string
	New    bool
}

type branchFormOptions struct {
	ProjectName   string
	NamePattern   string
	NameStrategy  string
	DefaultBase   string
	AllBranches   []string
	LocalBranches []string
	Worktrees     []WorktreeItem
}

type branchFormPrefill struct {
	Branch       string
	Alias        string
	RemoteBranch bool
	FocusAlias   bool
}

// BranchFormState is the single-screen form behind new, copy, and switch.
// Every rule lives here so it can be tested without a Model.
type BranchFormState struct {
	Mode formMode
	opts branchFormOptions

	order      []string
	local      map[string]bool
	remoteOnly map[string]bool
	inUse      map[string]*WorktreeItem

	Branch textinput.Model
	Base   textinput.Model
	Alias  textinput.Model

	aliasEdited bool
	Focus       formField
	Pick        int

	Busy   bool
	Log    *ActivityLog
	Status string

	Source   *WorktreeItem // copy: the worktree copied; switch: the worktree switched
	HasWIP   bool
	WIPFiles []string
	Changes  WIPStrategy

	wipDone bool
	wipErr  error
}

func newBranchForm(opts branchFormOptions) *BranchFormState {
	s := &BranchFormState{
		Mode:       formModeNew,
		opts:       opts,
		local:      map[string]bool{},
		remoteOnly: map[string]bool{},
		inUse:      map[string]*WorktreeItem{},
	}
	for _, b := range opts.LocalBranches {
		s.local[b] = true
	}
	seen := map[string]bool{}
	for _, b := range opts.AllBranches {
		if seen[b] {
			continue
		}
		seen[b] = true
		s.order = append(s.order, b)
		if !s.local[b] {
			s.remoteOnly[b] = true
		}
	}
	for i := range opts.Worktrees {
		if b := opts.Worktrees[i].Branch; b != "" {
			s.inUse[b] = &opts.Worktrees[i]
		}
	}
	s.Branch = newFormInput("type to search or create")
	s.Base = newFormInput("")
	s.Alias = newFormInput("")
	s.Base.SetValue(opts.DefaultBase)
	s.setFocus(fieldBranch)
	return s
}

func newFormInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = placeholder
	ti.CharLimit = 100
	return ti
}

func (s *BranchFormState) applyPrefill(p branchFormPrefill) tea.Cmd {
	if p.Branch != "" {
		if p.RemoteBranch && !s.known(p.Branch) {
			s.remoteOnly[p.Branch] = true
			s.order = append(s.order, p.Branch)
		}
		s.Branch.SetValue(p.Branch)
		s.Branch.CursorEnd()
		s.syncAlias()
	}
	if p.Alias != "" {
		s.Alias.SetValue(p.Alias)
		s.Alias.CursorEnd()
		s.aliasEdited = true
	}
	if p.FocusAlias {
		return s.setFocus(fieldAlias)
	}
	return nil
}

// newCopyForm copies source into a new worktree on a new branch cut from
// source's HEAD. Changes defaults to copy, which leaves the source as it is.
func newCopyForm(opts branchFormOptions, source WorktreeItem) *BranchFormState {
	s := newBranchForm(opts)
	s.Mode = formModeCopy
	s.Source = &source
	s.Changes = WIPCopy
	s.Branch.Placeholder = "new branch name"
	if source.Branch != "" {
		s.Branch.SetValue(source.Branch + "-copy")
		s.Branch.CursorEnd()
	}
	s.syncAlias()
	s.setFocus(fieldBranch)
	return s
}

// newSwitchForm switches target's checkout to another local branch.
func newSwitchForm(opts branchFormOptions, target WorktreeItem) *BranchFormState {
	s := newBranchForm(opts)
	s.Mode = formModeSwitch
	s.Source = &target
	s.Branch.Placeholder = "type to search local branches"
	s.setFocus(fieldBranch)
	return s
}

// setWIP records the uncommitted-changes check for the form's worktree. A
// result for any other path comes from an earlier form and is ignored.
func (s *BranchFormState) setWIP(path string, has bool, files []string, err error) {
	if s.Source == nil || s.Source.Path != path {
		return
	}
	s.wipDone, s.HasWIP, s.WIPFiles, s.wipErr = true, has, files, err
}

func (s *BranchFormState) wipChecked() bool { return s.wipDone }

var changesOrder = []WIPStrategy{WIPCopy, WIPMove, WIPLeave}

func changesLabel(w WIPStrategy) string {
	switch w {
	case WIPMove:
		return "move"
	case WIPLeave:
		return "leave"
	}
	return "copy"
}

func (s *BranchFormState) cycleChanges(delta int) {
	i := 0
	for j, w := range changesOrder {
		if w == s.Changes {
			i = j
		}
	}
	s.Changes = changesOrder[(i+delta+len(changesOrder))%len(changesOrder)]
}

func fileCount(n int) string {
	if n == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", n)
}

func (s *BranchFormState) busyVerb() string {
	switch s.Mode {
	case formModeCopy:
		return "copying…"
	case formModeSwitch:
		return "switching…"
	}
	return "creating…"
}

func (s *BranchFormState) title() string {
	switch s.Mode {
	case formModeCopy:
		return "Copy " + s.Source.ShortName
	case formModeSwitch:
		return "Switch branch of " + s.Source.ShortName
	}
	return "New worktree"
}

func (s *BranchFormState) branchValue() string { return strings.TrimSpace(s.Branch.Value()) }
func (s *BranchFormState) baseValue() string   { return strings.TrimSpace(s.Base.Value()) }
func (s *BranchFormState) aliasValue() string  { return strings.TrimSpace(s.Alias.Value()) }

func (s *BranchFormState) known(b string) bool { return s.local[b] || s.remoteOnly[b] }

// isNewBranch reports whether the Branch field names a branch that does not
// exist yet, which is the only case where Base applies.
func (s *BranchFormState) isNewBranch() bool {
	b := s.branchValue()
	return b != "" && !s.known(b)
}

func (s *BranchFormState) derivedAlias() string {
	if b := s.branchValue(); b != "" {
		return worktree.DeriveWorktreeName(b, s.opts.NameStrategy)
	}
	return ""
}

func (s *BranchFormState) syncAlias() {
	if s.aliasEdited {
		return
	}
	s.Alias.SetValue(s.derivedAlias())
	s.Alias.CursorEnd()
}

func (s *BranchFormState) noteFor(b string) string {
	if w := s.inUse[b]; w != nil {
		return "in use: " + w.ShortName
	}
	if s.remoteOnly[b] {
		return "remote"
	}
	return "existing"
}

func (s *BranchFormState) matchBranches(q string) []formSuggestion {
	ql := strings.ToLower(q)
	var out []formSuggestion
	for _, b := range s.order {
		if ql == "" || strings.Contains(strings.ToLower(b), ql) {
			out = append(out, formSuggestion{Branch: b, Note: s.noteFor(b)})
		}
	}
	return out
}

// switchCandidates lists local branches no worktree has checked out.
func (s *BranchFormState) switchCandidates(q string) []formSuggestion {
	ql := strings.ToLower(q)
	var out []formSuggestion
	for _, b := range s.order {
		if !s.local[b] || s.inUse[b] != nil {
			continue
		}
		if ql == "" || strings.Contains(strings.ToLower(b), ql) {
			out = append(out, formSuggestion{Branch: b, Note: "local"})
		}
	}
	return out
}

// suggestions lists the rows for the focused field.
func (s *BranchFormState) suggestions() []formSuggestion {
	switch s.Mode {
	case formModeCopy:
		return nil
	case formModeSwitch:
		if s.Focus != fieldBranch {
			return nil
		}
		return s.switchCandidates(s.branchValue())
	}
	switch s.Focus {
	case fieldBranch:
		q := s.branchValue()
		out := s.matchBranches(q)
		if q != "" && !s.known(q) {
			out = append(out, formSuggestion{Branch: q, Note: "new from " + s.baseValue(), New: true})
		}
		return out
	case fieldBase:
		return s.matchBranches(s.baseValue())
	}
	return nil
}

func (s *BranchFormState) fieldOrder() []formField {
	switch s.Mode {
	case formModeCopy:
		if s.HasWIP {
			return []formField{fieldBranch, fieldAlias, fieldChanges}
		}
		return []formField{fieldBranch, fieldAlias}
	case formModeSwitch:
		return []formField{fieldBranch}
	}
	if s.isNewBranch() {
		return []formField{fieldBranch, fieldBase, fieldAlias}
	}
	return []formField{fieldBranch, fieldAlias}
}

func (s *BranchFormState) input(f formField) *textinput.Model {
	switch f {
	case fieldBase:
		return &s.Base
	case fieldAlias:
		return &s.Alias
	}
	return &s.Branch
}

func (s *BranchFormState) setFocus(f formField) tea.Cmd {
	s.Branch.Blur()
	s.Base.Blur()
	s.Alias.Blur()
	if f == fieldChanges {
		s.Focus, s.Pick = f, 0
		return nil
	}
	s.Focus = f
	s.Pick = s.defaultPick()
	return s.input(f).Focus()
}

func (s *BranchFormState) moveFocus(delta int) tea.Cmd {
	order := s.fieldOrder()
	idx := 0
	for i, f := range order {
		if f == s.Focus {
			idx = i
		}
	}
	return s.setFocus(order[(idx+delta+len(order))%len(order)])
}

func (s *BranchFormState) movePick(delta int) {
	n := len(s.suggestions())
	if n == 0 {
		s.Pick = 0
		return
	}
	s.Pick = max(0, min(s.Pick+delta, n-1))
}

// defaultPick highlights the row that matches what enter would do: the exact
// match, else the new-branch row, else the first row.
func (s *BranchFormState) defaultPick() int {
	q := s.branchValue()
	if s.Focus == fieldBase {
		q = s.baseValue()
	}
	sg := s.suggestions()
	for i, x := range sg {
		if !x.New && x.Branch == q {
			return i
		}
	}
	for i, x := range sg {
		if x.New {
			return i
		}
	}
	return 0
}

// setWidth sizes the text inputs to the frame, so a long value scrolls to
// keep the cursor visible instead of running past the border.
func (s *BranchFormState) setWidth(termW, termH int) {
	w, _ := branchFormSize(termW, termH)
	iw := max(4, w-4-formLabelWidth-1)
	s.Branch.SetWidth(iw)
	s.Base.SetWidth(iw)
	s.Alias.SetWidth(iw)
}

// pickDefaultBase chooses Base for a new branch: the configured default
// branch, else the repository's detected default when it exists, else the
// main worktree's branch, else "main".
func pickDefaultBase(configured, detected, mainBranch string, known func(string) bool) string {
	switch {
	case configured != "":
		return configured
	case detected != "" && known(detected):
		return detected
	case mainBranch != "":
		return mainBranch
	}
	return "main"
}

// acceptPick copies the highlighted suggestion into the focused field.
func (s *BranchFormState) acceptPick() {
	sg := s.suggestions()
	if s.Pick < 0 || s.Pick >= len(sg) || sg[s.Pick].New {
		return
	}
	in := s.input(s.Focus)
	in.SetValue(sg[s.Pick].Branch)
	in.CursorEnd()
	if s.Focus == fieldBranch {
		s.syncAlias()
	}
}

// validate returns the first invalid field and why. An in-use branch is
// valid: submitting it switches to the worktree already using it.
func (s *BranchFormState) validate() (formField, string, bool) {
	switch s.Mode {
	case formModeCopy:
		return s.validateCopy()
	case formModeSwitch:
		return s.validateSwitch()
	}
	b := s.branchValue()
	if b == "" {
		return fieldBranch, "type a branch name", false
	}
	if s.inUse[b] != nil {
		return fieldBranch, "", true
	}
	if !s.known(b) {
		base := s.baseValue()
		if base == "" || !s.known(base) {
			return fieldBase, fmt.Sprintf("base branch %q not found", base), false
		}
	}
	return s.validateAlias()
}

func (s *BranchFormState) validateAlias() (formField, string, bool) {
	a := s.aliasValue()
	if a == "" {
		return fieldAlias, "type an alias", false
	}
	if msg := ValidateWorktreeName(a); msg != "" {
		return fieldAlias, msg, false
	}
	if dup := checkDuplicateWorktree(a, s.opts.Worktrees); dup != nil {
		return fieldAlias, fmt.Sprintf("alias %q is taken by %s", a, dup.ShortName), false
	}
	return fieldBranch, "", true
}

// validateWIP blocks submit until the uncommitted-changes check has
// returned, and when it failed, so changes are never handled blind.
func (s *BranchFormState) validateWIP() (formField, string, bool) {
	if s.wipErr != nil {
		return s.Focus, "could not check for uncommitted changes: " + s.wipErr.Error(), false
	}
	if !s.wipDone {
		return s.Focus, "checking for uncommitted changes…", false
	}
	return fieldBranch, "", true
}

func (s *BranchFormState) validateCopy() (formField, string, bool) {
	b := s.branchValue()
	if b == "" {
		return fieldBranch, "type a name for the new branch", false
	}
	if s.known(b) {
		return fieldBranch, fmt.Sprintf("branch %q already exists", b), false
	}
	if f, msg, ok := s.validateAlias(); !ok {
		return f, msg, false
	}
	return s.validateWIP()
}

func (s *BranchFormState) validateSwitch() (formField, string, bool) {
	b := s.branchValue()
	switch {
	case b == "":
		return fieldBranch, "type a branch name", false
	case b == s.Source.Branch:
		return fieldBranch, fmt.Sprintf("%s is already on %s", s.Source.ShortName, b), false
	case s.inUse[b] != nil:
		return fieldBranch, fmt.Sprintf("%s is checked out in %s", b, s.inUse[b].ShortName), false
	case !s.local[b]:
		return fieldBranch, fmt.Sprintf("no local branch %q", b), false
	}
	return s.validateWIP()
}

func (s *BranchFormState) intent() formIntent {
	switch s.Mode {
	case formModeCopy:
		in := formIntent{Kind: intentCopy, Alias: s.aliasValue(), Branch: s.branchValue(), Target: s.Source, WIP: WIPLeave}
		if s.HasWIP {
			in.WIP = s.Changes
		}
		return in
	case formModeSwitch:
		return formIntent{Kind: intentSwitchBranch, Branch: s.branchValue(), Target: s.Source, Stash: s.HasWIP}
	}
	b := s.branchValue()
	if w := s.inUse[b]; w != nil {
		return formIntent{Kind: intentSwitchTo, Branch: b, Target: w}
	}
	a := s.aliasValue()
	if s.known(b) {
		return formIntent{Kind: intentCreateExisting, Alias: a, Branch: b}
	}
	ref := s.baseValue()
	if s.remoteOnly[ref] {
		ref = "origin/" + ref
	}
	return formIntent{Kind: intentCreateNew, Alias: a, Branch: b, FromRef: ref}
}

func (s *BranchFormState) status() (string, formStatusKind) {
	if s.Busy {
		return s.busyVerb(), formStatusInfo
	}
	if s.Status != "" {
		return s.Status, formStatusError
	}
	if w := s.inUse[s.branchValue()]; s.Mode == formModeNew && w != nil {
		return fmt.Sprintf("in use by %s · ⏎ switches there", w.ShortName), formStatusInfo
	}
	if _, msg, ok := s.validate(); !ok {
		return msg, formStatusHint
	}
	if s.HasWIP {
		switch s.Mode {
		case formModeCopy:
			verb := map[WIPStrategy]string{WIPCopy: "copied", WIPMove: "moved", WIPLeave: "left behind"}[s.Changes]
			return fmt.Sprintf("✓ ready · %s changed, will be %s", fileCount(len(s.WIPFiles)), verb), formStatusReady
		case formModeSwitch:
			return fmt.Sprintf("✓ ready · %s changed, will be stashed", fileCount(len(s.WIPFiles))), formStatusReady
		}
	}
	return "✓ ready", formStatusReady
}

func (s *BranchFormState) keyLine() string {
	if s.Busy {
		return s.busyVerb()
	}
	if s.Log != nil {
		return "any key to edit · esc back"
	}
	act := "⏎ create"
	switch {
	case s.Mode == formModeCopy:
		act = "⏎ copy"
	case s.Mode == formModeSwitch, s.inUse[s.branchValue()] != nil:
		act = "⏎ switch"
	}
	parts := []string{act}
	if len(s.fieldOrder()) > 1 {
		parts = append(parts, "tab field")
	}
	if s.Focus == fieldChanges {
		parts = append(parts, "←/→ changes")
	} else if s.Mode != formModeCopy {
		parts = append(parts, "ctrl+j/k pick")
	}
	return strings.Join(append(parts, "esc back"), " · ")
}

// handleKey applies one key. Navigation keys are explicit strings, never the
// global Up/Down bindings, because those include j and k, which must type.
func (s *BranchFormState) handleKey(msg tea.KeyPressMsg, keys KeyMap) (formResult, tea.Cmd) {
	if s.Busy {
		return formContinue, nil
	}
	if s.Log != nil {
		if key.Matches(msg, keys.Escape) {
			return formCancel, nil
		}
		s.Log = nil
		return formContinue, nil
	}
	switch k := msg.String(); {
	case key.Matches(msg, keys.Escape):
		return formCancel, nil
	case k == "tab":
		s.acceptPick()
		return formContinue, s.moveFocus(1)
	case k == "shift+tab":
		return formContinue, s.moveFocus(-1)
	case k == "up" || k == "ctrl+k":
		s.movePick(-1)
		return formContinue, nil
	case k == "down" || k == "ctrl+j":
		s.movePick(1)
		return formContinue, nil
	case k == "enter":
		if s.Focus == fieldBranch || s.Focus == fieldBase {
			s.acceptPick()
		}
		if f, _, ok := s.validate(); !ok {
			return formContinue, s.setFocus(f)
		}
		s.Status = ""
		return formSubmit, nil
	}
	if s.Focus == fieldChanges {
		switch msg.String() {
		case "left":
			s.cycleChanges(-1)
		case "right", "space":
			s.cycleChanges(1)
		}
		return formContinue, nil
	}
	return formContinue, s.typeInto(msg)
}

func (s *BranchFormState) typeInto(msg tea.Msg) tea.Cmd {
	if s.Focus == fieldChanges {
		return nil
	}
	in := s.input(s.Focus)
	var cmd tea.Cmd
	*in, cmd = in.Update(msg)
	s.Status = ""
	switch s.Focus {
	case fieldBranch:
		s.syncAlias()
		s.Pick = s.defaultPick()
	case fieldBase:
		s.Pick = s.defaultPick()
	case fieldAlias:
		s.aliasEdited = s.aliasValue() != s.derivedAlias()
	}
	return cmd
}

// paste types pasted text into the focused field.
func (s *BranchFormState) paste(msg tea.PasteMsg) tea.Cmd {
	if s.Busy {
		return nil
	}
	s.Log = nil
	return s.typeInto(msg)
}

func (s *BranchFormState) getActivityLog() *ActivityLog { return s.Log }

func (s *BranchFormState) setCreatingDone(errMsg string) {
	s.Busy = false
	s.Status = errMsg
}

// openBranchForm opens the form in New mode over the current view.
func (m Model) openBranchForm(p branchFormPrefill) (tea.Model, tea.Cmd) {
	all, err := git.ListAllBranches(m.projectRoot)
	if err != nil {
		tuilog.Printf("warning: failed to list branches: %v", err)
	}
	local, err := git.ListLocalBranches(m.projectRoot)
	if err != nil {
		tuilog.Printf("warning: failed to list local branches: %v", err)
	}
	strategy, configured := "", ""
	if m.cfg != nil {
		strategy = m.cfg.TUI.WorktreeNameFromBranch
		configured = m.cfg.DefaultBranch
	}
	detected := ""
	if configured == "" {
		if d, err := git.DetectDefaultBranch(m.projectRoot); err == nil {
			detected = d
		}
	}
	worktrees := m.existingWorktreeItems()
	mainBranch := ""
	known := map[string]bool{}
	for _, w := range worktrees {
		if w.IsMain {
			mainBranch = w.Branch
		}
	}
	for _, b := range append(all, local...) {
		known[b] = true
	}
	s := newBranchForm(branchFormOptions{
		ProjectName:   m.projectName,
		NamePattern:   m.worktreeNamePattern(),
		NameStrategy:  strategy,
		DefaultBase:   pickDefaultBase(configured, detected, mainBranch, func(b string) bool { return known[b] }),
		AllBranches:   all,
		LocalBranches: local,
		Worktrees:     worktrees,
	})
	s.setWidth(m.width, m.height)
	cmd := s.applyPrefill(p)
	m.branchForm = s
	m.pushView(ViewBranchForm)
	return m, cmd
}

func (m Model) handleBranchFormKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.branchForm
	if s == nil {
		m.popView()
		return m, nil
	}
	res, cmd := s.handleKey(msg, m.keys)
	switch res {
	case formCancel:
		m.branchForm = nil
		m.popView()
		return m, nil
	case formSubmit:
		return m.submitBranchForm(s)
	}
	return m, cmd
}

func (m Model) submitBranchForm(s *BranchFormState) (tea.Model, tea.Cmd) {
	in := s.intent()
	switch in.Kind {
	case intentSwitchTo:
		m.switchTo = in.Target.Path
		m.switchToDisplayName = in.Target.displayName()
		m.branchForm = nil
		m.popToRoot()
		return m, tea.Quit
	case intentCreateExisting, intentCreateNew:
		if m.worktreeMgr == nil {
			s.Status = "cannot create a worktree: no repository loaded"
			return m, nil
		}
		s.Busy = true
		s.Status = ""
		s.Log = NewActivityLog(formLogWidth(m.width), 6)
		existing, newBranch := "", ""
		if in.Kind == intentCreateExisting {
			existing = in.Branch
		} else {
			newBranch = in.Branch
		}
		return m, tea.Batch(m.spinner.Tick,
			createWorktreeCmd(m.worktreeMgr, m.stateMgr, m.cfg, m.projectRoot, in.Alias, existing, newBranch, in.FromRef))
	}
	return m, nil
}
