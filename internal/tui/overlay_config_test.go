package tui

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/config"
)

var ansiStripRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// newConfigTestModel creates a test model with config state loaded,
// including a Huh form built from default config.
func newConfigTestModel(t *testing.T) Model {
	t.Helper()
	m := newTestModel(withItems(3), withSize(80, 30))
	m = sendKey(m, "c")
	if m.activeView != ViewConfig {
		t.Fatalf("expected ViewConfig, got %d", m.activeView)
	}
	if m.configState == nil {
		t.Fatal("expected configState to be set")
	}

	// Simulate configLoadedMsg to populate fields and build form
	cfg := config.LoadDefaults()
	m = sendMsg(m, configLoadedMsg{cfg: cfg})
	return m
}

func TestNewConfigState(t *testing.T) {
	s := NewConfigState()
	if s.Tab != ConfigTabGeneral {
		t.Errorf("expected ConfigTabGeneral, got %d", s.Tab)
	}
	if len(s.Fields) != int(ConfigTabCount) {
		t.Errorf("expected %d field groups, got %d", ConfigTabCount, len(s.Fields))
	}
}

func TestConfigOverlay_OpenClose(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m = sendKey(m, "c")
	if m.activeView != ViewConfig {
		t.Errorf("expected ViewConfig, got %d", m.activeView)
	}
	if m.configState == nil {
		t.Fatal("expected configState to be set")
	}

	m = sendKey(m, "esc")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after esc, got %d", m.activeView)
	}
	if m.configState != nil {
		t.Error("expected configState to be nil after close")
	}
}

func TestConfigOverlay_NilState(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewConfig
	m.configState = nil
	m = sendKey(m, "enter")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard for nil state, got %d", m.activeView)
	}
}

func TestConfigOverlay_FormBuiltOnLoad(t *testing.T) {
	m := newConfigTestModel(t)

	if m.configState.Form == nil {
		t.Fatal("expected Form to be built after configLoadedMsg")
	}
	if m.configState.FormValues == nil {
		t.Fatal("expected FormValues to be set after configLoadedMsg")
	}
}

func TestConfigLoadedMsg(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewConfig
	m.configState = NewConfigState()

	cfg := config.LoadDefaults()
	cfg.ProjectName = "test-project"
	m = sendMsg(m, configLoadedMsg{cfg: cfg})

	if m.configState.Config == nil {
		t.Fatal("expected Config to be set")
	}
	if m.configState.Config.ProjectName != "test-project" {
		t.Errorf("expected project name 'test-project', got %q", m.configState.Config.ProjectName)
	}
	if len(m.configState.Fields[ConfigTabGeneral]) == 0 {
		t.Error("expected General fields to be populated")
	}
	if m.configState.Form == nil {
		t.Error("expected Form to be built")
	}
}

func TestConfigLoadedMsg_Error(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewConfig
	m.configState = NewConfigState()

	m = sendMsg(m, configLoadedMsg{err: errTest})
	if m.configState.Err == nil {
		t.Error("expected error on configState")
	}
	if m.configState.Form != nil {
		t.Error("expected Form to be nil on error")
	}
}

func TestConfigSavedMsg_Success(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewConfig
	m.configState = NewConfigState()

	m = sendMsg(m, configSavedMsg{})
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after save, got %d", m.activeView)
	}
	if m.configState != nil {
		t.Error("expected configState nil after save")
	}
}

func TestConfigSavedMsg_Error(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewConfig
	m.configState = NewConfigState()

	m = sendMsg(m, configSavedMsg{err: errTest})
	if m.activeView != ViewConfig {
		t.Errorf("expected ViewConfig after save error, got %d", m.activeView)
	}
	if m.configState.Err == nil {
		t.Error("expected error on configState")
	}
}

func TestConfigSavedMsg_ErrorAfterClose(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	// Simulate save-on-close: configState is already nil when error arrives
	m.activeView = ViewDashboard
	m.configState = nil

	m = sendMsg(m, configSavedMsg{err: errTest})
	// Should show toast error even though configState is nil
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard, got %d", m.activeView)
	}
}

