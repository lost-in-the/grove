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
	picked      bool // the user moved the highlight since last typing

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
	ti := newTextInput()
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
	// A failed check is not "dirty": enter stays blocked and there is no
	// Changes choice to make.
	s.wipDone, s.HasWIP, s.WIPFiles, s.wipErr = true, has && err == nil, files, err
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

// switchPickApplies reports whether enter in Switch mode takes the
// highlighted branch: after partial text, or after the user moved the
// highlight. An exact branch name, even a rejected one, is never replaced by
// a neighbor, and an untouched empty field picks nothing.
func (s *BranchFormState) switchPickApplies() bool {
	b := s.branchValue()
	if b == "" {
		return s.picked
	}
	return !s.known(b) && s.inUse[b] == nil && b != s.Source.Branch
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
			out = append(out, formSuggestion{Branch: q, Note: "from " + s.baseValue(), New: true})
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

// focusCmd re-focuses the focused input and returns its cursor blink. The
// constructors focus a field before a Model exists to run the command.
func (s *BranchFormState) focusCmd() tea.Cmd {
	if s.Focus == fieldChanges {
		return nil
	}
	return s.input(s.Focus).Focus()
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
	s.picked = true
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

// setWidth sizes the text inputs to the form's line width, so a long value
// scrolls to keep the cursor visible instead of running past the edge.
func (s *BranchFormState) setWidth(inner int) {
	iw := max(4, inner-formLabelWidth-1)
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
		return fieldAlias, "type a name", false
	}
	if msg := ValidateWorktreeName(a); msg != "" {
		return fieldAlias, msg, false
	}
	if dup := checkDuplicateWorktree(a, s.opts.Worktrees); dup != nil {
		return fieldAlias, fmt.Sprintf("name %q is taken by %s", a, dup.ShortName), false
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
	if sg := s.suggestions(); s.Mode == formModeSwitch && s.switchPickApplies() && s.Pick < len(sg) {
		// enter takes the highlighted branch, so say so.
		if _, msg, ok := s.validateWIP(); !ok {
			return msg, formStatusHint
		}
		return "enter switches to " + sg[s.Pick].Branch, formStatusInfo
	}
	if w := s.inUse[s.branchValue()]; s.Mode == formModeNew && w != nil {
		return fmt.Sprintf("in use by %s · enter switches there", w.ShortName), formStatusInfo
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

// keyFooter is the form's key line for its state.
func (s *BranchFormState) keyFooter() Footer {
	if s.Busy {
		return Footer{Hints: []Hint{{"", "wait for this to finish"}}}
	}
	if s.Log != nil {
		return moveFooter(Hint{"esc", "cancel"}, Hint{"", "any key to edit"})
	}
	return s.keyFooterFor(s.Focus)
}

// keyFooterFor is the key line with field f focused. The Changes field has
// no text cursor, so the form is in move mode there.
func (s *BranchFormState) keyFooterFor(f formField) Footer {
	leave := Hint{"esc", "cancel"}
	verb := "create"
	switch {
	case s.Mode == formModeCopy:
		verb = "copy"
	case s.Mode == formModeSwitch, s.inUse[s.branchValue()] != nil:
		verb = "switch"
	}
	var hints []Hint
	if s.hasSuggestions(f) {
		hints = append(hints, Hint{"↑↓/ctrl+jk", "pick"})
	}
	if f == fieldChanges {
		hints = append(hints, Hint{"←→/hl", "change"})
	}
	if len(s.fieldOrder()) > 1 || f == fieldChanges {
		hints = append(hints, Hint{"tab", "next"})
	}
	hints = append(hints, Hint{"enter", verb})
	if f == fieldChanges {
		return moveFooter(leave, hints...)
	}
	return typeFooter(leave, hints...)
}

// keyRows is how many rows the key line takes at width: two when the
// widest line the mode can show needs them. It does not depend on the
// focused field or the changes check, so the status line never moves.
func (s *BranchFormState) keyRows(width int) int {
	fields := map[formMode][]formField{
		formModeNew:    {fieldBranch, fieldBase, fieldAlias},
		formModeCopy:   {fieldBranch, fieldAlias, fieldChanges},
		formModeSwitch: {fieldBranch},
	}[s.Mode]
	for _, f := range fields {
		if len(renderKeyLine(s.keyFooterFor(f), width, 2)) > 1 {
			return 2
		}
	}
	return 1
}

// keyLine is the key line as plain text, on one line.
func (s *BranchFormState) keyLine() string {
	f := s.keyFooter()
	var parts []string
	if f.Mode != modeNone {
		parts = append(parts, string(f.Mode))
	}
	for _, h := range append(f.Hints, f.Leave) {
		if p := strings.TrimSpace(h.Key + " " + h.Description); p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " · ")
}

// handleKey applies one key. Pick keys are explicit strings, never the
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
	case key.Matches(msg, keys.Tab):
		s.acceptPick()
		return formContinue, s.moveFocus(1)
	case key.Matches(msg, keys.ShiftTab):
		return formContinue, s.moveFocus(-1)
	case finderUp(k):
		s.movePick(-1)
		return formContinue, nil
	case finderDown(k):
		s.movePick(1)
		return formContinue, nil
	case key.Matches(msg, keys.Enter):
		if (s.Focus == fieldBranch || s.Focus == fieldBase) && (s.Mode != formModeSwitch || s.switchPickApplies()) {
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
		case "left", "h":
			s.cycleChanges(-1)
		case "right", "l", "space":
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
	s.picked = false
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

// branchFormOptionsFor gathers what every form mode needs from the repo.
func (m Model) branchFormOptionsFor() branchFormOptions {
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
	return branchFormOptions{
		ProjectName:   m.projectName,
		NamePattern:   m.worktreeNamePattern(),
		NameStrategy:  strategy,
		DefaultBase:   pickDefaultBase(configured, detected, mainBranch, func(b string) bool { return known[b] }),
		AllBranches:   all,
		LocalBranches: local,
		Worktrees:     worktrees,
	}
}

// openBranchForm opens the form in New mode, docked into the current view.
func (m Model) openBranchForm(p branchFormPrefill) (tea.Model, tea.Cmd) {
	s := newBranchForm(m.branchFormOptionsFor())
	s.setWidth(m.branchFormLayout().innerWidth())
	cmd := s.applyPrefill(p)
	if cmd == nil {
		cmd = s.focusCmd()
	}
	m.branchForm = s
	m.pushView(ViewBranchForm)
	return m, cmd
}

// openCopyForm opens the form in Copy mode for source, and starts the
// uncommitted-changes check that enter waits for.
func (m Model) openCopyForm(source WorktreeItem) (tea.Model, tea.Cmd) {
	s := newCopyForm(m.branchFormOptionsFor(), source)
	s.setWidth(m.branchFormLayout().innerWidth())
	m.branchForm = s
	m.pushView(ViewBranchForm)
	return m, tea.Batch(s.focusCmd(), wipCheckCmd(source.Path))
}

// openSwitchForm opens the form in Switch mode for target, and starts the
// uncommitted-changes check that enter waits for.
func (m Model) openSwitchForm(target WorktreeItem) (tea.Model, tea.Cmd) {
	s := newSwitchForm(m.branchFormOptionsFor(), target)
	s.setWidth(m.branchFormLayout().innerWidth())
	m.branchForm = s
	m.pushView(ViewBranchForm)
	return m, tea.Batch(s.focusCmd(), wipCheckCmd(target.Path))
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
		s.Log = NewActivityLog(m.branchFormLayout().innerWidth(), 6)
		existing, newBranch := "", ""
		if in.Kind == intentCreateExisting {
			existing = in.Branch
		} else {
			newBranch = in.Branch
		}
		return m, tea.Batch(m.spinner.Tick,
			createWorktreeCmd(m.worktreeMgr, m.stateMgr, m.cfg, m.projectRoot, in.Alias, existing, newBranch, in.FromRef))
	case intentCopy:
		if m.worktreeMgr == nil || m.stateMgr == nil {
			s.Status = "cannot copy a worktree: no repository loaded"
			return m, nil
		}
		s.Busy, s.Status = true, ""
		s.Log = NewActivityLog(m.branchFormLayout().innerWidth(), 6)
		s.Log.AddLine(fmt.Sprintf("Copying %s to %s on branch %s...", in.Target.ShortName, in.Alias, in.Branch))
		return m, tea.Batch(m.spinner.Tick, forkWorktreeCmd(m.worktreeMgr, m.stateMgr, m.cfg, m.projectRoot,
			forkRequest{Source: *in.Target, Name: in.Alias, Branch: in.Branch, HasWIP: s.HasWIP, WIP: in.WIP}))
	case intentSwitchBranch:
		s.Busy, s.Status = true, ""
		s.Log = NewActivityLog(m.branchFormLayout().innerWidth(), 6)
		s.Log.AddLine(fmt.Sprintf("Switching %s to branch %s...", in.Target.ShortName, in.Branch))
		return m, tea.Batch(m.spinner.Tick, checkoutBranchCmd(in.Target.Path, in.Branch, in.Stash))
	}
	return m, nil
}