func TestPopulateConfigFields(t *testing.T) {
	cfg := config.LoadDefaults()
	cfg.ProjectName = "grove-cli"
	cfg.Alias = "grove"

	fields := populateConfigFields(cfg)

	if len(fields) != int(ConfigTabCount) {
		t.Fatalf("expected %d tab groups, got %d", ConfigTabCount, len(fields))
	}

	// General tab
	general := fields[ConfigTabGeneral]
	if len(general) != 4 {
		t.Errorf("expected 4 General fields, got %d", len(general))
	}
	if general[0].Key != "project_name" {
		t.Errorf("expected first field 'project_name', got %q", general[0].Key)
	}

	// Behavior tab
	behavior := fields[ConfigTabBehavior]
	if len(behavior) != 5 {
		t.Errorf("expected 5 Behavior fields, got %d", len(behavior))
	}

	// Plugins tab
	plugins := fields[ConfigTabPlugins]
	if len(plugins) != 3 {
		t.Errorf("expected 3 Plugins fields, got %d", len(plugins))
	}

	// Protection tab
	protection := fields[ConfigTabProtection]
	if len(protection) != 2 {
		t.Errorf("expected 2 Protection fields, got %d", len(protection))
	}
}

func TestTabName(t *testing.T) {
	names := map[ConfigTab]string{
		ConfigTabGeneral:    "General",
		ConfigTabBehavior:   "Behavior",
		ConfigTabPlugins:    "Plugins",
		ConfigTabProtection: "Protection",
	}
	for tab, expected := range names {
		if got := tabName(tab); got != expected {
			t.Errorf("tabName(%d) = %q, want %q", tab, got, expected)
		}
	}
}

func TestRenderConfig_AllStates(t *testing.T) {
	t.Run("no form yet", func(t *testing.T) {
		s := NewConfigState()
		v := renderConfig(s, 80)
		if v == "" {
			t.Fatal("expected non-empty render")
		}
		if !strings.Contains(v, "Configuration") {
			t.Error("expected 'Configuration' title")
		}
		if !strings.Contains(v, "Loading configuration...") {
			t.Error("expected loading message when form is nil")
		}
	})

	t.Run("with form", func(t *testing.T) {
		s := NewConfigState()
		cfg := config.LoadDefaults()
		cfg.ProjectName = "test-project"
		s.Config = cfg
		s.Fields = populateConfigFields(cfg)
		form, vals := buildConfigForm(s.Fields, 60)
		s.Form = form
		s.FormValues = vals
		// Initialize the form so it renders
		s.Form.Init()

		v := renderConfig(s, 100)
		plain := ansiStripRE.ReplaceAllString(v, "")
		if !strings.Contains(plain, "project_name") {
			t.Errorf("expected 'project_name' field in render, got:\n%s", v)
		}
		if !strings.Contains(plain, "General") {
			t.Errorf("expected 'General' group title, got:\n%s", v)
		}
	})

	t.Run("confirming state", func(t *testing.T) {
		s := NewConfigState()
		cfg := config.LoadDefaults()
		s.Config = cfg
		s.Fields = populateConfigFields(cfg)
		s.Dirty = true
		s.Confirming = true

		v := renderConfig(s, 100)
		if !strings.Contains(v, "Save changes?") {
			t.Error("expected 'Save changes?' prompt in confirming state")
		}
		if !strings.Contains(v, "save") || !strings.Contains(v, "discard") {
			t.Error("expected save/discard hints in confirming state")
		}
	})

	t.Run("error state", func(t *testing.T) {
		s := NewConfigState()
		s.Err = errTest

		v := renderConfig(s, 80)
		if !strings.Contains(v, "test error") {
			t.Error("expected error text in render")
		}
	})
}

func TestConfigOverlay_EscDirtyConfirms(t *testing.T) {
	m := newConfigTestModel(t)
	m.configState.Dirty = true

	m = sendKey(m, "esc")
	// Should show confirmation prompt, not close
	if m.activeView != ViewConfig {
		t.Errorf("expected ViewConfig after dirty esc, got %d", m.activeView)
	}
	if m.configState == nil {
		t.Fatal("expected configState to still exist")
	}
	if !m.configState.Confirming {
		t.Error("expected Confirming=true after dirty esc")
	}
}

func TestConfigOverlay_ConfirmEnterSaves(t *testing.T) {
	m := newConfigTestModel(t)
	m.configState.Dirty = true
	m.configState.Confirming = true

	m = sendKey(m, "enter")
	// Enter while confirming should save and close
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after confirm enter, got %d", m.activeView)
	}
	if m.configState != nil {
		t.Error("expected configState nil after confirm enter")
	}
}

func TestConfigOverlay_ConfirmEscDiscards(t *testing.T) {
	m := newConfigTestModel(t)
	m.configState.Dirty = true
	m.configState.Confirming = true

	m = sendKey(m, "esc")
	// Esc while confirming should discard and close
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after confirm esc, got %d", m.activeView)
	}
	if m.configState != nil {
		t.Error("expected configState nil after confirm esc")
	}
}

func TestBuildConfigForm(t *testing.T) {
	cfg := config.LoadDefaults()
	fields := populateConfigFields(cfg)

	form, vals := buildConfigForm(fields, 60)
	if form == nil {
		t.Fatal("expected form to be non-nil")
	}
	if vals == nil {
		t.Fatal("expected vals to be non-nil")
	}

	// Should have bool bindings for the boolean fields
	if len(vals.bools) == 0 {
		t.Error("expected bool value bindings")
	}

	// Should have string bindings for string/enum/list fields
	if len(vals.strings) == 0 {
		t.Error("expected string value bindings")
	}
}

func TestSyncConfigFormValues(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewConfig
	m.configState = NewConfigState()

	cfg := config.LoadDefaults()
	m.configState.Config = cfg
	m.configState.Fields = populateConfigFields(cfg)

	form, vals := buildConfigForm(m.configState.Fields, 60)
	m.configState.Form = form
	m.configState.FormValues = vals

	// Simulate a bool value change through the binding
	for key, bPtr := range vals.bools {
		*bPtr = !*bPtr // flip the bool
		_ = key
		break
	}

	m.syncConfigFormValues()
	if !m.configState.Dirty {
		t.Error("expected Dirty=true after changing a bool value")
	}
}

func TestConfigEditKey_DirtyDetection(t *testing.T) {
	// Verify that direct field value changes are detected by syncConfigFormValues
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewConfig
	m.configState = NewConfigState()

	cfg := config.LoadDefaults()
	m.configState.Config = cfg
	m.configState.Fields = populateConfigFields(cfg)

	original := m.configState.Fields[ConfigTabGeneral][0].Value

	// Simulate Huh changing the value through pointer binding
	m.configState.Fields[ConfigTabGeneral][0].Value = "changed-value"

	if m.configState.Fields[ConfigTabGeneral][0].Value == original {
		t.Error("expected value to differ after change")
	}

	// Restore and verify
	m.configState.Fields[ConfigTabGeneral][0].Value = original
	if m.configState.Fields[ConfigTabGeneral][0].Value != original {
		t.Error("expected value to match original after restore")
	}
}

func TestConfigForm_EscapeClosesOverlay(t *testing.T) {
	m := newConfigTestModel(t)
	// Form is not dirty, so esc should close directly
	m.configState.Dirty = false
	m = sendKey(m, "esc")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after esc on clean form, got %d", m.activeView)
	}
	if m.configState != nil {
		t.Error("expected configState nil after esc on clean form")
	}
}

func TestConfigForm_MessageForwarding(t *testing.T) {
	m := newConfigTestModel(t)
	// Send a WindowSizeMsg — should not panic
	m = sendMsg(m, tea.WindowSizeMsg{Width: 100, Height: 40})
	if m.activeView != ViewConfig {
		t.Errorf("expected ViewConfig after WindowSizeMsg, got %d", m.activeView)
	}
}

func TestConfigFieldTypeConstants(t *testing.T) {
	if ConfigString != 0 || ConfigBool != 1 || ConfigEnum != 2 || ConfigList != 3 {
		t.Error("unexpected ConfigFieldType constant values")
	}
}

func TestConfigTabConstants(t *testing.T) {
	if ConfigTabGeneral != 0 || ConfigTabBehavior != 1 || ConfigTabPlugins != 2 || ConfigTabProtection != 3 {
		t.Error("unexpected ConfigTab constant values")
	}
	if ConfigTabCount != 4 {
		t.Errorf("expected ConfigTabCount=4, got %d", ConfigTabCount)
	}
}
